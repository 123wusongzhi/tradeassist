package operationtask

import (
	"fmt"

	"gorm.io/gorm"
)

func Migrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("operationtask migrate: db is nil")
	}
	if err := db.AutoMigrate(&OperationTask{}, &PlatformDraft{}); err != nil {
		return err
	}
	if err := migrateIndexes(db); err != nil {
		return err
	}
	return migrateConstraints(db)
}

func migrateIndexes(db *gorm.DB) error {
	stmts := []string{
		`CREATE INDEX IF NOT EXISTS idx_operation_tasks_tenant_status_updated ON operation_tasks (tenant_id, status, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_operation_tasks_tenant_platform_status_updated ON operation_tasks (tenant_id, platform, status, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_operation_tasks_tenant_task_type_created ON operation_tasks (tenant_id, task_type, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_operation_tasks_tenant_source ON operation_tasks (tenant_id, source_type, source_reference)`,
		`CREATE INDEX IF NOT EXISTS idx_platform_drafts_task_version ON platform_drafts (tenant_id, operation_task_id, draft_version DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_platform_drafts_tenant_status_updated ON platform_drafts (tenant_id, status, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_platform_drafts_tenant_platform_status ON platform_drafts (tenant_id, platform, status)`,
	}
	switch db.Dialector.Name() {
	case "postgres", "sqlite":
		stmts = append(stmts,
			`CREATE UNIQUE INDEX IF NOT EXISTS ux_operation_tasks_tenant_idempotency_key ON operation_tasks (tenant_id, idempotency_key) WHERE idempotency_key IS NOT NULL AND idempotency_key <> ''`,
		)
	default:
		stmts = append(stmts,
			`CREATE UNIQUE INDEX IF NOT EXISTS ux_operation_tasks_tenant_idempotency_key ON operation_tasks (tenant_id, idempotency_key)`,
		)
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateConstraints(db *gorm.DB) error {
	if db.Dialector.Name() != "postgres" {
		return nil
	}
	stmts := []string{
		`DO $$ BEGIN
			ALTER TABLE operation_tasks ADD CONSTRAINT chk_operation_tasks_revision CHECK (revision >= 1);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE platform_drafts ADD CONSTRAINT chk_platform_drafts_version CHECK (draft_version >= 1);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE platform_drafts ADD CONSTRAINT chk_platform_drafts_adapter_mode CHECK (adapter_mode IN ('mock','sandbox','local_draft_only'));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE platform_drafts ADD CONSTRAINT chk_platform_drafts_payload_hash CHECK (payload_hash = lower(payload_hash) AND payload_hash ~ '^[0-9a-f]{64}$');
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}
