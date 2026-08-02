package order

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func TestOrderWritesRequireOperateGrantForExactTenantShop(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:order_store_scope_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Order{}, &OrderItem{}, &OrderShipment{}, &shop.Shop{}); err != nil {
		t.Fatal(err)
	}
	tenantID := int64(41)
	shopA := shop.Shop{TenantID: tenantID, Platform: "test", ShopName: "A", Status: "active", AuthStatus: "ok"}
	shopB := shop.Shop{TenantID: tenantID, Platform: "test", ShopName: "B", Status: "active", AuthStatus: "ok"}
	if err := db.Create(&shopA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&shopB).Error; err != nil {
		t.Fatal(err)
	}
	ctx := func() *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/orders", nil)
		c.Set(ctxkey.TenantID, tenantID)
		c.Set("adminperm.principal", &adminperm.Principal{TenantID: tenantID, Role: adminperm.RoleOperator, StoreGrants: []adminperm.StoreGrant{{StoreID: shopA.ID, PermissionScope: "operate"}, {StoreID: shopB.ID, PermissionScope: "view"}}})
		return c
	}
	svc := &Service{DB: db}
	if _, err := svc.Create(ctx(), CreateBody{OrderNo: "blocked-" + uuid.NewString(), CustomerName: "x", ShopID: &shopB.ID}, nil); err == nil {
		t.Fatal("view-only shop B accepted order create")
	}
	var count int64
	if err := db.Model(&Order{}).Where("shop_id = ?", shopB.ID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("denied create persisted rows=%d err=%v", count, err)
	}
	allowed, err := svc.Create(ctx(), CreateBody{OrderNo: "allowed-" + uuid.NewString(), CustomerName: "x", ShopID: &shopA.ID}, nil)
	if err != nil {
		t.Fatalf("operate shop A create failed: %v", err)
	}
	blocked := &Order{TenantID: tenantID, ShopID: &shopB.ID, Platform: "manual", OrderNo: "existing-" + uuid.NewString(), CustomerName: "before", Status: StatusPending, PaymentStatus: PaymentUnpaid, FulfillmentStatus: FulfillmentUnfulfilled, Currency: "CNY"}
	if err := db.Create(blocked).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Update(ctx(), blocked.ID, UpdateBody{CustomerName: "after"}, nil); err == nil {
		t.Fatal("view-only shop B accepted update")
	}
	var got Order
	if err := db.First(&got, "id = ?", blocked.ID).Error; err != nil || got.CustomerName != "before" {
		t.Fatalf("denied update mutated order: %+v err=%v", got, err)
	}
	if _, err := svc.Update(ctx(), allowed.ID, UpdateBody{CustomerName: "after"}, nil); err != nil {
		t.Fatalf("operate shop A update failed: %v", err)
	}
}

func TestCreateShopReferenceKeepsTenantZeroExact(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:order_store_tenant_zero_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Order{}, &OrderItem{}, &OrderShipment{}, &shop.Shop{}); err != nil {
		t.Fatal(err)
	}
	foreign := shop.Shop{TenantID: 9, Platform: "test", ShopName: "foreign", Status: "active", AuthStatus: "ok"}
	if err := db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/orders", nil)
	c.Set(ctxkey.TenantID, int64(0))
	c.Set("adminperm.principal", &adminperm.Principal{TenantID: 0, Role: adminperm.RoleAdmin})
	if _, err := (&Service{DB: db}).Create(c, CreateBody{OrderNo: "zero-" + uuid.NewString(), CustomerName: "x", ShopID: &foreign.ID}, nil); err == nil {
		t.Fatal("tenant zero accepted a nonzero-tenant shop")
	}
}
