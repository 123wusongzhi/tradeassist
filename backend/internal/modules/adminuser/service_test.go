package adminuser

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAdminUserTestService(t *testing.T) (*Service, *gin.Context, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:adminuser_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&auth.Tenant{}, &admin.AdminUser{}, &admin.UserStorePermission{}, &shop.Shop{}); err != nil {
		t.Fatalf("migrate admin user test schema: %v", err)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/v1/admin/users", nil)
	svc := &Service{DB: db, Cfg: &config.Config{Auth: config.AuthConfig{PasswordMinLength: 12}}}
	return svc, c, db
}

func int64Ptr(value int64) *int64 { return &value }

func stringPtr(value string) *string { return &value }

func TestCreateEnforcesSharedPasswordPolicy(t *testing.T) {
	svc, c, _ := newAdminUserTestService(t)
	svc.Cfg = &config.Config{
		AppEnv:                 "production",
		BootstrapAdminPassword: "ProductionBootstrap42!",
		Auth:                   config.AuthConfig{PasswordMinLength: 12},
	}

	invalid := []struct {
		email    string
		password string
	}{
		{email: "short@example.com", password: "SafePass"},
		{email: "common@example.com", password: "Qwerty123"},
		{email: "bootstrap@example.com", password: "ProductionBootstrap42!"},
	}
	for _, tc := range invalid {
		if _, err := svc.Create(c, CreateBody{Email: tc.email, Password: tc.password}, nil); err == nil {
			t.Fatalf("Create accepted invalid password %q", tc.password)
		}
	}

	row, err := svc.Create(c, CreateBody{Email: "valid@example.com", Password: "SafePassphrase42!", Role: "admin"}, nil)
	if err != nil {
		t.Fatalf("Create rejected valid password: %v", err)
	}
	if row.TenantID != 0 {
		t.Fatalf("legacy create without tenant changed system tenant: %d", row.TenantID)
	}
}

func TestTenantAdminCreateRequiresKnownPositiveTenantAndLoadsPermissions(t *testing.T) {
	svc, c, db := newAdminUserTestService(t)
	tenant := auth.Tenant{Name: "测试租户"}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatal(err)
	}

	base := CreateBody{Password: "SafePassphrase42!", Role: adminperm.RoleTenantAdmin}
	missing := base
	missing.Email = "missing-tenant@example.test"
	if _, err := svc.Create(c, missing, nil); !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("missing tenant error=%v, want %v", err, ErrTenantRequired)
	}
	zero := base
	zero.Email = "zero-tenant@example.test"
	zero.TenantID = int64Ptr(0)
	if _, err := svc.Create(c, zero, nil); !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("zero tenant error=%v, want %v", err, ErrTenantRequired)
	}
	unknown := base
	unknown.Email = "unknown-tenant@example.test"
	unknown.TenantID = int64Ptr(999)
	if _, err := svc.Create(c, unknown, nil); !errors.Is(err, ErrTenantNotFound) {
		t.Fatalf("unknown tenant error=%v, want %v", err, ErrTenantNotFound)
	}

	valid := base
	valid.Email = "tenant-admin@example.test"
	valid.TenantID = int64Ptr(tenant.ID)
	row, err := svc.Create(c, valid, nil)
	if err != nil {
		t.Fatalf("create tenant admin: %v", err)
	}
	if row.TenantID != tenant.ID || row.Role != adminperm.RoleTenantAdmin {
		t.Fatalf("created row=%+v", row)
	}

	permissionContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	permissionContext.Request = httptest.NewRequest("GET", "/api/v1/auth/profile", nil)
	permissionContext.Set(ctxkey.AdminID, row.ID)
	principal, err := adminperm.LoadPrincipal(permissionContext, db)
	if err != nil {
		t.Fatalf("load principal: %v", err)
	}
	if !principal.IsTenantAdmin() ||
		!principal.Can(adminperm.PermProductView) ||
		!principal.Can(adminperm.PermProductWrite) ||
		!principal.Can(adminperm.PermPublishCreateDraft) ||
		!principal.Can(adminperm.PermCollectProfileManage) {
		t.Fatalf("tenant admin permissions not active: %+v", principal)
	}
}

