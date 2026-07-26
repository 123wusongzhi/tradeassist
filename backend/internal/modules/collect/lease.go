package collect

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasklease"
)

func (s *Service) collectLeaseTTL() time.Duration {
	sec := 600
	if s != nil && s.TaskLeaseTimeoutSeconds > 0 {
		sec = s.TaskLeaseTimeoutSeconds
	}
	colSec := 60
	if s != nil && s.CollectorTimeoutSeconds > 0 {
		colSec = s.CollectorTimeoutSeconds
	}
	if sec < colSec+60 {
		sec = colSec + 60
	}
	return time.Duration(sec) * time.Second
}

// tryClaimCollectTask atomically moves pending/retrying (due) to running with a lease.
func (s *Service) tryClaimCollectTask(ctx context.Context, taskID uuid.UUID, workerID string, lease time.Duration) (*CollectTask, *tasklease.ClaimResult, bool) {
	if s == nil || s.DB == nil {
		return nil, nil, false
	}
	claim, ok, err := tasklease.TryClaimPendingOrRetrying(ctx, s.DB, CollectTask{}.TableName(), StatusPending, StatusRetrying, StatusRunning, taskID, workerID, lease)
	if err != nil || !ok {
		return nil, nil, false
	}
	var task CollectTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return nil, nil, false
	}
	return &task, &claim, true
}

func (s *Service) startCollectLeaseRenewal(ctx context.Context, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult, leaseTTL time.Duration) (stop func()) {
	if s == nil || s.DB == nil || claim == nil {
		return func() {}
	}
	return tasklease.StartRenewal(ctx, s.DB, CollectTask{}.TableName(), StatusRunning, taskID, workerID, claim.ExecutionID, claim.LeaseVersion, leaseTTL)
}

func (s *Service) validateCollectLease(ctx context.Context, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult) error {
	if claim == nil {
		return tasklease.ErrLeaseLost
	}
	return tasklease.ValidateLease(ctx, s.DB, CollectTask{}.TableName(), StatusRunning, taskID, workerID, claim.ExecutionID, claim.LeaseVersion)
}

func (s *Service) finishCollectTask(ctx context.Context, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult, updates map[string]any) error {
	if err := s.validateCollectLease(ctx, taskID, workerID, claim); err != nil {
		slog.Warn("collect_lease_lost_on_finish", "taskId", taskID.String(), "workerId", workerID, "error", err.Error())
		return err
	}
	now := time.Now().UTC()
	updates["locked_by"] = nil
	updates["locked_until"] = nil
	updates["updated_at"] = now
	res := s.DB.WithContext(ctx).Model(&CollectTask{}).
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

// RecoverLeaseExpired is invoked by the task reaper when locked_until passes.
func (s *Service) RecoverLeaseExpired(ctx context.Context, taskID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("collect: no db")
	}
	now := time.Now().UTC()
	var task CollectTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	if task.Status != StatusRunning || task.LockedUntil == nil || !task.LockedUntil.Before(now) {
		return nil
	}
	_ = s.DB.WithContext(ctx).Model(&CollectTask{}).Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"locked_by":    nil,
			"locked_until": nil,
			"execution_id": nil,
			"heartbeat_at": nil,
			"updated_at":   now,
		}).Error
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	s.RecordTaskEvent(ctx, &task, TaskEventInput{
		EventType:    EventWorkerLeaseExpired,
		FromStatus:   StatusRunning,
		Message:      "worker lease expired",
		ErrorMessage: "worker lease expired",
		RetryCount:   task.RetryCount,
		MaxRetries:   s.effectiveMaxRetries(&task),
	})
	s.handleCollectJobError(ctx, &task, fmt.Errorf("worker lease expired"), "", nil)
	return nil
}

// RecoverLegacyRunning handles historical running rows without lease metadata.
func (s *Service) RecoverLegacyRunning(ctx context.Context, taskID uuid.UUID, legacyCutoff time.Time) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("collect: no db")
	}
	var task CollectTask
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
	now := time.Now().UTC()
	_ = s.DB.WithContext(ctx).Model(&CollectTask{}).Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"locked_by":    nil,
			"locked_until": nil,
			"execution_id": nil,
			"heartbeat_at": nil,
			"updated_at":   now,
		}).Error
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	s.RecordTaskEvent(ctx, &task, TaskEventInput{
		EventType:    EventWorkerLeaseRecovered,
		FromStatus:   StatusRunning,
		Message:      "legacy running task recovered (no lease)",
		ErrorMessage: "legacy running task recovered",
		RetryCount:   task.RetryCount,
		MaxRetries:   s.effectiveMaxRetries(&task),
	})
	s.handleCollectJobError(ctx, &task, fmt.Errorf("legacy running task recovered"), "", nil)
	return nil
}

func (s *Service) handleCollectPanic(parent context.Context, taskID uuid.UUID, workerID string, panicVal any) {
	if s == nil || s.DB == nil {
		return
	}
	ctx := parent
	if ctx == nil {
		ctx = context.Background()
	}
	var cur CollectTask
	if err := s.DB.WithContext(ctx).First(&cur, "id = ?", taskID).Error; err != nil {
		return
	}
	if cur.Status != StatusRunning || cur.LockedBy == nil || *cur.LockedBy != workerID {
		return
	}
	msg := truncateRunes(fmt.Sprintf("collect worker panic: %v", panicVal), 8000)
	s.failTask(ctx, &cur, StatusRunning, msg, map[string]any{"panic": true}, workerID, nil)
}
