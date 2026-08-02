package database

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/aioperationbatch"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
)

func TestMigrateImageTaskTenantScopeBackfillsTaskAndItems(t *testing.T) {
	dsn := fmt.Sprintf("file:migrate_imagetask_tenant_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}, &files.FileRecord{}, &product.Product{}, &product.ProductImage{}, &imagetask.ImageTask{}, &imagetask.ImageTaskItem{}); err != nil {
		t.Fatal(err)
	}
	p := product.Product{TenantID: 42, Source: "test", Status: product.StatusDraft, Title: "owned"}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	task := imagetask.ImageTask{TenantID: 0, TaskType: imagetask.TaskTypeGenerateScene, Provider: "test", Status: imagetask.StatusSuccess, ProductID: &p.ID}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	item := imagetask.ImageTaskItem{TenantID: 0, TaskID: task.ID, ProductID: &p.ID, Status: imagetask.ItemStatusSuccess}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateImageTaskTenantScope(db); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&task, "id = ?", task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&item, "id = ?", item.ID).Error; err != nil {
		t.Fatal(err)
	}
	if task.TenantID != 42 || item.TenantID != 42 {
		t.Fatalf("backfilled tenants task=%d item=%d", task.TenantID, item.TenantID)
	}

	foreignActor := admin.AdminUser{TenantID: 43, Username: admin.NewInternalUsername(), Email: "foreign-" + uuid.NewString() + "@example.test", PasswordHash: "test", Role: "admin", Status: "active"}
	if err := db.Create(&foreignActor).Error; err != nil {
		t.Fatal(err)
	}
	ambiguousTask := imagetask.ImageTask{TenantID: 0, TaskType: imagetask.TaskTypeGenerateScene, Provider: "test", Status: imagetask.StatusSuccess, ProductID: &p.ID, CreatedBy: &foreignActor.ID}
	if err := db.Create(&ambiguousTask).Error; err != nil {
		t.Fatal(err)
	}
	ambiguousItem := imagetask.ImageTaskItem{TenantID: 0, TaskID: ambiguousTask.ID, ProductID: &p.ID, Status: imagetask.ItemStatusSuccess}
	if err := db.Create(&ambiguousItem).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateImageTaskTenantScope(db); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&ambiguousTask, "id = ?", ambiguousTask.ID).Error; err != nil || ambiguousTask.TenantID != 0 {
		t.Fatalf("ambiguous image task must remain tenant zero: tenant=%d err=%v", ambiguousTask.TenantID, err)
	}
	if err := db.First(&ambiguousItem, "id = ?", ambiguousItem.ID).Error; err != nil || ambiguousItem.TenantID != 0 {
		t.Fatalf("item of ambiguous task must remain tenant zero: tenant=%d err=%v", ambiguousItem.TenantID, err)
	}
}

func TestMigrateAITaskTenantScopeOnlyBackfillsUniqueOwnership(t *testing.T) {
	dsn := fmt.Sprintf("file:migrate_aitask_tenant_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&product.Product{}, &aioperationbatch.AIOperationBatch{}, &aitask.AITask{}); err != nil {
		t.Fatal(err)
	}
	one := product.Product{TenantID: 42, Source: "test", Status: product.StatusDraft, Title: "one"}
	two := product.Product{TenantID: 43, Source: "test", Status: product.StatusDraft, Title: "two"}
	if err := db.Create(&one).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&two).Error; err != nil {
		t.Fatal(err)
	}
	batch := aioperationbatch.AIOperationBatch{TenantID: 42, BatchNo: "migration-ambiguous", OperationType: aioperationbatch.OperationTitleOptimize, Status: aioperationbatch.StatusSuccess}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	owned := aitask.AITask{TaskType: "test", Status: aitask.StatusSuccess, ProductID: &one.ID}
	ambiguous := aitask.AITask{TaskType: "test", Status: aitask.StatusSuccess, ProductID: &two.ID, BatchID: &batch.ID}
	if err := db.Create(&owned).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ambiguous).Error; err != nil {
		t.Fatal(err)
	}
	unknown := aitask.AITask{TaskType: "test", Status: aitask.StatusSuccess}
	if err := db.Create(&unknown).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateAITaskTenantScope(db); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&owned, "id = ?", owned.ID).Error; err != nil || owned.TenantID != 42 {
		t.Fatalf("unique ai task was not backfilled: tenant=%d err=%v", owned.TenantID, err)
	}
	if err := db.First(&unknown, "id = ?", unknown.ID).Error; err != nil || unknown.TenantID != 0 {
		t.Fatalf("unknown ai task must remain tenant zero: tenant=%d err=%v", unknown.TenantID, err)
	}
	if err := db.First(&ambiguous, "id = ?", ambiguous.ID).Error; err != nil || ambiguous.TenantID != 0 {
		t.Fatalf("ambiguous ai task must remain tenant zero: tenant=%d err=%v", ambiguous.TenantID, err)
	}
}
