package database

import (
	"fmt"

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
	stmts := []string{
		`UPDATE inventory_sync_tasks t SET tenant_id = s.tenant_id FROM shops s WHERE t.shop_id = s.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
		`UPDATE inventory_sync_batches t SET tenant_id = s.tenant_id FROM shops s WHERE t.shop_id = s.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
		`UPDATE inventory_change_logs t SET tenant_id = p.tenant_id FROM products p WHERE t.product_id = p.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
		`UPDATE order_sync_tasks t SET tenant_id = s.tenant_id FROM shops s WHERE t.shop_id = s.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
		`UPDATE customer_message_sync_tasks t SET tenant_id = s.tenant_id FROM shops s WHERE t.shop_id = s.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
		`UPDATE product_publish_tasks t SET tenant_id = s.tenant_id FROM shops s WHERE t.shop_id = s.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
		`UPDATE ai_product_text_batches t SET tenant_id = p.tenant_id FROM ai_product_text_items i JOIN products p ON i.product_id = p.id WHERE i.batch_id = t.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
		`UPDATE ai_product_image_batches t SET tenant_id = p.tenant_id FROM ai_product_image_items i JOIN products p ON i.product_id = p.id WHERE i.batch_id = t.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
		`UPDATE customer_conversations t SET tenant_id = s.tenant_id FROM shops s WHERE t.shop_id = s.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
		`UPDATE collect_tasks t SET tenant_id = p.tenant_id FROM products p WHERE t.result_product_id = p.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
		`UPDATE douyin_image_assets t SET tenant_id = s.tenant_id FROM shops s WHERE t.shop_id = s.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
	}
	for _, sql := range stmts {
		if err := db.Exec(sql).Error; err != nil {
			// SQLite dev may not support UPDATE FROM; skip non-fatal.
			_ = err
		}
	}
	return nil
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
		{"ai_product_image_batches", "idx_ai_image_tenant", "CREATE INDEX IF NOT EXISTS idx_ai_image_tenant ON ai_product_image_batches (tenant_id, created_at)"},
		{"export_jobs", "idx_export_jobs_tenant", "CREATE INDEX IF NOT EXISTS idx_export_jobs_tenant ON export_jobs (tenant_id, created_at)"},
		{"files", "idx_files_tenant_security", "CREATE INDEX IF NOT EXISTS idx_files_tenant_security ON files (tenant_id, security_status)"},
		{"task_failure_marks", "idx_task_failure_tenant", "CREATE INDEX IF NOT EXISTS idx_task_failure_tenant ON task_failure_marks (tenant_id, task_type)"},
		{"douyin_image_assets", "idx_douyin_img_tenant_shop", "CREATE INDEX IF NOT EXISTS idx_douyin_img_tenant_shop ON douyin_image_assets (tenant_id, shop_id)"},
	}
	for _, i := range indexes {
		if !db.Migrator().HasTable(i.table) {
			continue
		}
		if err := db.Exec(i.sql).Error; err != nil {
			return fmt.Errorf("p4.2 index %s: %w", i.name, err)
		}
	}
	return nil
}
