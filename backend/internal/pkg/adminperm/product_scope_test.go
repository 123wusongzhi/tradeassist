package adminperm

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func TestAllowedStoreIDsAdmin(t *testing.T) {
	p := &Principal{Role: RoleAdmin}
	if ids := p.AllowedStoreIDs(); ids != nil {
		t.Fatalf("admin should return nil allowed ids, got %v", ids)
	}
}

func TestEnsureProductOperateRequiresEveryLinkedStoreAndExactTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:product_operate_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE products (id text primary key, tenant_id integer, deleted_at datetime)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE product_platform_publish_configs (product_id text, shop_id text)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE product_publications (product_id text, shop_id text, deleted_at datetime)").Error; err != nil {
		t.Fatal(err)
	}
	productID := uuid.New()
	if err := db.Exec("INSERT INTO products (id, tenant_id) VALUES (?, ?)", productID.String(), 1).Error; err != nil {
		t.Fatal(err)
	}
	a, b := uuid.New(), uuid.New()
	if err := db.Exec("INSERT INTO product_platform_publish_configs (product_id, shop_id) VALUES (?, ?)", productID.String(), a.String()).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO product_publications (product_id, shop_id) VALUES (?, ?)", productID.String(), b.String()).Error; err != nil {
		t.Fatal(err)
	}
	ctx := gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New())
	ctx.Request = httptest.NewRequest("POST", "/", nil)
	ctx.Set(ctxkey.TenantID, int64(1))
	ctx.Set("adminperm.principal", &Principal{TenantID: 1, Role: RoleOperator, StoreGrants: []StoreGrant{{StoreID: a, PermissionScope: "operate"}, {StoreID: b, PermissionScope: "view"}}})
	if err := EnsureProductOperate(ctx, db, productID); err == nil {
		t.Fatal("shared product must require operate for every linked store")
	}
	ctx.Set("adminperm.principal", &Principal{TenantID: 1, Role: RoleOperator, StoreGrants: []StoreGrant{{StoreID: a, PermissionScope: "operate"}, {StoreID: b, PermissionScope: "operate"}}})
	if err := EnsureProductOperate(ctx, db, productID); err != nil {
		t.Fatalf("authorized shared product: %v", err)
	}
	ctx.Set(ctxkey.TenantID, int64(0))
	ctx.Set("adminperm.principal", &Principal{TenantID: 0, Role: RoleAdmin})
	if err := EnsureProductOperate(ctx, db, productID); err == nil {
		t.Fatal("tenant-0 admin must not operate tenant-1 product")
	}
	missing := uuid.New()
	if err := EnsureProductOperate(ctx, db, missing); err == nil {
		t.Fatal("missing tenant-0 product must fail closed")
	}
}

func TestCanViewStoreScoped(t *testing.T) {
	sid := uuid.New()
	p := &Principal{
		Role:        RoleOperator,
		StoreGrants: []StoreGrant{{StoreID: sid, PermissionScope: "view"}},
	}
	if !p.CanViewStore(sid) {
		t.Fatal("expected view access")
	}
	if p.CanViewStore(uuid.New()) {
		t.Fatal("expected deny for other store")
	}
}
