package database

import (
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/securitymod"
	"gorm.io/gorm"
)

// migrateP41Security applies Phase P4.1 tenant enforcement, key rotation and file security schema.
func migrateP41Security(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate p4.1: db is nil")
	}
	if err := db.AutoMigrate(
		&securitymod.KeyRotationJob{},
		&securitymod.KeyRotationItemFailure{},
		&files.FileRecord{},
	); err != nil {
		return err
	}
	return migrateP41Indexes(db)
}

func migrateP41Indexes(db *gorm.DB) error {
	type idx struct {
		table string
		name  string
		sql   string
	}
	indexes := []idx{
		{"files", "idx_files_tenant_created", "CREATE INDEX IF NOT EXISTS idx_files_tenant_created ON files (tenant_id, created_at)"},
		{"files", "idx_files_security_status", "CREATE INDEX IF NOT EXISTS idx_files_security_status ON files (security_status)"},
		{"key_rotation_jobs", "idx_key_rotation_status", "CREATE INDEX IF NOT EXISTS idx_key_rotation_status ON key_rotation_jobs (status, created_at)"},
		{"key_rotation_item_failures", "idx_key_rotation_failures_rotation", "CREATE INDEX IF NOT EXISTS idx_key_rotation_failures_rotation ON key_rotation_item_failures (rotation_id)"},
	}
	for _, i := range indexes {
		if !db.Migrator().HasTable(i.table) {
			continue
		}
		if err := db.Exec(i.sql).Error; err != nil {
			return fmt.Errorf("p4.1 index %s: %w", i.name, err)
		}
	}
	return nil
}
