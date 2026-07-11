package productpublish

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	douyinmetrics "github.com/trademind-ai/trademind/backend/internal/metrics/douyin"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productcheck"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasklease"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// DouyinCreateDraftBody POST create-draft request.
type DouyinCreateDraftBody struct {
	ShopID      string     `json:"shopId"`
	PublishMode string     `json:"publishMode"`
	Force       bool       `json:"force"`
	BatchID     *uuid.UUID `json:"-"`
}

type douyinDraftSnapshot struct {
	PublicationID uuid.UUID      `json:"publicationId"`
	ConfigID      string         `json:"configId,omitempty"`
	PublishMode   string         `json:"publishMode"`
	MappingHash   string         `json:"mappingHash,omitempty"`
	Mapping       map[string]any `json:"mappingSnapshot,omitempty"`
}

// CreateDouyinDraftTask validates readiness, builds payload, persists task and runs worker.
func (s *Service) CreateDouyinDraftTask(c *gin.Context, productID uuid.UUID, body DouyinCreateDraftBody, adminID *uuid.UUID) (*TaskDTO, error) {
	if s == nil || s.DB == nil || s.Shops == nil {
		return nil, fmt.Errorf("product publish unavailable")
	}
	ctx := c.Request.Context()
	publishMode := strings.TrimSpace(body.PublishMode)
	if publishMode == "" {
		publishMode = PublishModeSaveAsPlatformDraft
	}
	if publishMode != PublishModeSaveAsPlatformDraft {
		return nil, fmt.Errorf("only save_as_platform_draft is supported in this phase")
	}
	sid, err := uuid.Parse(strings.TrimSpace(body.ShopID))
	if err != nil || sid == uuid.Nil {
		return nil, fmt.Errorf("invalid shopId")
	}

	var cfg product.ProductPlatformPublishConfig
	if err := s.DB.WithContext(ctx).Where("product_id = ? AND platform = ?", productID, "douyin_shop").First(&cfg).Error; err != nil {
		return nil, fmt.Errorf("douyin mapping config not found")
	}
	if cfg.ShopID == nil || *cfg.ShopID != sid {
		return nil, fmt.Errorf("douyin shopId does not match saved config")
	}
	if cfg.LastMappedAt == nil || strings.TrimSpace(cfg.MappedTitle) == "" {
		return nil, fmt.Errorf("douyin mapping does not exist")
	}

	shopRow, _, err := s.Shops.PlainAuthForProviderCtx(ctx, sid)
	if err != nil || shopRow == nil {
		return nil, fmt.Errorf("shop not found")
	}
	if strings.TrimSpace(shopRow.AuthStatus) != shop.AuthAuthorized {
		return nil, fmt.Errorf("shop is not authorized")
	}

	var readinessSnap *productcheck.CheckProductReadinessResult
	if s.Readiness != nil {
		rres, err := s.Readiness.CheckProductReadiness(ctx, productcheck.CheckProductReadinessRequest{
			ProductID: productID,
			Platform:  "douyin_shop",
			ShopID:    &sid,
			Mode:      "publish",
		})
		if err != nil {
			return nil, err
		}
		if rres.ErrorCount > 0 {
			return nil, &productcheck.BlockedError{Result: rres}
		}
		if rres.WarningCount > 0 {
			readinessSnap = rres
		}
	}

	buildRes, err := BuildDouyinProductPayload(ctx, s.DB, productID, cfg.ID.String())
	if err != nil {
		return nil, err
	}
	if len(buildRes.Errors) > 0 {
		return nil, fmt.Errorf("douyin payload invalid: %s", buildRes.Errors[0].Message)
	}

	running := []string{TaskPending, TaskRunning}
	var existing ProductPublishTask
	dupQ := s.DB.WithContext(ctx).Where(
		"product_id = ? AND shop_id = ? AND platform = ? AND publish_mode = ? AND status IN ?",
		productID, sid, "douyin_shop", publishMode, running,
	)
	if err := dupQ.First(&existing).Error; err == nil {
		return nil, fmt.Errorf("a running douyin draft task already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var priorPub ProductPublication
	if err := s.DB.WithContext(ctx).Where("product_id = ? AND shop_id = ? AND platform = ? AND external_product_id <> ''",
		productID, sid, "douyin_shop").Order("updated_at DESC").First(&priorPub).Error; err == nil {
		if !body.Force {
			return nil, fmt.Errorf("platform product already exists; confirm to create again")
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	mapping := product.DouyinDraftMappingFromConfig(cfg)
	mapSnap, _ := json.Marshal(mapping)
	checkSnap, _ := json.Marshal(readinessSnap)
	payloadSnap, _ := json.Marshal(sanitizeDouyinPayloadForDisplay(buildRes.Payload))

	pubRow := ProductPublication{
		ProductID:     productID,
		ShopID:        sid,
		Platform:      "douyin_shop",
		Status:        StatusDraft,
		PublishStatus: StatusDraftCreated,
		Title:         strings.TrimSpace(buildRes.Payload.Name),
		Currency:      "CNY",
		CreatedBy:     adminID,
	}
	if err := s.DB.WithContext(ctx).Create(&pubRow).Error; err != nil {
		return nil, err
	}

	snap := douyinDraftSnapshot{
		PublicationID: pubRow.ID,
		ConfigID:      cfg.ID.String(),
		PublishMode:   publishMode,
		MappingHash:   douyinMappingContentHash(mapping),
	}
	snapRaw, _ := json.Marshal(snap)

	task := ProductPublishTask{
		ProductID:       productID,
		ShopID:          sid,
		TargetStoreID:   sid,
		Platform:        "douyin_shop",
		TaskType:        TaskTypeDouyinDraftCreate,
		Status:          TaskPending,
		PublishStatus:   StatusChecking,
		Mode:            publishMode,
		PublishMode:     publishMode,
		Title:           buildRes.Payload.Name,
		Description:     buildRes.Payload.Description,
		CheckResult:     datatypes.JSON(checkSnap),
		MappingSnapshot: datatypes.JSON(mapSnap),
		PlatformPayload: datatypes.JSON(payloadSnap),
		Input:           datatypes.JSON(snapRaw),
		CreatedBy:       adminID,
	}
	if err := s.DB.WithContext(ctx).Create(&task).Error; err != nil {
		return nil, err
	}
	_ = s.DB.WithContext(ctx).Model(&ProductPublication{}).Where("id = ?", pubRow.ID).
		Updates(map[string]any{"publish_task_id": task.ID}).Error

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "douyin.product.draft.create.start",
			Resource:    "product_publish_task",
			ResourceID:  task.ID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s productId=%s shopId=%s", task.ID, productID, sid),
		})
	}

	runInline := func() error {
		return s.ProcessDouyinDraftTask(context.Background(), task.ID, worker.GenerateInlineWorkerID(worker.TypeProductPublish))
	}
	if s.QueueEnabled && s.Redis != nil && s.Redis.Client != nil {
		var enqueueErr error
		if body.BatchID != nil && *body.BatchID != uuid.Nil {
			enqueueErr = s.enqueuePublishTaskIdempotent(ctx, c, *body.BatchID, task.ID, task.TaskType)
		} else {
			enqueueErr = s.enqueue(ctx, task.ID)
		}
		if enqueueErr != nil {
			slog.Warn("douyin_draft_enqueue_failed_run_inline", "taskId", task.ID.String(), "error", enqueueErr)
			if err := runInline(); err != nil {
				return nil, err
			}
		}
	} else if err := runInline(); err != nil {
		return nil, err
	}

	out, err := s.GetDTO(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	if readinessSnap != nil {
		out.Readiness = readinessSnap
	}
	return &out, nil
}

// ProcessDouyinDraftTask executes douyin_shop save_as_platform_draft publish tasks.
func (s *Service) ProcessDouyinDraftTask(ctx context.Context, taskID uuid.UUID, workerID string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("productpublish: no db")
	}
	taskRow, claim, claimed, err := s.tryClaimProductPublishTask(ctx, taskID, workerID, s.publishLeaseTTL())
	if err != nil || !claimed || taskRow == nil {
		return err
	}
	if err := s.guardDouyinWorker(ctx, taskID, taskRow.ShopID, platformdouyin.FeatureProductDraft, false, taskRow.CreatedBy); err != nil {
		return err
	}
	cancelRen := s.startPublishLeaseRenewal(ctx, taskID, workerID, claim, s.publishLeaseTTL())
	defer cancelRen()

	_ = s.DB.WithContext(ctx).Model(&ProductPublishTask{}).Where("id = ?", taskID).
		Updates(map[string]any{"publish_status": StatusCreatingPlatformDraft}).Error

	fail := func(code, msg string, retryable bool, requestID string, raw map[string]any) error {
		fin := time.Now().UTC()
		rawJSON, _ := json.Marshal(sanitizeRawErrorMap(raw))
		updates := map[string]any{
			"status":             TaskFailed,
			"publish_status":     StatusPubFailed,
			"error_code":         code,
			"error_message":      msg,
			"retryable":          retryable,
			"request_id":         requestID,
			"finished_at":        &fin,
			"platform_raw_error": datatypes.JSON(rawJSON),
		}
		_ = s.finishProductPublishTask(ctx, taskID, workerID, claim, updates)
		if snap, ok := parseDouyinDraftSnapshot(taskRow.Input); ok {
			_ = s.DB.WithContext(ctx).Model(&ProductPublication{}).Where("id = ?", snap.PublicationID).
				Updates(map[string]any{"status": StatusPubFailed, "publish_status": StatusPubFailed, "updated_at": fin}).Error
		}
		if s.OpLog != nil {
			_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
				AdminUserID: taskRow.CreatedBy,
				Action:      "douyin.product.draft.create.failed",
				Resource:    "product_publish_task",
				ResourceID:  taskID.String(),
				Status:      "failed",
				Message:     fmt.Sprintf("taskId=%s code=%s err=%s requestId=%s", taskID, code, truncateMsg(msg), requestID),
			})
		}
		douyinmetrics.RecordProductDraftCreate(false)
		return fmt.Errorf("%s", msg)
	}

	snap, ok := parseDouyinDraftSnapshot(taskRow.Input)
	if !ok {
		return fail(ErrorDouyinProductPayloadInvalid, "invalid task snapshot", false, "", nil)
	}

	buildRes, err := BuildDouyinProductPayload(ctx, s.DB, taskRow.ProductID, snap.ConfigID)
	if err != nil || len(buildRes.Errors) > 0 {
		msg := "payload build failed"
		if err != nil {
			msg = err.Error()
		} else if len(buildRes.Errors) > 0 {
			msg = buildRes.Errors[0].Message
		}
		return fail(ErrorDouyinProductPayloadInvalid, msg, false, "", nil)
	}
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: taskRow.CreatedBy,
			Action:      "douyin.product.payload.build",
			Resource:    "product_publish_task",
			ResourceID:  taskID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s productId=%s", taskID, taskRow.ProductID),
		})
	}

	client, _, err := s.Shops.DouyinClientForShopContext(ctx, taskRow.ShopID, taskRow.CreatedBy)
	if err != nil {
		code := inferDouyinPublishErrorCode(err)
		return fail(code, err.Error(), douyinErrRetryable(err), "", nil)
	}

	pubCfg := map[string]string{}
	if s.Settings != nil {
		if m, err := s.Settings.PlainByGroup(ctx, 0, "platform_publish_douyin_shop"); err == nil {
			pubCfg = m
		}
	}
	buildRes.APIReq.PublishConfig = pubCfg

	xctx, cancel := context.WithTimeout(ctx, s.execTimeout())
	defer cancel()

	var res *platformdouyin.PlatformProductResult
	recoveredRes, recovered, recErr := tryRecoverDouyinDraftFromPlatform(xctx, client, taskRow.ShopID.String(), taskRow.ProductID.String())
	if recErr != nil {
		code := inferDouyinPublishErrorCode(recErr)
		return fail(code, recErr.Error(), douyinErrRetryable(recErr), "", nil)
	}
	if recovered {
		res = recoveredRes
		if s.OpLog != nil {
			_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
				AdminUserID: taskRow.CreatedBy,
				Action:      "douyin.product.draft.recover",
				Resource:    "product_publish_task",
				ResourceID:  taskID.String(),
				Status:      "success",
				Message:     fmt.Sprintf("taskId=%s platformProductId=%s recovered via product.detail", taskID, res.PlatformProductID),
			})
		}
	} else {
		publishVersion := snap.MappingHash
		if publishVersion == "" {
			publishVersion = taskID.String()
		}
		idemKey := idempotency.DouyinProductDraftCreate(taskRow.ShopID.String(), taskRow.ProductID.String(), publishVersion)
		idemJob, replayRes, acqErr := s.acquirePublishIdempotency(ctx, idemKey, []byte(idemKey), "douyin-draft-create")
		if acqErr != nil {
			return fail(inferDouyinPublishErrorCode(acqErr), acqErr.Error(), true, "", nil)
		}
		if replayRes != nil && replayRes.Replay && strings.TrimSpace(replayRes.ResourceID) != "" {
			res = &platformdouyin.PlatformProductResult{PlatformProductID: replayRes.ResourceID, PlatformStatus: "draft"}
		} else {
			var pubErr error
			res, pubErr = client.CreateProductDraft(xctx, taskRow.ShopID.String(), buildRes.APIReq)
			if pubErr != nil {
				code := inferDouyinPublishErrorCode(pubErr)
				var de *platformdouyin.Error
				raw := map[string]any{}
				retryable := false
				requestID := ""
				if errors.As(pubErr, &de) {
					retryable = de.Retryable
					requestID = de.RequestID
					raw = map[string]any{"platformCode": de.PlatformCode, "platformMessage": de.PlatformMessage}
					if de.UnknownResult || de.Code == platformdouyin.CodeDouyinRequestTimeout || de.Code == platformdouyin.CodeDouyinUnknownResult {
						// unknown_result: try recover before failing; do not auto-recreate
						if recoveredRes2, recovered2, recErr2 := tryRecoverDouyinDraftFromPlatform(xctx, client, taskRow.ShopID.String(), taskRow.ProductID.String()); recErr2 == nil && recovered2 && recoveredRes2 != nil {
							res = recoveredRes2
							if idemJob != nil {
								_ = s.completePublishIdempotency(ctx, idemJob, map[string]string{"platformProductId": res.PlatformProductID}, res.PlatformProductID)
							}
						} else {
							if idemJob != nil {
								s.failPublishIdempotency(ctx, idemJob, platformdouyin.CodeDouyinUnknownResult, false)
							}
							s.markDouyinStale(ctx, taskID, platformdouyin.CodeDouyinUnknownResult, platformdouyin.RecoveryResultUnknown, taskRow.CreatedBy)
							return fail(platformdouyin.CodeDouyinUnknownResult, "抖店草稿创建结果未知，请先回查平台草稿箱", false, requestID, raw)
						}
					} else {
						if idemJob != nil {
							s.failPublishIdempotency(ctx, idemJob, code, retryable)
						}
						return fail(code, pubErr.Error(), retryable, requestID, raw)
					}
				} else {
					if idemJob != nil {
						s.failPublishIdempotency(ctx, idemJob, code, retryable)
					}
					return fail(code, pubErr.Error(), retryable, requestID, raw)
				}
			} else if idemJob != nil && res != nil {
				_ = s.completePublishIdempotency(ctx, idemJob, map[string]string{"platformProductId": res.PlatformProductID}, res.PlatformProductID)
			}
		}
	}
	if res == nil || strings.TrimSpace(res.PlatformProductID) == "" {
		return fail(ErrorDouyinCreateProductFailed, "platform did not return product id", true, "", nil)
	}

	return s.completeDouyinDraftSuccess(ctx, taskRow, taskID, workerID, claim, snap, buildRes, res)
}

