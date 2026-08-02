package imagetask

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasklease"
)

type imageLeaseCtxKey struct{}

type imageLeaseBag struct {
	WorkerID string
	Claim    *tasklease.ClaimResult
}

func withImageLease(ctx context.Context, workerID string, claim *tasklease.ClaimResult) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if claim == nil {
		return ctx
	}
	return context.WithValue(ctx, imageLeaseCtxKey{}, imageLeaseBag{WorkerID: workerID, Claim: claim})
}

func imageLeaseFrom(ctx context.Context) (string, *tasklease.ClaimResult) {
	if ctx == nil {
		return "", nil
	}
	v, ok := ctx.Value(imageLeaseCtxKey{}).(imageLeaseBag)
	if !ok || v.Claim == nil {
		return "", nil
	}
	return v.WorkerID, v.Claim
}

func (s *Service) computeExecutionTimeout(ctx context.Context, task *ImageTask) time.Duration {
	if task == nil {
		return 60 * time.Second
	}
	timeout := imageOperationTimeout(ctx, s.Settings)
	if strings.EqualFold(strings.TrimSpace(task.Provider), "comfyui") {
		if b := s.comfyUIExecutionBudget(ctx); b > timeout {
			timeout = b
		}
	}
	if s != nil && s.TaskTimeoutMax > 0 && timeout > s.TaskTimeoutMax {
		timeout = s.TaskTimeoutMax
	}
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return timeout
}

func (s *Service) tryClaimImageTask(ctx context.Context, taskID uuid.UUID, workerID string, lease time.Duration) (*ImageTask, *tasklease.ClaimResult, bool, error) {
	if s == nil || s.DB == nil {
		return nil, nil, false, fmt.Errorf("imagetask: no db")
	}
	claim, ok, err := tasklease.TryClaimPendingOrRetrying(ctx, s.DB, ImageTask{}.TableName(), StatusPending, StatusRetrying, StatusRunning, taskID, workerID, lease)
	if err != nil {
		return nil, nil, false, err
	}
	if !ok {
		return nil, nil, false, nil
	}
	var task ImageTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return nil, nil, false, err
	}
	return &task, &claim, true, nil
}

func (s *Service) startImageLeaseRenewal(ctx context.Context, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult, leaseTTL time.Duration) (stop func()) {
	if s == nil || s.DB == nil || claim == nil {
		return func() {}
	}
	return tasklease.StartRenewal(ctx, s.DB, ImageTask{}.TableName(), StatusRunning, taskID, workerID, claim.ExecutionID, claim.LeaseVersion, leaseTTL)
}

func (s *Service) validateImageLease(ctx context.Context, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult) error {
	if claim == nil {
		return tasklease.ErrLeaseLost
	}
	return tasklease.ValidateLease(ctx, s.DB, ImageTask{}.TableName(), StatusRunning, taskID, workerID, claim.ExecutionID, claim.LeaseVersion)
}

func (s *Service) finishImageTask(ctx context.Context, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult, updates map[string]any) error {
	if err := s.validateImageLease(ctx, taskID, workerID, claim); err != nil {
		slog.Warn("image_lease_lost_on_finish", "taskId", taskID.String(), "workerId", workerID, "error", err.Error())
		return err
	}
	now := time.Now().UTC()
	updates["locked_by"] = nil
	updates["locked_until"] = nil
	updates["updated_at"] = now
	res := s.DB.WithContext(ctx).Model(&ImageTask{}).
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

func (s *Service) handleImagePanic(parent context.Context, httpCtx *gin.Context, taskID uuid.UUID, workerID string, panicVal any) {
	if s == nil || s.DB == nil {
		return
	}
	ctx := parent
	if ctx == nil {
		ctx = context.Background()
	}
	var cur ImageTask
	if err := s.DB.WithContext(ctx).First(&cur, "id = ?", taskID).Error; err != nil {
		return
	}
	if cur.Status != StatusRunning || cur.LockedBy == nil || *cur.LockedBy != workerID {
		return
	}
	msg := fmt.Sprintf("image worker panic: %v", panicVal)
	_ = s.finalizeImageFailed(ctx, httpCtx, &cur, redactSensitiveErr(msg), false)
}

// RecoverLeaseExpired requeues or fails a stale running image task.
func (s *Service) RecoverLeaseExpired(ctx context.Context, taskID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("imagetask: no db")
	}
	now := time.Now().UTC()
	var task ImageTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	if task.Status != StatusRunning || task.LockedUntil == nil || !task.LockedUntil.Before(now) {
		return nil
	}
	recovery := s.DB.WithContext(ctx).Model(&ImageTask{}).
		Where("id = ? AND tenant_id = ? AND status = ? AND locked_until IS NOT NULL AND locked_until < ? AND lock_version = ?", taskID, task.TenantID, StatusRunning, now, task.LockVersion)
	if task.LockedBy != nil {
		recovery = recovery.Where("locked_by = ?", *task.LockedBy)
	}
	if task.ExecutionID != nil {
		recovery = recovery.Where("execution_id = ?", *task.ExecutionID)
	}
	result := recovery.
		Updates(map[string]any{
			"locked_by":    nil,
			"locked_until": nil,
			"execution_id": nil,
			"heartbeat_at": nil,
			"updated_at":   now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "image.task.lease_expired",
			Resource:    "image_task",
			ResourceID:  taskID.String(),
			Status:      "failed",
			Message:     "worker lease expired (reaper)",
		})
	}
	return s.handleImageTaskFailure(ctx, nil, &task, ErrWorkerLeaseExpired)
}

// RecoverLegacyRunning clears stuck historical rows without lease metadata.
func (s *Service) RecoverLegacyRunning(ctx context.Context, taskID uuid.UUID, legacyCutoff time.Time) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("imagetask: no db")
	}
	var task ImageTask
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
	recovery := s.DB.WithContext(ctx).Model(&ImageTask{}).
		Where("id = ? AND tenant_id = ? AND status = ? AND locked_by IS NULL AND locked_until IS NULL AND execution_id IS NULL AND heartbeat_at IS NULL AND updated_at < ? AND lock_version = ?",
			taskID, task.TenantID, StatusRunning, legacyCutoff, task.LockVersion)
	result := recovery.
		Updates(map[string]any{
			"locked_by":    nil,
			"locked_until": nil,
			"execution_id": nil,
			"heartbeat_at": nil,
			"updated_at":   now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "image.task.lease_expired",
			Resource:    "image_task",
			ResourceID:  taskID.String(),
			Status:      "failed",
			Message:     "legacy running image task recovered",
		})
	}
	return s.handleImageTaskFailure(ctx, nil, &task, ErrWorkerLeaseExpired)
}
