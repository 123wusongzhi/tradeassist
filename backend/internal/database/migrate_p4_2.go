package database

import (
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/aioperationbatch"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproductimage"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproducttext"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/customersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/exportmod"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"gorm.io/gorm"
)

// migrateP42Security applies Phase P4.2 tenant columns, export jobs and security worker indexes.
func migrateP42Security(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate p4.2: db is nil")
	}
	if err := db.AutoMigrate(
		&inventory.InventorySyncTask{},
		&inventory.InventorySyncBatch{},
		&inventory.InventoryChangeLog{},
		&ordersync.OrderSyncTask{},
		&customersync.CustomerMessageSyncTask{},
		&productpublish.ProductPublishTask{},
		&aioperationbatch.AIOperationBatch{},
		&aiproducttext.AIProductTextBatch{},
		&aiproductimage.AIProductImageBatch{},
		&customerchat.CustomerConversation{},
		&collect.CollectTask{},
		&collect.CollectBatch{},
		&taskcenter.TaskFailureMark{},
		&taskcenter.TaskAlert{},
		&product.DouyinImageAsset{},
		&exportmod.ExportJob{},
		&files.FileRecord{},
	); err != nil {
		return err
	}
	if err := backfillP42TenantIDs(db); err != nil {
		return err
	}
	return migrateP42Indexes(db)
}

func backfillP42TenantIDs(db *gorm.DB) error {
	return runP42BackfillStatements(db, p42BackfillStatements())
}

type p42BackfillStatement struct {
	name        string
	postgresSQL string
	sqliteSQL   string
}