func TestUpdateTenantAdminAssignmentPersistsAndInvalidatesSessionVersion(t *testing.T) {
	svc, c, db := newAdminUserTestService(t)
	tenant := auth.Tenant{Name: "更新目标租户"}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatal(err)
	}
	user := admin.AdminUser{
		Base:         model.Base{ID: uuid.New()},
		TenantID:     0,
		Username:     admin.NewInternalUsername(),
		Email:        "operator@example.test",
		PasswordHash: "test-hash",
		Role:         adminperm.RoleOperator,
		Status:       "active",
		TokenVersion: 4,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	row, err := svc.Update(c, user.ID, UpdateBody{
		Role:     stringPtr(adminperm.RoleTenantAdmin),
		TenantID: int64Ptr(tenant.ID),
	}, nil)
	if err != nil {
		t.Fatalf("update tenant assignment: %v", err)
	}
	if row.TenantID != tenant.ID || row.Role != adminperm.RoleTenantAdmin {
		t.Fatalf("updated row=%+v", row)
	}
	var saved admin.AdminUser
	if err := db.First(&saved, "id = ?", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.TokenVersion != 5 {
		t.Fatalf("token version=%d, want 5", saved.TokenVersion)
	}

	if _, err := svc.Update(c, user.ID, UpdateBody{TenantID: int64Ptr(0)}, nil); !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("invalid tenant reset error=%v, want %v", err, ErrTenantRequired)
	}
	if err := db.First(&saved, "id = ?", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.TenantID != tenant.ID || saved.TokenVersion != 5 {
		t.Fatalf("failed update mutated user: tenant=%d tokenVersion=%d", saved.TenantID, saved.TokenVersion)
	}
}

func TestUpdatePreservesSelfDowngradeGuardAndAllowsLegacyTenantRepair(t *testing.T) {
	svc, c, db := newAdminUserTestService(t)
	user := admin.AdminUser{
		Base:         model.Base{ID: uuid.New()},
		TenantID:     7,
		Username:     admin.NewInternalUsername(),
		Email:        "legacy-global-admin@example.test",
		PasswordHash: "test-hash",
		Role:         adminperm.RoleAdmin,
		Status:       "active",
		TokenVersion: 2,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Update(c, user.ID, UpdateBody{
		Role:     stringPtr(adminperm.RoleTenantAdmin),
		TenantID: int64Ptr(user.TenantID),
	}, &user.ID); !errors.Is(err, ErrSelfRoleDowngrade) {
		t.Fatalf("self downgrade error=%v, want %v", err, ErrSelfRoleDowngrade)
	}

	row, err := svc.Update(c, user.ID, UpdateBody{TenantID: int64Ptr(0)}, &user.ID)
	if err != nil {
		t.Fatalf("repair legacy global admin tenant: %v", err)
	}
	if row.Role != adminperm.RoleAdmin || row.TenantID != 0 {
		t.Fatalf("repaired row=%+v", row)
	}
	var saved admin.AdminUser
	if err := db.First(&saved, "id = ?", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.TokenVersion != 3 {
		t.Fatalf("token version=%d, want 3", saved.TokenVersion)
	}
}

func TestListTenantsIncludesRegistryAndLegacyTenantSources(t *testing.T) {
	svc, c, db := newAdminUserTestService(t)
	registered := auth.Tenant{Name: "正式租户"}
	if err := db.Create(&registered).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&shop.Shop{
		TenantID:   2,
		Platform:   "ozon",
		ShopName:   "legacy-shop",
		Status:     "active",
		AuthStatus: "authorized",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uuid.New()},
		TenantID:     3,
		Username:     admin.NewInternalUsername(),
		Email:        "legacy-user@example.test",
		PasswordHash: "test-hash",
		Role:         adminperm.RoleTenantAdmin,
		Status:       "active",
	}).Error; err != nil {
		t.Fatal(err)
	}

	items, err := svc.ListTenants(c)
	if err != nil {
		t.Fatalf("list tenants: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("tenant options=%+v", items)
	}
	if items[0].ID != registered.ID || items[0].Name != "正式租户" {
		t.Fatalf("registered tenant=%+v", items[0])
	}
	if items[1].ID != 2 || len(items[1].ShopNames) != 1 || items[1].ShopNames[0] != "legacy-shop" {
		t.Fatalf("legacy shop tenant=%+v", items[1])
	}
	if items[2].ID != 3 {
		t.Fatalf("legacy user tenant=%+v", items[2])
	}
}
