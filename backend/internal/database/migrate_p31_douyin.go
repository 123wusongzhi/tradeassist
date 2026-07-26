package database

import (
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"gorm.io/gorm"
)

// migrateP31Douyin closes P3.1 schema additions (non-destructive).
func migrateP31Douyin(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate p3.1 douyin: db is nil")
	}
	if err := db.AutoMigrate(&order.Order{}); err != nil {
		return fmt.Errorf("p3.1 order AutoMigrate: %w", err)
	}
	if err := migrateP31OrderIndexes(db); err != nil {
		return err
	}
	return nil
}

func migrateP31OrderIndexes(db *gorm.DB) error {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return nil
	}
	stmts := []string{
		`CREATE INDEX IF NOT EXISTS ix_orders_platform_updated_at ON orders (platform_updated_at)`,
		`CREATE INDEX IF NOT EXISTS ix_orders_platform_revision ON orders (platform_revision)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("p3.1 index: %w", err)
		}
	}
	return nil
}
