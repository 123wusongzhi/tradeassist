package productpublish

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	OzonReconcileCreated    = "platform_created"
	OzonReconcileNotCreated = "platform_not_created"
)

type ozonReconcileValidationError struct {
	message string
}

func (e *ozonReconcileValidationError) Error() string       { return e.message }
func (e *ozonReconcileValidationError) HTTPStatus() int     { return http.StatusBadRequest }
func (e *ozonReconcileValidationError) SafeMessage() string { return e.message }

func invalidOzonReconciliation(message string) error {
	return &ozonReconcileValidationError{message: message}
}

// ReconcileOzonTask records a human-confirmed external fact. This method never
// invokes a provider and never automatically retries the original mutation.
func (s *Service) ReconcileOzonTask(c *gin.Context, taskID uuid.UUID, body ReconcileOzonTaskBody, adminID *uuid.UUID) (*TaskDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("productpublish: no db")
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var initial ProductPublishTask
	if err := s.DB.WithContext(c.Request.Context()).Where("id = ? AND tenant_id = ?", taskID, tenantID).First(&initial).Error; err != nil {
		return nil, err
	}
	if err := adminperm.EnsureStoreOperate(c, s.DB, initial.ShopID); err != nil {
		return nil, err
	}
	if err := adminperm.EnsureProductOperate(c, s.DB, initial.ProductID); err != nil {
		return nil, err
	}

	outcome := strings.ToLower(strings.TrimSpace(body.Outcome))
	evidence := strings.TrimSpace(body.Evidence)
	if outcome != OzonReconcileCreated && outcome != OzonReconcileNotCreated {
		return nil, invalidOzonReconciliation("outcome must be platform_created or platform_not_created")
	}
	if evidence == "" || len([]rune(evidence)) > 1000 {
		return nil, invalidOzonReconciliation("evidence is required and must not exceed 1000 characters")
	}
	if len(strings.TrimSpace(body.ExternalProductID)) > 512 || len(strings.TrimSpace(body.ExternalSPUID)) > 512 || len(strings.TrimSpace(body.ExternalURL)) > 2048 {
		return nil, invalidOzonReconciliation("external reconciliation fields are too long")
	}

	now := time.Now().UTC()
	var reconciledStatus string
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var task ProductPublishTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", taskID, tenantID).First(&task).Error; err != nil {
			return err
		}
		if !strings.EqualFold(strings.TrimSpace(task.Platform), "ozon") {
			return invalidOzonReconciliation("only Ozon tasks can use this reconciliation endpoint")
		}
		if strings.TrimSpace(task.Status) != TaskFailed {
			return invalidOzonReconciliation("only failed Ozon tasks can be reconciled")
		}
		publicationID, ok := snapshotPublicationFromTask(&task)
		if !ok {
			return fmt.Errorf("task snapshot does not contain a publication")
		}

		record := map[string]any{
			"manualReconciliation": true,
			"outcome":              outcome,
			"evidence":             evidence,
			"reconciledAt":         now.Format(time.RFC3339Nano),
			"sellableVerified":     body.SellableVerified,
		}
		if adminID != nil {
			record["reconciledBy"] = adminID.String()
		}

		if outcome == OzonReconcileNotCreated {
			reconciledStatus = "confirmed_not_created"
			raw, _ := json.Marshal(record)
			pubResult := tx.Model(&ProductPublication{}).
				Where("id = ? AND tenant_id = ? AND product_id = ? AND shop_id = ? AND platform = ?", publicationID, tenantID, task.ProductID, task.ShopID, "ozon").
				Updates(map[string]any{
					"status": StatusPubFailed, "publish_status": StatusPubFailed,
					"external_product_id": "", "external_spu_id": "", "external_url": "",
					"published_at": nil, "raw_data": datatypes.JSON(raw), "updated_at": now,
				})
			if pubResult.Error != nil {
				return pubResult.Error
			}
			if pubResult.RowsAffected != 1 {
				return fmt.Errorf("publication %s not found in tenant %d", publicationID, tenantID)
			}
			taskResult := tx.Model(&ProductPublishTask{}).Where("id = ? AND tenant_id = ? AND status = ?", taskID, tenantID, TaskFailed).
				Updates(map[string]any{
					"publish_status": StatusPubFailed, "platform_product_id": "",
					"retryable": true, "error_code": ErrorPublishReconciledNotCreated,
					"error_message": "已人工确认 Ozon 未创建商品；任务可由运营再次发起重试",
					"output":        datatypes.JSON(raw), "updated_at": now,
				})
			if taskResult.Error != nil {
				return taskResult.Error
			}
			if taskResult.RowsAffected != 1 {
				return fmt.Errorf("task reconciliation state changed")
			}
			return nil
		}

		externalProductID := strings.TrimSpace(body.ExternalProductID)
		if externalProductID == "" {
			externalProductID = strings.TrimSpace(task.PlatformProductID)
		}
		if externalProductID == "" {
			return invalidOzonReconciliation("externalProductId is required when Ozon created the product")
		}
		platformStatus := strings.ToLower(strings.TrimSpace(body.PlatformStatus))
		if platformStatus == "" {
			platformStatus = platformp.PublishStatusImported
		}
		reconciledStatus = normalizePublicationStatus(platformStatus)
		switch reconciledStatus {
		case StatusImported, StatusPendingReview, StatusNeedsAction:
		case StatusPublishedRecord:
			if !body.SellableVerified {
				return invalidOzonReconciliation("sellableVerified must be true before confirming an Ozon listing as published")
			}
		default:
			return invalidOzonReconciliation("platformStatus must be imported, pending_review, needs_action, or sellable")
		}
		record["externalProductId"] = externalProductID
		record["externalSpuId"] = strings.TrimSpace(body.ExternalSPUID)
		record["externalUrl"] = strings.TrimSpace(body.ExternalURL)
		record["platformStatus"] = platformStatus
		raw, _ := json.Marshal(record)
		var publishedAt *time.Time
		if reconciledStatus == StatusPublishedRecord {
			publishedAt = &now
		}
		pubResult := tx.Model(&ProductPublication{}).
			Where("id = ? AND tenant_id = ? AND product_id = ? AND shop_id = ? AND platform = ?", publicationID, tenantID, task.ProductID, task.ShopID, "ozon").
			Updates(map[string]any{
				"status": reconciledStatus, "publish_status": reconciledStatus,
				"external_product_id": externalProductID,
				"external_spu_id":     strings.TrimSpace(body.ExternalSPUID),
				"external_url":        strings.TrimSpace(body.ExternalURL),
				"published_at":        publishedAt, "last_synced_at": &now,
				"raw_data": datatypes.JSON(raw), "updated_at": now,
			})
		if pubResult.Error != nil {
			return pubResult.Error
		}
		if pubResult.RowsAffected != 1 {
			return fmt.Errorf("publication %s not found in tenant %d", publicationID, tenantID)
		}
		taskStatus, taskPublishStatus, errorCode, errorMessage := genericTaskOutcome("ozon", reconciledStatus)
		taskResult := tx.Model(&ProductPublishTask{}).Where("id = ? AND tenant_id = ? AND status = ?", taskID, tenantID, TaskFailed).
			Updates(map[string]any{
				"status": taskStatus, "publish_status": taskPublishStatus,
				"platform_product_id": externalProductID, "retryable": false,
				"error_code": errorCode, "error_message": errorMessage,
				"finished_at": &now, "output": datatypes.JSON(raw), "updated_at": now,
			})
		if taskResult.Error != nil {
			return taskResult.Error
		}
		if taskResult.RowsAffected != 1 {
			return fmt.Errorf("task reconciliation state changed")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "ozon.product.publish.reconcile",
			Resource:    "product_publish_task",
			ResourceID:  taskID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s outcome=%s publishStatus=%s", taskID, outcome, reconciledStatus),
		})
	}
	out, err := s.GetDTO(c.Request.Context(), tenantID, taskID)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
