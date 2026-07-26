package operationtask

import (
	"fmt"

	"gorm.io/gorm"
)

func Migrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("operationtask migrate: db is nil")
	}
	if err := db.AutoMigrate(&OperationTask{}, &PlatformDraft{}, &ApprovalRecord{}, &ExecutionAttempt{}, &ExecutionError{}, &OperationTaskEvent{}); err != nil {
		return err
	}
	if err := migrateIndexes(db); err != nil {
		return err
	}
	if err := migrateConstraints(db); err != nil {
		return err
	}
	return migrateImmutableGuards(db)
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
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_operation_tasks_tenant_id ON operation_tasks (tenant_id, id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_platform_drafts_tenant_id ON platform_drafts (tenant_id, id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_platform_drafts_tenant_task_id ON platform_drafts (tenant_id, operation_task_id, id)`,
		`CREATE INDEX IF NOT EXISTS idx_approval_records_task_created ON approval_records (tenant_id, operation_task_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_approval_records_draft_created ON approval_records (tenant_id, platform_draft_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_approval_records_task_decision_created ON approval_records (tenant_id, operation_task_id, decision, created_at DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_approval_records_tenant_id ON approval_records (tenant_id, id)`,
		`CREATE INDEX IF NOT EXISTS idx_execution_attempts_task_attempt ON execution_attempts (tenant_id, operation_task_id, attempt_number ASC)`,
		`CREATE INDEX IF NOT EXISTS idx_execution_attempts_task_created ON execution_attempts (tenant_id, operation_task_id, created_at DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_execution_attempts_tenant_id ON execution_attempts (tenant_id, id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_execution_attempts_task_attempt ON execution_attempts (tenant_id, operation_task_id, attempt_number)`,
		`CREATE INDEX IF NOT EXISTS idx_execution_errors_attempt_sequence ON execution_errors (tenant_id, execution_attempt_id, sequence ASC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_execution_errors_attempt_sequence ON execution_errors (tenant_id, execution_attempt_id, sequence)`,
		`CREATE INDEX IF NOT EXISTS idx_operation_task_events_task_sequence ON operation_task_events (tenant_id, operation_task_id, sequence ASC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_operation_task_events_task_sequence ON operation_task_events (tenant_id, operation_task_id, sequence)`,
	}
	switch db.Dialector.Name() {
	case "postgres", "sqlite":
		stmts = append(stmts,
			`CREATE UNIQUE INDEX IF NOT EXISTS ux_operation_tasks_tenant_idempotency_key ON operation_tasks (tenant_id, idempotency_key) WHERE idempotency_key IS NOT NULL AND idempotency_key <> ''`,
			`CREATE UNIQUE INDEX IF NOT EXISTS ux_approval_records_task_idempotency ON approval_records (tenant_id, operation_task_id, idempotency_key) WHERE idempotency_key IS NOT NULL AND idempotency_key <> ''`,
			`CREATE UNIQUE INDEX IF NOT EXISTS ux_execution_attempts_task_idempotency ON execution_attempts (tenant_id, operation_task_id, idempotency_key) WHERE idempotency_key IS NOT NULL AND idempotency_key <> ''`,
		)
	default:
		stmts = append(stmts,
			`CREATE UNIQUE INDEX IF NOT EXISTS ux_operation_tasks_tenant_idempotency_key ON operation_tasks (tenant_id, idempotency_key)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS ux_approval_records_task_idempotency ON approval_records (tenant_id, operation_task_id, idempotency_key)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS ux_execution_attempts_task_idempotency ON execution_attempts (tenant_id, operation_task_id, idempotency_key)`,
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
		`DO $$ BEGIN
			ALTER TABLE approval_records ADD CONSTRAINT chk_approval_records_decision CHECK (decision IN ('approved','rejected'));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE approval_records ADD CONSTRAINT chk_approval_records_draft_version CHECK (draft_version >= 1);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE approval_records ADD CONSTRAINT chk_approval_records_payload_hash CHECK (draft_payload_hash = lower(draft_payload_hash) AND draft_payload_hash ~ '^[0-9a-f]{64}$');
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE execution_attempts ADD CONSTRAINT chk_execution_attempts_status CHECK (status IN ('queued','running','succeeded','failed','cancelled'));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE execution_attempts ADD CONSTRAINT chk_execution_attempts_adapter_mode CHECK (adapter_mode IN ('mock','sandbox','local_draft_only'));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE execution_attempts ADD CONSTRAINT chk_execution_attempts_hashes CHECK (
				approved_draft_payload_hash = lower(approved_draft_payload_hash)
				AND approved_draft_payload_hash ~ '^[0-9a-f]{64}$'
				AND executed_draft_payload_hash = lower(executed_draft_payload_hash)
				AND executed_draft_payload_hash ~ '^[0-9a-f]{64}$'
			);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE execution_errors ADD CONSTRAINT chk_execution_errors_category CHECK (category IN ('validation_error','permission_denied','state_conflict','adapter_unavailable','provider_timeout','provider_rejected','idempotency_conflict','internal_error'));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE operation_task_events ADD CONSTRAINT chk_operation_task_events_type CHECK (event_type IN ('task_created','draft_generated','draft_updated','review_requested','approved','rejected','execution_queued','execution_started','draft_written','execution_failed','retry_requested','cancelled'));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE operation_task_events ADD CONSTRAINT chk_operation_task_events_actor CHECK (actor_type IN ('user','system','ai','rule') AND (actor_type <> 'user' OR actor_id IS NOT NULL));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateImmutableGuards(db *gorm.DB) error {
	switch db.Dialector.Name() {
	case "sqlite":
		return migrateSQLiteImmutableGuards(db)
	case "postgres":
		return migratePostgresImmutableGuards(db)
	default:
		return nil
	}
}

func migrateSQLiteImmutableGuards(db *gorm.DB) error {
	stmts := []string{
		`CREATE TRIGGER IF NOT EXISTS trg_approval_records_no_update BEFORE UPDATE ON approval_records BEGIN SELECT RAISE(ABORT, 'immutable_record'); END;`,
		`CREATE TRIGGER IF NOT EXISTS trg_approval_records_no_delete BEFORE DELETE ON approval_records BEGIN SELECT RAISE(ABORT, 'immutable_record'); END;`,
		`CREATE TRIGGER IF NOT EXISTS trg_execution_errors_no_update BEFORE UPDATE ON execution_errors BEGIN SELECT RAISE(ABORT, 'immutable_record'); END;`,
		`CREATE TRIGGER IF NOT EXISTS trg_execution_errors_no_delete BEFORE DELETE ON execution_errors BEGIN SELECT RAISE(ABORT, 'immutable_record'); END;`,
		`CREATE TRIGGER IF NOT EXISTS trg_operation_task_events_no_update BEFORE UPDATE ON operation_task_events BEGIN SELECT RAISE(ABORT, 'immutable_record'); END;`,
		`CREATE TRIGGER IF NOT EXISTS trg_operation_task_events_no_delete BEFORE DELETE ON operation_task_events BEGIN SELECT RAISE(ABORT, 'immutable_record'); END;`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

func migratePostgresImmutableGuards(db *gorm.DB) error {
	stmts := []string{
		`CREATE OR REPLACE FUNCTION operationtask_reject_immutable_change() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'immutable_record';
		END;
		$$ LANGUAGE plpgsql;`,
		`DO $$ BEGIN
			CREATE TRIGGER trg_approval_records_no_update BEFORE UPDATE ON approval_records FOR EACH ROW EXECUTE FUNCTION operationtask_reject_immutable_change();
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			CREATE TRIGGER trg_approval_records_no_delete BEFORE DELETE ON approval_records FOR EACH ROW EXECUTE FUNCTION operationtask_reject_immutable_change();
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			CREATE TRIGGER trg_execution_errors_no_update BEFORE UPDATE ON execution_errors FOR EACH ROW EXECUTE FUNCTION operationtask_reject_immutable_change();
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			CREATE TRIGGER trg_execution_errors_no_delete BEFORE DELETE ON execution_errors FOR EACH ROW EXECUTE FUNCTION operationtask_reject_immutable_change();
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			CREATE TRIGGER trg_operation_task_events_no_update BEFORE UPDATE ON operation_task_events FOR EACH ROW EXECUTE FUNCTION operationtask_reject_immutable_change();
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			CREATE TRIGGER trg_operation_task_events_no_delete BEFORE DELETE ON operation_task_events FOR EACH ROW EXECUTE FUNCTION operationtask_reject_immutable_change();
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}
