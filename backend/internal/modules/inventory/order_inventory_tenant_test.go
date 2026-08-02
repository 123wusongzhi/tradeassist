package inventory

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

func TestDeductInventoryForOrderRejectsForeignTenantSKUWithoutMutation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:inventory_order_tenant_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&orderMirror{}, &orderLineMirror{}, &product.Product{}, &product.ProductSKU{}, &InventoryChangeLog{}, &OrderInventoryEffect{}); err != nil {
		t.Fatal(err)
	}
	foreignProduct := product.Product{TenantID: 2, Source: "test", Status: product.StatusDraft}
	if err := db.Create(&foreignProduct).Error; err != nil {
		t.Fatal(err)
	}
	stock := 9
	foreignSKU := product.ProductSKU{ProductID: foreignProduct.ID, Stock: &stock}
	if err := db.Create(&foreignSKU).Error; err != nil {
		t.Fatal(err)
	}
	o := orderMirror{TenantID: 1, OrderNo: "order-" + uuid.NewString(), Status: "paid", PaymentStatus: "paid"}
	if err := db.Create(&o).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&orderLineMirror{OrderID: o.ID, ProductID: &foreignProduct.ID, ProductSKUID: &foreignSKU.ID, Quantity: 2}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := (&Service{DB: db}).DeductInventoryForOrder(context.Background(), o.ID, OrderInventoryOptions{}); err == nil {
		t.Fatal("foreign-tenant SKU deduction was accepted")
	}
	var got product.ProductSKU
	if err := db.First(&got, "id = ?", foreignSKU.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Stock == nil || *got.Stock != stock {
		t.Fatalf("foreign SKU stock mutated: %+v", got.Stock)
	}
	var logCount int64
	if err := db.Model(&InventoryChangeLog{}).Count(&logCount).Error; err != nil || logCount != 0 {
		t.Fatalf("foreign deduction created logs count=%d err=%v", logCount, err)
	}
}

func TestValidateOrderSKUProductsTenantUsesExactSystemTenantScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:inventory_order_system_tenant_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&product.Product{}, &product.ProductSKU{}); err != nil {
		t.Fatal(err)
	}

	systemProduct := product.Product{TenantID: 0, Source: "test", Status: product.StatusDraft}
	otherProduct := product.Product{TenantID: 1, Source: "test", Status: product.StatusDraft}
	if err := db.Create(&systemProduct).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&otherProduct).Error; err != nil {
		t.Fatal(err)
	}
	systemSKU := product.ProductSKU{ProductID: systemProduct.ID}
	otherSKU := product.ProductSKU{ProductID: otherProduct.ID}
	if err := db.Create(&systemSKU).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&otherSKU).Error; err != nil {
		t.Fatal(err)
	}

	svc := &Service{DB: db}
	if err := svc.validateOrderSKUProductsTenant(context.Background(), 0, []orderLineMirror{{ProductSKUID: &systemSKU.ID}}); err != nil {
		t.Fatalf("system-tenant SKU was rejected: %v", err)
	}
	if err := svc.validateOrderSKUProductsTenant(context.Background(), 0, []orderLineMirror{{ProductSKUID: &otherSKU.ID}}); err == nil {
		t.Fatal("non-system SKU was accepted for system tenant")
	}
}

func TestOrderInventoryDeductAndRestoreSupportSystemTenantSKU(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:inventory_order_system_flow_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&orderMirror{}, &orderLineMirror{}, &product.Product{}, &product.ProductSKU{}, &InventoryChangeLog{}, &OrderInventoryEffect{}); err != nil {
		t.Fatal(err)
	}
	stock := 5
	prod := product.Product{TenantID: 0, Source: "test", Status: product.StatusDraft}
	if err := db.Create(&prod).Error; err != nil {
		t.Fatal(err)
	}
	sku := product.ProductSKU{ProductID: prod.ID, Stock: &stock}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatal(err)
	}
	o := orderMirror{TenantID: 0, OrderNo: "order-" + uuid.NewString(), Status: "paid", PaymentStatus: "paid"}
	if err := db.Create(&o).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&orderLineMirror{OrderID: o.ID, ProductID: &prod.ID, ProductSKUID: &sku.ID, Quantity: 2}).Error; err != nil {
		t.Fatal(err)
	}

	svc := &Service{DB: db}
	if _, err := svc.DeductInventoryForOrder(context.Background(), o.ID, OrderInventoryOptions{}); err != nil {
		t.Fatalf("deduct system-tenant SKU: %v", err)
	}
	if _, err := svc.RestoreInventoryForOrder(context.Background(), o.ID, OrderInventoryOptions{}); err != nil {
		t.Fatalf("restore system-tenant SKU: %v", err)
	}
	var got product.ProductSKU
	if err := db.First(&got, "id = ?", sku.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Stock == nil || *got.Stock != stock {
		t.Fatalf("unexpected stock after restore: %+v", got.Stock)
	}
}

func TestOrderInventoryRejectsMismatchedTrustedTenantContext(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:inventory_order_context_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&orderMirror{}, &orderLineMirror{}, &product.Product{}, &product.ProductSKU{}, &InventoryChangeLog{}, &OrderInventoryEffect{}); err != nil {
		t.Fatal(err)
	}
	localProduct := product.Product{TenantID: 1, Source: "test", Status: product.StatusDraft}
	if err := db.Create(&localProduct).Error; err != nil {
		t.Fatal(err)
	}
	stock := 9
	localSKU := product.ProductSKU{ProductID: localProduct.ID, Stock: &stock}
	if err := db.Create(&localSKU).Error; err != nil {
		t.Fatal(err)
	}
	o := orderMirror{TenantID: 1, OrderNo: "order-" + uuid.NewString(), Status: "paid", PaymentStatus: "paid"}
	if err := db.Create(&o).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&orderLineMirror{OrderID: o.ID, ProductID: &localProduct.ID, ProductSKUID: &localSKU.ID, Quantity: 2}).Error; err != nil {
		t.Fatal(err)
	}

	foreignCtx := security.WithTenantContext(context.Background(), &security.TenantContext{TenantID: 2})
	svc := &Service{DB: db}
	if _, err := svc.DeductInventoryForOrder(foreignCtx, o.ID, OrderInventoryOptions{}); !errors.Is(err, security.ErrTenantAccessDenied) {
		t.Fatalf("expected tenant denial on deduct, got %v", err)
	}
	if _, err := svc.RestoreInventoryForOrder(foreignCtx, o.ID, OrderInventoryOptions{}); !errors.Is(err, security.ErrTenantAccessDenied) {
		t.Fatalf("expected tenant denial on restore, got %v", err)
	}
	var got product.ProductSKU
	if err := db.First(&got, "id = ?", localSKU.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Stock == nil || *got.Stock != stock {
		t.Fatalf("mismatched tenant context mutated SKU stock: %+v", got.Stock)
	}
}

func TestCreateInventorySyncTasksForSKUStockRejectsUnscopedSystemTenant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:inventory_sync_system_tenant_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&product.Product{}); err != nil {
		t.Fatal(err)
	}
	prod := product.Product{TenantID: 0, Source: "test", Status: product.StatusDraft}
	if err := db.Create(&prod).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := (&Service{DB: db}).CreateInventorySyncTasksForSKUStock(context.Background(), prod.ID, uuid.New(), 1, nil); err == nil {
		t.Fatal("unscoped system-tenant product was allowed to enqueue inventory sync")
	}
}
