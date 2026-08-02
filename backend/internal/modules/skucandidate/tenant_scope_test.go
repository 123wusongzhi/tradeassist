package skucandidate

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
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func TestGetByItemUsesFreshHardDeleteSKUSchemaAndTenantScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:sku_candidate_scope_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&order.Order{}, &order.OrderItem{}, &product.Product{}, &product.ProductSKU{}); err != nil {
		t.Fatal(err)
	}

	localProduct := product.Product{TenantID: 0, Source: "test", Status: product.StatusDraft, Title: "local"}
	foreignProduct := product.Product{TenantID: 9, Source: "test", Status: product.StatusDraft, Title: "foreign"}
	if err := db.Create(&localProduct).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&foreignProduct).Error; err != nil {
		t.Fatal(err)
	}
	localSKU := product.ProductSKU{ProductID: localProduct.ID, SKUCode: "scope-code"}
	foreignSKU := product.ProductSKU{ProductID: foreignProduct.ID, SKUCode: "scope-code"}
	if err := db.Create(&localSKU).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&foreignSKU).Error; err != nil {
		t.Fatal(err)
	}
	localOrder := order.Order{TenantID: 0, OrderNo: "scope-local", Platform: "manual", CustomerName: "test", Status: order.StatusPending, PaymentStatus: order.PaymentUnpaid, FulfillmentStatus: order.FulfillmentUnfulfilled, Currency: "USD"}
	foreignOrder := order.Order{TenantID: 9, OrderNo: "scope-foreign", Platform: "manual", CustomerName: "test", Status: order.StatusPending, PaymentStatus: order.PaymentUnpaid, FulfillmentStatus: order.FulfillmentUnfulfilled, Currency: "USD"}
	if err := db.Create(&localOrder).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&foreignOrder).Error; err != nil {
		t.Fatal(err)
	}
	localItem := order.OrderItem{OrderID: localOrder.ID, ProductTitle: "local line", SKUCode: "scope-code", Quantity: 1}
	foreignItem := order.OrderItem{OrderID: foreignOrder.ID, ProductTitle: "foreign line", SKUCode: "scope-code", Quantity: 1}
	if err := db.Create(&localItem).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&foreignItem).Error; err != nil {
		t.Fatal(err)
	}

	h := &Handler{Svc: &Service{DB: db}}
	request := func(tenantID int64, itemID uuid.UUID) *httptest.ResponseRecorder {
		r := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(r)
		c.Request = httptest.NewRequest(http.MethodGet, "/order-items/"+itemID.String()+"/sku-candidates", nil)
		c.Params = gin.Params{{Key: "itemId", Value: itemID.String()}}
		c.Set(ctxkey.TenantID, tenantID)
		h.GetByItem(c)
		return r
	}

	if got := request(0, localItem.ID); got.Code != http.StatusOK {
		t.Fatalf("tenant 0 own item returned %d: %s", got.Code, got.Body.String())
	} else if !strings.Contains(got.Body.String(), localSKU.ID.String()) || strings.Contains(got.Body.String(), foreignSKU.ID.String()) {
		t.Fatalf("tenant 0 candidate response leaked or omitted SKU: %s", got.Body.String())
	}
	if got := request(0, foreignItem.ID); got.Code != http.StatusNotFound {
		t.Fatalf("tenant 0 accessed foreign item: %d: %s", got.Code, got.Body.String())
	}
}
