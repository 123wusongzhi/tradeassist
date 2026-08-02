package database

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"gorm.io/gorm"
)

func TestMigrateOrderTenantOrderNoUniqueIndexSQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:order_tenant_order_no_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE orders (id text primary key, tenant_id integer not null default 0, order_no text not null)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX idx_orders_order_no ON orders (order_no)`).Error; err != nil {
		t.Fatal(err)
	}

	if err := migrateOrderTenantOrderNoUniqueIndex(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasIndex(&order.Order{}, orderTenantOrderNoUniqueIndex) {
		t.Fatal("tenant/order number unique index was not created")
	}
	if db.Migrator().HasIndex(&order.Order{}, legacyOrderNoUniqueIndex) {
		t.Fatal("legacy global order number unique index was not removed")
	}

	if err := db.Exec(`INSERT INTO orders (id, tenant_id, order_no) VALUES ('one', 1, 'ORD-1')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO orders (id, tenant_id, order_no) VALUES ('two', 2, 'ORD-1')`).Error; err != nil {
		t.Fatalf("different tenants should be able to reuse an order number: %v", err)
	}
	if err := db.Exec(`INSERT INTO orders (id, tenant_id, order_no) VALUES ('three', 1, 'ORD-1')`).Error; err == nil {
		t.Fatal("same tenant duplicate order number must be rejected")
	}
}
