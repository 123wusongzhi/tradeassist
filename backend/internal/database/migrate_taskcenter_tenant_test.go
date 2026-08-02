package database

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproductimage"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproducttext"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"gorm.io/gorm"
)

func TestMigrateTaskcenterTenantScopeBackfillsOnlyTrustedParents(t *testing.T) {
	dsn := fmt.Sprintf("file:migrate_taskcenter_tenant_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&collect.CollectTask{}, &taskcenter.TaskAlert{}, &taskcenter.TaskFailureMark{}, &taskcenter.TaskAlertNotification{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	parent := collect.CollectTask{TenantID: 41, Source: "test", SourceURL: "https://example.test/owned", Status: collect.StatusFailed}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	ownedAlert := taskcenter.TaskAlert{ID: uuid.New(), TaskType: taskcenter.TaskTypeCollect, SourceID: parent.ID.String(), FailureCategory: "unknown", Status: taskcenter.TaskAlertStatusOpen, FirstSeenAt: now, LastSeenAt: now}
	unknownAlert := taskcenter.TaskAlert{ID: uuid.New(), TaskType: taskcenter.TaskTypeCollect, SourceID: uuid.NewString(), FailureCategory: "unknown", Status: taskcenter.TaskAlertStatusOpen, FirstSeenAt: now, LastSeenAt: now}
	ownedMark := taskcenter.TaskFailureMark{TaskType: taskcenter.TaskTypeCollect, SourceID: parent.ID.String(), SourceTable: "collect_tasks", MarkType: taskcenter.MarkIgnored}
	unknownMark := taskcenter.TaskFailureMark{TaskType: taskcenter.TaskTypeCollect, SourceID: uuid.NewString(), SourceTable: "collect_tasks", MarkType: taskcenter.MarkHandled}
	ownedNotification := taskcenter.TaskAlertNotification{ID: uuid.New(), AlertID: ownedAlert.ID, Channel: "mail", Status: taskcenter.TaskAlertNotifStatusSuccess}
	for _, row := range []any{&ownedAlert, &unknownAlert, &ownedMark, &unknownMark, &ownedNotification} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := migrateTaskcenterTenantScope(db); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		name  string
		value any
	}{{"owned alert", &ownedAlert}, {"owned mark", &ownedMark}, {"owned notification", &ownedNotification}} {
		if err := db.First(check.value).Error; err != nil {
			t.Fatalf("reload %s: %v", check.name, err)
		}
	}
	if ownedAlert.TenantID != 41 || ownedMark.TenantID != 41 || ownedNotification.TenantID != 41 {
		t.Fatalf("trusted ownership not propagated: alert=%d mark=%d notification=%d", ownedAlert.TenantID, ownedMark.TenantID, ownedNotification.TenantID)
	}
	for _, check := range []struct {
		name  string
		value any
	}{{"unknown alert", &unknownAlert}, {"unknown mark", &unknownMark}} {
		if err := db.First(check.value).Error; err != nil {
			t.Fatalf("reload %s: %v", check.name, err)
		}
	}
	if unknownAlert.TenantID != 0 || unknownMark.TenantID != 0 {
		t.Fatalf("unknown ownership must remain global: alert=%d mark=%d", unknownAlert.TenantID, unknownMark.TenantID)
	}
	duplicateAlert := taskcenter.TaskAlert{ID: uuid.New(), TenantID: 42, TaskType: ownedAlert.TaskType, SourceID: ownedAlert.SourceID, FailureCategory: ownedAlert.FailureCategory, Status: taskcenter.TaskAlertStatusOpen, FirstSeenAt: now, LastSeenAt: now}
	duplicateMark := taskcenter.TaskFailureMark{TenantID: 42, TaskType: ownedMark.TaskType, SourceID: ownedMark.SourceID, SourceTable: ownedMark.SourceTable, MarkType: ownedMark.MarkType}
	if err := db.Create(&duplicateAlert).Error; err != nil {
		t.Fatalf("tenant-composite alert uniqueness missing: %v", err)
	}
	if err := db.Create(&duplicateMark).Error; err != nil {
		t.Fatalf("tenant-composite mark uniqueness missing: %v", err)
	}
}

