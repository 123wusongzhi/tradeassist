package idor_test

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproductimage"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproducttext"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/exportmod"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

// --- Service-layer cross-tenant denial via tenant-scoped data access patterns ---

func TestIDOR_InventoryListTasksExcludesOtherTenant(t *testing.T) {
	db := openP42DB(t)
	shopA := seedShopForTenant(t, db, tenantA, "a")
	shopB := seedShopForTenant(t, db, tenantB, "b")
	pidA := seedProductForTenant(t, db, tenantA, "a")
	pidB := seedProductForTenant(t, db, tenantB, secretTenantB)
	seedInventoryTask(t, db, tenantA, shopA, pidA)
	seedInventoryTask(t, db, tenantB, shopB, pidB)
	c := ginWithTenant(tenantA, uuid.New())
	var rows []inventory.InventorySyncTask
	tx, _, err := applyTenantList(c, db.Model(&inventory.InventorySyncTask{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 inventory task, got %d", len(rows))
	}
	for _, row := range rows {
		assertNoSensitiveLeak(t, row.ErrorMessage)
	}
}

func TestIDOR_OrderSyncListExcludesOtherTenant(t *testing.T) {
	db := openP42DB(t)
	shopA := seedShopForTenant(t, db, tenantA, "a")
	shopB := seedShopForTenant(t, db, tenantB, "b")
	seedOrderSyncTask(t, db, tenantA, shopA)
	seedOrderSyncTask(t, db, tenantB, shopB)
	c := ginWithTenant(tenantA, uuid.New())
	var rows []ordersync.OrderSyncTask
	tx, _, err := applyTenantList(c, db.Model(&ordersync.OrderSyncTask{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 order sync task, got %d", len(rows))
	}
}

func TestIDOR_ProductPublishListExcludesOtherTenant(t *testing.T) {
	db := openP42DB(t)
	shopA := seedShopForTenant(t, db, tenantA, "a")
	shopB := seedShopForTenant(t, db, tenantB, "b")
	pidA := seedProductForTenant(t, db, tenantA, "a")
	pidB := seedProductForTenant(t, db, tenantB, secretTenantB)
	seedProductPublishTask(t, db, tenantA, shopA, pidA)
	seedProductPublishTask(t, db, tenantB, shopB, pidB)
	c := ginWithTenant(tenantA, uuid.New())
	var rows []productpublish.ProductPublishTask
	tx, _, err := applyTenantList(c, db.Model(&productpublish.ProductPublishTask{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 publish task, got %d", len(rows))
	}
}

func TestIDOR_AITextBatchListExcludesOtherTenant(t *testing.T) {
	db := openP42DB(t)
	seedAITextBatch(t, db, tenantA)
	seedAITextBatch(t, db, tenantB)
	c := ginWithTenant(tenantA, uuid.New())
	var rows []aiproducttext.AIProductTextBatch
	tx, _, err := applyTenantList(c, db.Model(&aiproducttext.AIProductTextBatch{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 ai text batch, got %d", len(rows))
	}
}

func TestIDOR_AIImageBatchListExcludesOtherTenant(t *testing.T) {
	db := openP42DB(t)
	seedAIImageBatch(t, db, tenantA)
	seedAIImageBatch(t, db, tenantB)
	c := ginWithTenant(tenantA, uuid.New())
	var rows []aiproductimage.AIProductImageBatch
	tx, _, err := applyTenantList(c, db.Model(&aiproductimage.AIProductImageBatch{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 ai image batch, got %d", len(rows))
	}
}

func TestIDOR_CustomerChatListExcludesOtherTenant(t *testing.T) {
	db := openP42DB(t)
	shopA := seedShopForTenant(t, db, tenantA, "a")
	shopB := seedShopForTenant(t, db, tenantB, "b")
	seedCustomerConversation(t, db, tenantA, shopA, "alice")
	seedCustomerConversation(t, db, tenantB, shopB, secretTenantB)
	c := ginWithTenant(tenantA, uuid.New())
	var rows []customerchat.CustomerConversation
	tx, _, err := applyTenantList(c, db.Model(&customerchat.CustomerConversation{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(rows))
	}
	for _, row := range rows {
		assertNoSensitiveLeak(t, row.CustomerName)
	}
}

func TestIDOR_ExportServiceListNoCrossTenantLeak(t *testing.T) {
	db := openP42DB(t)
	svc := &exportmod.Service{DB: db}
	seedExportJob(t, db, tenantA, exportmod.ExportTypeOrders)
	seedExportJob(t, db, tenantB, exportmod.ExportTypeCustomers)
	c := ginWithTenant(tenantA, uuid.New())
	rows, total, err := svc.ListJobs(c, 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	for _, row := range rows {
		assertNoSensitiveLeak(t, row.ErrorMessage)
	}
}

func applyTenantList(c *gin.Context, tx *gorm.DB) (*gorm.DB, int64, error) {
	return adminperm.ApplyTenantScope(c, tx)
}
