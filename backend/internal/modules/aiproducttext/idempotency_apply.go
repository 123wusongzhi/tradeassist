package aiproducttext

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
)

type textApplyAcquire struct {
	RecordID uuid.UUID
	Owner    string
	Key      string
}

func textApplyRequestHash(batchID, itemID, productID, targetVersion, operationType, resultHash string) string {
	payload, _ := json.Marshal(map[string]any{
		"batchId":              batchID,
		"itemId":               itemID,
		"targetProductId":      productID,
		"targetVersion":        targetVersion,
		"operationType":        operationType,
		"normalizedResultHash": resultHash,
	})
	return idempotency.HashRequest(payload)
}

func textUndoRequestHash(applicationID, targetVersion string) string {
	payload, _ := json.Marshal(map[string]any{
		"applicationId": applicationID,
		"targetVersion": targetVersion,
	})
	return idempotency.HashRequest(payload)
}

func (s *Service) acquireTextApply(ctx context.Context, cOwner string, batchID, itemID, productID, targetVersion, operationType, resultHash string) (*textApplyAcquire, *idempotency.AcquireResult, error) {
	if s == nil || s.Idempotency == nil {
		return nil, nil, nil
	}
	key := idempotency.AITextApply(batchID, itemID, productID, targetVersion)
	owner := cOwner
	if owner == "" {
		owner = "ai-text-apply"
	}
	hash := textApplyRequestHash(batchID, itemID, productID, targetVersion, operationType, resultHash)
	res, err := s.Idempotency.Acquire(ctx, idempotency.ScopeAIText, key, hash, owner, idempotency.DefaultLease)
	decision, rec, _ := idempotency.Classify(res, err)
	switch decision {
	case idempotency.DecisionAlreadySucceeded:
		if res == nil && rec != nil {
			res = &idempotency.AcquireResult{
				Record:        rec,
				Replay:        true,
				ReplaySummary: rec.ResponseSummary,
				ResourceID:    rec.ResourceID,
			}
		}
		return nil, res, nil
	case idempotency.DecisionInProgress:
		return nil, res, fmt.Errorf("%s", idempotency.ErrCodeInProgress)
	case idempotency.DecisionKeyConflict, idempotency.DecisionPermanentFailure:
		return nil, res, fmt.Errorf("%s", idempotency.ErrCodeKeyConflict)
	case idempotency.DecisionAcquired, idempotency.DecisionRetryAllowed:
		if rec == nil && res != nil {
			rec = res.Record
		}
		if rec == nil {
			return nil, res, fmt.Errorf("idempotency: missing record")
		}
		return &textApplyAcquire{RecordID: rec.ID, Owner: owner, Key: key}, res, nil
	default:
		return nil, res, err
	}
}

func (s *Service) completeTextApply(ctx context.Context, job *textApplyAcquire, summary map[string]string, applicationID string) error {
	if s == nil || s.Idempotency == nil || job == nil {
		return nil
	}
	body, _ := json.Marshal(summary)
	return s.Idempotency.Complete(ctx, job.RecordID, job.Owner, idempotency.CompleteResult{
		ResponseCode:    "AI_TEXT_APPLY_SUCCESS",
		ResponseSummary: string(body),
		ResourceType:    "product_ai_content_application",
		ResourceID:      applicationID,
	})
}

func (s *Service) failTextApply(ctx context.Context, job *textApplyAcquire, code string, retryable bool) {
	if s == nil || s.Idempotency == nil || job == nil {
		return
	}
	_ = s.Idempotency.Fail(ctx, job.RecordID, job.Owner, code, retryable)
}

func (s *Service) acquireTextUndo(ctx context.Context, cOwner, applicationID, targetVersion string) (*textApplyAcquire, *idempotency.AcquireResult, error) {
	if s == nil || s.Idempotency == nil {
		return nil, nil, nil
	}
	key := idempotency.AITextUndo(applicationID, targetVersion)
	owner := cOwner
	if owner == "" {
		owner = "ai-text-undo"
	}
	hash := textUndoRequestHash(applicationID, targetVersion)
	res, err := s.Idempotency.Acquire(ctx, idempotency.ScopeAIText, key, hash, owner, idempotency.DefaultLease)
	decision, rec, _ := idempotency.Classify(res, err)
	switch decision {
	case idempotency.DecisionAlreadySucceeded:
		if res == nil && rec != nil {
			res = &idempotency.AcquireResult{
				Record:        rec,
				Replay:        true,
				ReplaySummary: rec.ResponseSummary,
				ResourceID:    rec.ResourceID,
			}
		}
		return nil, res, nil
	case idempotency.DecisionInProgress:
		return nil, res, fmt.Errorf("%s", idempotency.ErrCodeInProgress)
	case idempotency.DecisionKeyConflict, idempotency.DecisionPermanentFailure:
		return nil, res, fmt.Errorf("%s", idempotency.ErrCodeKeyConflict)
	case idempotency.DecisionAcquired, idempotency.DecisionRetryAllowed:
		if rec == nil && res != nil {
			rec = res.Record
		}
		if rec == nil {
			return nil, res, fmt.Errorf("idempotency: missing record")
		}
		return &textApplyAcquire{RecordID: rec.ID, Owner: owner, Key: key}, res, nil
	default:
		return nil, res, err
	}
}

func (s *Service) completeTextUndo(ctx context.Context, job *textApplyAcquire, applicationID string) error {
	if s == nil || s.Idempotency == nil || job == nil {
		return nil
	}
	summary, _ := json.Marshal(map[string]string{"applicationId": applicationID})
	return s.Idempotency.Complete(ctx, job.RecordID, job.Owner, idempotency.CompleteResult{
		ResponseCode:    "AI_TEXT_UNDO_SUCCESS",
		ResponseSummary: string(summary),
		ResourceType:    "product_ai_content_application",
		ResourceID:      applicationID,
	})
}

func (s *Service) failTextUndo(ctx context.Context, job *textApplyAcquire, code string, retryable bool) {
	if s == nil || s.Idempotency == nil || job == nil {
		return
	}
	_ = s.Idempotency.Fail(ctx, job.RecordID, job.Owner, code, retryable)
}
