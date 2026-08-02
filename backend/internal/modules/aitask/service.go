package aitask

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Service persists ai_tasks.
type Service struct {
	DB *gorm.DB
}

// Create inserts a new task row (defaults status running and started_at).
func (s *Service) Create(ctx context.Context, row *AITask) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("aitask: no db")
	}
	if row == nil {
		return fmt.Errorf("aitask: nil row")
	}
	if err := s.assignTrustedTenant(ctx, row); err != nil {
		return err
	}
	if row.Status == "" {
		row.Status = StatusRunning
	}
	if row.StartedAt == nil {
		now := time.Now().UTC()
		row.StartedAt = &now
	}
	return s.DB.WithContext(ctx).Create(row).Error
}

// assignTrustedTenant derives ownership from stable domain records. A task with
// conflicting or absent ownership is not persisted, so a later tenant-scoped
// read cannot expose an unknown audit record.
func (s *Service) assignTrustedTenant(ctx context.Context, row *AITask) error {
	candidates, err := s.trustedTenantCandidates(ctx, row)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		if row.TenantID == 0 {
			return fmt.Errorf("aitask: tenant ownership is required")
		}
		return nil // explicit non-zero tenant IDs are supplied by trusted internal callers.
	}
	if len(candidates) != 1 {
		return fmt.Errorf("aitask: ambiguous tenant ownership")
	}
	for tenantID := range candidates {
		if row.TenantID != 0 && row.TenantID != tenantID {
			return fmt.Errorf("aitask: tenant ownership conflicts with linked record")
		}
		row.TenantID = tenantID
	}
	return nil
}

func (s *Service) trustedTenantCandidates(ctx context.Context, row *AITask) (map[int64]struct{}, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("aitask: no db")
	}
	candidates := make(map[int64]struct{})
	lookups := []struct {
		table string
		id    *uuid.UUID
	}{
		{table: "products", id: row.ProductID},
		{table: "customer_conversations", id: row.ConversationID},
		{table: "admin_users", id: row.CreatedBy},
		{table: "ai_operation_batches", id: row.BatchID},
	}
	for _, lookup := range lookups {
		if lookup.id == nil || !s.DB.Migrator().HasTable(lookup.table) {
			continue
		}
		var tenantID int64
		err := s.DB.WithContext(ctx).Table(lookup.table).Select("tenant_id").Where("id = ?", *lookup.id).Take(&tenantID).Error
		if err == nil {
			candidates[tenantID] = struct{}{}
			continue
		}
		if err != gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("aitask: resolve tenant from %s: %w", lookup.table, err)
		}
	}
	return candidates, nil
}

// MarkSuccess updates a task with output and token usage.
func (s *Service) MarkSuccess(ctx context.Context, id uuid.UUID, output json.RawMessage, raw json.RawMessage, inTok, outTok int, model string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("aitask: no db")
	}
	now := time.Now().UTC()
	updates := map[string]any{
		"status":        StatusSuccess,
		"output":        datatypes.JSON(output),
		"raw_response":  datatypes.JSON(raw),
		"token_input":   inTok,
		"token_output":  outTok,
		"finished_at":   &now,
		"error_message": "",
	}
	if strings.TrimSpace(model) != "" {
		updates["model"] = strings.TrimSpace(model)
	}
	return s.DB.WithContext(ctx).Model(&AITask{}).Where("id = ?", id).Updates(updates).Error
}

// MarkFailed records failure.
func (s *Service) MarkFailed(ctx context.Context, id uuid.UUID, msg string) error {
	return s.MarkFailedWithMeta(ctx, id, msg, nil, 0, 0, "")
}

// MarkFailedWithMeta records failure and optionally persists raw response / token usage for debugging.
func (s *Service) MarkFailedWithMeta(ctx context.Context, id uuid.UUID, msg string, raw json.RawMessage, inTok, outTok int, model string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("aitask: no db")
	}
	now := time.Now().UTC()
	updates := map[string]any{
		"status":        StatusFailed,
		"error_message": msg,
		"finished_at":   &now,
	}
	if len(raw) > 0 {
		updates["raw_response"] = datatypes.JSON(raw)
	}
	if inTok > 0 {
		updates["token_input"] = inTok
	}
	if outTok > 0 {
		updates["token_output"] = outTok
	}
	if strings.TrimSpace(model) != "" {
		updates["model"] = strings.TrimSpace(model)
	}
	return s.DB.WithContext(ctx).Model(&AITask{}).Where("id = ?", id).Updates(updates).Error
}

// GetByID loads one task.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*AITask, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("aitask: no db")
	}
	var row AITask
	if err := s.DB.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// GetByIDForTenant loads one task only when it belongs to the authenticated tenant.
func (s *Service) GetByIDForTenant(c *gin.Context, id uuid.UUID) (*AITask, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("aitask: no db")
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var row AITask
	if err := s.DB.WithContext(c.Request.Context()).Where("tenant_id = ?", tenantID).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// ListRecentForProduct returns latest tasks for a product (newest first).
func (s *Service) ListRecentForProduct(ctx context.Context, productID uuid.UUID, limit int) ([]AITask, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("aitask: no db")
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	var rows []AITask
	if err := s.DB.WithContext(ctx).
		Select("id", "task_type", "provider", "model", "prompt_code", "status", "error_message", "token_input", "token_output", "cost_amount", "product_id", "conversation_id", "created_by", "batch_id", "batch_no", "started_at", "finished_at", "created_at", "updated_at").
		Where("product_id = ?", productID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
