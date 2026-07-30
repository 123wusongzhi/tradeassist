package inventorysyncp9

import (
	"fmt"

	"gorm.io/gorm"
)

func Migrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("inventorysyncp9 migrate: db is nil")
	}
	if err := db.AutoMigrate(
		&InventorySyncRun{},
		&InventorySnapshotItem{},
		&SKUBinding{},
		&SKUBindingCalibration{},
		&ManualBindingRequest{},
		&ManualBindingDecision{},
	); err != nil {
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
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_p9_inventory_sync_runs_tenant_id ON p9_inventory_sync_runs (tenant_id, id)`,
		`CREATE INDEX IF NOT EXISTS idx_p9_inventory_sync_runs_tenant_shop_status ON p9_inventory_sync_runs (tenant_id, shop_connection_id, platform, status, updated_at DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_p9_inventory_snapshots_tenant_id ON p9_inventory_snapshot_items (tenant_id, id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_p9_inventory_snapshots_tenant_run_external_sku ON p9_inventory_snapshot_items (tenant_id, inventory_sync_run_id, external_sku_id)`,
		`CREATE INDEX IF NOT EXISTS idx_p9_inventory_snapshots_tenant_run ON p9_inventory_snapshot_items (tenant_id, inventory_sync_run_id, created_at ASC)`,
		`CREATE INDEX IF NOT EXISTS idx_p9_inventory_snapshots_tenant_shop ON p9_inventory_snapshot_items (tenant_id, shop_connection_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_p9_sku_bindings_tenant_id ON p9_sku_bindings (tenant_id, id)`,
		`CREATE INDEX IF NOT EXISTS idx_p9_sku_bindings_tenant_external ON p9_sku_bindings (tenant_id, shop_connection_id, external_sku_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_p9_sku_bindings_tenant_local ON p9_sku_bindings (tenant_id, local_product_id, local_sku_id, created_at DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_p9_sku_calibrations_tenant_id ON p9_sku_binding_calibrations (tenant_id, id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_p9_sku_calibrations_candidate_version ON p9_sku_binding_calibrations (tenant_id, inventory_sync_run_id, inventory_snapshot_item_id, candidate_local_sku_id, calibration_version)`,
		`CREATE INDEX IF NOT EXISTS idx_p9_sku_calibrations_tenant_run ON p9_sku_binding_calibrations (tenant_id, inventory_sync_run_id, created_at ASC)`,
		`CREATE INDEX IF NOT EXISTS idx_p9_sku_calibrations_snapshot ON p9_sku_binding_calibrations (tenant_id, inventory_snapshot_item_id, confidence DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_p9_manual_binding_requests_tenant_id ON p9_manual_binding_requests (tenant_id, id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_p9_manual_binding_requests_tenant_request_id ON p9_manual_binding_requests (tenant_id, request_id)`,
		`CREATE INDEX IF NOT EXISTS idx_p9_manual_binding_requests_run ON p9_manual_binding_requests (tenant_id, inventory_sync_run_id, created_at ASC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_p9_manual_binding_decisions_idempotency ON p9_manual_binding_decisions (tenant_id, manual_binding_request_id, operation, idempotency_key_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_p9_manual_binding_decisions_request ON p9_manual_binding_decisions (tenant_id, manual_binding_request_id, created_at ASC)`,
	}
	switch db.Dialector.Name() {
	case "postgres", "sqlite":
		stmts = append(stmts,
			`CREATE UNIQUE INDEX IF NOT EXISTS ux_p9_inventory_sync_runs_tenant_idempotency ON p9_inventory_sync_runs (tenant_id, idempotency_key_hash) WHERE idempotency_key_hash IS NOT NULL AND idempotency_key_hash <> ''`,
			`CREATE UNIQUE INDEX IF NOT EXISTS ux_p9_inventory_sync_runs_rerun_claim ON p9_inventory_sync_runs (tenant_id, rerun_of_run_id, rerun_source_revision) WHERE rerun_of_run_id IS NOT NULL AND rerun_source_revision > 0`,
			`CREATE UNIQUE INDEX IF NOT EXISTS ux_p9_sku_bindings_current_confirmed ON p9_sku_bindings (tenant_id, shop_connection_id, external_sku_id) WHERE binding_status = 'confirmed'`,
			`CREATE UNIQUE INDEX IF NOT EXISTS ux_p9_manual_binding_requests_pending ON p9_manual_binding_requests (tenant_id, shop_connection_id, external_sku_id) WHERE status = 'pending'`,
			`CREATE UNIQUE INDEX IF NOT EXISTS ux_p9_manual_binding_requests_tenant_idempotency ON p9_manual_binding_requests (tenant_id, idempotency_key_hash) WHERE idempotency_key_hash IS NOT NULL AND idempotency_key_hash <> ''`,
		)
	default:
		stmts = append(stmts,
			`CREATE UNIQUE INDEX IF NOT EXISTS ux_p9_inventory_sync_runs_tenant_idempotency ON p9_inventory_sync_runs (tenant_id, idempotency_key_hash)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS ux_p9_inventory_sync_runs_rerun_claim ON p9_inventory_sync_runs (tenant_id, rerun_of_run_id, rerun_source_revision)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS ux_p9_sku_bindings_current_confirmed ON p9_sku_bindings (tenant_id, shop_connection_id, external_sku_id, binding_status)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS ux_p9_manual_binding_requests_pending ON p9_manual_binding_requests (tenant_id, shop_connection_id, external_sku_id, status)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS ux_p9_manual_binding_requests_tenant_idempotency ON p9_manual_binding_requests (tenant_id, idempotency_key_hash)`,
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
			ALTER TABLE p9_inventory_sync_runs ADD CONSTRAINT chk_p9_inventory_sync_runs_revision CHECK (revision >= 1);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE p9_inventory_sync_runs ADD CONSTRAINT chk_p9_inventory_sync_runs_status CHECK (status IN ('pending','running','succeeded','failed','cancelled'));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE p9_inventory_sync_runs ADD CONSTRAINT chk_p9_inventory_sync_runs_provider_mode CHECK (provider_mode IN ('mock','sandbox','local_draft_only'));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE p9_inventory_sync_runs ADD CONSTRAINT chk_p9_inventory_sync_runs_counts CHECK (snapshot_count >= 0 AND calibration_count >= 0 AND manual_request_count >= 0);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE p9_inventory_sync_runs ADD CONSTRAINT chk_p9_inventory_sync_runs_time CHECK (finished_at IS NULL OR started_at IS NULL OR finished_at >= started_at);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE p9_inventory_sync_runs ADD CONSTRAINT chk_p9_inventory_sync_runs_hashes CHECK (
				(idempotency_key_hash = '' OR idempotency_key_hash = lower(idempotency_key_hash) AND idempotency_key_hash ~ '^[0-9a-f]{64}$')
				AND (input_fingerprint = '' OR input_fingerprint = lower(input_fingerprint) AND input_fingerprint ~ '^[0-9a-f]{64}$')
			);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE p9_inventory_snapshot_items ADD CONSTRAINT chk_p9_inventory_snapshot_items_quantities CHECK (available_quantity >= 0 AND reserved_quantity >= 0 AND total_quantity >= 0);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE p9_inventory_snapshot_items ADD CONSTRAINT chk_p9_inventory_snapshot_items_payload_hash CHECK (payload_hash = lower(payload_hash) AND payload_hash ~ '^[0-9a-f]{64}$');
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE p9_sku_bindings ADD CONSTRAINT chk_p9_sku_bindings_source CHECK (binding_source IN ('automatic','manual'));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE p9_sku_bindings ADD CONSTRAINT chk_p9_sku_bindings_status CHECK (binding_status IN ('proposed','confirmed','rejected','stale','conflict'));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE p9_sku_bindings ADD CONSTRAINT chk_p9_sku_bindings_revision_confidence CHECK (revision >= 1 AND calibration_version >= 1 AND confidence >= 0 AND confidence <= 10000);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE p9_sku_binding_calibrations ADD CONSTRAINT chk_p9_sku_binding_calibrations_strategy CHECK (match_strategy IN ('exact_sku_code','exact_barcode','normalized_sku_code','normalized_barcode','normalized_title_variant','composite_match','manual'));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE p9_sku_binding_calibrations ADD CONSTRAINT chk_p9_sku_binding_calibrations_status CHECK (status IN ('candidate','selected','rejected','conflict'));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE p9_sku_binding_calibrations ADD CONSTRAINT chk_p9_sku_binding_calibrations_confidence CHECK (confidence >= 0 AND confidence <= 10000 AND calibration_version >= 1);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE p9_sku_binding_calibrations ADD CONSTRAINT chk_p9_sku_binding_calibrations_fingerprint CHECK (input_fingerprint = lower(input_fingerprint) AND input_fingerprint ~ '^[0-9a-f]{64}$');
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE p9_manual_binding_requests ADD CONSTRAINT chk_p9_manual_binding_requests_status CHECK (status IN ('pending','confirmed','rejected','cancelled'));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE p9_manual_binding_requests ADD CONSTRAINT chk_p9_manual_binding_requests_revision_candidate CHECK (revision >= 1 AND candidate_count >= 0);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE p9_manual_binding_requests ADD CONSTRAINT chk_p9_manual_binding_requests_hashes CHECK (
				(input_fingerprint = lower(input_fingerprint) AND input_fingerprint ~ '^[0-9a-f]{64}$')
				AND (idempotency_key_hash = '' OR idempotency_key_hash = lower(idempotency_key_hash) AND idempotency_key_hash ~ '^[0-9a-f]{64}$')
			);
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
		`CREATE TRIGGER IF NOT EXISTS trg_p9_inventory_snapshot_items_no_update BEFORE UPDATE ON p9_inventory_snapshot_items BEGIN SELECT RAISE(ABORT, 'immutable_record'); END;`,
		`CREATE TRIGGER IF NOT EXISTS trg_p9_inventory_snapshot_items_no_delete BEFORE DELETE ON p9_inventory_snapshot_items BEGIN SELECT RAISE(ABORT, 'immutable_record'); END;`,
		`CREATE TRIGGER IF NOT EXISTS trg_p9_sku_binding_calibrations_no_update BEFORE UPDATE ON p9_sku_binding_calibrations BEGIN SELECT RAISE(ABORT, 'immutable_record'); END;`,
		`CREATE TRIGGER IF NOT EXISTS trg_p9_sku_binding_calibrations_no_delete BEFORE DELETE ON p9_sku_binding_calibrations BEGIN SELECT RAISE(ABORT, 'immutable_record'); END;`,
		`CREATE TRIGGER IF NOT EXISTS trg_p9_manual_binding_decisions_no_update BEFORE UPDATE ON p9_manual_binding_decisions BEGIN SELECT RAISE(ABORT, 'immutable_record'); END;`,
		`CREATE TRIGGER IF NOT EXISTS trg_p9_manual_binding_decisions_no_delete BEFORE DELETE ON p9_manual_binding_decisions BEGIN SELECT RAISE(ABORT, 'immutable_record'); END;`,
		`CREATE TRIGGER IF NOT EXISTS trg_p9_operation_logs_no_update BEFORE UPDATE ON operation_logs WHEN OLD.resource IN ('inventory_sync','sku_binding') BEGIN SELECT RAISE(ABORT, 'immutable_record'); END;`,
		`CREATE TRIGGER IF NOT EXISTS trg_p9_operation_logs_no_delete BEFORE DELETE ON operation_logs WHEN OLD.resource IN ('inventory_sync','sku_binding') BEGIN SELECT RAISE(ABORT, 'immutable_record'); END;`,
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
		`CREATE OR REPLACE FUNCTION inventorysyncp9_reject_immutable_change() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'immutable_record';
		END;
		$$ LANGUAGE plpgsql;`,
		`DO $$ BEGIN
			CREATE TRIGGER trg_p9_inventory_snapshot_items_no_update BEFORE UPDATE ON p9_inventory_snapshot_items FOR EACH ROW EXECUTE FUNCTION inventorysyncp9_reject_immutable_change();
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			CREATE TRIGGER trg_p9_inventory_snapshot_items_no_delete BEFORE DELETE ON p9_inventory_snapshot_items FOR EACH ROW EXECUTE FUNCTION inventorysyncp9_reject_immutable_change();
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			CREATE TRIGGER trg_p9_sku_binding_calibrations_no_update BEFORE UPDATE ON p9_sku_binding_calibrations FOR EACH ROW EXECUTE FUNCTION inventorysyncp9_reject_immutable_change();
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			CREATE TRIGGER trg_p9_sku_binding_calibrations_no_delete BEFORE DELETE ON p9_sku_binding_calibrations FOR EACH ROW EXECUTE FUNCTION inventorysyncp9_reject_immutable_change();
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			CREATE TRIGGER trg_p9_manual_binding_decisions_no_update BEFORE UPDATE ON p9_manual_binding_decisions FOR EACH ROW EXECUTE FUNCTION inventorysyncp9_reject_immutable_change();
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			CREATE TRIGGER trg_p9_manual_binding_decisions_no_delete BEFORE DELETE ON p9_manual_binding_decisions FOR EACH ROW EXECUTE FUNCTION inventorysyncp9_reject_immutable_change();
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			CREATE TRIGGER trg_p9_operation_logs_no_update BEFORE UPDATE ON operation_logs FOR EACH ROW WHEN (OLD.resource IN ('inventory_sync','sku_binding')) EXECUTE FUNCTION inventorysyncp9_reject_immutable_change();
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			CREATE TRIGGER trg_p9_operation_logs_no_delete BEFORE DELETE ON operation_logs FOR EACH ROW WHEN (OLD.resource IN ('inventory_sync','sku_binding')) EXECUTE FUNCTION inventorysyncp9_reject_immutable_change();
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}
