package database

import (
	"fmt"

	"gorm.io/gorm"
)

// migrateP21Reliability adds P2.1 task lease columns and domain idempotency indexes.
func migrateP21Reliability(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate p21: db is nil")
	}
	return migrateP21Indexes(db)
}

func migrateP21Indexes(db *gorm.DB) error {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return nil
	}
	stmts := []string{
		// Task lease columns (idempotent ADD COLUMN IF NOT EXISTS)
		`ALTER TABLE order_sync_tasks ADD COLUMN IF NOT EXISTS heartbeat_at TIMESTAMPTZ`,
		`ALTER TABLE order_sync_tasks ADD COLUMN IF NOT EXISTS execution_id VARCHAR(36)`,
		`ALTER TABLE inventory_sync_tasks ADD COLUMN IF NOT EXISTS heartbeat_at TIMESTAMPTZ`,
		`ALTER TABLE inventory_sync_tasks ADD COLUMN IF NOT EXISTS execution_id VARCHAR(36)`,
		`ALTER TABLE product_publish_tasks ADD COLUMN IF NOT EXISTS heartbeat_at TIMESTAMPTZ`,
		`ALTER TABLE product_publish_tasks ADD COLUMN IF NOT EXISTS execution_id VARCHAR(36)`,
		`ALTER TABLE collect_tasks ADD COLUMN IF NOT EXISTS heartbeat_at TIMESTAMPTZ`,
		`ALTER TABLE collect_tasks ADD COLUMN IF NOT EXISTS execution_id VARCHAR(36)`,
		`ALTER TABLE image_tasks ADD COLUMN IF NOT EXISTS heartbeat_at TIMESTAMPTZ`,
		`ALTER TABLE image_tasks ADD COLUMN IF NOT EXISTS execution_id VARCHAR(36)`,
		`ALTER TABLE customer_message_sync_tasks ADD COLUMN IF NOT EXISTS heartbeat_at TIMESTAMPTZ`,
		`ALTER TABLE customer_message_sync_tasks ADD COLUMN IF NOT EXISTS execution_id VARCHAR(36)`,
		// Indexes
		`CREATE INDEX IF NOT EXISTS ix_order_sync_status_next ON order_sync_tasks (status)`,
		`CREATE INDEX IF NOT EXISTS ix_order_sync_status_locked_until ON order_sync_tasks (status, locked_until)`,
		`CREATE INDEX IF NOT EXISTS ix_order_sync_execution_id ON order_sync_tasks (execution_id)`,
		`CREATE INDEX IF NOT EXISTS ix_order_sync_heartbeat_at ON order_sync_tasks (heartbeat_at)`,
		`CREATE INDEX IF NOT EXISTS ix_inventory_sync_status_locked_until ON inventory_sync_tasks (status, locked_until)`,
		`CREATE INDEX IF NOT EXISTS ix_inventory_sync_execution_id ON inventory_sync_tasks (execution_id)`,
		`CREATE INDEX IF NOT EXISTS ix_product_publish_status_locked_until ON product_publish_tasks (status, locked_until)`,
		`CREATE INDEX IF NOT EXISTS ix_product_publish_execution_id ON product_publish_tasks (execution_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_inventory_change_business_event_key ON inventory_change_logs (business_event_key) WHERE business_event_key IS NOT NULL AND business_event_key <> ''`,
	}
	for _, sql := range stmts {
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("p21 index: %w", err)
		}
	}
	return nil
}
