package order_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func openOrderTenantTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:order_tenant_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&order.Order{}, &order.OrderItem{}, &order.OrderShipment{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func tenantContext(t *testing.T, tenantID int64) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodDelete, "/", nil)
	c.Set(ctxkey.TenantID, tenantID)
	return c
}

func seedTenantOrder(t *testing.T, db *gorm.DB, tenantID int64, shopID *uuid.UUID) *order.Order {
	t.Helper()
	o := &order.Order{TenantID: tenantID, ShopID: shopID, Platform: "manual", OrderNo: "order-" + uuid.NewString(), CustomerName: "test", Status: order.StatusPending, PaymentStatus: order.PaymentUnpaid, FulfillmentStatus: order.FulfillmentUnfulfilled, Currency: "CNY"}
	if err := db.Create(o).Error; err != nil {
		t.Fatal(err)
	}
	return o
}

func TestOrderMutationsAreTenantScopedIncludingNilShop(t *testing.T) {
	db := openOrderTenantTestDB(t)
	svc := &order.Service{DB: db}
	tenantA, tenantB := int64(101), int64(202)
	foreign := seedTenantOrder(t, db, tenantB, nil)

	if err := svc.Delete(tenantContext(t, tenantA), foreign.ID, nil); err == nil {
		t.Fatal("tenant A must not delete tenant B order")
	}
	var got order.Order
	if err := db.First(&got, "id = ?", foreign.ID).Error; err != nil {
		t.Fatalf("cross-tenant delete removed the order: %v", err)
	}

	if _, err := svc.Update(tenantContext(t, tenantA), foreign.ID, order.UpdateBody{CustomerName: "attacker"}, nil); err == nil {
		t.Fatal("tenant A must not update tenant B order")
	}
	if err := db.First(&got, "id = ?", foreign.ID).Error; err != nil || got.CustomerName != "test" {
		t.Fatalf("cross-tenant update changed the order: %+v, %v", got, err)
	}

	local := seedTenantOrder(t, db, tenantA, nil)
	if _, err := svc.Update(tenantContext(t, tenantA), local.ID, order.UpdateBody{CustomerName: "allowed"}, nil); err != nil {
		t.Fatalf("same-tenant update failed: %v", err)
	}
	if err := svc.Delete(tenantContext(t, tenantA), local.ID, nil); err != nil {
		t.Fatalf("same-tenant delete failed: %v", err)
	}
}

func TestReadonlyCannotAppendOrderItem(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openOrderTenantTestDB(t)
	o := seedTenantOrder(t, db, 303, nil)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.TenantID, int64(303))
		c.Set(ctxkey.AdminID, uuid.NewString())
	})
	order.Register(r.Group(""), &order.Handler{Svc: &order.Service{DB: db}})

	req := httptest.NewRequest(http.MethodPost, "/orders/"+o.ID.String()+"/items", strings.NewReader(`{"productTitle":"should not persist","quantity":1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected readonly write to be forbidden, got %d: %s", rec.Code, rec.Body.String())
	}
	var count int64
	if err := db.Model(&order.OrderItem{}).Where("order_id = ?", o.ID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("readonly request had side effect count=%d err=%v", count, err)
	}
}
