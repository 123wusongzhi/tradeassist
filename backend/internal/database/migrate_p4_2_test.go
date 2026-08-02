package database

import (
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRunP42BackfillDoesNotSwallowGeneralErrors(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:p42_backfill_error?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	err = runP42BackfillStatements(db, []p42BackfillStatement{{
		name:        "missing_table",
		postgresSQL: `UPDATE table_that_does_not_exist SET tenant_id = 1`,
		sqliteSQL:   `UPDATE table_that_does_not_exist SET tenant_id = 1`,
	}})
	if err == nil || !strings.Contains(err.Error(), "missing_table") {
		t.Fatalf("general migration error was swallowed: %v", err)
	}
}

type p42NamedDialector struct {
	gorm.Dialector
	name string
}

func (d p42NamedDialector) Name() string { return d.name }

func TestRunP42BackfillDoesNotSwallowPostgresErrors(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:p42_backfill_postgres_error?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	postgresDB := db.Session(&gorm.Session{NewDB: true})
	postgresDB.Dialector = p42NamedDialector{Dialector: db.Dialector, name: "postgres"}
	err = runP42BackfillStatements(postgresDB, []p42BackfillStatement{{
		name:        "postgres_failure",
		postgresSQL: `UPDATE table_that_does_not_exist SET tenant_id = 1`,
		sqliteSQL:   `UPDATE table_that_does_not_exist SET tenant_id = 1`,
	}})
	if err == nil || !strings.Contains(err.Error(), "postgres_failure") {
		t.Fatalf("PostgreSQL migration error was swallowed: %v", err)
	}
}

func TestP42SQLiteFallbackSQLBackfillsSafely(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:p42_backfill_sqlite?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE shops (id text primary key, tenant_id integer not null)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE inventory_sync_tasks (id text primary key, shop_id text, tenant_id integer)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO shops (id, tenant_id) VALUES ('shop-a', 7)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO inventory_sync_tasks (id, shop_id, tenant_id) VALUES ('task-a', 'shop-a', 0)`).Error; err != nil {
		t.Fatal(err)
	}
	stmt := p42BackfillStatements()[0]
	if err := db.Exec(stmt.sqliteSQL).Error; err != nil {
		t.Fatalf("SQLite fallback failed: %v", err)
	}
	var tenantID int64
	if err := db.Raw(`SELECT tenant_id FROM inventory_sync_tasks WHERE id = 'task-a'`).Scan(&tenantID).Error; err != nil {
		t.Fatal(err)
	}
	if tenantID != 7 {
		t.Fatalf("SQLite fallback tenant_id = %d, want 7", tenantID)
	}
}

func TestP42InventoryChangeLogBackfillUsesProductTenant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:p42_inventory_log_tenant?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`CREATE TABLE products (id text primary key, tenant_id integer not null)`,
		`CREATE TABLE inventory_change_logs (id text primary key, product_id text, tenant_id integer)`,
		`INSERT INTO products (id, tenant_id) VALUES ('product-a', 17)`,
		`INSERT INTO inventory_change_logs (id, product_id, tenant_id) VALUES ('log-a', 'product-a', 0)`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	var stmt p42BackfillStatement
	for _, candidate := range p42BackfillStatements() {
		if candidate.name == "inventory_change_logs" {
			stmt = candidate
			break
		}
	}
	if err := db.Exec(stmt.sqliteSQL).Error; err != nil {
		t.Fatal(err)
	}
	var tenantID int64
	if err := db.Raw(`SELECT tenant_id FROM inventory_change_logs WHERE id = 'log-a'`).Scan(&tenantID).Error; err != nil {
		t.Fatal(err)
	}
	if tenantID != 17 {
		t.Fatalf("tenant_id = %d, want 17", tenantID)
	}
}

func TestP42PublishTaskBackfillRequiresMatchingProductAndShopTenants(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:p42_publish_task_tenant?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`CREATE TABLE products (id text primary key, tenant_id integer not null)`,
		`CREATE TABLE shops (id text primary key, tenant_id integer not null)`,
		`CREATE TABLE product_publish_tasks (id text primary key, product_id text, shop_id text, tenant_id integer)`,
		`INSERT INTO products (id, tenant_id) VALUES ('product-a', 7), ('product-b', 11)`,
		`INSERT INTO shops (id, tenant_id) VALUES ('shop-a', 7), ('shop-b', 22)`,
		`INSERT INTO product_publish_tasks (id, product_id, shop_id, tenant_id) VALUES ('safe', 'product-a', 'shop-a', 0), ('mixed', 'product-b', 'shop-b', 0)`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	var stmt p42BackfillStatement
	for _, candidate := range p42BackfillStatements() {
		if candidate.name == "product_publish_tasks" {
			stmt = candidate
			break
		}
	}
	if err := db.Exec(stmt.sqliteSQL).Error; err != nil {
		t.Fatal(err)
	}
	var safe, mixed int64
	_ = db.Raw(`SELECT tenant_id FROM product_publish_tasks WHERE id = 'safe'`).Scan(&safe).Error
	_ = db.Raw(`SELECT tenant_id FROM product_publish_tasks WHERE id = 'mixed'`).Scan(&mixed).Error
	if safe != 7 || mixed != 0 {
		t.Fatalf("safe=%d mixed=%d; mismatch must not be inferred from shop", safe, mixed)
	}
}

func TestP42InventoryEventKeyUniqueWithinTenant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:p42_inventory_event_key?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`CREATE TABLE inventory_change_logs (id text primary key, tenant_id integer not null, business_event_key text)`,
		`CREATE UNIQUE INDEX idx_inv_change_tenant_event ON inventory_change_logs (tenant_id, business_event_key)`,
		`CREATE UNIQUE INDEX idx_inventory_change_logs_business_event_key ON inventory_change_logs (business_event_key)`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := migrateP42Indexes(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO inventory_change_logs (id, tenant_id, business_event_key) VALUES ('a', 1, 'event-1'), ('b', 2, 'event-1')`).Error; err != nil {
		t.Fatalf("cross-tenant duplicate should be allowed: %v", err)
	}
	if err := db.Exec(`INSERT INTO inventory_change_logs (id, tenant_id, business_event_key) VALUES ('c', 1, 'event-1')`).Error; err == nil {
		t.Fatal("same-tenant duplicate must remain blocked")
	}
	if err := db.Exec(`INSERT INTO inventory_change_logs (id, tenant_id, business_event_key) VALUES ('empty-a', 1, ''), ('empty-b', 1, '')`).Error; err != nil {
		t.Fatalf("empty event keys should not be unique: %v", err)
	}
	for _, legacy := range []string{"idx_inv_change_tenant_event", "idx_inventory_change_logs_business_event_key"} {
		if db.Migrator().HasIndex("inventory_change_logs", legacy) {
			t.Fatalf("legacy index %s was not replaced", legacy)
		}
	}
	if !db.Migrator().HasIndex("inventory_change_logs", "idx_inv_change_tenant_event_v2") {
		t.Fatal("versioned partial event-key index was not created")
	}
}

