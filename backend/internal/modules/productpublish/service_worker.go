package productpublish

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasklease"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Service) execTimeout() time.Duration {
	if s.TaskTimeout <= 0 {
		return 180 * time.Second
	}
	return s.TaskTimeout
}

func (s *Service) ProcessQueuedTask(ctx context.Context, taskID uuid.UUID, workerID string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("productpublish: no db")
	}
	var peek ProductPublishTask
	if err := s.DB.WithContext(ctx).Select("id", "platform", "task_type", "publish_mode").First(&peek, "id = ?", taskID).Error; err == nil {
		if peek.Platform == "douyin_shop" && (peek.TaskType == TaskTypeDouyinDraftCreate || peek.PublishMode == PublishModeSaveAsPlatformDraft) {
			return s.ProcessDouyinDraftTask(ctx, taskID, workerID)
		}
	}
	return s.processGenericPublishTask(ctx, taskID, workerID)
}

func (s *Service) processGenericPublishTask(ctx context.Context, taskID uuid.UUID, workerID string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("productpublish: no db")
	}
	defer func() {
		if r := recover(); r != nil {
			s.handlePublishPanic(ctx, taskID, workerID, r)
		}
	}()

	lease := s.publishLeaseTTL()
	taskRow, claim, claimed, err := s.tryClaimProductPublishTask(ctx, taskID, workerID, lease)
	if err != nil {
		return err
	}
	if !claimed || taskRow == nil {
		return nil
	}

	cancelRen := s.startPublishLeaseRenewal(ctx, taskID, workerID, claim, lease)
	defer cancelRen()

	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: taskRow.CreatedBy,
			Action:      "product.publish.running",
			Resource:    "product_publish_task",
			ResourceID:  taskID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s shopId=%s platform=%s", taskID.String(), taskRow.ShopID.String(), taskRow.Platform),
		})
	}
	if err := s.setProductPublishStage(ctx, taskID, workerID, claim, StatusChecking); err != nil {
		return err
	}

	failBeforeMutation := func(msg string) error {
		if err := s.finishGenericPreMutationFailure(ctx, taskRow, taskID, workerID, claim, msg); err != nil {
			return err
		}
		if s.OpLog != nil {
			_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
				AdminUserID: taskRow.CreatedBy,
				Action:      "product.publish.failed",
				Resource:    "product_publish_task",
				ResourceID:  taskID.String(),
				Status:      "failed",
				Message:     fmt.Sprintf("taskId=%s err=%s", taskID.String(), truncateMsg(msg)),
			})
		}
		return fmt.Errorf("%s", msg)
	}
	failAfterMutation := func(msg string, res *platformp.PublishProductResult) error {
		if strings.EqualFold(strings.TrimSpace(taskRow.Platform), "ozon") {
			if err := s.markGenericResultUnknown(ctx, taskRow, taskID, workerID, claim, res, msg); err != nil {
				if lateErr := s.recordLateGenericExternalFact(ctx, taskRow, taskID, res, msg); lateErr != nil {
					return fmt.Errorf("mark Ozon result unknown: %v; preserve late external fact: %w", err, lateErr)
				}
			}
			return fmt.Errorf("%s", msg)
		}
		if err := s.finishGenericPostMutationFailure(ctx, taskRow, taskID, workerID, claim, msg); err != nil {
			return err
		}
		return fmt.Errorf("%s", msg)
	}

	snap, err := parsePublishSnapshot(taskRow.Input)
	if err != nil {
		return failBeforeMutation(err.Error())
	}

	var draft platformp.PlatformProductDraft
	if snap.Draft != nil {
		draft = *snap.Draft
	} else {
		// Compatibility for tasks written before immutable draft snapshots.
		var prod product.Product
		if err := s.DB.WithContext(ctx).Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC, created_at ASC") }).Preload("SKUs", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC") }).Where("id = ? AND tenant_id = ?", taskRow.ProductID, taskRow.TenantID).First(&prod).Error; err != nil {
			return failBeforeMutation(fmt.Sprintf("load product: %v", err))
		}
		var buildErr error
		draft, buildErr = s.buildPlatformDraftForProductShop(ctx, prod, taskRow.Platform, &taskRow.ShopID, snap.MergedPublish, snap.MergedPublish["currency_code"])
		if buildErr != nil {
			return failBeforeMutation(buildErr.Error())
		}
	}

	_, plainAuth, err := s.Shops.PlainAuthForProviderCtx(ctx, taskRow.TenantID, taskRow.ShopID)
	if err != nil {
		return failBeforeMutation("shop not available")
	}

	prov := platformp.Get(strings.TrimSpace(strings.ToLower(taskRow.Platform)))
	if prov == nil || !platformp.IsProductPublishRunnable(prov) {
		return failBeforeMutation(platformp.ErrProductPublishNotImplemented.Error())
	}
	pp, ok := platformp.AsProductPublish(prov)
	if !ok || pp == nil {
		return failBeforeMutation(platformp.ErrProductPublishNotImplemented.Error())
	}

	pubCfg := stringifyPublishMap(snap.MergedPublish)
	req := platformp.PublishProductRequest{
		ShopID:        taskRow.ShopID,
		Platform:      taskRow.Platform,
		Auth:          plainAuth,
		Product:       draft,
		PublishConfig: pubCfg,
		Options:       snap.Options,
	}

	xctx, cancel := context.WithTimeout(ctx, s.execTimeout())
	defer cancel()
	// From this point onward a timeout/error cannot prove that Ozon did not
	// accept the import. Persist the boundary before invoking the provider.
	if err := s.setProductPublishStage(ctx, taskID, workerID, claim, StatusPublishing); err != nil {
		return err
	}
	res, pubErr := pp.PublishProduct(xctx, req)
	if pubErr != nil {
		msg := pubErr.Error()
		if err := failAfterMutation(msg, res); err != nil {
			return err
		}
		if errors.Is(pubErr, platformp.ErrPlatformProductPublishPermissionDenied) {
			return platformp.ErrPlatformProductPublishPermissionDenied
		}
		return fmt.Errorf("%s", msg)
	}
	if res == nil {
		return failAfterMutation("empty publish result", nil)
	}
	if strings.TrimSpace(res.ExternalProductID) == "" {
		return failAfterMutation("platform did not return external product id", res)
	}

	fin := time.Now().UTC()
	outSnap := platformp.TrimRawMap(map[string]any{
		"externalProductId": res.ExternalProductID,
		"externalSpuId":     res.ExternalSPUID,
		"externalUrl":       res.ExternalURL,
		"status":            res.Status,
		"providerSummary":   res.RawSummary,
	}, 20, 300)
	outSnap["warnings"] = res.Warnings
	rawOut, _ := json.Marshal(outSnap)

	pubStatus := publicationStatusForResult(taskRow.Platform, res)
	taskStatus, taskPublishStatus, outcomeCode, outcomeMessage := genericTaskOutcome(taskRow.Platform, pubStatus)
	if err := s.completeGenericPublicationResult(ctx, taskRow, taskID, workerID, claim, snap.PublicationID, res, fin, rawOut, pubStatus, taskStatus, taskPublishStatus, outcomeCode, outcomeMessage); err != nil {
		msg := fmt.Sprintf("persist platform publish result: %v", err)
		if unknownErr := s.markGenericResultUnknown(ctx, taskRow, taskID, workerID, claim, res, msg); unknownErr != nil {
			if lateErr := s.recordLateGenericExternalFact(ctx, taskRow, taskID, res, msg); lateErr != nil {
				return fmt.Errorf("%v; mark result unknown: %v; preserve late external fact: %w", err, unknownErr, lateErr)
			}
		}
		return fmt.Errorf("%s", msg)
	}

	if s.OpLog != nil {
		action, logStatus := "product.publish.success", "success"
		if taskStatus != TaskSuccess {
			action, logStatus = "product.publish.needs_attention", "failed"
		} else if pubStatus != StatusPublishedRecord {
			action = "product.publish.accepted"
		}
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: taskRow.CreatedBy,
			Action:      action,
			Resource:    "product_publish_task",
			ResourceID:  taskID.String(),
			Status:      logStatus,
			Message: fmt.Sprintf("taskId=%s publicationId=%s externalProductId=%s publishStatus=%s skuMappings=%d",
				taskID.String(), snap.PublicationID.String(), res.ExternalProductID, pubStatus, len(res.SKUMappings)),
		})
	}
	return nil
}

