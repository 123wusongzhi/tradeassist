package productpublish

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	douyinmetrics "github.com/trademind-ai/trademind/backend/internal/metrics/douyin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"gorm.io/datatypes"
)

func (s *Service) guardDouyinWorker(ctx context.Context, taskID uuid.UUID, shopID uuid.UUID, feature string, isScheduled bool, createdBy *uuid.UUID) error {
	if err := s.validateDouyinTaskTenant(ctx, taskID, shopID); err != nil {
		return s.failDouyinTenantMismatch(ctx, taskID, err, createdBy)
	}
	if ge := platformdouyin.GuardWorkerWithShop(ctx, shopID.String(), feature, true, isScheduled); ge != nil {
		douyinmetrics.RecordRuntimeBlockedTask()
		return s.blockDouyinTask(ctx, taskID, ge, createdBy)
	}
	return nil
}

// validateDouyinTaskTenant is the common fail-closed boundary for every
// Douyin worker and recovery path. It deliberately accepts exact tenant 0,
// which is the system tenant, but never an inferred or mixed tenant tuple.
func (s *Service) validateDouyinTaskTenant(ctx context.Context, taskID, shopID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("productpublish: no db")
	}
	var task ProductPublishTask
	if err := s.DB.WithContext(ctx).Select("id", "tenant_id", "product_id", "shop_id", "input").First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	if task.ShopID != shopID {
		return fmt.Errorf("douyin task shop does not match claimed shop")
	}
	var count int64
	if err := s.DB.WithContext(ctx).Table("products AS p").Joins("JOIN shops s ON s.id = ?", task.ShopID).
		Where("p.id = ? AND p.tenant_id = ? AND s.tenant_id = ?", task.ProductID, task.TenantID, task.TenantID).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("douyin task, product, and shop tenants must match")
	}
	snap, ok := parseDouyinDraftSnapshot(task.Input)
	if !ok || strings.TrimSpace(snap.ConfigID) == "" {
		return fmt.Errorf("douyin task mapping snapshot is invalid")
	}
	var cfg product.ProductPlatformPublishConfig
	if err := s.DB.WithContext(ctx).Joins("JOIN products p ON p.id = product_platform_publish_configs.product_id AND p.tenant_id = ?", task.TenantID).
		Where("product_platform_publish_configs.id = ? AND product_platform_publish_configs.product_id = ? AND product_platform_publish_configs.platform = ?", snap.ConfigID, task.ProductID, "douyin_shop").First(&cfg).Error; err != nil {
		return fmt.Errorf("douyin mapping config is outside the validated tenant: %w", err)
	}
	if cfg.ShopID != nil && *cfg.ShopID != task.ShopID {
		return fmt.Errorf("douyin mapping shop does not match task shop")
	}
	return nil
}

func (s *Service) failDouyinTenantMismatch(ctx context.Context, taskID uuid.UUID, cause error, createdBy *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return cause
	}
	fin := time.Now().UTC()
	message := "douyin task tenant integrity check failed"
	_ = s.DB.WithContext(ctx).Model(&ProductPublishTask{}).Where("id = ?", taskID).Updates(map[string]any{
		"status": TaskFailed, "publish_status": StatusPubFailed, "error_code": ErrorDouyinTenantMismatch,
		"error_message": message, "retryable": false, "finished_at": &fin, "locked_by": nil, "locked_until": nil, "updated_at": fin,
	}).Error
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{AdminUserID: createdBy, Action: "douyin.product.tenant_mismatch", Resource: "product_publish_task", ResourceID: taskID.String(), Status: "failed", Message: cause.Error()})
	}
	return fmt.Errorf("%s: %w", ErrorDouyinTenantMismatch, cause)
}