func TestMigrateTaskcenterTenantScopeBackfillsAIItemsFromBatches(t *testing.T) {
	dsn := fmt.Sprintf("file:migrate_taskcenter_ai_tenant_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&aiproducttext.AIProductTextBatch{}, &aiproducttext.AIProductTextItem{},
		&aiproductimage.AIProductImageBatch{}, &aiproductimage.AIProductImageItem{},
		&taskcenter.TaskAlert{}, &taskcenter.TaskFailureMark{},
	); err != nil {
		t.Fatal(err)
	}
	textBatch := aiproducttext.AIProductTextBatch{TenantID: 51, BatchNo: "text-owned", Status: aiproducttext.BatchFailed}
	imageBatch := aiproductimage.AIProductImageBatch{TenantID: 52, BatchNo: "image-owned", Status: aiproductimage.BatchFailed}
	unknownBatch := aiproducttext.AIProductTextBatch{BatchNo: "text-unassigned", Status: aiproducttext.BatchFailed}
	for _, batch := range []any{&textBatch, &imageBatch, &unknownBatch} {
		if err := db.Create(batch).Error; err != nil {
			t.Fatal(err)
		}
	}
	textItem := aiproducttext.AIProductTextItem{BatchID: textBatch.ID, ProductID: uuid.New(), OperationType: aiproducttext.OpTitle, Status: aiproducttext.ItemFailed}
	imageItem := aiproductimage.AIProductImageItem{BatchID: imageBatch.ID, ProductID: uuid.New(), OperationType: aiproductimage.OpQualityCheck, Status: aiproductimage.ItemFailed}
	unknownItem := aiproducttext.AIProductTextItem{BatchID: unknownBatch.ID, ProductID: uuid.New(), OperationType: aiproducttext.OpTitle, Status: aiproducttext.ItemFailed}
	for _, item := range []any{&textItem, &imageItem, &unknownItem} {
		if err := db.Create(item).Error; err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	rows := []any{
		&taskcenter.TaskAlert{ID: uuid.New(), TaskType: taskcenter.TaskTypeAIText, SourceID: textItem.ID.String(), FailureCategory: "unknown", Status: taskcenter.TaskAlertStatusOpen, FirstSeenAt: now, LastSeenAt: now},
		&taskcenter.TaskFailureMark{TaskType: taskcenter.TaskTypeAIText, SourceID: textItem.ID.String(), SourceTable: "ai_product_text_items", MarkType: taskcenter.MarkIgnored},
		&taskcenter.TaskAlert{ID: uuid.New(), TaskType: taskcenter.TaskTypeAIImage, SourceID: imageItem.ID.String(), FailureCategory: "unknown", Status: taskcenter.TaskAlertStatusOpen, FirstSeenAt: now, LastSeenAt: now},
		&taskcenter.TaskFailureMark{TaskType: taskcenter.TaskTypeAIImage, SourceID: imageItem.ID.String(), SourceTable: "ai_product_image_items", MarkType: taskcenter.MarkIgnored},
		&taskcenter.TaskAlert{ID: uuid.New(), TaskType: taskcenter.TaskTypeAIText, SourceID: unknownItem.ID.String(), FailureCategory: "unknown", Status: taskcenter.TaskAlertStatusOpen, FirstSeenAt: now, LastSeenAt: now},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := migrateTaskcenterTenantScope(db); err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name string
		id   string
		want int64
	}{
		{"text alert", textItem.ID.String(), 51}, {"text mark", textItem.ID.String(), 51},
		{"image alert", imageItem.ID.String(), 52}, {"image mark", imageItem.ID.String(), 52},
		{"unassigned alert", unknownItem.ID.String(), 0},
	}
	for _, check := range checks {
		var tenantID int64
		var query string
		if check.name == "text mark" || check.name == "image mark" {
			query = `SELECT tenant_id FROM task_failure_marks WHERE source_id = ?`
		} else {
			query = `SELECT tenant_id FROM task_alerts WHERE source_id = ?`
		}
		if err := db.Raw(query, check.id).Scan(&tenantID).Error; err != nil {
			t.Fatalf("load %s: %v", check.name, err)
		}
		if tenantID != check.want {
			t.Fatalf("%s tenant_id = %d, want %d", check.name, tenantID, check.want)
		}
	}
}

func TestAutoMigrateBackfillsLegacyAIAlertAfterBatchTenantBackfill(t *testing.T) {
	dsn := fmt.Sprintf("file:migrate_taskcenter_legacy_ai_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&product.Product{}, &aiproducttext.AIProductTextBatch{}, &aiproducttext.AIProductTextItem{}, &taskcenter.TaskAlert{}); err != nil {
		t.Fatal(err)
	}
	ownedProduct := product.Product{TenantID: 63, Title: "legacy AI item"}
	if err := db.Create(&ownedProduct).Error; err != nil {
		t.Fatal(err)
	}
	batch := aiproducttext.AIProductTextBatch{BatchNo: "legacy-text-batch", Status: aiproducttext.BatchFailed}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	item := aiproducttext.AIProductTextItem{BatchID: batch.ID, ProductID: ownedProduct.ID, OperationType: aiproducttext.OpTitle, Status: aiproducttext.ItemFailed}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	alert := taskcenter.TaskAlert{ID: uuid.New(), TaskType: taskcenter.TaskTypeAIText, SourceID: item.ID.String(), FailureCategory: "unknown", Status: taskcenter.TaskAlertStatusOpen, FirstSeenAt: now, LastSeenAt: now}
	if err := db.Create(&alert).Error; err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate legacy AI alert: %v", err)
	}
	if err := db.First(&alert, "id = ?", alert.ID).Error; err != nil {
		t.Fatal(err)
	}
	if alert.TenantID != 63 {
		t.Fatalf("legacy alert tenant_id = %d, want 63", alert.TenantID)
	}
}
