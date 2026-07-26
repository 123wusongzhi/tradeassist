package tasktenant

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

// ShopTenantRow is a minimal shop projection for tenant resolution.
type ShopTenantRow struct {
	ID       uuid.UUID `gorm:"column:id"`
	TenantID int64     `gorm:"column:tenant_id"`
}

// ResolveShopTenant loads tenant_id for a shop. Missing or zero tenant is an error.
func ResolveShopTenant(ctx context.Context, db *gorm.DB, shopID uuid.UUID) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("tasktenant: nil db")
	}
	if shopID == uuid.Nil {
		return 0, security.ErrTaskTenantMissing
	}
	var row ShopTenantRow
	err := db.WithContext(ctx).Table("shops").Select("id, tenant_id").Where("id = ?", shopID).First(&row).Error
	if err != nil {
		return 0, err
	}
	if err := RequireTaskTenant(row.TenantID); err != nil {
		return 0, err
	}
	return row.TenantID, nil
}

// ResolveProductTenant loads tenant_id for a product row.
func ResolveProductTenant(ctx context.Context, db *gorm.DB, productID uuid.UUID) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("tasktenant: nil db")
	}
	if productID == uuid.Nil {
		return 0, security.ErrTaskTenantMissing
	}
	var tid int64
	err := db.WithContext(ctx).Table("products").Select("tenant_id").Where("id = ?", productID).Scan(&tid).Error
	if err != nil {
		return 0, err
	}
	if err := RequireTaskTenant(tid); err != nil {
		return 0, err
	}
	return tid, nil
}
