package aiproducttext

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

// ReconcileTextApply on retry_allowed acquire attempts to repair idempotency after commit gap.
func (s *Service) ReconcileTextApply(ctx context.Context, job *textApplyAcquire, item *AIProductTextItem) (bool, error) {
	if s == nil || s.DB == nil || s.Products == nil || job == nil || item == nil || item.AITaskID == nil {
		return false, nil
	}
	fieldType := product.AIContentFieldTitle
	if item.OperationType == OpDescription {
		fieldType = product.AIContentFieldDescription
	}
	err := s.Products.ReconcileAIApply(ctx, item.ProductID, fieldType, item.AITaskID.String(), item.SourceSnapshotHash, job.RecordID, job.Owner)
	if err != nil {
		if strings.Contains(err.Error(), product.CodeAIApplyReconciliationConflict) {
			return false, err
		}
		return false, nil
	}
	var app product.ProductAIContentApplication
	_ = s.DB.WithContext(ctx).
		Where("product_id = ? AND ai_task_id = ? AND status = ?", item.ProductID, item.AITaskID, product.AIContentApplyStatusApplied).
		Order("applied_at DESC").First(&app).Error
	if app.ID == uuid.Nil {
		return false, nil
	}
	summary := map[string]string{
		"batchId":       item.BatchID.String(),
		"itemId":        item.ID.String(),
		"productId":     item.ProductID.String(),
		"applicationId": app.ID.String(),
		"reconciled":    "true",
	}
	if err := s.completeTextApply(ctx, job, summary, app.ID.String()); err != nil {
		return false, err
	}
	_ = s.DB.WithContext(ctx).Model(item).Updates(map[string]any{"status": ItemApplied}).Error
	return true, nil
}
