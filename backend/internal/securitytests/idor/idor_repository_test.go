package idor_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproductimage"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproducttext"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/exportmod"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/webhook"
	"github.com/trademind-ai/trademind/backend/internal/pkg/repository"
	"gorm.io/gorm"
)

// Repository-level IDOR tests verify tenant_id isolation for P4.2 task tables.

func TestIDOR_InventoryTaskFindByIDCrossTenant(t *testing.T) {
	db := openP42DB(t)
	shopB := seedShopForTenant(t, db, tenantB, "shop-b")
	pidB := seedProductForTenant(t, db, tenantB, secretTenantB)
	tid := seedInventoryTask(t, db, tenantB, shopB, pidB)
	var row inventory.InventorySyncTask
	assertFindByIDDenied(t, db, tenantA, tid, &row)
	assertNoSensitiveLeak(t, row.ErrorMessage)
}

func TestIDOR_OrderSyncTaskFindByIDCrossTenant(t *testing.T) {
	db := openP42DB(t)
	shopB := seedShopForTenant(t, db, tenantB, "shop-b")
	tid := seedOrderSyncTask(t, db, tenantB, shopB)
	var row ordersync.OrderSyncTask
	assertFindByIDDenied(t, db, tenantA, tid, &row)
	assertNoSensitiveLeak(t, row.ErrorMessage)
}

func TestIDOR_ProductPublishTaskFindByIDCrossTenant(t *testing.T) {
	db := openP42DB(t)
	shopB := seedShopForTenant(t, db, tenantB, "shop-b")
	pidB := seedProductForTenant(t, db, tenantB, secretTenantB)
	tid := seedProductPublishTask(t, db, tenantB, shopB, pidB)
	var row productpublish.ProductPublishTask
	assertFindByIDDenied(t, db, tenantA, tid, &row)
	assertNoSensitiveLeak(t, row.ErrorMessage)
}

func TestIDOR_AITextBatchFindByIDCrossTenant(t *testing.T) {
	db := openP42DB(t)
	bid := seedAITextBatch(t, db, tenantB)
	var row aiproducttext.AIProductTextBatch
	assertFindByIDDenied(t, db, tenantA, bid, &row)
}

func TestIDOR_AIImageBatchFindByIDCrossTenant(t *testing.T) {
	db := openP42DB(t)
	bid := seedAIImageBatch(t, db, tenantB)
	var row aiproductimage.AIProductImageBatch
	assertFindByIDDenied(t, db, tenantA, bid, &row)
}

func TestIDOR_CustomerConversationFindByIDCrossTenant(t *testing.T) {
	db := openP42DB(t)
	shopB := seedShopForTenant(t, db, tenantB, "shop-b")
	cid := seedCustomerConversation(t, db, tenantB, shopB, secretTenantB)
	var row customerchat.CustomerConversation
	assertFindByIDDenied(t, db, tenantA, cid, &row)
	assertNoSensitiveLeak(t, row.CustomerName)
}

func TestIDOR_WebhookEventFindByIDCrossTenant(t *testing.T) {
	db := openP42DB(t)
	eid := seedWebhookEvent(t, db, tenantB)
	var row webhook.Event
	assertFindByIDDenied(t, db, tenantA, eid, &row)
	assertNoSensitiveLeak(t, row.RawSummary)
}

func TestIDOR_ExportJobFindByIDCrossTenant(t *testing.T) {
	db := openP42DB(t)
	jid := seedExportJob(t, db, tenantB, exportmod.ExportTypeInventory)
	var row exportmod.ExportJob
	assertFindByIDDenied(t, db, tenantA, jid, &row)
	assertNoSensitiveLeak(t, row.ErrorMessage)
}

func TestIDOR_SameTenantCanLoadOwnRow(t *testing.T) {
	db := openP42DB(t)
	shopA := seedShopForTenant(t, db, tenantA, "shop-a")
	pidA := seedProductForTenant(t, db, tenantA, "own")
	tid := seedInventoryTask(t, db, tenantA, shopA, pidA)
	var row inventory.InventorySyncTask
	ctx := ginWithTenant(tenantA, uuid.New()).Request.Context()
	if err := repositoryFindByID(ctx, db, &row, tenantA, tid); err != nil {
		t.Fatalf("same-tenant load should succeed: %v", err)
	}
}

func repositoryFindByID(ctx context.Context, db *gorm.DB, dest any, tenantID int64, id uuid.UUID) error {
	return repository.FindByID(ctx, db, dest, tenantID, id)
}