func (s *Service) completeDouyinDraftSuccess(ctx context.Context, taskRow *ProductPublishTask, taskID uuid.UUID, workerID string, claim *tasklease.ClaimResult, snap douyinDraftSnapshot, buildRes *DouyinPayloadBuildResult, res *platformdouyin.PlatformProductResult) error {
	fin := time.Now().UTC()
	outSnap := map[string]any{
		"platformProductId": res.PlatformProductID,
		"platformStatus":    res.PlatformStatus,
		"requestId":         res.RequestID,
	}
	rawOut, _ := json.Marshal(outSnap)
	updates := map[string]any{
		"status":              TaskSuccess,
		"publish_status":      StatusDraftCreated,
		"platform_product_id": res.PlatformProductID,
		"request_id":          res.RequestID,
		"retryable":           false,
		"error_code":          "",
		"error_message":       "",
		"finished_at":         &fin,
		"output":              datatypes.JSON(rawOut),
		"platform_result":     datatypes.JSON(rawOut),
	}
	if claim != nil && strings.TrimSpace(workerID) != "" {
		_ = s.finishProductPublishTask(ctx, taskID, workerID, claim, updates)
	} else {
		updates["locked_by"] = nil
		updates["locked_until"] = nil
		updates["updated_at"] = fin
		_ = s.DB.WithContext(ctx).Model(&ProductPublishTask{}).Where("id = ?", taskID).Updates(updates).Error
	}

	rd, _ := json.Marshal(sanitizeRawErrorMap(res.Raw))
	_ = s.DB.WithContext(ctx).Model(&ProductPublication{}).Where("id = ?", snap.PublicationID).
		Updates(map[string]any{
			"external_product_id":  res.PlatformProductID,
			"status":               StatusDraft,
			"publish_status":       StatusDraftCreated,
			"publish_mode":         snap.PublishMode,
			"platform_category_id": buildRes.Payload.CategoryLeafID,
			"raw_data":             datatypes.JSON(rd),
			"last_synced_at":       &fin,
			"updated_at":           fin,
		}).Error

	mapping := product.DouyinDraftMappingFromConfig(product.ProductPlatformPublishConfig{ProductID: taskRow.ProductID})
	var cfg product.ProductPlatformPublishConfig
	if err := s.DB.WithContext(ctx).Where("product_id = ? AND platform = ?", taskRow.ProductID, "douyin_shop").First(&cfg).Error; err == nil {
		mapping = product.DouyinDraftMappingFromConfig(cfg)
	}
	_ = s.DB.WithContext(ctx).Where("publication_id = ?", snap.PublicationID).Delete(&ProductPublicationSKU{}).Error
	skuMap := map[string]platformdouyin.SKUMapping{}
	for _, sm := range res.SKUMappings {
		if sm.OuterSKUID != "" {
			skuMap[sm.OuterSKUID] = sm
		}
	}
	for _, sku := range mapping.SKUs {
		row := ProductPublicationSKU{
			PublicationID: snap.PublicationID,
			SKUCode:       strings.TrimSpace(sku.Name),
			Price:         &sku.Price,
			Stock:         sku.Stock,
		}
		if uid, err := uuid.Parse(strings.TrimSpace(sku.LocalSkuID)); err == nil {
			row.ProductSKUID = &uid
		}
		if sm, ok := skuMap[sku.LocalSkuID]; ok {
			row.ExternalSKUID = strings.TrimSpace(sm.PlatformSKUID)
		}
		rdm, _ := json.Marshal(map[string]any{"outerSkuId": sku.LocalSkuID})
		row.RawData = datatypes.JSON(rdm)
		_ = s.DB.WithContext(ctx).Create(&row).Error
	}

	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: taskRow.CreatedBy,
			Action:      "douyin.product.draft.create.success",
			Resource:    "product_publish_task",
			ResourceID:  taskID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s platformProductId=%s", taskID, res.PlatformProductID),
		})
	}
	douyinmetrics.RecordProductDraftCreate(true)
	return nil
}

