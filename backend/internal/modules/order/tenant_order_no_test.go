package order

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestOrderNoIsUniqueWithinTenant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:order_no_scope_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Order{}); err != nil {
		t.Fatal(err)
	}
	first := Order{TenantID: 1, Platform: "test", OrderNo: "ORD-1", CustomerName: "one", Status: StatusPending, PaymentStatus: PaymentUnpaid, FulfillmentStatus: FulfillmentUnfulfilled, Currency: "CNY"}
	if err := db.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	second := first
	second.ID = uuid.New()
	second.TenantID = 2
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("different tenants should be able to reuse an order number: %v", err)
	}
	duplicate := first
	duplicate.ID = uuid.New()
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("same tenant duplicate order number must be rejected")
	}
}
