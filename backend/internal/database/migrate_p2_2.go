package database

import (
	"fmt"

	"gorm.io/gorm"
)

// migrateP22Reliability adds P2.2 lease indexes for collect/image/customer_sync and webhook payload hash.
func migrateP22Reliability(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate p22: db is nil")
	}
	return migrateP22Indexes(db)
}

func migrateP22Indexes(db *gorm.DB) error {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return nil
	}
	stmts := []string{
		`CREATE INDEX IF NOT EXISTS ix_collect_execution_id ON collect_tasks (execution_id)`,
		`CREATE INDEX IF NOT EXISTS ix_collect_heartbeat_at ON collect_tasks (heartbeat_at)`,
		`CREATE INDEX IF NOT EXISTS ix_image_execution_id ON image_tasks (execution_id)`,
		`CREATE INDEX IF NOT EXISTS ix_image_heartbeat_at ON image_tasks (heartbeat_at)`,
		`CREATE INDEX IF NOT EXISTS ix_customer_sync_execution_id ON customer_message_sync_tasks (execution_id)`,
		`CREATE INDEX IF NOT EXISTS ix_customer_sync_heartbeat_at ON customer_message_sync_tasks (heartbeat_at)`,
		`CREATE INDEX IF NOT EXISTS ix_webhook_payload_hash ON webhook_events (platform, payload_hash)`,
	}
	for _, sql := range stmts {
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("p22 index: %w", err)
		}
	}
	return nil
}