func parseDouyinDraftSnapshot(raw datatypes.JSON) (douyinDraftSnapshot, bool) {
	var snap douyinDraftSnapshot
	if len(raw) == 0 {
		return snap, false
	}
	if err := json.Unmarshal(raw, &snap); err != nil || snap.PublicationID == uuid.Nil {
		return snap, false
	}
	return snap, true
}

func inferDouyinPublishErrorCode(err error) string {
	if err == nil {
		return ErrorUnknownDouyinPublish
	}
	var de *platformdouyin.Error
	if errors.As(err, &de) && de.Code != "" {
		return de.Code
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "not authorized") || strings.Contains(msg, "auth expired"):
		return platformdouyin.CodeDouyinStoreNotAuthorized
	case strings.Contains(msg, "category"):
		return platformdouyin.CodeDouyinCategoryMissing
	case strings.Contains(msg, "attribute") || strings.Contains(msg, "required"):
		return platformdouyin.CodeDouyinRequiredAttrMissing
	case strings.Contains(msg, "image"):
		return platformdouyin.CodeDouyinMainImageNotUploaded
	case strings.Contains(msg, "rate"):
		return platformdouyin.CodeDouyinRateLimited
	case strings.Contains(msg, "permission"):
		return platformdouyin.CodeDouyinPermissionDenied
	default:
		return ErrorDouyinCreateProductFailed
	}
}

func douyinErrRetryable(err error) bool {
	var de *platformdouyin.Error
	if errors.As(err, &de) {
		return de.Retryable
	}
	return false
}
