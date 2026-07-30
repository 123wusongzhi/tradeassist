package customersync

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasklease"
)

func (s *Service) taskLeaseTTL() time.Duration {
	if s != nil && s.TaskTimeout > 0 {
		return s.TaskTimeout
	}
	return 120 * time.Second
}

func (s *Service) tryClaimTask(ctx context.Context, taskID uuid.UUID, workerID string, lease time.Duration) (*CustomerMessageSyncTask, *tasklease.ClaimResult, bool, error) {
	if s == nil || s.DB == nil {
		return nil, nil, false, fmt.Errorf("customersync: no db")
	}
	claim, ok, err := tasklease.TryClaim(ctx, s.DB, CustomerMessageSyncTask{}.TableName(), StatusPending, StatusRunning, taskID, workerID, lease)
	if err != nil {
		return nil, nil, false, err
	}
	if !ok {
		return nil, nil, false, nil
	}
	var task CustomerMessageSyncTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return nil, nil, false, err
	}
	return &task, &claim, true, nil
}

func (s *Service) startLeaseRenewal(ctx context.Context, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult, leaseTTL time.Duration) (stop func()) {
	if s == nil || s.DB == nil || claim == nil {
		return func() {}
	}
	return tasklease.StartRenewal(ctx, s.DB, CustomerMessageSyncTask{}.TableName(), StatusRunning, taskID, workerID, claim.ExecutionID, claim.LeaseVersion, leaseTTL)
}

func (s *Service) validateCustomerSyncLease(ctx context.Context, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult) error {
	if claim == nil {
		return tasklease.ErrLeaseLost
	}
	return tasklease.ValidateLease(ctx, s.DB, CustomerMessageSyncTask{}.TableName(), StatusRunning, taskID, workerID, claim.ExecutionID, claim.LeaseVersion)
}

func (s *Service) finishCustomerSyncTask(ctx context.Context, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult, updates map[string]any) error {
	if err := s.validateCustomerSyncLease(ctx, taskID, workerID, claim); err != nil {
		slog.Warn("customer_sync_lease_lost_on_finish", "taskId", taskID.String(), "workerId", workerID, "error", err.Error())
		return err
	}
	now := time.Now().UTC()
	updates["locked_by"] = nil
	updates["locked_until"] = nil
	updates["updated_at"] = now
	res := s.DB.WithContext(ctx).Model(&CustomerMessageSyncTask{}).
		Where("id = ? AND locked_by = ? AND execution_id = ? AND lock_version = ?",
			taskID, workerID, claim.ExecutionID.String(), claim.LeaseVersion).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return tasklease.ErrLeaseLost
	}
	return nil
}

func (s *Service) handlePanic(parent context.Context, taskID uuid.UUID, workerID string, panicVal any) {
	if s == nil || s.DB == nil {
		return
	}
	ctx := parent
	if ctx == nil {
		ctx = context.Background()
	}
	var cur CustomerMessageSyncTask
	if err := s.DB.WithContext(ctx).First(&cur, "id = ?", taskID).Error; err != nil {
		return
	}
	if cur.Status != StatusRunning || cur.LockedBy == nil || *cur.LockedBy != workerID {
		return
	}
	msg := fmt.Sprintf("customer message sync worker panic: %v", panicVal)
	fin := time.Now().UTC()
	_ = s.DB.WithContext(ctx).Model(&CustomerMessageSyncTask{}).Where("id = ?", taskID).
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
			Action:      "customer.message_sync.failed",
			Resource:    "customer_message_sync_task",
			ResourceID:  taskID.String(),
			Status:      "failed",
			Message:     fmt.Sprintf("taskId=%s panic recovery", taskID.String()),
		})
	}
}

// RecoverLeaseExpired marks task failed for human retry.
func (s *Service) RecoverLeaseExpired(ctx context.Context, taskID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("customersync: no db")
	}
	now := time.Now().UTC()
	var task CustomerMessageSyncTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	if task.Status != StatusRunning || task.LockedUntil == nil || !task.LockedUntil.Before(now) {
		return nil
	}
	fin := now
	_ = s.DB.WithContext(ctx).Model(&CustomerMessageSyncTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":        StatusFailed,
			"error_message": "worker lease expired",
			"finished_at":   &fin,
			"locked_by":     nil,
			"locked_until":  nil,
			"execution_id":  nil,
			"heartbeat_at":  nil,
			"updated_at":    fin,
		}).Error
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "customer.message_sync.lease_expired",
			Resource:    "customer_message_sync_task",
			ResourceID:  taskID.String(),
			Status:      "failed",
			Message:     fmt.Sprintf("taskId=%s shopId=%s", taskID.String(), task.ShopID.String()),
		})
	}
	return nil
}

// RecoverLegacyRunning fails stuck rows without lease metadata.
func (s *Service) RecoverLegacyRunning(ctx context.Context, taskID uuid.UUID, legacyCutoff time.Time) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("customersync: no db")
	}
	var task CustomerMessageSyncTask
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
	_ = s.DB.WithContext(ctx).Model(&CustomerMessageSyncTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":        StatusFailed,
			"error_message": "legacy running task recovered (no lease)",
			"finished_at":   &fin,
			"locked_by":     nil,
			"locked_until":  nil,
			"execution_id":  nil,
			"heartbeat_at":  nil,
			"updated_at":    fin,
		}).Error
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "customer.message_sync.lease_expired",
			Resource:    "customer_message_sync_task",
			ResourceID:  taskID.String(),
			Status:      "failed",
			Message:     "legacy customer_message_sync_task recovered",
		})
	}
	return nil
}
