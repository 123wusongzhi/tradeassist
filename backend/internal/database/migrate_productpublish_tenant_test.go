package database

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func TestMigrateProductPublishTenantScopeSQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:publish_tenant_%s?mode=memory", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&product.Product{}, &shop.Shop{}, &productpublish.ProductPublishTask{}, &productpublish.ProductPublishBatch{}, &productpublish.ProductPublication{}); err != nil {
		t.Fatal(err)
	}
	pid, sid := uuid.New(), uuid.New()
	if err := db.Create(&product.Product{Base: model.Base{ID: pid}, TenantID: 7, Source: "test", Title: "p", Status: product.StatusDraft}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&shop.Shop{Base: model.Base{ID: sid}, TenantID: 7, Platform: "shopee", ShopName: "s", Status: shop.StatusActive}).Error; err != nil {
		t.Fatal(err)
	}
	batch := productpublish.ProductPublishBatch{BatchType: productpublish.BatchTypeMultiProduct, Status: productpublish.BatchRunning}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	task := productpublish.ProductPublishTask{TenantID: 0, ProductID: pid, ShopID: sid, BatchID: &batch.ID, Platform: "shopee", Status: productpublish.TaskPending, TaskType: productpublish.TaskTypeLocalDraftCreate, Mode: productpublish.PublishModeSaveAsPlatformDraft}
	pub := productpublish.ProductPublication{TenantID: 0, ProductID: pid, ShopID: sid, Platform: "shopee", Status: productpublish.StatusDraft, PublishStatus: productpublish.StatusDraft}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&pub).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateProductPublishTenantScope(db); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&task, "id = ?", task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&pub, "id = ?", pub.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&batch, "id = ?", batch.ID).Error; err != nil {
		t.Fatal(err)
	}
	if task.TenantID != 7 || pub.TenantID != 7 || batch.TenantID != 7 {
		t.Fatalf("trusted backfill failed: task=%d pub=%d batch=%d", task.TenantID, pub.TenantID, batch.TenantID)
	}
}

func TestMigrateProductPublishTenantScopeLeavesMismatchedRowsUnassigned(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:publish_tenant_mismatch_%s?mode=memory", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&product.Product{}, &shop.Shop{}, &productpublish.ProductPublishTask{}, &productpublish.ProductPublication{}); err != nil {
		t.Fatal(err)
	}
	pid, sid := uuid.New(), uuid.New()
	if err := db.Create(&product.Product{Base: model.Base{ID: pid}, TenantID: 11, Source: "test", Title: "p", Status: product.StatusDraft}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&shop.Shop{Base: model.Base{ID: sid}, TenantID: 22, Platform: "shopee", ShopName: "s", Status: shop.StatusActive}).Error; err != nil {
		t.Fatal(err)
	}
	task := productpublish.ProductPublishTask{ProductID: pid, ShopID: sid, Platform: "shopee", Status: productpublish.TaskPending, TaskType: productpublish.TaskTypeLocalDraftCreate, Mode: productpublish.PublishModeSaveAsPlatformDraft}
	pub := productpublish.ProductPublication{ProductID: pid, ShopID: sid, Platform: "shopee", Status: productpublish.StatusDraft, PublishStatus: productpublish.StatusDraft}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&pub).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateProductPublishTenantScope(db); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&task, "id = ?", task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&pub, "id = ?", pub.ID).Error; err != nil {
		t.Fatal(err)
	}
	if task.TenantID != 0 || pub.TenantID != 0 {
		t.Fatalf("mismatched rows must remain unassigned: task=%d pub=%d", task.TenantID, pub.TenantID)
	}
}
