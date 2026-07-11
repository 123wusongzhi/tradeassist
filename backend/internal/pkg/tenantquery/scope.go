package tenantquery

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/repository"
	"gorm.io/gorm"
)

// ScopeTenant applies tenant_id filter when column exists on the queried table.
func ScopeTenant(tx *gorm.DB, tenantID int64) *gorm.DB {
	return repository.ScopeTenant(tx, tenantID)
}

// ScopeShopTenant restricts rows to shops owned by tenant via subquery.
func ScopeShopTenant(tx *gorm.DB, tenantID int64, shopColumn string) *gorm.DB {
	if tenantID <= 0 {
		return tx.Where("1 = 0")
	}
	col := fmt.Sprintf("%s", shopColumn)
	return tx.Where(col+" IN (SELECT id FROM shops WHERE tenant_id = ?)", tenantID)
}

// ScopeProductTenant restricts rows to products owned by tenant via subquery.
func ScopeProductTenant(tx *gorm.DB, tenantID int64, productColumn string) *gorm.DB {
	if tenantID <= 0 {
		return tx.Where("1 = 0")
	}
	return tx.Where(productColumn+" IN (SELECT id FROM products WHERE tenant_id = ?)", tenantID)
}

// FindByIDTenant loads one row with tenant scope on tenant_id column.
func FindByIDTenant(ctx context.Context, tx *gorm.DB, dest any, tenantID int64, id any) error {
	return repository.FindByID(ctx, tx, dest, tenantID, id)
}

// FindByIDShopTenant loads one row scoped by shop tenant join.
func FindByIDShopTenant(ctx context.Context, tx *gorm.DB, dest any, model any, tenantID int64, shopColumn string, id uuid.UUID) error {
	if tx == nil {
		return fmt.Errorf("tenantquery: nil db")
	}
	q := tx.WithContext(ctx).Model(model).
		Where("id = ?", id).
		Where(shopColumn+" IN (SELECT id FROM shops WHERE tenant_id = ?)", tenantID)
	return q.First(dest).Error
}
