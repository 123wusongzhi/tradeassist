package database

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/orderexception"
	"gorm.io/gorm"
)

// migrateOrderExceptionTenantScope backfills marks only when their source has
// one authoritative parent order. Anything unknown remains tenant 0, which is
// intentionally invisible to non-zero tenant requests.
func migrateOrderExceptionTenantScope(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&orderexception.OrderExceptionMark{}) {
		return nil
	}
	if err := db.AutoMigrate(&orderexception.OrderExceptionMark{}); err != nil {
		return fmt.Errorf("migrate order exception tenant columns: %w", err)
	}
	var marks []orderexception.OrderExceptionMark
	if err := db.Where("tenant_id = ?", 0).Find(&marks).Error; err != nil {
		return fmt.Errorf("list legacy order exception marks: %w", err)
	}
	for _, mark := range marks {
		tid, ok, err := orderExceptionMarkTenant(db, mark)
		if err != nil {
			return fmt.Errorf("resolve legacy order exception mark %s: %w", mark.ID, err)
		}
		if ok && tid != 0 {
			if err := db.Model(&orderexception.OrderExceptionMark{}).Where("id = ? AND tenant_id = ?", mark.ID, 0).Update("tenant_id", tid).Error; err != nil {
				return fmt.Errorf("backfill order exception mark %s: %w", mark.ID, err)
			}
		}
	}
	const idx = "ux_order_exception_mark_quad"
	if db.Migrator().HasIndex(&orderexception.OrderExceptionMark{}, idx) {
		if err := db.Migrator().DropIndex(&orderexception.OrderExceptionMark{}, idx); err != nil {
			return fmt.Errorf("drop legacy order exception mark index: %w", err)
		}
	}
	if err := db.Migrator().CreateIndex(&orderexception.OrderExceptionMark{}, idx); err != nil {
		return fmt.Errorf("create tenant order exception mark index: %w", err)
	}
	return nil
}

func orderExceptionMarkTenant(db *gorm.DB, mark orderexception.OrderExceptionMark) (int64, bool, error) {
	var oid uuid.UUID
	switch strings.TrimSpace(mark.SourceType) {
	case orderexception.SourceOrderItemSKUMatch:
		var row order.OrderItemSKUMatch
		if err := db.Select("order_id").First(&row, "id = ?", mark.SourceID).Error; err != nil {
			return 0, false, nil
		}
		oid = row.OrderID
	case orderexception.SourceOrderItem:
		var row order.OrderItem
		if err := db.Select("order_id").First(&row, "id = ?", mark.SourceID).Error; err != nil {
			return 0, false, nil
		}
		oid = row.OrderID
	case orderexception.SourceOrderInventoryEffect:
		var row inventory.OrderInventoryEffect
		if err := db.Select("order_id").First(&row, "id = ?", mark.SourceID).Error; err != nil {
			return 0, false, nil
		}
		oid = row.OrderID
	default:
		return 0, false, nil
	}
	var row order.Order
	if err := db.Select("tenant_id").First(&row, "id = ? AND deleted_at IS NULL", oid).Error; err != nil {
		return 0, false, nil
	}
	return row.TenantID, true, nil
}
