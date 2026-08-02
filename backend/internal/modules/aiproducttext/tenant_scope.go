package aiproducttext

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

type tenantContextKey struct{}

func withTenantContext(c *gin.Context) (context.Context, int64, error) {
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil || tenantID < 0 {
		return nil, 0, fmt.Errorf("ai text batch: tenant context required")
	}
	return context.WithValue(c.Request.Context(), tenantContextKey{}, tenantID), tenantID, nil
}

// verifyApplyTargets makes direct service calls fail closed as well as HTTP calls.
// Items do not carry tenant_id, so their batch and every side-effect target must
// agree with the trusted tenant before an apply/undo can proceed.
func (s *Service) verifyApplyTargets(ctx context.Context, item *AIProductTextItem) error {
	tenantID, err := tenantIDFromContext(ctx)
	if err != nil {
		return err
	}
	if item == nil || item.AITaskID == nil {
		return fmt.Errorf("ai text batch: apply target missing")
	}
	var batch AIProductTextBatch
	if err := s.DB.WithContext(ctx).First(&batch, "id = ? AND tenant_id = ?", item.BatchID, tenantID).Error; err != nil {
		return err
	}
	var p product.Product
	if err := s.DB.WithContext(ctx).First(&p, "id = ? AND tenant_id = ?", item.ProductID, tenantID).Error; err != nil {
		return err
	}
	var task aitask.AITask
	if err := s.DB.WithContext(ctx).First(&task, "id = ? AND tenant_id = ? AND product_id = ?", *item.AITaskID, tenantID, item.ProductID).Error; err != nil {
		return err
	}
	return nil
}

func tenantIDFromContext(ctx context.Context) (int64, error) {
	tenantID, ok := ctx.Value(tenantContextKey{}).(int64)
	if !ok || tenantID < 0 {
		return 0, fmt.Errorf("ai text batch: tenant context required")
	}
	return tenantID, nil
}
