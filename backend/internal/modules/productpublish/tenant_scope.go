package productpublish

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

// productForTenant is the sole product ownership lookup used by publish flows.
// A tenant of zero is retained only for legacy rows that have not yet been assigned.
func (s *Service) productForTenant(ctx context.Context, tenantID int64, productID uuid.UUID) (*product.Product, error) {
	if s == nil || s.DB == nil || productID == uuid.Nil {
		return nil, gorm.ErrRecordNotFound
	}
	var row product.Product
	if err := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ?", productID, tenantID).First(&row).Error; err != nil {
		return nil, err
	}
	if row.DeletedAt.Valid {
		return nil, fmt.Errorf("deleted product cannot be published")
	}
	return &row, nil
}

// trustedTenantOrLegacy keeps service-level legacy/worker calls on tenant 0;
// HTTP routes are authenticated and populate the Gin tenant before they reach it.
func trustedTenantOrLegacy(c *gin.Context) (int64, error) {
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return 0, nil
	}
	return tenantID, nil
}
