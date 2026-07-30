package idor_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproductimage"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproducttext"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/exportmod"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/securitymod"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"github.com/trademind-ai/trademind/backend/internal/modules/webhook"
	"github.com/trademind-ai/trademind/backend/internal/pkg/repository"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

const secretTenantB = "SECRET-TENANT-B-MARKER"

func openP42DB(t *testing.T) *gorm.DB {
	t.Helper()
	db := openIDORDB(t)
	if err := db.AutoMigrate(
		&exportmod.ExportJob{},
		&operationlog.OperationLog{},
		&inventory.InventorySyncTask{},
		&inventory.InventorySyncBatch{},
		&ordersync.OrderSyncTask{},
		&productpublish.ProductPublishTask{},
		&aiproducttext.AIProductTextBatch{},
		&aiproducttext.AIProductTextItem{},
		&aiproductimage.AIProductImageBatch{},
		&customerchat.CustomerConversation{},
		&taskcenter.TaskAlert{},
		&taskcenter.TaskFailureMark{},
		&webhook.Event{},
		&securitymod.KeyRotationJob{},
		&collect.CollectTask{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func assertCrossTenantDenied(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected cross-tenant access denied")
	}
}

func assertNoSensitiveLeak(t *testing.T, payload string) {
	t.Helper()
	if strings.Contains(payload, secretTenantB) {
		t.Fatalf("sensitive cross-tenant marker leaked: %q", secretTenantB)
	}
}

func seedShopForTenant(t *testing.T, db *gorm.DB, tenantID int64, name string) uuid.UUID {
	t.Helper()
	s := &shop.Shop{TenantID: tenantID, Platform: "douyin_shop", ShopName: name, Status: "active"}
	if err := db.Create(s).Error; err != nil {
		t.Fatal(err)
	}
	return s.ID
}

func seedProductForTenant(t *testing.T, db *gorm.DB, tenantID int64, title string) uuid.UUID {
	t.Helper()
	p := &product.Product{TenantID: tenantID, Title: title, OriginalTitle: title, Status: product.StatusDraft, Currency: "CNY", Source: "test"}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	return p.ID
}

func seedExportJob(t *testing.T, db *gorm.DB, tenantID int64, exportType string) uuid.UUID {
	t.Helper()
	msg := ""
	if tenantID == tenantB {
		msg = secretTenantB
	}
	row := &exportmod.ExportJob{
		TenantID: tenantID, ExportType: exportType, Status: exportmod.ExportStatusPending,
		MaskedPII: true, ErrorMessage: msg,
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func seedOpLog(t *testing.T, db *gorm.DB, tenantID int64, shopID *uuid.UUID, message string) uuid.UUID {
	t.Helper()
	row := &operationlog.OperationLog{
		TenantID: tenantID, Username: "u", Action: "test.action", Resource: "test",
		Method: "GET", Path: "/test", Status: "success", Message: message, CreatedAt: time.Now().UTC(),
	}
	if shopID != nil {
		row.ShopID = shopID
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func seedInventoryTask(t *testing.T, db *gorm.DB, tenantID int64, shopID, productID uuid.UUID) uuid.UUID {
	t.Helper()
	msg := ""
	if tenantID == tenantB {
		msg = secretTenantB
	}
	row := &inventory.InventorySyncTask{
		TenantID: tenantID, ShopID: shopID, ProductID: productID, Platform: "douyin_shop",
		TaskType: "sync", Status: inventory.StatusFailed, Mode: "manual", TargetStock: 1,
		ErrorMessage: msg,
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func seedOrderSyncTask(t *testing.T, db *gorm.DB, tenantID int64, shopID uuid.UUID) uuid.UUID {
	t.Helper()
	msg := ""
	if tenantID == tenantB {
		msg = secretTenantB
	}
	row := &ordersync.OrderSyncTask{
		TenantID: tenantID, ShopID: shopID, Platform: "douyin_shop", TaskType: "sync",
		Status: ordersync.StatusFailed, Mode: "manual", ErrorMessage: msg,
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func seedProductPublishTask(t *testing.T, db *gorm.DB, tenantID int64, shopID, productID uuid.UUID) uuid.UUID {
	t.Helper()
	msg := ""
	if tenantID == tenantB {
		msg = secretTenantB
	}
	row := &productpublish.ProductPublishTask{
		TenantID: tenantID, ShopID: shopID, ProductID: productID, TargetStoreID: shopID,
		Platform: "douyin_shop", TaskType: "publish", Status: productpublish.TaskFailed,
		Mode: "manual", ErrorMessage: msg,
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func seedAITextBatch(t *testing.T, db *gorm.DB, tenantID int64) uuid.UUID {
	t.Helper()
	row := &aiproducttext.AIProductTextBatch{
		TenantID: tenantID, BatchNo: fmt.Sprintf("B%d-%s", tenantID, uuid.NewString()[:8]),
		BatchType: aiproducttext.BatchTypeAIText, Status: aiproducttext.BatchFailed,
		IdempotencyKey: uuid.NewString(),
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func seedAIImageBatch(t *testing.T, db *gorm.DB, tenantID int64) uuid.UUID {
	t.Helper()
	row := &aiproductimage.AIProductImageBatch{
		TenantID: tenantID, BatchNo: fmt.Sprintf("I%d-%s", tenantID, uuid.NewString()[:8]),
		BatchType: aiproductimage.BatchTypeAIImage, Status: aiproductimage.BatchFailed,
		IdempotencyKey: uuid.NewString(),
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func seedCustomerConversation(t *testing.T, db *gorm.DB, tenantID int64, shopID uuid.UUID, customerName string) uuid.UUID {
	t.Helper()
	row := &customerchat.CustomerConversation{
		TenantID: tenantID, Platform: "manual", ShopID: &shopID,
		CustomerName: customerName, CustomerLanguage: "en", Status: customerchat.StatusOpen,
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func seedTaskAlert(t *testing.T, db *gorm.DB, tenantID int64, sourceID string) uuid.UUID {
	t.Helper()
	now := time.Now().UTC()
	title := "alert"
	if tenantID == tenantB {
		title = secretTenantB
	}
	row := &taskcenter.TaskAlert{
		ID: uuid.New(), TenantID: tenantID, TaskType: taskcenter.TaskTypeOrderSync, SourceID: sourceID,
		FailureCategory: "sync_error", Severity: "high", Title: title,
		Status: taskcenter.TaskAlertStatusOpen, FirstSeenAt: now, LastSeenAt: now,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func seedTaskFailureMark(t *testing.T, db *gorm.DB, tenantID int64, sourceID string) uuid.UUID {
	t.Helper()
	remark := "mark"
	if tenantID == tenantB {
		remark = secretTenantB
	}
	row := &taskcenter.TaskFailureMark{
		TenantID: tenantID, TaskType: taskcenter.TaskTypeOrderSync, SourceID: sourceID,
		SourceTable: "order_sync_tasks", MarkType: "ignored", Remark: remark,
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func seedWebhookEvent(t *testing.T, db *gorm.DB, tenantID int64) uuid.UUID {
	t.Helper()
	summary := "summary"
	if tenantID == tenantB {
		summary = secretTenantB
	}
	row := &webhook.Event{
		Platform: "douyin_shop", TenantID: tenantID, EventID: uuid.NewString(),
		PayloadHash: uuid.NewString(), Status: webhook.StatusReceived, RawSummary: summary,
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func seedRotationJob(t *testing.T, db *gorm.DB, tenantID int64) uuid.UUID {
	t.Helper()
	row := &securitymod.KeyRotationJob{
		TenantID: tenantID, ActiveKeyID: "k1", Scope: "tenant", Status: securitymod.RotationRunning,
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func seedCollectTaskFailed(t *testing.T, db *gorm.DB, tenantID int64) uuid.UUID {
	t.Helper()
	msg := ""
	if tenantID == tenantB {
		msg = secretTenantB
	}
	row := &collect.CollectTask{
		TenantID: tenantID, Source: "1688", SourceURL: "https://example.com/item", Status: collect.StatusFailed,
		ErrorMessage: msg,
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func assertFindByIDDenied(t *testing.T, db *gorm.DB, tenantID int64, id uuid.UUID, dest any) {
	t.Helper()
	ctx := context.Background()
	err := repository.FindByID(ctx, db, dest, tenantID, id)
	assertCrossTenantDenied(t, err)
}

func assertTaskTenantMissing(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected task tenant missing error")
	}
	if err != security.ErrTaskTenantMissing {
		t.Fatalf("expected ErrTaskTenantMissing, got %v", err)
	}
}
