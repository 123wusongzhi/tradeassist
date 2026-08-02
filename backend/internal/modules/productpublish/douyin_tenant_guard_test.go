package productpublish

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestProcessDouyinDraftTaskFailsBeforeCredentialsForMixedTenantTask(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:douyin_tenant_guard?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&product.Product{}, &shop.Shop{}, &ProductPublishTask{}); err != nil {
		t.Fatal(err)
	}
	pid, sid := uuid.New(), uuid.New()
	if err := db.Create(&product.Product{Base: model.Base{ID: pid}, TenantID: 10, Source: "test", Title: "product", Status: product.StatusDraft}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&shop.Shop{Base: model.Base{ID: sid}, TenantID: 20, Platform: "douyin_shop", ShopName: "shop", Status: shop.StatusActive}).Error; err != nil {
		t.Fatal(err)
	}
	task := ProductPublishTask{TenantID: 20, ProductID: pid, ShopID: sid, Platform: "douyin_shop", TaskType: TaskTypeDouyinDraftCreate, Status: TaskPending, Mode: PublishModeSaveAsPlatformDraft, PublishMode: PublishModeSaveAsPlatformDraft, Input: datatypes.JSON(`{"configId":"` + uuid.NewString() + `"}`)}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	// Shops is intentionally nil: reaching credential/provider code would panic.
	svc := &Service{DB: db}
	if err := svc.ProcessDouyinDraftTask(context.Background(), task.ID, "tenant-guard-test"); err == nil {
		t.Fatal("expected tenant integrity failure")
	}
	if err := db.First(&task, "id = ?", task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskFailed || task.ErrorCode != ErrorDouyinTenantMismatch || task.Retryable {
		t.Fatalf("unsafe task was not terminally blocked: status=%s code=%s retryable=%v", task.Status, task.ErrorCode, task.Retryable)
	}
}
