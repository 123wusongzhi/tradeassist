package database

import (
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"gorm.io/gorm"
)

const (
	orderTenantOrderNoUniqueIndex = "ux_orders_tenant_order_no"
	legacyOrderNoUniqueIndex      = "idx_orders_order_no"
)

// migrateOrderTenantOrderNoUniqueIndex scopes order-number uniqueness to a
// tenant. Create the replacement index before dropping the legacy global one
// so a failed migration cannot leave orders without a uniqueness constraint.
func migrateOrderTenantOrderNoUniqueIndex(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate order tenant order number index: db is nil")
	}
	if !db.Migrator().HasTable(&order.Order{}) {
		return nil
	}
	if !db.Migrator().HasIndex(&order.Order{}, orderTenantOrderNoUniqueIndex) {
		if err := db.Migrator().CreateIndex(&order.Order{}, orderTenantOrderNoUniqueIndex); err != nil {
			return fmt.Errorf("create orders tenant/order number unique index: %w", err)
		}
	}
	if db.Migrator().HasIndex(&order.Order{}, legacyOrderNoUniqueIndex) {
		if err := db.Migrator().DropIndex(&order.Order{}, legacyOrderNoUniqueIndex); err != nil {
			return fmt.Errorf("drop legacy orders order number unique index: %w", err)
		}
	}
	return nil
}
