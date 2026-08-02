package order

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func TestCreatePersistsTrustedTenantAndRejectsForeignProductRefs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:order_create_tenant_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Order{}, &OrderItem{}, &OrderShipment{}, &product.Product{}, &product.ProductSKU{}); err != nil {
		t.Fatal(err)
	}
	newContext := func(tid int64) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/orders", nil)
		c.Set(ctxkey.TenantID, tid)
		return c
	}
	svc := &Service{DB: db}
	created, err := svc.Create(newContext(11), CreateBody{OrderNo: "order-" + uuid.NewString(), CustomerName: "tenant one"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var stored Order
	if err := db.First(&stored, "id = ?", created.ID).Error; err != nil || stored.TenantID != 11 {
		t.Fatalf("trusted tenant was not persisted: %+v err=%v", stored, err)
	}
	if _, err := svc.Get(newContext(22), created.ID); err == nil {
		t.Fatal("other tenant could read created order")
	}
	foreignProduct := product.Product{TenantID: 22, Source: "test", Status: product.StatusDraft}
	if err := db.Create(&foreignProduct).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(newContext(11), CreateBody{OrderNo: "order-" + uuid.NewString(), CustomerName: "bad ref", Items: []OrderItemInput{{ProductID: &foreignProduct.ID, ProductTitle: "x", Quantity: 1}}}, nil); err == nil {
		t.Fatal("foreign product reference was accepted")
	}
}
