package productpublish

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

const (
	errPublishBatchInProgress   = "PUBLISH_BATCH_IN_PROGRESS"
	errPublishBatchKeyConflict  = "PUBLISH_BATCH_KEY_CONFLICT"
	errPublishEnqueueInProgress = "PUBLISH_ENQUEUE_IN_PROGRESS"
)

type publishBatchAcquire struct {
	RecordID uuid.UUID
	Owner    string
}

func tenantIdempotencyKey(tenantID int64, key string) string {
	return fmt.Sprintf("tenant:%d:%s", tenantID, key)
}

func requestIdempotencyOwner(c *gin.Context, fallback string) string {
	requestID := ""
	if c != nil {
		requestID = strings.TrimSpace(c.GetString(ctxkey.TraceID))
	}
	if requestID == "" {
		requestID = uuid.NewString()
	}
	return idempotency.OwnerFromRequest(requestID, fallback)
}

func (s *Service) acquirePublishIdempotency(ctx context.Context, idemKey string, reqHash []byte, owner string) (*publishBatchAcquire, *idempotency.AcquireResult, error) {
	if s == nil || s.Idempotency == nil || idemKey == "" {
		return nil, nil, nil
	}
	if owner == "" {
		owner = "douyin-draft-create"
	}
	res, err := s.Idempotency.Acquire(ctx, idempotency.ScopePublish, idemKey, idempotency.HashRequest(reqHash), owner, idempotency.DefaultLease)
	decision, rec, _ := idempotency.Classify(res, err)
	switch decision {
	case idempotency.DecisionAlreadySucceeded:
		return nil, res, nil
	case idempotency.DecisionInProgress:
		return nil, res, fmt.Errorf("%s", errPublishBatchInProgress)
	case idempotency.DecisionKeyConflict, idempotency.DecisionPermanentFailure:
		return nil, res, fmt.Errorf("%s", errPublishBatchKeyConflict)
	case idempotency.DecisionAcquired, idempotency.DecisionRetryAllowed:
		if rec == nil && res != nil {
			rec = res.Record
		}
		if rec == nil {
			return nil, res, fmt.Errorf("idempotency: missing record")
		}
		return &publishBatchAcquire{RecordID: rec.ID, Owner: owner}, res, nil
	default:
		return nil, res, err
	}
}

func (s *Service) completePublishIdempotency(ctx context.Context, job *publishBatchAcquire, summary map[string]string, resourceID string) error {
	return s.completePublishIdempotencyResult(ctx, job, summary, resourceID, "DOUYIN_DRAFT_CREATED", "platform_product_draft")
}

func (s *Service) completeProductPublishTaskIdempotency(ctx context.Context, job *publishBatchAcquire, taskID uuid.UUID) error {
	return s.completePublishIdempotencyResult(ctx, job, map[string]string{"taskId": taskID.String()}, taskID.String(), "PUBLISH_TASK_CREATED", "product_publish_task")
}

func (s *Service) completePublishIdempotencyResult(ctx context.Context, job *publishBatchAcquire, summary map[string]string, resourceID, responseCode, resourceType string) error {
	if s == nil || s.Idempotency == nil || job == nil {
		return nil
	}
	body, _ := json.Marshal(summary)
	return s.Idempotency.Complete(ctx, job.RecordID, job.Owner, idempotency.CompleteResult{
		ResponseCode:    responseCode,
		ResponseSummary: string(body),
		ResourceType:    resourceType,
		ResourceID:      resourceID,
	})
}

func (s *Service) failPublishIdempotency(ctx context.Context, job *publishBatchAcquire, code string, retryable bool) {
	if s == nil || s.Idempotency == nil || job == nil {
		return
	}
	_ = s.Idempotency.Fail(ctx, job.RecordID, job.Owner, code, retryable)
}

func (s *Service) acquirePublishBatch(ctx context.Context, c *gin.Context, idemKey string, reqHash []byte) (*publishBatchAcquire, *idempotency.AcquireResult, error) {
	if s == nil || s.Idempotency == nil || idemKey == "" {
		return nil, nil, nil
	}
	owner := "publish-batch-create"
	tenantID, err := trustedTenantOrLegacy(c)
	if err != nil {
		return nil, nil, err
	}
	idemKey = tenantIdempotencyKey(tenantID, idemKey)
	owner = requestIdempotencyOwner(c, owner)
	res, err := s.Idempotency.Acquire(ctx, idempotency.ScopePublish, idemKey, idempotency.HashRequest(reqHash), owner, idempotency.DefaultLease)
	decision, rec, _ := idempotency.Classify(res, err)
	switch decision {
	case idempotency.DecisionAlreadySucceeded:
		return nil, res, nil
	case idempotency.DecisionInProgress:
		return nil, res, fmt.Errorf("%s", errPublishBatchInProgress)
	case idempotency.DecisionKeyConflict, idempotency.DecisionPermanentFailure:
		return nil, res, fmt.Errorf("%s", errPublishBatchKeyConflict)
	case idempotency.DecisionAcquired, idempotency.DecisionRetryAllowed:
		if rec == nil && res != nil {
			rec = res.Record
		}
		if rec == nil {
			return nil, res, fmt.Errorf("idempotency: missing record")
		}
		return &publishBatchAcquire{RecordID: rec.ID, Owner: owner}, res, nil
	default:
		return nil, res, err
	}
}

