package database

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"gorm.io/gorm"
)

func openProductPublishSchemaMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:publish_schema_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestMigrateLegacyPublicationSPUColumnRenamesAndPreservesData(t *testing.T) {
	db := openProductPublishSchemaMigrationDB(t)
	if err := db.Exec(`CREATE TABLE product_publications (id text primary key, external_sp_uid varchar(512))`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO product_publications (id, external_sp_uid) VALUES ('publication-1', 'legacy-spu')`).Error; err != nil {
		t.Fatal(err)
	}

	if err := migrateLegacyPublicationSPUColumn(db); err != nil {
		t.Fatal(err)
	}
	if db.Migrator().HasColumn(productPublicationsTable, legacyExternalSPUIDColumn) {
		t.Fatalf("legacy column %s should have been renamed", legacyExternalSPUIDColumn)
	}
	if !db.Migrator().HasColumn(productPublicationsTable, canonicalExternalSPUIDColumn) {
		t.Fatalf("canonical column %s was not created", canonicalExternalSPUIDColumn)
	}
	var got string
	if err := db.Raw(`SELECT external_spu_id FROM product_publications WHERE id = 'publication-1'`).Scan(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got != "legacy-spu" {
		t.Fatalf("renamed column lost data: got %q", got)
	}

	if err := migrateLegacyPublicationSPUColumn(db); err != nil {
		t.Fatalf("migration must be idempotent: %v", err)
	}
}

func TestMigrateLegacyPublicationSPUColumnBackfillsPartiallyUpgradedSchema(t *testing.T) {
	db := openProductPublishSchemaMigrationDB(t)
	if err := db.Exec(`CREATE TABLE product_publications (
		id text primary key,
		external_sp_uid varchar(512),
		external_spu_id varchar(512)
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO product_publications (id, external_sp_uid, external_spu_id) VALUES
		('empty-canonical', 'legacy-value', ''),
		('canonical-wins', 'legacy-conflict', 'canonical-value')`).Error; err != nil {
		t.Fatal(err)
	}

	if err := migrateLegacyPublicationSPUColumn(db); err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	var values []struct {
		ID            string
		ExternalSPUID string `gorm:"column:external_spu_id"`
	}
	if err := db.Raw(`SELECT id, external_spu_id FROM product_publications ORDER BY id`).Scan(&values).Error; err != nil {
		t.Fatal(err)
	}
	for _, value := range values {
		rows[value.ID] = value.ExternalSPUID
	}
	if rows["empty-canonical"] != "legacy-value" {
		t.Fatalf("legacy value was not backfilled: %#v", rows)
	}
	if rows["canonical-wins"] != "canonical-value" {
		t.Fatalf("canonical value must not be overwritten: %#v", rows)
	}
	if !db.Migrator().HasColumn(productPublicationsTable, legacyExternalSPUIDColumn) {
		t.Fatal("legacy column must be retained when both columns exist")
	}

	if err := migrateLegacyPublicationSPUColumn(db); err != nil {
		t.Fatalf("partially upgraded migration must be idempotent: %v", err)
	}
}

func TestProductPublishModelsUseWorkerCompatibleColumns(t *testing.T) {
	db := openProductPublishSchemaMigrationDB(t)
	if err := db.AutoMigrate(
		&productpublish.ProductPublishTask{},
		&productpublish.ProductPublishBatch{},
		&productpublish.ProductPublication{},
		&productpublish.ProductPublicationSKU{},
	); err != nil {
		t.Fatal(err)
	}
	if err := verifyProductPublishWorkerColumns(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasColumn(productPublicationsTable, canonicalExternalSPUIDColumn) {
		t.Fatalf("model did not create %s", canonicalExternalSPUIDColumn)
	}
	if db.Migrator().HasColumn(productPublicationsTable, legacyExternalSPUIDColumn) {
		t.Fatalf("model unexpectedly created legacy column %s", legacyExternalSPUIDColumn)
	}
}

func TestVerifyProductPublishWorkerColumnsRejectsIncompleteSchema(t *testing.T) {
	db := openProductPublishSchemaMigrationDB(t)
	if err := db.Exec(`CREATE TABLE product_publish_tasks (id text primary key)`).Error; err != nil {
		t.Fatal(err)
	}
	err := verifyProductPublishWorkerColumns(db)
	if err == nil || !strings.Contains(err.Error(), "missing required column") {
		t.Fatalf("expected missing-column error, got %v", err)
	}
}
