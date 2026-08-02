package aiproductimage

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

type tenantContextKey struct{}

func withTenantContext(c *gin.Context) (context.Context, int64, error) {
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil || tenantID < 0 {
		return nil, 0, fmt.Errorf("ai image batch: tenant context required")
	}
	return context.WithValue(c.Request.Context(), tenantContextKey{}, tenantID), tenantID, nil
}

// verifyApplyTargets protects direct service calls before any product/image/task
// write. Batch items are tenant-owned through their batch, not their own column.
func (s *Service) verifyApplyTargets(ctx context.Context, item *AIProductImageItem) error {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return err
	}
	if item == nil || item.ImageTaskID == nil {
		return fmt.Errorf("ai image batch: apply target missing")
	}
	var batch AIProductImageBatch
	if err := s.DB.WithContext(ctx).First(&batch, "id = ? AND tenant_id = ?", item.BatchID, tenantID).Error; err != nil {
		return err
	}
	var p product.Product
	if err := s.DB.WithContext(ctx).First(&p, "id = ? AND tenant_id = ?", item.ProductID, tenantID).Error; err != nil {
		return err
	}
	var task imagetask.ImageTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ? AND tenant_id = ? AND product_id = ?", *item.ImageTaskID, tenantID, item.ProductID).Error; err != nil {
		return err
	}
	if item.ImageID != nil {
		var image product.ProductImage
		if err := s.DB.WithContext(ctx).Joins("JOIN products p ON p.id = product_images.product_id AND p.tenant_id = ?", tenantID).First(&image, "product_images.id = ? AND product_images.product_id = ?", *item.ImageID, item.ProductID).Error; err != nil {
			return err
		}
	}
	return nil
}

func tenantIDFromContext(ctx context.Context) (int64, error) {
	tenantID, ok := ctx.Value(tenantContextKey{}).(int64)
	if !ok || tenantID < 0 {
		return 0, fmt.Errorf("ai image batch: tenant context required")
	}
	return tenantID, nil
}