func TestP42AIBatchIdempotencyKeysAreUniqueWithinTenant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:p42_ai_batch_idempotency?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`CREATE TABLE ai_product_text_batches (id text primary key, tenant_id integer not null, idempotency_key text, created_at datetime)`,
		`CREATE UNIQUE INDEX idx_ai_product_text_batches_idempotency_key ON ai_product_text_batches (idempotency_key)`,
		`CREATE TABLE ai_product_image_batches (id text primary key, tenant_id integer not null, idempotency_key text, created_at datetime)`,
		`CREATE UNIQUE INDEX idx_ai_product_image_batches_idempotency_key ON ai_product_image_batches (idempotency_key)`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := migrateP42Indexes(db); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"ai_product_text_batches", "ai_product_image_batches"} {
		if err := db.Exec(`INSERT INTO ` + table + ` (id, tenant_id, idempotency_key) VALUES ('a', 1, 'same-key'), ('b', 2, 'same-key')`).Error; err != nil {
			t.Fatalf("%s cross-tenant duplicate should be allowed: %v", table, err)
		}
		if err := db.Exec(`INSERT INTO ` + table + ` (id, tenant_id, idempotency_key) VALUES ('c', 1, 'same-key')`).Error; err == nil {
			t.Fatalf("%s same-tenant duplicate must remain blocked", table)
		}
	}
}

func TestSQLiteUpdateFromCompatibilityIsBounded(t *testing.T) {
	if !isSQLiteUpdateFromUnsupported(errors.New(`near "FROM": syntax error`)) {
		t.Fatal("expected explicit UPDATE FROM syntax error to use fallback")
	}
	if !isSQLiteUpdateFromUnsupported(errors.New(`near "t": syntax error`)) {
		t.Fatal("expected SQLite UPDATE alias syntax error to use fallback")
	}
	if !isSQLiteUpdateFromUnsupported(errors.New(`near "b": syntax error`)) {
		t.Fatal("expected SQLite batch UPDATE alias syntax error to use fallback")
	}
	for _, err := range []error{errors.New("permission denied"), errors.New("no such table"), errors.New("connection refused")} {
		if isSQLiteUpdateFromUnsupported(err) {
			t.Fatalf("non-syntax error incorrectly accepted for fallback: %v", err)
		}
	}
}

func TestP42BatchBackfillLeavesMixedTenantBatchesUnassigned(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:p42_batch_tenant_mixed?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`CREATE TABLE products (id text primary key, tenant_id integer not null)`,
		`CREATE TABLE ai_product_text_batches (id text primary key, tenant_id integer)`,
		`CREATE TABLE ai_product_text_items (id text primary key, batch_id text, product_id text)`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec(`INSERT INTO products (id, tenant_id) VALUES ('p-one', 1), ('p-two', 2), ('p-three', 3)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO ai_product_text_batches (id, tenant_id) VALUES ('mixed', 0), ('single', 0)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO ai_product_text_items (id, batch_id, product_id) VALUES ('i-one', 'mixed', 'p-one'), ('i-two', 'mixed', 'p-two'), ('i-three', 'single', 'p-three')`).Error; err != nil {
		t.Fatal(err)
	}
	var stmt p42BackfillStatement
	for _, candidate := range p42BackfillStatements() {
		if candidate.name == "ai_product_text_batches" {
			stmt = candidate
			break
		}
	}
	if err := db.Exec(stmt.sqliteSQL).Error; err != nil {
		t.Fatal(err)
	}
	var mixed, single int64
	_ = db.Raw(`SELECT tenant_id FROM ai_product_text_batches WHERE id = 'mixed'`).Scan(&mixed).Error
	_ = db.Raw(`SELECT tenant_id FROM ai_product_text_batches WHERE id = 'single'`).Scan(&single).Error
	if mixed != 0 || single != 3 {
		t.Fatalf("mixed=%d single=%d; mixed must remain unassigned and single must backfill", mixed, single)
	}
}