func (s *Service) blockDouyinTask(ctx context.Context, taskID uuid.UUID, ge *platformdouyin.Error, createdBy *uuid.UUID) error {
	if s == nil || s.DB == nil || ge == nil {
		return ge
	}
	fin := time.Now().UTC()
	out := platformdouyin.MarshalRecoveryOutput(nil, platformdouyin.TaskRecoveryMeta{
		RecoveryStatus: platformdouyin.RecoverySkipped,
		LastErrorCode:  ge.Code,
		UserMessage:    ge.Message,
		TechnicalCode:  ge.Code,
	})
	_ = s.DB.WithContext(ctx).Model(&ProductPublishTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":         TaskCancelled,
			"publish_status": StatusPubFailed,
			"error_code":     ge.Code,
			"error_message":  ge.Message,
			"retryable":      false,
			"finished_at":    &fin,
			"output":         datatypes.JSON(out),
			"locked_by":      nil,
			"locked_until":   nil,
			"updated_at":     fin,
		}).Error
	return ge
}

func (s *Service) markDouyinStale(ctx context.Context, taskID uuid.UUID, code, recoveryStatus string, createdBy *uuid.UUID) {
	if s == nil || s.DB == nil {
		return
	}
	douyinmetrics.RecordStaleTask()
	fin := time.Now().UTC()
	meta := platformdouyin.TaskRecoveryMeta{
		RecoveryStatus: recoveryStatus,
		LastErrorCode:  code,
		UserMessage:    platformdouyin.UserMessageForRecovery(recoveryStatus),
		TechnicalCode:  code,
	}
	out := platformdouyin.MarshalRecoveryOutput(nil, meta)
	_ = s.DB.WithContext(ctx).Model(&ProductPublishTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":         TaskFailed,
			"publish_status": StatusPubFailed,
			"error_code":     code,
			"error_message":  meta.UserMessage,
			"retryable":      true,
			"finished_at":    &fin,
			"output":         datatypes.JSON(out),
			"locked_by":      nil,
			"locked_until":   nil,
			"updated_at":     fin,
		}).Error
}

func (s *Service) touchDouyinTaskProgress(ctx context.Context, taskID uuid.UUID, patch map[string]any) {
	if s == nil || s.DB == nil {
		return
	}
	patch["updated_at"] = time.Now().UTC()
	_ = s.DB.WithContext(ctx).Model(&ProductPublishTask{}).Where("id = ?", taskID).Updates(patch).Error
}

func parseTaskOutputMap(raw datatypes.JSON) map[string]any {
	out := map[string]any{}
	if len(raw) == 0 {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func mergeTaskOutput(existing datatypes.JSON, patch map[string]any) datatypes.JSON {
	base := parseTaskOutputMap(existing)
	for k, v := range patch {
		base[k] = v
	}
	b, _ := json.Marshal(base)
	return datatypes.JSON(b)
}

// RecoverDouyinDraftStale attempts product.detail recovery for result_unknown tasks.
func (s *Service) RecoverDouyinDraftStale(ctx context.Context, tenantID int64, taskID uuid.UUID) error {
	if s == nil || s.DB == nil || s.Shops == nil {
		return nil
	}
	var task ProductPublishTask
	if err := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ?", taskID, tenantID).First(&task).Error; err != nil {
		return err
	}
	if task.Platform != "douyin_shop" || task.Status == TaskSuccess || task.Status == TaskCancelled {
		return nil
	}
	if err := s.guardDouyinWorker(ctx, taskID, task.ShopID, platformdouyin.FeatureProductDraft, false, task.CreatedBy); err != nil {
		return err
	}
	client, _, err := s.Shops.DouyinClientForShopContext(ctx, task.TenantID, task.ShopID, task.CreatedBy)
	if err != nil {
		return err
	}
	res, recovered, recErr := tryRecoverDouyinDraftFromPlatform(ctx, client, task.ShopID.String(), task.ProductID.String())
	if recErr != nil {
		return recErr
	}
	if !recovered || res == nil {
		s.markDouyinStale(ctx, taskID, platformdouyin.CodeDouyinTaskRecoveryRequired, platformdouyin.RecoveryRequired, task.CreatedBy)
		douyinmetrics.RecordRecoveryFailed()
		return nil
	}
	snap, ok := parseDouyinDraftSnapshot(task.Input)
	if !ok {
		return nil
	}
	buildRes, err := BuildDouyinProductPayload(ctx, s.DB, task.TenantID, task.ProductID, snap.ConfigID)
	if err != nil {
		return err
	}
	douyinmetrics.RecordRecoverySuccess()
	return s.completeDouyinDraftSuccess(ctx, &task, taskID, "", nil, snap, buildRes, res)
}
