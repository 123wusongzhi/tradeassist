package inventory

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasklease"
)

func (s *Service) inventoryLeaseTTL() time.Duration {
	if s != nil && s.TaskTimeout > 0 {
		return s.TaskTimeout
	}
	return 120 * time.Second
}

func (s *Service) tryClaimInventorySyncTask(ctx context.Context, taskID uuid.UUID, workerID string, lease time.Duration) (*InventorySyncTask, *tasklease.ClaimResult, bool, error) {
	if s == nil || s.DB == nil {
		return nil, nil, false, fmt.Errorf("inventory: no db")
	}
	claim, ok, err := tasklease.TryClaim(ctx, s.DB, InventorySyncTask{}.TableName(), StatusPending, StatusRunning, taskID, workerID, lease)
	if err != nil {
		return nil, nil, false, err
	}
	if !ok {
		return nil, nil, false, nil
	}
	var task InventorySyncTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return nil, nil, false, err
	}
	return &task, &claim, true, nil
}

func (s *Service) startInventoryLeaseRenewal(ctx context.Context, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult, leaseTTL time.Duration) (stop func()) {
	if s == nil || s.DB == nil || claim == nil {
		return func() {}
	}
	return tasklease.StartRenewal(ctx, s.DB, InventorySyncTask{}.TableName(), StatusRunning, taskID, workerID, claim.ExecutionID, claim.LeaseVersion, leaseTTL)
}

func (s *Service) validateInventoryLease(ctx context.Context, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult) error {
	if claim == nil {
		return tasklease.ErrLeaseLost
	}
	return tasklease.ValidateLease(ctx, s.DB, InventorySyncTask{}.TableName(), StatusRunning, taskID, workerID, claim.ExecutionID, claim.LeaseVersion)
}

func (s *Service) finishInventorySyncTask(ctx context.Context, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult, updates map[string]any) error {
	tc := security.FromContext(ctx)
	if tc == nil || tc.TenantID < 0 {
		return security.ErrTenantContextMissing
	}
	if err := s.validateInventoryLease(ctx, taskID, workerID, claim); err != nil {
		slog.Warn("inventory_sync_lease_lost_on_finish", "taskId", taskID.String(), "workerId", workerID, "error", err.Error())
		return err
	}
	now := time.Now().UTC()
	updates["locked_by"] = nil
	updates["locked_until"] = nil
	updates["updated_at"] = now
	res := s.DB.WithContext(ctx).Model(&InventorySyncTask{}).
		Where("id = ? AND tenant_id = ? AND locked_by = ? AND execution_id = ? AND lock_version = ?",
			taskID, tc.TenantID, workerID, claim.ExecutionID.String(), claim.LeaseVersion).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return tasklease.ErrLeaseLost
	}
	return nil
}

func (s *Service) handleInventoryPanic(parent context.Context, taskID uuid.UUID, workerID string, panicVal any) {
	if s == nil || s.DB == nil {
		return
	}
	ctx := parent
	if ctx == nil {
		ctx = context.Background()
	}
	var cur InventorySyncTask
	if err := s.DB.WithContext(ctx).First(&cur, "id = ?", taskID).Error; err != nil {
		return
	}
	if cur.Status != StatusRunning || cur.LockedBy == nil || *cur.LockedBy != workerID {
		return
	}
	msg := fmt.Sprintf("inventory sync worker panic: %v", panicVal)
	fin := time.Now().UTC()
	_ = s.DB.WithContext(ctx).Model(&InventorySyncTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":        StatusFailed,
			"error_message": msg,
			"finished_at":   &fin,
			"locked_by":     nil,
			"locked_until":  nil,
			"updated_at":    fin,
		}).Error
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: cur.CreatedBy,
			Action:      "inventory.sync.failed",
			Resource:    "inventory_sync_task",
			ResourceID:  taskID.String(),
			Status:      "failed",
			Message:     fmt.Sprintf("taskId=%s panic recovery", taskID.String()),
		})
	}
	s.maybeReconcileInventoryBatch(ctx, cur.BatchID)
}

// RecoverLeaseExpired marks overdue running inventory tasks failed for human retry.
func (s *Service) RecoverLeaseExpired(ctx context.Context, taskID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("inventory: no db")
	}
	now := time.Now().UTC()
	var task InventorySyncTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	if task.Status != StatusRunning || task.LockedUntil == nil || !task.LockedUntil.Before(now) {
		return nil
	}
	fin := now
	recovery := s.DB.WithContext(ctx).Model(&InventorySyncTask{}).
		Where("id = ? AND tenant_id = ? AND status = ? AND locked_until IS NOT NULL AND locked_until < ? AND lock_version = ?", taskID, task.TenantID, StatusRunning, now, task.LockVersion)
	if task.LockedBy != nil {
		recovery = recovery.Where("locked_by = ?", *task.LockedBy)
	}
	if task.ExecutionID != nil {
		recovery = recovery.Where("execution_id = ?", *task.ExecutionID)
	}
	result := recovery.
		Updates(map[string]any{
			"status":        StatusFailed,
			"error_message": "worker lease expired",
			"finished_at":   &fin,
			"locked_by":     nil,
			"locked_until":  nil,
			"updated_at":    fin,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}
	bid := task.BatchID
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "inventory.sync.failed",
			Resource:    "inventory_sync_task",
			ResourceID:  taskID.String(),
			Status:      "failed",
			Message:     fmt.Sprintf("taskId=%s lease_expired shopId=%s", taskID.String(), task.ShopID.String()),
		})
	}
	s.maybeReconcileInventoryBatch(ctx, bid)
	return nil
}

// RecoverLegacyRunning fails stuck inventory rows lacking lease metadata.
func (s *Service) RecoverLegacyRunning(ctx context.Context, taskID uuid.UUID, legacyCutoff time.Time) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("inventory: no db")
	}
	var task InventorySyncTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	if task.Status != StatusRunning {
		return nil
	}
	if task.LockedBy != nil && task.LockedUntil != nil {
		return nil
	}
	if !task.UpdatedAt.Before(legacyCutoff) {
		return nil
	}
	fin := time.Now().UTC()
	result := s.DB.WithContext(ctx).Model(&InventorySyncTask{}).
		Where("id = ? AND tenant_id = ? AND status = ?", taskID, task.TenantID, StatusRunning).
		Where("locked_by IS NULL AND locked_until IS NULL AND execution_id IS NULL AND heartbeat_at IS NULL").
		Where("lock_version = ? AND updated_at < ?", task.LockVersion, legacyCutoff).
		Updates(map[string]any{
			"status":        StatusFailed,
			"error_message": "legacy running inventory_sync_task recovered (no lease)",
			"finished_at":   &fin,
			"locked_by":     nil,
			"locked_until":  nil,
			"updated_at":    fin,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}
	return nil
}
