package productpublish

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productcheck"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasktenant"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxPublishIdempotencyKeyLength = 200

func validatePublishIdempotencyKey(platform, raw string) (string, error) {
	key := strings.TrimSpace(raw)
	if platform == "ozon" && key == "" {
		return "", fmt.Errorf("Ozon 真实提交必须提供 Idempotency-Key 请求头")
	}
	if len(key) > maxPublishIdempotencyKeyLength {
		return "", fmt.Errorf("Idempotency-Key 请求头不能超过 %d 个字符", maxPublishIdempotencyKeyLength)
	}
	return key, nil
}

// CreatePublishTask validates settings + draft, persists task + publishing row snapshot, enqueue or runs inline.
func (s *Service) CreatePublishTask(c *gin.Context, productID uuid.UUID, body PublishRequestBody, adminID *uuid.UUID) (*TaskDTO, error) {
	if s == nil || s.DB == nil || s.Settings == nil || s.Shops == nil || c == nil {
		return nil, fmt.Errorf("product publish unavailable")
	}
	ctx := c.Request.Context()
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	if err := adminperm.EnsureProductOperate(c, s.DB, productID); err != nil {
		return nil, err
	}
	sid, err := uuid.Parse(strings.TrimSpace(body.ShopID))
	if err != nil {
		return nil, fmt.Errorf("invalid shopId")
	}
	if !adminperm.RequireStoreOperate(c, s.DB, sid) {
		return nil, fmt.Errorf("store operate permission required")
	}

	var prod product.Product
	if err := s.DB.WithContext(ctx).
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC, created_at ASC") }).
		Preload("SKUs", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC") }).
		Where("tenant_id = ?", tenantID).
		First(&prod, "id = ?", productID).Error; err != nil {
		return nil, err
	}
	if prod.DeletedAt.Valid {
		return nil, fmt.Errorf("deleted product cannot be published")
	}
	var draft platformp.PlatformProductDraft

	row, plainAuth, err := s.Shops.PlainAuthForProviderCtx(ctx, tenantID, sid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("shop not found")
		}
		return nil, err
	}
	if row == nil {
		return nil, fmt.Errorf("shop not found")
	}
	if prod.TenantID != row.TenantID {
		return nil, fmt.Errorf("product tenant does not match shop tenant")
	}

	platKey := strings.TrimSpace(strings.ToLower(row.Platform))
	rawIdempotencyKey, err := validatePublishIdempotencyKey(platKey, c.GetHeader("Idempotency-Key"))
	if err != nil {
		return nil, err
	}
	if platKey == "ozon" && s.Idempotency == nil {
		return nil, fmt.Errorf("Ozon 幂等控制不可用，不能创建提交任务")
	}
	var ozonConfig *product.ProductPlatformPublishConfig
	if platKey == "ozon" {
		cfg, _, configErr := product.FindProductPlatformPublishConfig(ctx, s.DB, prod.ID, "ozon", &sid, true)
		if configErr != nil {
			return nil, fmt.Errorf("ozon product configuration is required")
		}
		if cfg.ShopID != nil && *cfg.ShopID != sid {
			return nil, fmt.Errorf("ozon product configuration must select this authorized shop")
		}
		if strings.TrimSpace(cfg.CategoryID) == "" || strings.TrimSpace(cfg.SchemaHash) == "" {
			return nil, fmt.Errorf("ozon product category configuration is incomplete")
		}
		if body.Options == nil {
			body.Options = map[string]any{}
		}
		parts := strings.SplitN(cfg.CategoryID, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid Ozon composite category id")
		}
		body.Options["description_category_id"] = parts[0]
		body.Options["type_id"] = parts[1]
		body.Options["platform_attributes"] = json.RawMessage(cfg.PlatformAttributes)
		body.Options["ozon_schema_hash"] = cfg.SchemaHash
		ozonConfig = cfg
	}

	prov := platformp.Get(platKey)
	if prov == nil {
		return nil, fmt.Errorf("unknown platform")
	}
	if !platformp.IsProductPublishRunnable(prov) {
		return nil, platformp.ErrProductPublishNotImplemented
	}
	if _, ok := platformp.AsProductPublish(prov); !ok {
		return nil, platformp.ErrProductPublishNotImplemented
	}

	if err := ensurePartnerOpenConfig(ctx, s.Settings, prov); err != nil {
		return nil, err
	}

	if err := ensureShopAuthorizedForPublish(row, plainAuth); err != nil {
		return nil, err
	}

	var readinessSnap *productcheck.CheckProductReadinessResult
	var readinessResult *productcheck.CheckProductReadinessResult
	if s.Readiness != nil {
		var rres *productcheck.CheckProductReadinessResult
		var err error
		if platKey == "ozon" {
			rres, err = s.Readiness.ValidateOzonReadiness(ctx, tenantID, productID, sid)
		} else {
			rres, err = s.Readiness.CheckProductReadiness(ctx, productcheck.CheckProductReadinessRequest{
				TenantID:       tenantID,
				ProductID:      productID,
				Platform:       platKey,
				ShopID:         &sid,
				Mode:           "publish",
				PublishOptions: body.Options,
			})
		}
		if err != nil {
			return nil, err
		}
		if rres.ErrorCount > 0 {
			return nil, &productcheck.BlockedError{Result: rres}
		}
		readinessResult = rres
		if rres.WarningCount > 0 {
			readinessSnap = rres
		}
	}

	pubSch := prov.PublishConfigSchema()
	pubGK := strings.TrimSpace(pubSch.GroupKey)
	if pubGK == "" {
		return nil, fmt.Errorf("platform %q does not expose publish configuration schema", platKey)
	}
	curPub, err := s.Settings.PlainByGroup(ctx, 0, pubGK)
	if err != nil {
		return nil, err
	}
	var resolvedOzon *product.OzonResolvedListingDTO
	if platKey == "ozon" {
		if readinessResult != nil && readinessResult.ResolvedOzon != nil {
			copyResolved := *readinessResult.ResolvedOzon
			resolvedOzon = &copyResolved
		} else {
			copyResolved := product.ResolveOzonListing(prod, ozonConfig, curPub, "")
			resolvedOzon = &copyResolved
		}
		applyResolvedOzonPublishOptions(body.Options, *resolvedOzon)
	}
	base := mergePublishBaseline(pubSch, curPub)
	merged := ApplyPublishOptions(base, body.Options)
	if err := validateMergedPublishAgainstSchema(pubSch, merged); err != nil {
		return nil, err
	}
	if platKey == "ozon" {
		if resolvedOzon == nil {
			return nil, fmt.Errorf("Ozon resolved listing unavailable")
		}
		if resolvedOzon.Currency.Value == "" && strings.TrimSpace(merged["currency_code"]) != "" {
			copyResolved := product.ResolveOzonListing(prod, ozonConfig, curPub, merged["currency_code"])
			resolvedOzon = &copyResolved
			applyResolvedOzonPublishOptions(body.Options, *resolvedOzon)
			merged = ApplyPublishOptions(base, body.Options)
		}
		draft, err = BuildOzonPlatformDraftFromResolved(prod, *resolvedOzon)
	} else {
		draft, err = BuildPlatformDraftFromProduct(prod)
	}
	if err != nil {
		return nil, err
	}
	// Only claim idempotency after every non-mutating validation has passed. A
	// malformed request therefore never leaves an in-progress key behind.
	var idemJob *publishBatchAcquire
	completedIdempotency := false
	if rawIdempotencyKey != "" {
		reqRaw, _ := json.Marshal(map[string]any{"productId": productID.String(), "body": body})
		job, replay, acquireErr := s.acquirePublishIdempotency(ctx, tenantIdempotencyKey(tenantID, rawIdempotencyKey), reqRaw, requestIdempotencyOwner(c, "product-publish-create"))
		if acquireErr != nil {
			return nil, acquireErr
		}
		if replay != nil && replay.Replay && strings.TrimSpace(replay.ResourceID) != "" {
			if existingID, parseErr := uuid.Parse(replay.ResourceID); parseErr == nil {
				if out, getErr := s.GetDTO(ctx, tenantID, existingID); getErr == nil {
					return &out, nil
				}
			}
			return nil, fmt.Errorf("idempotency replay task unavailable")
		}
		idemJob = job
	}
	defer func() {
		if idemJob != nil && !completedIdempotency {
			s.failPublishIdempotency(ctx, idemJob, "PUBLISH_TASK_CREATE_FAILED", true)
		}
	}()
	imgSnap, skuSnap, minPrice := taskImagesAndSKUsSnapshot(draft)
	checkSnap, _ := json.Marshal(readinessResult)
	payloadSnap, _ := json.Marshal(platformPayloadSnapshot(draft, merged))

	optsSnap := map[string]any{}
	if body.Options != nil {
		for k, v := range body.Options {
			optsSnap[k] = v
		}
	}

	title := strings.TrimSpace(draft.Title)
	currency := strings.TrimSpace(draft.Currency)

	var task ProductPublishTask
	if err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize task creation per product. This closes the gap where callers
		// could use different idempotency keys before either task became visible.
		var lockedProduct product.Product
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").
			Where("id = ? AND tenant_id = ?", prod.ID, tenantID).First(&lockedProduct).Error; err != nil {
			return err
		}

		var activeTask ProductPublishTask
		if err := tx.
			Where("tenant_id = ? AND product_id = ? AND shop_id = ? AND platform = ? AND task_type = ? AND status IN ?", tenantID, productID, sid, platKey, TaskTypeProductPublish, []string{TaskPending, TaskRunning}).
			Order("created_at DESC").First(&activeTask).Error; err == nil {
			return fmt.Errorf("a product publish task is already pending or running")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if platKey == "ozon" {
			var uncertainTask ProductPublishTask
			if err := tx.
				Where("tenant_id = ? AND product_id = ? AND shop_id = ? AND platform = ? AND task_type = ? AND status = ? AND (retryable = ? OR platform_product_id <> '')", tenantID, productID, sid, platKey, TaskTypeProductPublish, TaskFailed, false).
				Order("updated_at DESC").First(&uncertainTask).Error; err == nil {
				return fmt.Errorf("存在 Ozon 失败任务，需先人工核对或恢复外部刊登结果，不能创建新任务")
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			var existingTask ProductPublishTask
			if err := tx.
				Where("tenant_id = ? AND product_id = ? AND shop_id = ? AND platform = ? AND task_type = ? AND platform_product_id <> ''", tenantID, productID, sid, platKey, TaskTypeProductPublish).
				Order("updated_at DESC").First(&existingTask).Error; err == nil {
				return fmt.Errorf("Ozon 商品已存在或结果待恢复，不能重复创建；请使用后续更新或恢复流程")
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			var existingPublication ProductPublication
			if err := tx.
				Where("tenant_id = ? AND product_id = ? AND shop_id = ? AND platform = ? AND external_product_id <> ''", tenantID, productID, sid, platKey).
				Order("updated_at DESC").First(&existingPublication).Error; err == nil {
				return fmt.Errorf("Ozon 商品已存在，不能重复创建；请使用后续更新流程")
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}

		var pubRow ProductPublication
		q := tx.Where("tenant_id = ? AND product_id = ? AND shop_id = ? AND platform = ? AND publish_status = ?",
			tenantID, prod.ID, sid, platKey, StatusPublishing).
			Order("updated_at DESC")
		if err := q.First(&pubRow).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			pubRow = ProductPublication{
				TenantID:      tenantID,
				ProductID:     prod.ID,
				ShopID:        sid,
				Platform:      platKey,
				Status:        StatusPublishing,
				PublishStatus: StatusPublishing,
				Title:         title,
				Currency:      currency,
				CreatedBy:     adminID,
			}
			if err := tx.Create(&pubRow).Error; err != nil {
				return err
			}
		} else if err := tx.Model(&ProductPublication{}).Where("id = ?", pubRow.ID).
			Updates(map[string]any{
				"title":               title,
				"currency":            currency,
				"publish_task_id":     nil,
				"external_product_id": "",
				"external_spu_id":     "",
				"external_url":        "",
				"raw_data":            datatypes.JSON(nil),
				"published_at":        nil,
				"status":              StatusPublishing,
				"publish_status":      StatusPublishing,
			}).Error; err != nil {
			return err
		}

		inp := publishSnapshot{
			PublicationID: pubRow.ID,
			MergedPublish: merged,
			Options:       optsSnap,
			Draft:         &draft,
		}
		rawIn, err := json.Marshal(inp)
		if err != nil {
			return err
		}
		task = ProductPublishTask{
			TenantID:        tenantID,
			ProductID:       prod.ID,
			ShopID:          sid,
			TargetStoreID:   sid,
			Platform:        platKey,
			TaskType:        TaskTypeProductPublish,
			Status:          TaskPending,
			PublishStatus:   StatusReady,
			Mode:            ModeManual,
			PublishMode:     ModeManual,
			Title:           draft.Title,
			Description:     draft.Description,
			Images:          datatypes.JSON(imgSnap),
			SKUs:            datatypes.JSON(skuSnap),
			Price:           minPrice,
			Currency:        draft.Currency,
			CheckResult:     datatypes.JSON(checkSnap),
			PlatformPayload: datatypes.JSON(payloadSnap),
			Input:           rawIn,
			CreatedBy:       adminID,
		}
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		return tx.Model(&ProductPublication{}).
			Where("id = ? AND tenant_id = ?", pubRow.ID, tenantID).
			Updates(map[string]any{"publish_task_id": task.ID, "updated_at": task.CreatedAt}).Error
	}); err != nil {
		return nil, err
	}

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "product.publish.create",
			Resource:    "product_publish_task",
			ResourceID:  task.ID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s productId=%s shopId=%s platform=%s", task.ID.String(), prod.ID.String(), sid.String(), platKey),
		})
	}

	runInline := func() error {
		actorID := uuid.Nil
		if task.CreatedBy != nil {
			actorID = *task.CreatedBy
		}
		workerCtx := tasktenant.BuildWorkerContext(tasktenant.TaskScope{
			TenantID: task.TenantID,
			ShopID:   task.ShopID,
		}, actorID, "product_publish_inline")
		return s.ProcessQueuedTask(workerCtx, task.ID, worker.GenerateInlineWorkerID(worker.TypeProductPublish))
	}

	if s.QueueEnabled && s.Redis != nil && s.Redis.Client != nil {
		if err := s.enqueue(ctx, task.ID); err != nil {
			slog.Warn("product_publish_enqueue_failed_run_inline", "taskId", task.ID.String(), "error", err)
			if err := runInline(); err != nil {
				return nil, err
			}
		}
	} else {
		if err := runInline(); err != nil {
			return nil, err
		}
	}
	if err := s.completeProductPublishTaskIdempotency(ctx, idemJob, task.ID); err != nil {
		return nil, err
	}
	completedIdempotency = true

	out, err := s.GetDTO(ctx, task.TenantID, task.ID)
	if err != nil {
		return nil, err
	}
	if readinessSnap != nil {
		out.Readiness = readinessSnap
	}
	return &out, nil
}

func applyResolvedOzonPublishOptions(options map[string]any, resolved product.OzonResolvedListingDTO) {
	if options == nil {
		return
	}
	options["default_weight"] = resolved.Package.WeightG.Value
	options["default_width"] = resolved.Package.WidthMM.Value
	options["default_height"] = resolved.Package.HeightMM.Value
	options["default_depth"] = resolved.Package.DepthMM.Value
	options["warehouse_id"] = resolved.Package.WarehouseID.Value
	options["vat"] = resolved.Package.VAT.Value
	if strings.TrimSpace(resolved.Currency.Value) != "" {
		options["currency_code"] = resolved.Currency.Value
	}
}
