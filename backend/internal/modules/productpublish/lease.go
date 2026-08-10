package productpublish

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasklease"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func (s *Service) publishLeaseTTL() time.Duration {
	if s != nil && s.TaskTimeout > 0 {
		return s.TaskTimeout
	}
	return 180 * time.Second
}

func (s *Service) tryClaimProductPublishTask(ctx context.Context, taskID uuid.UUID, workerID string, lease time.Duration) (*ProductPublishTask, *tasklease.ClaimResult, bool, error) {
	if s == nil || s.DB == nil {
		return nil, nil, false, fmt.Errorf("productpublish: no db")
	}
	claim, ok, err := tasklease.TryClaim(ctx, s.DB, ProductPublishTask{}.TableName(), TaskPending, TaskRunning, taskID, workerID, lease)
	if err != nil {
		return nil, nil, false, err
	}
	if !ok {
		return nil, nil, false, nil
	}
	var task ProductPublishTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return nil, nil, false, err
	}
	return &task, &claim, true, nil
}

func (s *Service) startPublishLeaseRenewal(ctx context.Context, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult, leaseTTL time.Duration) func() {
	if s == nil || s.DB == nil || claim == nil {
		return func() {}
	}
	return tasklease.StartRenewal(ctx, s.DB, ProductPublishTask{}.TableName(), TaskRunning, taskID, workerID, claim.ExecutionID, claim.LeaseVersion, leaseTTL)
}

func (s *Service) validatePublishLease(ctx context.Context, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult) error {
	if claim == nil {
		return tasklease.ErrLeaseLost
	}
	return tasklease.ValidateLease(ctx, s.DB, ProductPublishTask{}.TableName(), TaskRunning, taskID, workerID, claim.ExecutionID, claim.LeaseVersion)
}

func (s *Service) finishProductPublishTask(ctx context.Context, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult, updates map[string]any) error {
	return s.finishProductPublishTaskWithDB(ctx, s.DB, taskID, workerID, claim, updates)
}

// finishProductPublishTaskWithDB performs the lease check and terminal update
// in one SQL statement. Passing a transaction lets callers atomically commit a
// task outcome together with its publication rows.
func (s *Service) finishProductPublishTaskWithDB(ctx context.Context, db *gorm.DB, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult, updates map[string]any) error {
	if db == nil || claim == nil {
		return tasklease.ErrLeaseLost
	}
	now := time.Now().UTC()
	terminal := make(map[string]any, len(updates)+3)
	for key, value := range updates {
		terminal[key] = value
	}
	terminal["locked_by"] = nil
	terminal["locked_until"] = nil
	terminal["updated_at"] = now
	res := db.WithContext(ctx).Model(&ProductPublishTask{}).
		Where("id = ? AND status = ? AND locked_by = ? AND execution_id = ? AND lock_version = ? AND locked_until IS NOT NULL AND locked_until > ?",
			taskID, TaskRunning, workerID, claim.ExecutionID.String(), claim.LeaseVersion, now).
		Updates(terminal)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		slog.Warn("product_publish_lease_lost_on_finish", "taskId", taskID.String(), "workerId", workerID)
		return tasklease.ErrLeaseLost
	}
	return nil
}