func (s *Service) completeGenericPublicationResult(ctx context.Context, task *ProductPublishTask, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult, publicationID uuid.UUID, res *platformp.PublishProductResult, finishedAt time.Time, rawOut []byte, publicationStatus, taskStatus, taskPublishStatus, errorCode, errorMessage string) error {
	if s == nil || s.DB == nil || task == nil || res == nil {
		return fmt.Errorf("productpublish: missing publish result persistence dependencies")
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.persistGenericPublicationResult(tx, task, publicationID, res, finishedAt, publicationStatus); err != nil {
			return err
		}
		return s.finishProductPublishTaskWithDB(ctx, tx, taskID, workerID, claim, map[string]any{
			"status":              taskStatus,
			"publish_status":      taskPublishStatus,
			"platform_product_id": strings.TrimSpace(res.ExternalProductID),
			"retryable":           false,
			"error_message":       errorMessage,
			"error_code":          errorCode,
			"finished_at":         &finishedAt,
			"output":              datatypes.JSON(rawOut),
			"platform_result":     datatypes.JSON(rawOut),
		})
	})
}

func (s *Service) persistGenericPublicationResult(tx *gorm.DB, task *ProductPublishTask, publicationID uuid.UUID, res *platformp.PublishProductResult, finishedAt time.Time, pubStatus string) error {
	if tx == nil || task == nil || res == nil {
		return fmt.Errorf("productpublish: missing publish result persistence dependencies")
	}
	pubSnap := map[string]any{
		"externalProductId": res.ExternalProductID,
		"externalSpuId":     res.ExternalSPUID,
		"externalUrl":       res.ExternalURL,
		"status":            pubStatus,
		"skuMapped":         len(res.SKUMappings),
		"warnings":          res.Warnings,
		"providerSummary":   platformp.TrimRawMap(res.RawSummary, 20, 300),
	}
	rd, err := json.Marshal(pubSnap)
	if err != nil {
		return fmt.Errorf("marshal publication result: %w", err)
	}
	var publishedAt *time.Time
	if pubStatus == StatusPublishedRecord {
		publishedAt = &finishedAt
	}
	result := tx.Model(&ProductPublication{}).Where("id = ? AND tenant_id = ?", publicationID, task.TenantID).
		Updates(map[string]any{
			"publish_status":      pubStatus,
			"status":              pubStatus,
			"external_product_id": strings.TrimSpace(res.ExternalProductID),
			"external_spu_id":     strings.TrimSpace(res.ExternalSPUID),
			"external_url":        strings.TrimSpace(res.ExternalURL),
			"published_at":        publishedAt,
			"last_synced_at":      &finishedAt,
			"raw_data":            datatypes.JSON(rd),
			"updated_at":          finishedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("publication %s not found in tenant %d", publicationID, task.TenantID)
	}
	if err := tx.Where("publication_id = ? AND EXISTS (SELECT 1 FROM product_publications p WHERE p.id = product_publication_skus.publication_id AND p.tenant_id = ?)", publicationID, task.TenantID).Delete(&ProductPublicationSKU{}).Error; err != nil {
		return err
	}
	for _, m := range res.SKUMappings {
		skuRow := ProductPublicationSKU{PublicationID: publicationID, ProductSKUID: nilUUIDPtr(m.LocalSKUID), ExternalSKUID: strings.TrimSpace(m.ExternalSKUID), SKUCode: strings.TrimSpace(m.SKUCode), Price: m.Price, Stock: m.Stock}
		if skuRow.ExternalSKUID == "" {
			continue
		}
		rawData := m.RawData
		if len(rawData) == 0 {
			rawData = platformp.TrimRawMap(map[string]any{"mapped": true}, 6, 80)
		}
		rawJSON, marshalErr := json.Marshal(platformp.TrimRawMap(rawData, 12, 200))
		if marshalErr != nil {
			return fmt.Errorf("marshal sku mapping: %w", marshalErr)
		}
		skuRow.RawData = datatypes.JSON(rawJSON)
		if err := tx.Create(&skuRow).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) finishGenericPreMutationFailure(ctx context.Context, task *ProductPublishTask, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult, message string) error {
	return s.finishGenericFailureState(ctx, task, taskID, workerID, claim, StatusPubFailed, ErrorPublishNotSent, message, true, map[string]any{
		"recoveryState": "definite_not_sent",
		"mutationSent":  false,
		"causeCode":     inferPublishErrorCode(message),
	})
}

func (s *Service) finishGenericPostMutationFailure(ctx context.Context, task *ProductPublishTask, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult, message string) error {
	return s.finishGenericFailureState(ctx, task, taskID, workerID, claim, StatusPubFailed, inferPublishErrorCode(message), message, false, map[string]any{
		"recoveryState": "failed_after_mutation",
		"mutationSent":  true,
	})
}

func (s *Service) finishGenericFailureState(ctx context.Context, task *ProductPublishTask, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult, publishStatus, errorCode, message string, retryable bool, output map[string]any) error {
	if s == nil || s.DB == nil || task == nil {
		return fmt.Errorf("productpublish: missing failure persistence dependencies")
	}
	fin := time.Now().UTC()
	raw, _ := json.Marshal(output)
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if publicationID, ok := snapshotPublicationFromTask(task); ok {
			result := tx.Model(&ProductPublication{}).Where("id = ? AND tenant_id = ?", publicationID, task.TenantID).
				Updates(map[string]any{"status": publishStatus, "publish_status": publishStatus, "updated_at": fin})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return fmt.Errorf("publication %s not found in tenant %d", publicationID, task.TenantID)
			}
		}
		return s.finishProductPublishTaskWithDB(ctx, tx, taskID, workerID, claim, map[string]any{
			"status":         TaskFailed,
			"publish_status": publishStatus,
			"error_code":     errorCode,
			"error_message":  message,
			"retryable":      retryable,
			"finished_at":    &fin,
			"output":         datatypes.JSON(raw),
		})
	})
}