func p42BackfillStatements() []p42BackfillStatement {
	return []p42BackfillStatement{
		{"inventory_sync_tasks", `UPDATE inventory_sync_tasks t SET tenant_id = s.tenant_id FROM shops s WHERE t.shop_id = s.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`, `UPDATE inventory_sync_tasks AS t SET tenant_id = (SELECT s.tenant_id FROM shops s WHERE s.id = t.shop_id) WHERE (t.tenant_id IS NULL OR t.tenant_id = 0) AND EXISTS (SELECT 1 FROM shops s WHERE s.id = t.shop_id)`},
		{"inventory_sync_batches", `UPDATE inventory_sync_batches t SET tenant_id = s.tenant_id FROM shops s WHERE t.shop_id = s.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`, `UPDATE inventory_sync_batches AS t SET tenant_id = (SELECT s.tenant_id FROM shops s WHERE s.id = t.shop_id) WHERE (t.tenant_id IS NULL OR t.tenant_id = 0) AND EXISTS (SELECT 1 FROM shops s WHERE s.id = t.shop_id)`},
		{"inventory_change_logs", `UPDATE inventory_change_logs t SET tenant_id = p.tenant_id FROM products p WHERE t.product_id = p.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`, `UPDATE inventory_change_logs AS t SET tenant_id = (SELECT p.tenant_id FROM products p WHERE p.id = t.product_id) WHERE (t.tenant_id IS NULL OR t.tenant_id = 0) AND EXISTS (SELECT 1 FROM products p WHERE p.id = t.product_id)`},
		{"order_sync_tasks", `UPDATE order_sync_tasks t SET tenant_id = s.tenant_id FROM shops s WHERE t.shop_id = s.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`, `UPDATE order_sync_tasks AS t SET tenant_id = (SELECT s.tenant_id FROM shops s WHERE s.id = t.shop_id) WHERE (t.tenant_id IS NULL OR t.tenant_id = 0) AND EXISTS (SELECT 1 FROM shops s WHERE s.id = t.shop_id)`},
		{"customer_message_sync_tasks", `UPDATE customer_message_sync_tasks t SET tenant_id = s.tenant_id FROM shops s WHERE t.shop_id = s.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`, `UPDATE customer_message_sync_tasks AS t SET tenant_id = (SELECT s.tenant_id FROM shops s WHERE s.id = t.shop_id) WHERE (t.tenant_id IS NULL OR t.tenant_id = 0) AND EXISTS (SELECT 1 FROM shops s WHERE s.id = t.shop_id)`},
		// Publishing tasks are dual-owned: never infer a tenant from only the shop.
		// A legacy zero remains zero unless both product and shop prove the same
		// non-system tenant. Exact tenant-0 pairs remain intentionally unassigned.
		{"product_publish_tasks", `UPDATE product_publish_tasks t SET tenant_id = p.tenant_id FROM products p, shops s WHERE t.product_id = p.id AND t.shop_id = s.id AND (t.tenant_id IS NULL OR t.tenant_id = 0) AND p.tenant_id = s.tenant_id AND p.tenant_id <> 0`, `UPDATE product_publish_tasks AS t SET tenant_id = (SELECT p.tenant_id FROM products p JOIN shops s ON s.id = t.shop_id WHERE p.id = t.product_id AND p.tenant_id = s.tenant_id AND p.tenant_id <> 0) WHERE (t.tenant_id IS NULL OR t.tenant_id = 0) AND EXISTS (SELECT 1 FROM products p JOIN shops s ON s.id = t.shop_id WHERE p.id = t.product_id AND p.tenant_id = s.tenant_id AND p.tenant_id <> 0)`},
		{"ai_product_text_batches", `UPDATE ai_product_text_batches b SET tenant_id = src.tenant_id FROM (SELECT i.batch_id, MIN(p.tenant_id) AS tenant_id FROM ai_product_text_items i JOIN products p ON p.id = i.product_id GROUP BY i.batch_id HAVING COUNT(DISTINCT p.tenant_id) = 1) src WHERE b.id = src.batch_id AND (b.tenant_id IS NULL OR b.tenant_id = 0)`, `UPDATE ai_product_text_batches AS b SET tenant_id = (SELECT MIN(p.tenant_id) FROM ai_product_text_items i JOIN products p ON p.id = i.product_id WHERE i.batch_id = b.id GROUP BY i.batch_id HAVING COUNT(DISTINCT p.tenant_id) = 1) WHERE (b.tenant_id IS NULL OR b.tenant_id = 0) AND EXISTS (SELECT 1 FROM ai_product_text_items i JOIN products p ON p.id = i.product_id WHERE i.batch_id = b.id GROUP BY i.batch_id HAVING COUNT(DISTINCT p.tenant_id) = 1)`},
		{"ai_product_image_batches", `UPDATE ai_product_image_batches b SET tenant_id = src.tenant_id FROM (SELECT i.batch_id, MIN(p.tenant_id) AS tenant_id FROM ai_product_image_items i JOIN products p ON p.id = i.product_id GROUP BY i.batch_id HAVING COUNT(DISTINCT p.tenant_id) = 1) src WHERE b.id = src.batch_id AND (b.tenant_id IS NULL OR b.tenant_id = 0)`, `UPDATE ai_product_image_batches AS b SET tenant_id = (SELECT MIN(p.tenant_id) FROM ai_product_image_items i JOIN products p ON p.id = i.product_id WHERE i.batch_id = b.id GROUP BY i.batch_id HAVING COUNT(DISTINCT p.tenant_id) = 1) WHERE (b.tenant_id IS NULL OR b.tenant_id = 0) AND EXISTS (SELECT 1 FROM ai_product_image_items i JOIN products p ON p.id = i.product_id WHERE i.batch_id = b.id GROUP BY i.batch_id HAVING COUNT(DISTINCT p.tenant_id) = 1)`},
		{"ai_operation_batches_text", `UPDATE ai_operation_batches b SET tenant_id = src.tenant_id FROM (SELECT t.batch_id, MIN(p.tenant_id) AS tenant_id FROM ai_tasks t JOIN products p ON p.id = t.product_id WHERE t.batch_id IS NOT NULL GROUP BY t.batch_id HAVING COUNT(DISTINCT p.tenant_id) = 1) src WHERE b.id = src.batch_id AND (b.tenant_id IS NULL OR b.tenant_id = 0)`, `UPDATE ai_operation_batches AS b SET tenant_id = (SELECT MIN(p.tenant_id) FROM ai_tasks t JOIN products p ON p.id = t.product_id WHERE t.batch_id = b.id GROUP BY t.batch_id HAVING COUNT(DISTINCT p.tenant_id) = 1) WHERE (b.tenant_id IS NULL OR b.tenant_id = 0) AND EXISTS (SELECT 1 FROM ai_tasks t JOIN products p ON p.id = t.product_id WHERE t.batch_id = b.id GROUP BY t.batch_id HAVING COUNT(DISTINCT p.tenant_id) = 1)`},
		{"ai_operation_batches_image", `UPDATE ai_operation_batches b SET tenant_id = src.tenant_id FROM (SELECT t.batch_id, MIN(t.tenant_id) AS tenant_id FROM image_tasks t WHERE t.batch_id IS NOT NULL GROUP BY t.batch_id HAVING COUNT(DISTINCT t.tenant_id) = 1) src WHERE b.id = src.batch_id AND (b.tenant_id IS NULL OR b.tenant_id = 0)`, `UPDATE ai_operation_batches AS b SET tenant_id = (SELECT MIN(t.tenant_id) FROM image_tasks t WHERE t.batch_id = b.id GROUP BY t.batch_id HAVING COUNT(DISTINCT t.tenant_id) = 1) WHERE (b.tenant_id IS NULL OR b.tenant_id = 0) AND EXISTS (SELECT 1 FROM image_tasks t WHERE t.batch_id = b.id GROUP BY t.batch_id HAVING COUNT(DISTINCT t.tenant_id) = 1)`},
		{"customer_conversations", `UPDATE customer_conversations t SET tenant_id = s.tenant_id FROM shops s WHERE t.shop_id = s.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`, `UPDATE customer_conversations AS t SET tenant_id = (SELECT s.tenant_id FROM shops s WHERE s.id = t.shop_id) WHERE (t.tenant_id IS NULL OR t.tenant_id = 0) AND EXISTS (SELECT 1 FROM shops s WHERE s.id = t.shop_id)`},
		{"collect_tasks", `UPDATE collect_tasks t SET tenant_id = p.tenant_id FROM products p WHERE t.result_product_id = p.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`, `UPDATE collect_tasks AS t SET tenant_id = (SELECT p.tenant_id FROM products p WHERE p.id = t.result_product_id) WHERE (t.tenant_id IS NULL OR t.tenant_id = 0) AND EXISTS (SELECT 1 FROM products p WHERE p.id = t.result_product_id)`},
		{"douyin_image_assets", `UPDATE douyin_image_assets t SET tenant_id = s.tenant_id FROM shops s WHERE t.shop_id = s.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`, `UPDATE douyin_image_assets AS t SET tenant_id = (SELECT s.tenant_id FROM shops s WHERE s.id = t.shop_id) WHERE (t.tenant_id IS NULL OR t.tenant_id = 0) AND EXISTS (SELECT 1 FROM shops s WHERE s.id = t.shop_id)`},
	}
}

