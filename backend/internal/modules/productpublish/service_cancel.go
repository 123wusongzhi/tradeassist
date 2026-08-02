package productpublish

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

// CancelTask marks a pending/running publish task as cancelled.
func (s *Service) CancelTask(c *gin.Context, taskID uuid.UUID, adminID *uuid.UUID) (*TaskDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("productpublish: no db")
	}
	tid, err := trustedTenantOrLegacy(c)
	if err != nil {
		return nil, err
	}
	var task ProductPublishTask
	if err := s.DB.WithContext(c.Request.Context()).Where("id = ? AND tenant_id = ?", taskID, tid).First(&task).Error; err != nil {
		return nil, err
	}
	if !adminperm.RequireStoreOperate(c, s.DB, task.ShopID) {
		return nil, fmt.Errorf("store operate permission required")
	}
	st := strings.TrimSpace(task.Status)
	if st != TaskPending && st != TaskRunning {
		return nil, fmt.Errorf("only pending or running tasks can be cancelled")
	}
	fin := time.Now().UTC()
	if err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		update := tx.Model(&ProductPublishTask{}).
			Where("id = ? AND tenant_id = ? AND status = ? AND lock_version = ?", taskID, tid, st, task.LockVersion)
		if st == TaskRunning {
			if task.LockedBy != nil {
				update = update.Where("locked_by = ?", *task.LockedBy)
			}
			if task.ExecutionID != nil {
				update = update.Where("execution_id = ?", *task.ExecutionID)
			}
		}
		result := update.Updates(map[string]any{
			"status": TaskCancelled, "publish_status": TaskCancelled, "finished_at": &fin,
			"locked_by": nil, "locked_until": nil, "updated_at": fin,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("publish task state changed; cancel was not applied")
		}
		publicationID := uuid.Nil
		if snap, ok := parseDouyinDraftSnapshot(task.Input); ok {
			publicationID = snap.PublicationID
		} else if rid, ok := snapshotPublicationFromTask(&task); ok {
			publicationID = rid
		}
		if publicationID != uuid.Nil {
			if err := tx.Model(&ProductPublication{}).Where("id = ? AND tenant_id = ?", publicationID, tid).
				Updates(map[string]any{"status": TaskCancelled, "publish_status": TaskCancelled, "updated_at": fin}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	action := "product.publish.cancel"
	if task.Platform == "douyin_shop" {
		action = "douyin.product.publish_task.cancel"
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      action,
			Resource:    "product_publish_task",
			ResourceID:  taskID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s platform=%s", taskID, task.Platform),
		})
	}
	out, err := s.GetDTO(c.Request.Context(), tid, taskID)
	return &out, err
}