func (s *Service) markGenericResultUnknown(ctx context.Context, task *ProductPublishTask, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult, res *platformp.PublishProductResult, cause string) error {
	if s == nil || s.DB == nil || task == nil {
		return fmt.Errorf("productpublish: missing result recovery dependencies")
	}
	fin := time.Now().UTC()
	out := genericPublishOutput(res, StatusResultUnknown)
	out["recoveryState"] = StatusResultUnknown
	out["mutationSent"] = true
	out["cause"] = truncateMsg(cause)
	raw, _ := json.Marshal(out)
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if publicationID, ok := snapshotPublicationFromTask(task); ok {
			updates := map[string]any{
				"status": StatusResultUnknown, "publish_status": StatusResultUnknown,
				"raw_data": datatypes.JSON(raw), "updated_at": fin,
			}
			if res != nil {
				// Keep ExternalSPUID in raw_data here: this path must remain usable
				// even when the schema mismatch itself caused the primary failure.
				updates["external_product_id"] = strings.TrimSpace(res.ExternalProductID)
				updates["external_url"] = strings.TrimSpace(res.ExternalURL)
			}
			result := tx.Model(&ProductPublication{}).Where("id = ? AND tenant_id = ?", publicationID, task.TenantID).Updates(updates)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return fmt.Errorf("publication %s not found in tenant %d", publicationID, task.TenantID)
			}
		}
		externalID := ""
		if res != nil {
			externalID = strings.TrimSpace(res.ExternalProductID)
		}
		return s.finishProductPublishTaskWithDB(ctx, tx, taskID, workerID, claim, map[string]any{
			"status": TaskFailed, "publish_status": StatusResultUnknown,
			"platform_product_id": externalID, "retryable": false,
			"error_code": ErrorPublishResultUnknown, "error_message": cause,
			"finished_at": &fin, "output": datatypes.JSON(raw), "platform_result": datatypes.JSON(raw),
		})
	})
}