func (s *Service) completePublishBatchCreate(ctx context.Context, job *publishBatchAcquire, batchID uuid.UUID) error {
	if s == nil || s.Idempotency == nil || job == nil {
		return nil
	}
	summary, _ := json.Marshal(map[string]string{"batchId": batchID.String()})
	return s.Idempotency.Complete(ctx, job.RecordID, job.Owner, idempotency.CompleteResult{
		ResponseCode:    "PUBLISH_BATCH_CREATED",
		ResponseSummary: string(summary),
		ResourceType:    "product_publish_batch",
		ResourceID:      batchID.String(),
	})
}

func (s *Service) failPublishBatchCreate(ctx context.Context, job *publishBatchAcquire, code string, retryable bool) {
	if s == nil || s.Idempotency == nil || job == nil {
		return
	}
	_ = s.Idempotency.Fail(ctx, job.RecordID, job.Owner, code, retryable)
}

func (s *Service) resolveExistingPublishBatch(ctx context.Context, res *idempotency.AcquireResult, idemKey string) (*BatchTargetsCreateDraftsResponse, error) {
	rid := ""
	if res != nil {
		rid = res.ResourceID
		if rid == "" && res.Record != nil {
			rid = res.Record.ResourceID
		}
	}
	if rid != "" {
		if bid, err := uuid.Parse(rid); err == nil {
			var batch ProductPublishBatch
			if err := s.DB.WithContext(ctx).First(&batch, "id = ?", bid).Error; err == nil {
				return s.batchCreateResponseFromExisting(ctx, &batch)
			}
		}
	}
	if idemKey != "" {
		var batch ProductPublishBatch
		if err := s.DB.WithContext(ctx).Where("idempotency_key = ? AND status NOT IN ?", idemKey, []string{BatchFailed, BatchCancelled}).
			Order("created_at DESC").First(&batch).Error; err == nil {
			return s.batchCreateResponseFromExisting(ctx, &batch)
		}
	}
	return nil, fmt.Errorf("%s", errPublishBatchInProgress)
}

func (s *Service) enqueuePublishTaskIdempotent(ctx context.Context, c *gin.Context, batchID uuid.UUID, taskID uuid.UUID, taskType string) error {
	if s == nil {
		return fmt.Errorf("product publish unavailable")
	}
	if batchID == uuid.Nil || s.Idempotency == nil {
		return s.enqueue(ctx, taskID)
	}
	owner := "publish-enqueue"
	owner = requestIdempotencyOwner(c, owner)
	key := idempotency.PublishEnqueue(batchID.String(), taskType)
	reqHash := idempotency.HashRequest([]byte(taskID.String()))
	res, err := s.Idempotency.Acquire(ctx, idempotency.ScopePublish, key, reqHash, owner, idempotency.DefaultLease)
	decision, rec, classifyErr := idempotency.Classify(res, err)
	switch decision {
	case idempotency.DecisionAlreadySucceeded:
		return nil
	case idempotency.DecisionInProgress:
		return fmt.Errorf("%s", errPublishEnqueueInProgress)
	case idempotency.DecisionKeyConflict, idempotency.DecisionPermanentFailure:
		return classifyErr
	case idempotency.DecisionAcquired, idempotency.DecisionRetryAllowed:
		if rec == nil && res != nil {
			rec = res.Record
		}
	default:
		if classifyErr != nil {
			return classifyErr
		}
	}
	if err := s.enqueue(ctx, taskID); err != nil {
		if rec != nil {
			_ = s.Idempotency.Fail(ctx, rec.ID, owner, "PUBLISH_ENQUEUE_FAILED", true)
		}
		return err
	}
	if rec != nil {
		summary, _ := json.Marshal(map[string]string{"taskId": taskID.String()})
		return s.Idempotency.Complete(ctx, rec.ID, owner, idempotency.CompleteResult{
			ResponseCode:    "PUBLISH_ENQUEUED",
			ResponseSummary: string(summary),
			ResourceType:    "product_publish_task",
			ResourceID:      taskID.String(),
		})
	}
	return nil
}