func runP42BackfillStatements(db *gorm.DB, statements []p42BackfillStatement) error {
	if db == nil {
		return fmt.Errorf("p4.2 backfill: db is nil")
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt.postgresSQL).Error; err == nil {
			continue
		} else if db.Dialector.Name() != "sqlite" || !isSQLiteUpdateFromUnsupported(err) {
			return fmt.Errorf("p4.2 backfill %s: %w", stmt.name, err)
		}
		if err := db.Exec(stmt.sqliteSQL).Error; err != nil {
			return fmt.Errorf("p4.2 sqlite backfill %s: %w", stmt.name, err)
		}
	}
	return nil
}

func isSQLiteUpdateFromUnsupported(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, `near "from": syntax error`) ||
		strings.Contains(message, "near 'from': syntax error") ||
		strings.Contains(message, `near "t": syntax error`) ||
		strings.Contains(message, "near 't': syntax error") ||
		strings.Contains(message, `near "b": syntax error`) ||
		strings.Contains(message, "near 'b': syntax error")
}

func migrateP42Indexes(db *gorm.DB) error {
	type idx struct {
		table string
		name  string
		sql   string
	}
	indexes := []idx{
		{"inventory_sync_tasks", "idx_inv_sync_tenant_shop", "CREATE INDEX IF NOT EXISTS idx_inv_sync_tenant_shop ON inventory_sync_tasks (tenant_id, shop_id)"},
		{"order_sync_tasks", "idx_order_sync_tenant_shop", "CREATE INDEX IF NOT EXISTS idx_order_sync_tenant_shop ON order_sync_tasks (tenant_id, shop_id)"},
		{"product_publish_tasks", "idx_publish_tenant_shop", "CREATE INDEX IF NOT EXISTS idx_publish_tenant_shop ON product_publish_tasks (tenant_id, shop_id)"},
		{"ai_product_text_batches", "idx_ai_text_tenant", "CREATE INDEX IF NOT EXISTS idx_ai_text_tenant ON ai_product_text_batches (tenant_id, created_at)"},
		{"ai_product_text_batches", "idx_ai_text_tenant_idempotency", "CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_text_tenant_idempotency ON ai_product_text_batches (tenant_id, idempotency_key)"},
		{"ai_product_image_batches", "idx_ai_image_tenant", "CREATE INDEX IF NOT EXISTS idx_ai_image_tenant ON ai_product_image_batches (tenant_id, created_at)"},
		{"ai_product_image_batches", "idx_ai_image_tenant_idempotency", "CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_image_tenant_idempotency ON ai_product_image_batches (tenant_id, idempotency_key)"},
		{"ai_operation_batches", "idx_ai_operation_batches_tenant", "CREATE INDEX IF NOT EXISTS idx_ai_operation_batches_tenant ON ai_operation_batches (tenant_id, created_at)"},
		{"export_jobs", "idx_export_jobs_tenant", "CREATE INDEX IF NOT EXISTS idx_export_jobs_tenant ON export_jobs (tenant_id, created_at)"},
		{"files", "idx_files_tenant_security", "CREATE INDEX IF NOT EXISTS idx_files_tenant_security ON files (tenant_id, security_status)"},
		{"task_failure_marks", "idx_task_failure_tenant", "CREATE INDEX IF NOT EXISTS idx_task_failure_tenant ON task_failure_marks (tenant_id, task_type)"},
		{"douyin_image_assets", "idx_douyin_img_tenant_shop", "CREATE INDEX IF NOT EXISTS idx_douyin_img_tenant_shop ON douyin_image_assets (tenant_id, shop_id)"},
	}
	// Create the versioned replacement before removing historical indexes so a
	// failed replacement never leaves inventory idempotency unprotected.
	if db.Migrator().HasTable(&inventory.InventoryChangeLog{}) {
		if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_inv_change_tenant_event_v2 ON inventory_change_logs (tenant_id, business_event_key) WHERE business_event_key <> ''").Error; err != nil {
			return fmt.Errorf("p4.2 index idx_inv_change_tenant_event_v2: %w", err)
		}
		for _, name := range []string{"idx_inv_change_tenant_event", "idx_inventory_change_logs_business_event_key"} {
			if db.Migrator().HasIndex(&inventory.InventoryChangeLog{}, name) {
				if err := db.Migrator().DropIndex(&inventory.InventoryChangeLog{}, name); err != nil {
					return fmt.Errorf("p4.2 drop legacy inventory event index %s: %w", name, err)
				}
			}
		}
	}
	for _, i := range indexes {
		if !db.Migrator().HasTable(i.table) {
			continue
		}
		if err := db.Exec(i.sql).Error; err != nil {
			return fmt.Errorf("p4.2 index %s: %w", i.name, err)
		}
	}
	for _, legacy := range []struct {
		model any
		name  string
	}{
		{&aiproducttext.AIProductTextBatch{}, "idx_ai_product_text_batches_idempotency_key"},
		{&aiproductimage.AIProductImageBatch{}, "idx_ai_product_image_batches_idempotency_key"},
	} {
		if db.Migrator().HasTable(legacy.model) && db.Migrator().HasIndex(legacy.model, legacy.name) {
			if err := db.Migrator().DropIndex(legacy.model, legacy.name); err != nil {
				return fmt.Errorf("p4.2 drop global AI batch idempotency index %s: %w", legacy.name, err)
			}
		}
	}
	return nil
}