// recordLateGenericExternalFact is the last-resort path when the worker loses
// its lease after a provider already returned. It cannot turn the task into a
// success; it only preserves the external ID/evidence and forces an uncertain
// terminal state so no automatic retry can create a duplicate.
func (s *Service) recordLateGenericExternalFact(ctx context.Context, original *ProductPublishTask, taskID uuid.UUID, res *platformp.PublishProductResult, cause string) error {
	if s == nil || s.DB == nil || original == nil || !strings.EqualFold(strings.TrimSpace(original.Platform), "ozon") {
		return nil
	}
	now := time.Now().UTC()
	out := genericPublishOutput(res, StatusResultUnknown)
	out["recoveryState"], out["mutationSent"], out["lateResult"] = StatusResultUnknown, true, true
	out["cause"] = truncateMsg(cause)
	raw, _ := json.Marshal(out)
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current ProductPublishTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", taskID, original.TenantID).First(&current).Error; err != nil {
			return err
		}
		if current.Status == TaskSuccess {
			return nil
		}
		if current.Status == TaskRunning && (current.LockedUntil == nil || !current.LockedUntil.Before(now)) {
			return fmt.Errorf("task still has an active lease")
		}
		if current.Status != TaskRunning && current.Status != TaskFailed && current.Status != TaskCancelled {
			return fmt.Errorf("task state %s cannot accept a late external result", current.Status)
		}
		publicationID, ok := snapshotPublicationFromTask(&current)
		if !ok {
			return fmt.Errorf("task snapshot does not contain a publication")
		}
		pubUpdates := map[string]any{
			"status": StatusResultUnknown, "publish_status": StatusResultUnknown,
			"raw_data": datatypes.JSON(raw), "updated_at": now,
		}
		externalID := ""
		if res != nil {
			externalID = strings.TrimSpace(res.ExternalProductID)
			pubUpdates["external_product_id"] = externalID
			pubUpdates["external_url"] = strings.TrimSpace(res.ExternalURL)
		}
		publication := tx.Model(&ProductPublication{}).Where("id = ? AND tenant_id = ?", publicationID, current.TenantID).Updates(pubUpdates)
		if publication.Error != nil {
			return publication.Error
		}
		if publication.RowsAffected != 1 {
			return fmt.Errorf("publication %s not found in tenant %d", publicationID, current.TenantID)
		}
		result := tx.Model(&ProductPublishTask{}).Where("id = ? AND tenant_id = ? AND status = ? AND lock_version = ?", taskID, current.TenantID, current.Status, current.LockVersion).
			Updates(map[string]any{
				"status": TaskFailed, "publish_status": StatusResultUnknown,
				"platform_product_id": externalID, "retryable": false,
				"error_code": ErrorPublishResultUnknown, "error_message": cause,
				"finished_at": &now, "output": datatypes.JSON(raw), "platform_result": datatypes.JSON(raw),
				"locked_by": nil, "locked_until": nil, "updated_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return tasklease.ErrLeaseLost
		}
		return nil
	})
}

