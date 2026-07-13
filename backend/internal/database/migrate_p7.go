package database

import (
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/modules/performance"
	"gorm.io/gorm"
)

func migrateP7Performance(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("p7 migrate: db is nil")
	}
	if err := db.AutoMigrate(
		&performance.TestRun{},
		&performance.Regression{},
		&performance.CapacitySnapshot{},
		&performance.RateLimitPolicy{},
		&performance.QuotaPolicy{},
	); err != nil {
		return err
	}
	if db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return nil
	}
	for _, stmt := range []string{
		`CREATE INDEX IF NOT EXISTS idx_products_p7_tenant_created_id ON products (tenant_id, created_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_p7_tenant_created_id ON orders (tenant_id, created_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_p7_tenant_shop_created_id ON orders (tenant_id, shop_id, created_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_inventory_sync_tasks_p7_tenant_status_updated ON inventory_sync_tasks (tenant_id, status, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_collect_tasks_p7_tenant_updated_id ON collect_tasks (tenant_id, updated_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_webhook_events_p7_tenant_status_created ON webhook_events (tenant_id, status, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_operation_logs_p7_tenant_created_id ON operation_logs (tenant_id, created_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_files_p7_tenant_security_created ON files (tenant_id, security_status, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_backup_jobs_p7_status_created ON backup_jobs (status, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_release_runs_p7_environment_state ON release_runs (environment, state)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}
