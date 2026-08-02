package adminperm

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestTenantAdminRoleIsScopedAndCannotGainGlobalPermissions(t *testing.T) {
	p := &Principal{Role: RoleTenantAdmin, TenantID: 42, Permissions: PermissionsForRole(RoleTenantAdmin)}
	if p.IsAdmin() || !p.IsTenantAdmin() {
		t.Fatalf("tenant administrator global=%v tenant=%v", p.IsAdmin(), p.IsTenantAdmin())
	}
	for _, permission := range []string{PermSettingsManage, PermUserManage, PermAuditRead, PermConfigManage, PermBackupRead, PermRestoreExecute, PermSecurityKeyRotate} {
		if p.Can(permission) {
			t.Fatalf("tenant administrator unexpectedly has %s", permission)
		}
	}
	if !p.Can(PermProductWrite) || !p.Can(PermStoreOperate) || !p.Can(PermInventorySyncRun) || !p.Can(PermCollectProfileManage) {
		t.Fatal("tenant administrator is missing tenant business permissions")
	}
}

func TestAdminRoleOutsideSystemTenantIsNotGlobalAdmin(t *testing.T) {
	p := &Principal{Role: RoleAdmin, TenantID: 42, Permissions: PermissionsForRole(RoleAdmin)}
	if p.IsAdmin() {
		t.Fatal("admin role on a non-system tenant must not grant global administration")
	}
	if !p.IsTenantAdmin() || !p.Can(PermProductWrite) {
		t.Fatal("legacy non-system admin should be constrained to tenant-admin business scope")
	}
	for _, permission := range []string{PermSettingsManage, PermUserManage, PermAuditRead, PermBackupRead, PermRestoreExecute, PermSecurityKeyRotate} {
		if p.Can(permission) {
			t.Fatalf("legacy non-system admin unexpectedly has %s", permission)
		}
	}
}

func TestTenantAdminStoreScopeUsesTenantSubquery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:tenant_admin_scope?mode=memory&cache=shared"), &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(ctxPrincipalKey, &Principal{Role: RoleTenantAdmin, TenantID: 42})
	scoped, err := ApplyStoreScope(c, db, db.Table("orders"), "shop_id")
	if err != nil {
		t.Fatal(err)
	}
	stmt := scoped.Find(&[]map[string]any{}).Statement
	if stmt.SQL.String() == "" || stmt.Vars[0] != int64(42) {
		t.Fatalf("tenant store scope missing: sql=%q vars=%v", stmt.SQL.String(), stmt.Vars)
	}
}

func TestTenantAdminStoreAccessRejectsOtherTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:tenant_admin_access?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE shops (id text primary key, tenant_id integer not null)`).Error; err != nil {
		t.Fatal(err)
	}
	owned, other := "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"
	if err := db.Exec("INSERT INTO shops (id, tenant_id) VALUES (?, ?)", owned, 7).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO shops (id, tenant_id) VALUES (?, ?)", other, 8).Error; err != nil {
		t.Fatal(err)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set(ctxPrincipalKey, &Principal{Role: RoleTenantAdmin, TenantID: 7})
	if !RequireStoreView(c, db, mustUUID(t, owned)) {
		t.Fatal("tenant admin should view own tenant store")
	}
	c, _ = gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set(ctxPrincipalKey, &Principal{Role: RoleTenantAdmin, TenantID: 7})
	if RequireStoreView(c, db, mustUUID(t, other)) {
		t.Fatal("tenant admin must not view another tenant store")
	}
}

func mustUUID(t *testing.T, raw string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
