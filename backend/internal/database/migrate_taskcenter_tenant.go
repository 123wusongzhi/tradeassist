package database

import (
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"gorm.io/gorm"
)

// migrateTaskcenterTenantScope assigns ownership only when the task alert/mark
// has one direct, trusted parent relationship. Rows without such a relationship
// intentionally stay in tenant 0; guessing here would expose tenant data.
func migrateTaskcenterTenantScope(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate taskcenter tenant scope: nil db")
	}
	if err := db.AutoMigrate(&taskcenter.TaskFailureMark{}, &taskcenter.TaskAlert{}, &taskcenter.TaskAlertNotification{}); err != nil {
		return fmt.Errorf("migrate taskcenter tenant schema: %w", err)
	}
	if db.Migrator().HasIndex(&taskcenter.TaskAlert{}, "uq_task_alert_type_src_cat") {
		if err := db.Migrator().DropIndex(&taskcenter.TaskAlert{}, "uq_task_alert_type_src_cat"); err != nil {
			return fmt.Errorf("drop legacy task alert uniqueness: %w", err)
		}
	}
	if db.Migrator().HasIndex(&taskcenter.TaskFailureMark{}, "uniq_task_failure_mark") {
		if err := db.Migrator().DropIndex(&taskcenter.TaskFailureMark{}, "uniq_task_failure_mark"); err != nil {
			return fmt.Errorf("drop legacy task mark uniqueness: %w", err)
		}
	}
	if err := db.AutoMigrate(&taskcenter.TaskAlert{}); err != nil {
		return fmt.Errorf("create tenant task alert uniqueness: %w", err)
	}
	if !db.Migrator().HasTable("task_alerts") {
		return nil
	}
	mappings := []struct{ taskType, table string }{
		{"collect", "collect_tasks"}, {"image", "image_tasks"}, {"order_sync", "order_sync_tasks"},
		{"customer_message_sync", "customer_message_sync_tasks"}, {"product_publish", "product_publish_tasks"},
		{"inventory_sync", "inventory_sync_tasks"},
	}
	for _, m := range mappings {
		if !db.Migrator().HasTable(m.table) {
			continue
		}
		q := fmt.Sprintf(`UPDATE task_alerts SET tenant_id = (SELECT tenant_id FROM %s p WHERE CAST(p.id AS TEXT) = task_alerts.source_id) WHERE tenant_id = 0 AND task_type = ? AND EXISTS (SELECT 1 FROM %s p WHERE CAST(p.id AS TEXT) = task_alerts.source_id)`, m.table, m.table)
		if err := db.Exec(q, m.taskType).Error; err != nil {
			return fmt.Errorf("backfill task alerts from %s: %w", m.table, err)
		}
		q = fmt.Sprintf(`UPDATE task_failure_marks SET tenant_id = (SELECT tenant_id FROM %s p WHERE CAST(p.id AS TEXT) = task_failure_marks.source_id) WHERE tenant_id = 0 AND task_type = ? AND EXISTS (SELECT 1 FROM %s p WHERE CAST(p.id AS TEXT) = task_failure_marks.source_id)`, m.table, m.table)
		if db.Migrator().HasTable("task_failure_marks") {
			if err := db.Exec(q, m.taskType).Error; err != nil {
				return fmt.Errorf("backfill task marks from %s: %w", m.table, err)
			}
		}
	}
	// AI task-center source IDs reference item IDs. Item rows deliberately do not
	// carry tenant_id, so ownership must come from their parent batch. Require a
	// non-zero batch tenant to keep mixed, unknown, and unbackfilled batches
	// fail-closed.
	aiMappings := []struct {
		taskType, itemTable, batchTable string
	}{
		{"ai_text", "ai_product_text_items", "ai_product_text_batches"},
		{"ai_image", "ai_product_image_items", "ai_product_image_batches"},
	}
	for _, m := range aiMappings {
		if !db.Migrator().HasTable(m.itemTable) || !db.Migrator().HasTable(m.batchTable) {
			continue
		}
		q := fmt.Sprintf(`UPDATE task_alerts SET tenant_id = (SELECT b.tenant_id FROM %s i JOIN %s b ON b.id = i.batch_id WHERE CAST(i.id AS TEXT) = task_alerts.source_id AND b.tenant_id <> 0) WHERE tenant_id = 0 AND task_type = ? AND EXISTS (SELECT 1 FROM %s i JOIN %s b ON b.id = i.batch_id WHERE CAST(i.id AS TEXT) = task_alerts.source_id AND b.tenant_id <> 0)`, m.itemTable, m.batchTable, m.itemTable, m.batchTable)
		if err := db.Exec(q, m.taskType).Error; err != nil {
			return fmt.Errorf("backfill task alerts from %s batch: %w", m.itemTable, err)
		}
		if db.Migrator().HasTable("task_failure_marks") {
			q = fmt.Sprintf(`UPDATE task_failure_marks SET tenant_id = (SELECT b.tenant_id FROM %s i JOIN %s b ON b.id = i.batch_id WHERE CAST(i.id AS TEXT) = task_failure_marks.source_id AND b.tenant_id <> 0) WHERE tenant_id = 0 AND task_type = ? AND EXISTS (SELECT 1 FROM %s i JOIN %s b ON b.id = i.batch_id WHERE CAST(i.id AS TEXT) = task_failure_marks.source_id AND b.tenant_id <> 0)`, m.itemTable, m.batchTable, m.itemTable, m.batchTable)
			if err := db.Exec(q, m.taskType).Error; err != nil {
				return fmt.Errorf("backfill task marks from %s batch: %w", m.itemTable, err)
			}
		}
	}
	if db.Migrator().HasTable("task_alert_notifications") {
		if err := db.Exec(`UPDATE task_alert_notifications SET tenant_id = (SELECT tenant_id FROM task_alerts a WHERE a.id = task_alert_notifications.alert_id) WHERE tenant_id = 0 AND EXISTS (SELECT 1 FROM task_alerts a WHERE a.id = task_alert_notifications.alert_id)`).Error; err != nil {
			return fmt.Errorf("backfill task alert notifications: %w", err)
		}
	}
	return nil
}