func genericPublishOutput(res *platformp.PublishProductResult, status string) map[string]any {
	out := map[string]any{"status": status}
	if res == nil {
		return out
	}
	out["externalProductId"] = strings.TrimSpace(res.ExternalProductID)
	out["externalSpuId"] = strings.TrimSpace(res.ExternalSPUID)
	out["externalUrl"] = strings.TrimSpace(res.ExternalURL)
	out["providerStatus"] = strings.TrimSpace(res.Status)
	out["providerSummary"] = platformp.TrimRawMap(res.RawSummary, 20, 300)
	out["warnings"] = res.Warnings
	return out
}

func truncateMsg(msg string) string {
	runes := []rune(msg)
	if len(runes) > 480 {
		return string(runes[:480]) + "…"
	}
	return msg
}

func nilUUIDPtr(u uuid.UUID) *uuid.UUID {
	if u == uuid.Nil {
		return nil
	}
	return &u
}

func normalizePublicationStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case StatusDraft:
		return StatusDraft
	case StatusPublishing, "submitted", "processing":
		return StatusPublishing
	case platformp.PublishStatusImported:
		return StatusImported
	case platformp.PublishStatusPendingReview:
		return StatusPendingReview
	case platformp.PublishStatusNeedsAction:
		return StatusNeedsAction
	case platformp.PublishStatusSellable, StatusPublishedRecord, "success", "live", "active":
		return StatusPublishedRecord
	case StatusRejected:
		return StatusRejected
	case StatusOffline:
		return StatusOffline
	case StatusPubFailed:
		return StatusPubFailed
	case StatusResultUnknown:
		return StatusResultUnknown
	default:
		return StatusResultUnknown
	}
}

