package adminperm

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestPermissionsForRole(t *testing.T) {
	if !HasPermission(RoleAdmin, PermUserManage) {
		t.Fatal("admin should manage users")
	}
	if len(PermissionsForRole(RoleAdmin)) < 10 {
		t.Fatal("admin perms too short")
	}
	if HasPermission(RoleOperator, PermSettingsManage) {
		t.Fatal("operator must not manage settings")
	}
	if !HasPermission(RoleOperator, PermOrderOperate) {
		t.Fatal("operator should operate orders")
	}
	if HasPermission(RoleReadonly, PermProductWrite) {
		t.Fatal("readonly must not write products")
	}
	if !HasPermission(RoleReadonly, PermOrderView) {
		t.Fatal("readonly should view orders")
	}
	if !StrictHasPermission(RoleReviewer, PermOperationTaskReview) {
		t.Fatal("reviewer should review operation tasks")
	}
	if !StrictHasPermission(RoleReviewer, PermOperationTaskExecute) || !StrictHasPermission(RoleReviewer, PermOperationTaskRetry) {
		t.Fatal("reviewer should execute and retry operation tasks")
	}
	if StrictHasPermission(RoleReviewer, PermOperationTaskEdit) {
		t.Fatal("reviewer must not edit operation tasks")
	}
	if !StrictHasPermission(RoleOperator, PermOperationTaskEdit) {
		t.Fatal("operator should edit operation tasks")
	}
	if StrictHasPermission(RoleOperator, PermOperationTaskReview) || StrictHasPermission(RoleOperator, PermOperationTaskExecute) || StrictHasPermission(RoleOperator, PermOperationTaskRetry) {
		t.Fatal("operator must not review, execute, or retry operation tasks")
	}
	if !StrictHasPermission(RoleOperator, PermInventorySyncRun) || !StrictHasPermission(RoleOperator, PermInventorySyncRerun) || !StrictHasPermission(RoleOperator, PermSKUBindingManage) {
		t.Fatal("operator should run fixture inventory sync and manage SKU bindings")
	}
	if StrictHasPermission(RoleOperator, PermSKUBindingResolveManual) || StrictHasPermission(RoleOperator, PermInventorySyncAuditRead) {
		t.Fatal("operator must not resolve manual bindings or read inventory sync audit")
	}
	if !StrictHasPermission(RoleReviewer, PermSKUBindingResolveManual) || !StrictHasPermission(RoleReviewer, PermInventorySyncAuditRead) {
		t.Fatal("reviewer should resolve manual bindings and read inventory sync audit")
	}
	if StrictHasPermission(RoleReviewer, PermInventorySyncRun) || StrictHasPermission(RoleReviewer, PermInventorySyncRerun) || StrictHasPermission(RoleReadonly, PermInventorySyncRun) {
		t.Fatal("reviewer and readonly must not run inventory sync")
	}
	if !StrictHasPermission(RoleReadonly, PermInventorySyncRead) || !StrictHasPermission(RoleReadonly, PermInventorySnapshotRead) || !StrictHasPermission(RoleReadonly, PermSKUBindingRead) {
		t.Fatal("readonly should read inventory sync, snapshots, and SKU bindings")
	}
	if StrictHasPermission("surprise", PermOperationTaskReview) || StrictHasPermission("surprise", PermUserManage) || StrictHasPermission("surprise", PermInventorySyncRun) || StrictHasPermission(RoleAdmin, "inventory.run") {
		t.Fatal("unknown roles and synonymous permissions must not inherit permissions on strict path")
	}
	if HasPermission("surprise", PermUserManage) || (&Principal{Role: "surprise"}).Can(PermProductView) || (&Principal{Role: "surprise"}).CanOperateStore(uuid.New()) {
		t.Fatal("unknown roles must fail closed and must not inherit admin access")
	}
}

func TestUnknownRoleIsDeniedByHTTPGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(ctxPrincipalKey, &Principal{Role: "surprise"})
	if RequirePermission(c, nil, PermProductWrite) {
		t.Fatal("unknown role unexpectedly authorized")
	}
	if w.Code != 403 {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestMissingAuthContextFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name string
		ctx  *gin.Context
	}{
		{name: "nil context", ctx: nil},
		{name: "missing admin id", ctx: func() *gin.Context { c, _ := gin.CreateTestContext(httptest.NewRecorder()); return c }()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var db *gorm.DB
			if tc.ctx != nil {
				db = &gorm.DB{}
			}
			p, err := LoadPrincipal(tc.ctx, db)
			if err != nil || p == nil || !p.Disabled || p.IsAdmin() || p.Can(PermUserManage) {
				t.Fatalf("principal = %+v, err = %v", p, err)
			}
		})
	}
}

func TestRequireWriteRoleMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name string
		role string
		perm string
		want bool
	}{
		{name: "readonly denied", role: RoleReadonly, perm: PermProductWrite, want: false},
		{name: "unknown denied", role: "vendor", perm: PermProductWrite, want: false},
		{name: "operator reaches product write", role: RoleOperator, perm: PermProductWrite, want: true},
		{name: "admin reaches config write", role: RoleAdmin, perm: PermConfigManage, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set(ctxPrincipalKey, &Principal{Role: tc.role, Permissions: PermissionsForRole(tc.role)})
			if got := RequireWrite(c, nil, tc.perm); got != tc.want {
				t.Fatalf("RequireWrite(%s, %s) = %v, want %v", tc.role, tc.perm, got, tc.want)
			}
			if !tc.want && w.Code != 403 {
				t.Fatalf("denied status = %d, want 403", w.Code)
			}
		})
	}
}

func TestRequireGlobalAdminChecksSystemTenantAndRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name      string
		principal *Principal
		want      bool
	}{
		{name: "system admin", principal: &Principal{Role: RoleAdmin, TenantID: 0, Permissions: PermissionsForRole(RoleAdmin)}, want: true},
		{name: "non-system admin label", principal: &Principal{Role: RoleAdmin, TenantID: 9, Permissions: PermissionsForRole(RoleAdmin)}},
		{name: "system operator", principal: &Principal{Role: RoleOperator, TenantID: 0, Permissions: PermissionsForRole(RoleOperator)}},
		{name: "tenant admin", principal: &Principal{Role: RoleTenantAdmin, TenantID: 9, Permissions: PermissionsForRole(RoleTenantAdmin)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set(ctxPrincipalKey, tc.principal)
			if got := RequireGlobalAdmin(c, nil); got != tc.want {
				t.Fatalf("RequireGlobalAdmin() = %v, want %v", got, tc.want)
			}
			if !tc.want && w.Code != 403 {
				t.Fatalf("denied status = %d, want 403", w.Code)
			}
		})
	}
}

func TestPrincipalStoreAccess(t *testing.T) {
	sid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	other := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	p := &Principal{
		Role: RoleOperator,
		StoreGrants: []StoreGrant{
			{StoreID: sid, PermissionScope: "operate"},
		},
	}
	if !p.CanViewStore(sid) {
		t.Fatal("should view granted store")
	}
	if !p.CanOperateStore(sid) {
		t.Fatal("should operate granted store")
	}
	if p.CanViewStore(other) {
		t.Fatal("must not view other store")
	}
	if p.CanOperateStore(other) {
		t.Fatal("must not operate other store")
	}
}
