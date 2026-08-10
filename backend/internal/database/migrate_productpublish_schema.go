package database

import (
	"fmt"

	"gorm.io/gorm"
)

const (
	productPublicationsTable     = "product_publications"
	legacyExternalSPUIDColumn    = "external_sp_uid"
	canonicalExternalSPUIDColumn = "external_spu_id"
)

// migrateLegacyPublicationSPUColumn aligns the historical GORM-derived
// external_sp_uid column with the explicit external_spu_id column used by the
// publication worker's raw updates. It runs before AutoMigrate so GORM cannot
// create a second, empty canonical column beside populated legacy data.
func migrateLegacyPublicationSPUColumn(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(productPublicationsTable) {
		return nil
	}
	if !db.Migrator().HasColumn(productPublicationsTable, legacyExternalSPUIDColumn) {
		return nil
	}
	if !db.Migrator().HasColumn(productPublicationsTable, canonicalExternalSPUIDColumn) {
		if err := db.Migrator().RenameColumn(productPublicationsTable, legacyExternalSPUIDColumn, canonicalExternalSPUIDColumn); err != nil {
			return fmt.Errorf("rename %s.%s to %s: %w", productPublicationsTable, legacyExternalSPUIDColumn, canonicalExternalSPUIDColumn, err)
		}
		return nil
	}

	// A partially upgraded database may contain both columns. Fill only empty
	// canonical values; a non-empty canonical value remains authoritative while
	// the legacy column is retained so conflicting data is never destroyed.
	if err := db.Exec(`UPDATE product_publications
		SET external_spu_id = external_sp_uid
		WHERE COALESCE(external_spu_id, '') = ''
		  AND COALESCE(external_sp_uid, '') <> ''`).Error; err != nil {
		return fmt.Errorf("backfill %s.%s from %s: %w", productPublicationsTable, canonicalExternalSPUIDColumn, legacyExternalSPUIDColumn, err)
	}
	return nil
}

// verifyProductPublishWorkerColumns fails startup when the migrated schema
// cannot satisfy raw-column updates in the publish worker. These names are
// intentionally database column names rather than Go field names.
func verifyProductPublishWorkerColumns(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("verify product publish schema: nil db")
	}
	required := map[string][]string{
		"product_publish_tasks": {
			"status",
			"publish_status",
			"platform_result",
			"platform_product_id",
			"platform_raw_error",
			"retryable",
			"finished_at",
			"error_code",
			"error_message",
			"output",
		},
		productPublicationsTable: {
			"publish_status",
			"status",
			"external_product_id",
			canonicalExternalSPUIDColumn,
			"external_url",
			"published_at",
			"last_synced_at",
			"raw_data",
		},
		"product_publication_skus": {
			"publication_id",
			"product_sku_id",
			"external_sku_id",
			"sku_code",
			"price",
			"stock",
			"raw_data",
		},
	}
	for table, columns := range required {
		for _, column := range columns {
			if !db.Migrator().HasColumn(table, column) {
				return fmt.Errorf("verify product publish schema: missing required column %s.%s", table, column)
			}
		}
	}
	return nil
}