func publicationStatusForResult(platform string, res *platformp.PublishProductResult) string {
	if res == nil {
		return StatusResultUnknown
	}
	status := normalizePublicationStatus(res.Status)
	if !strings.EqualFold(strings.TrimSpace(platform), "ozon") || status != StatusPublishedRecord {
		return status
	}
	verified, _ := res.RawSummary["sellableVerified"].(bool)
	if strings.EqualFold(strings.TrimSpace(res.Status), platformp.PublishStatusSellable) || verified {
		return StatusPublishedRecord
	}
	return StatusResultUnknown
}

func genericTaskOutcome(platform, publicationStatus string) (taskStatus, taskPublishStatus, errorCode, errorMessage string) {
	isOzon := strings.EqualFold(strings.TrimSpace(platform), "ozon")
	switch publicationStatus {
	case StatusNeedsAction:
		return TaskFailed, StatusNeedsAction, ErrorPublishNeedsAction, "Ozon 已接收商品，但返回需要处理的校验或库存问题"
	case StatusResultUnknown:
		return TaskFailed, StatusResultUnknown, ErrorPublishResultUnknown, "平台写入结果未确认，需要对账后再决定是否重试"
	case StatusRejected, StatusPubFailed:
		return TaskFailed, publicationStatus, inferPublishErrorCode(publicationStatus), "平台未接受发布结果"
	}
	if isOzon {
		return TaskSuccess, publicationStatus, "", ""
	}
	return TaskSuccess, StatusSuccess, "", ""
}

func (s *Service) handlePublishPanic(parent context.Context, taskID uuid.UUID, workerID string, panicVal any) {
	if s == nil || s.DB == nil {
		return
	}
	ctx := parent
	if ctx == nil {
		ctx = context.Background()
	}
	var cur ProductPublishTask
	if err := s.DB.WithContext(ctx).First(&cur, "id = ?", taskID).Error; err != nil {
		return
	}
	if cur.Status != TaskRunning || cur.LockedBy == nil || *cur.LockedBy != workerID {
		return
	}
	msg := fmt.Sprintf("publish worker panic: %v", panicVal)
	if strings.EqualFold(strings.TrimSpace(cur.Platform), "ozon") && cur.ExecutionID != nil {
		if executionID, err := uuid.Parse(strings.TrimSpace(*cur.ExecutionID)); err == nil {
			claim := &tasklease.ClaimResult{ExecutionID: executionID, LeaseVersion: cur.LockVersion}
			if strings.TrimSpace(cur.PublishStatus) == StatusPublishing {
				if recoveryErr := s.markGenericResultUnknown(ctx, &cur, taskID, workerID, claim, nil, msg); recoveryErr != nil {
					_ = s.recordLateGenericExternalFact(ctx, &cur, taskID, nil, msg)
				}
			} else {
				_ = s.finishGenericPreMutationFailure(ctx, &cur, taskID, workerID, claim, msg)
			}
			slog.Warn("product_publish_worker_panic_recovered", "taskId", taskID.String(), "worker", workerID)
			return
		}
	}
	fin := time.Now().UTC()
	_ = s.DB.WithContext(ctx).Model(&ProductPublishTask{}).Where("id = ? AND tenant_id = ?", taskID, cur.TenantID).
		Updates(map[string]any{
			"status":         TaskFailed,
			"publish_status": StatusPubFailed,
			"error_code":     inferPublishErrorCode(msg),
			"error_message":  msg,
			"finished_at":    &fin,
			"locked_by":      nil,
			"locked_until":   nil,
			"updated_at":     fin,
		}).Error
	if rid, ok := snapshotPublicationFromTask(&cur); ok {
		_ = s.DB.WithContext(ctx).Model(&ProductPublication{}).Where("id = ? AND tenant_id = ?", rid, cur.TenantID).
			Updates(map[string]any{
				"status":         StatusPubFailed,
				"publish_status": StatusPubFailed,
				"updated_at":     fin,
			}).Error
	}
	slog.Warn("product_publish_worker_panic_recovered", "taskId", taskID.String(), "worker", workerID)
}
