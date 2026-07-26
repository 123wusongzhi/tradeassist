package database

import (
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/modules/webhook"
	"gorm.io/gorm"
)

// migrateP32Webhook adds multi-shop webhook routing schema and replaces the
// earlier platform-only event uniqueness with a tenant/shop scoped key.
func migrateP32Webhook(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate p3.2 webhook: db is nil")
	}
	if err := db.AutoMigrate(&webhook.Event{}); err != nil {
		return err
	}
	if db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return nil
	}
	stmts := []string{
		`DROP INDEX IF EXISTS ux_webhook_platform_event`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_webhook_shop_event ON webhook_events (platform, tenant_id, platform_shop_id, event_id) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS ix_webhook_shop_scope ON webhook_events (platform, tenant_id, platform_shop_id)`,
		`CREATE INDEX IF NOT EXISTS ix_webhook_binding ON webhook_events (binding_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_shops_platform_external_active ON shops (platform, external_shop_id) WHERE deleted_at IS NULL AND external_shop_id IS NOT NULL AND external_shop_id <> ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_shops_tenant_platform_external_active ON shops (tenant_id, platform, external_shop_id) WHERE deleted_at IS NULL AND external_shop_id IS NOT NULL AND external_shop_id <> ''`,
		`CREATE INDEX IF NOT EXISTS ix_shop_auth_tokens_platform_app_key ON shop_auth_tokens (platform, app_key)`,
	}
	for _, sql := range stmts {
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("p3.2 webhook index: %w", err)
		}
	}
	return nil
}