func (s *Service) setProductPublishStage(ctx context.Context, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult, stage string) error {
	if s == nil || s.DB == nil || claim == nil {
		return tasklease.ErrLeaseLost
	}
	now := time.Now().UTC()
	res := s.DB.WithContext(ctx).Model(&ProductPublishTask{}).
		Where("id = ? AND status = ? AND locked_by = ? AND execution_id = ? AND lock_version = ? AND locked_until IS NOT NULL AND locked_until > ?",
			taskID, TaskRunning, workerID, claim.ExecutionID.String(), claim.LeaseVersion, now).
		Updates(map[string]any{"publish_status": stage, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return tasklease.ErrLeaseLost
	}
	return nil
}

func (s *Service) RecoverLeaseExpired(ctx context.Context, taskID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("productpublish: no db")
	}
	now := time.Now().UTC()
	var task ProductPublishTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	if task.Status != TaskRunning || task.LockedUntil == nil || !task.LockedUntil.Before(now) {
		return nil
	}
	fin := now
	msg := "worker lease expired"
	code := platformdouyin.CodeDouyinTaskStale
	recovery := platformdouyin.RecoveryStale
	applied := false
	if task.Platform == "douyin_shop" {
		out := platformdouyin.MarshalRecoveryOutput(nil, platformdouyin.TaskRecoveryMeta{
			RecoveryStatus: recovery,
			LastErrorCode:  code,
			UserMessage:    platformdouyin.UserMessageForRecovery(recovery),
			TechnicalCode:  code,
		})
		recoveryUpdate := s.DB.WithContext(ctx).Model(&ProductPublishTask{}).
			Where("id = ? AND tenant_id = ? AND status = ? AND locked_until IS NOT NULL AND locked_until < ? AND lock_version = ?", taskID, task.TenantID, TaskRunning, now, task.LockVersion)
		if task.LockedBy != nil {
			recoveryUpdate = recoveryUpdate.Where("locked_by = ?", *task.LockedBy)
		}
		if task.ExecutionID != nil {
			recoveryUpdate = recoveryUpdate.Where("execution_id = ?", *task.ExecutionID)
		}
		result := recoveryUpdate.
			Updates(map[string]any{
				"status":        TaskFailed,
				"error_code":    code,
				"error_message": platformdouyin.UserMessageForRecovery(recovery),
				"retryable":     true,
				"finished_at":   &fin,
				"output":        datatypes.JSON(out),
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
		applied = true
	} else {
		publishStatus, errorCode, errorMessage, retryable := StatusPubFailed, inferPublishErrorCode(msg), msg, false
		out := map[string]any{"reason": "lease_expired"}
		if strings.EqualFold(strings.TrimSpace(task.Platform), "ozon") {
			if strings.TrimSpace(task.PublishStatus) == StatusPublishing {
				publishStatus, errorCode, errorMessage = StatusResultUnknown, ErrorPublishResultUnknown, "Ozon 发布请求执行期间 worker 租约过期；平台结果需要对账"
				out["recoveryState"], out["mutationSent"] = StatusResultUnknown, true
			} else {
				publishStatus, errorCode, errorMessage, retryable = StatusPubFailed, ErrorPublishNotSent, "Ozon 发布请求发出前 worker 租约过期；可安全重新创建或重试", true
				out["recoveryState"], out["mutationSent"] = "definite_not_sent", false
			}
		}
		raw, _ := json.Marshal(out)
		if err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			recoveryUpdate := tx.Model(&ProductPublishTask{}).
				Where("id = ? AND tenant_id = ? AND status = ? AND locked_until IS NOT NULL AND locked_until < ? AND lock_version = ?", taskID, task.TenantID, TaskRunning, now, task.LockVersion)
			if task.LockedBy != nil {
				recoveryUpdate = recoveryUpdate.Where("locked_by = ?", *task.LockedBy)
			}
			if task.ExecutionID != nil {
				recoveryUpdate = recoveryUpdate.Where("execution_id = ?", *task.ExecutionID)
			}
			result := recoveryUpdate.Updates(map[string]any{
				"status": TaskFailed, "publish_status": publishStatus,
				"error_code": errorCode, "error_message": errorMessage, "retryable": retryable,
				"finished_at": &fin, "output": datatypes.JSON(raw),
				"locked_by": nil, "locked_until": nil, "updated_at": fin,
			})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return nil
			}
			if rid, ok := snapshotPublicationFromTask(&task); ok {
				publication := tx.Model(&ProductPublication{}).Where("id = ? AND tenant_id = ?", rid, task.TenantID).
					Updates(map[string]any{"status": publishStatus, "publish_status": publishStatus, "updated_at": fin})
				if publication.Error != nil {
					return publication.Error
				}
				if publication.RowsAffected != 1 {
					return fmt.Errorf("publication %s not found in tenant %d", rid, task.TenantID)
				}
			}
			applied = true
			return nil
		}); err != nil {
			return err
		}
		if !applied {
			return nil
		}
	}
	if task.Platform == "douyin_shop" {
		if rid, ok := snapshotPublicationFromTask(&task); ok {
			_ = s.DB.WithContext(ctx).Model(&ProductPublication{}).Where("id = ? AND tenant_id = ?", rid, task.TenantID).
				Updates(map[string]any{
					"status":         StatusPubFailed,
					"publish_status": StatusPubFailed,
					"updated_at":     fin,
				}).Error
		}
	}
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "product.publish.failed",
			Resource:    "product_publish_task",
			ResourceID:  taskID.String(),
			Status:      "failed",
			Message:     fmt.Sprintf("taskId=%s reason=lease_expired shopId=%s", taskID.String(), task.ShopID.String()),
		})
	}
	return nil
}

func (s *Service) RecoverLegacyRunning(ctx context.Context, taskID uuid.UUID, legacyCutoff time.Time) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("productpublish: no db")
	}
	var task ProductPublishTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	if task.Status != TaskRunning {
		return nil
	}
	if task.LockedBy != nil && task.LockedUntil != nil {
		return nil
	}
	if !task.UpdatedAt.Before(legacyCutoff) {
		return nil
	}
	fin := time.Now().UTC()
	msg := "legacy running publish task recovered (no lease)"
	result := s.DB.WithContext(ctx).Model(&ProductPublishTask{}).
		Where("id = ? AND tenant_id = ? AND status = ?", taskID, task.TenantID, TaskRunning).
		Where("locked_by IS NULL AND locked_until IS NULL AND execution_id IS NULL AND heartbeat_at IS NULL").
		Where("lock_version = ? AND updated_at < ?", task.LockVersion, legacyCutoff).
		Updates(map[string]any{
			"status":        TaskFailed,
			"error_message": msg,
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
