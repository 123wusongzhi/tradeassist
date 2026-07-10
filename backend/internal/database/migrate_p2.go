package database

import (
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/webhook"
	"gorm.io/gorm"
)

// migrateP2Reliability adds P2 idempotency, webhook, and constraint indexes.
func migrateP2Reliability(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate p2: db is nil")
	}
	if err := db.AutoMigrate(
		&idempotency.Record{},
		&webhook.Event{},
	); err != nil {
		return err
	}
	if err := migrateP2Indexes(db); err != nil {
		return err
	}
	return nil
}

func migrateP2Indexes(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	driver := ""
	if db.Dialector != nil {
		driver = db.Dialector.Name()
	}
	if driver == "postgres" {
		stmts := []string{
			`CREATE UNIQUE INDEX IF NOT EXISTS ux_customer_msg_client_id ON customer_messages (conversation_id, client_message_id) WHERE client_message_id IS NOT NULL AND client_message_id <> ''`,
			`CREATE UNIQUE INDEX IF NOT EXISTS ux_webhook_platform_event ON webhook_events (platform, event_id) WHERE deleted_at IS NULL`,
			`CREATE INDEX IF NOT EXISTS ix_idempotency_status ON idempotency_records (status)`,
			`CREATE INDEX IF NOT EXISTS ix_idempotency_locked_until ON idempotency_records (locked_until)`,
		}
		for _, sql := range stmts {
			if err := db.Exec(sql).Error; err != nil {
				return fmt.Errorf("p2 index: %w", err)
			}
		}
	}
	return nil
}
