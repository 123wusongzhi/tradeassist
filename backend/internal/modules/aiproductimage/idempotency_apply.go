package aiproductimage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
)

type imageApplyAcquire struct {
	RecordID uuid.UUID
	Owner    string
	Key      string
}

func imageApplyRequestHash(batchID, itemID, productID, targetVersion, slot, applyMode, resultHash string) string {
	payload, _ := json.Marshal(map[string]any{
		"batchId":              batchID,
		"itemId":               itemID,
		"targetProductId":      productID,
		"targetVersion":        targetVersion,
		"slot":                 slot,
		"applyMode":            applyMode,
		"normalizedResultHash": resultHash,
	})
	return idempotency.HashRequest(payload)
}

func imageUndoRequestHash(applicationID, targetVersion string) string {
	payload, _ := json.Marshal(map[string]any{
		"applicationId": applicationID,
		"targetVersion": targetVersion,
	})
	return idempotency.HashRequest(payload)
}

// applySlot returns a stable slot key for idempotency.
func applySlot(item *AIProductImageItem, mode string) string {
	mode = strings.TrimSpace(mode)
	switch mode {
	case ApplySetMain:
		return "main"
	case ApplySaveToGallery:
		return "gallery:0"
	case ApplyAddDetail:
		if item != nil && item.ImageID != nil {
			return fmt.Sprintf("detail:%s", item.ImageID.String())
		}
		return "detail:0"
	case ApplyReplaceImage:
		id := ""
		if item != nil && item.ImageID != nil {
			id = item.ImageID.String()
		}
		return "replace:" + id
	default:
		if item != nil && item.OperationType == OpWhiteBackground {
			return "white_background"
		}
		if mode != "" {
			return mode
		}
		return "gallery:0"
	}
}

func (s *Service) acquireImageApply(ctx context.Context, owner, batchID, itemID, productID, targetVersion, slot, applyMode, resultHash string) (*imageApplyAcquire, *idempotency.AcquireResult, error) {
	if s == nil || s.Idempotency == nil {
		return nil, nil, nil
	}
	key := idempotency.AIImageApply(batchID, itemID, productID, targetVersion, slot)
	if owner == "" {
		owner = "ai-image-apply"
	}
	hash := imageApplyRequestHash(batchID, itemID, productID, targetVersion, slot, applyMode, resultHash)
	res, err := s.Idempotency.Acquire(ctx, idempotency.ScopeAIImage, key, hash, owner, idempotency.DefaultLease)
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
		return &imageApplyAcquire{RecordID: rec.ID, Owner: owner, Key: key}, res, nil
	default:
		return nil, res, err
	}
}

func (s *Service) completeImageApply(ctx context.Context, job *imageApplyAcquire, summary map[string]string, applicationID string) error {
	if s == nil || s.Idempotency == nil || job == nil {
		return nil
	}
	body, _ := json.Marshal(summary)
	return s.Idempotency.Complete(ctx, job.RecordID, job.Owner, idempotency.CompleteResult{
		ResponseCode:    "AI_IMAGE_APPLY_SUCCESS",
		ResponseSummary: string(body),
		ResourceType:    "product_image_application",
		ResourceID:      applicationID,
	})
}

func (s *Service) failImageApply(ctx context.Context, job *imageApplyAcquire, code string, retryable bool) {
	if s == nil || s.Idempotency == nil || job == nil {
		return
	}
	_ = s.Idempotency.Fail(ctx, job.RecordID, job.Owner, code, retryable)
}

func (s *Service) acquireImageUndo(ctx context.Context, owner, applicationID, targetVersion string) (*imageApplyAcquire, *idempotency.AcquireResult, error) {
	if s == nil || s.Idempotency == nil {
		return nil, nil, nil
	}
	key := idempotency.AIImageUndo(applicationID, targetVersion)
	if owner == "" {
		owner = "ai-image-undo"
	}
	hash := imageUndoRequestHash(applicationID, targetVersion)
	res, err := s.Idempotency.Acquire(ctx, idempotency.ScopeAIImage, key, hash, owner, idempotency.DefaultLease)
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
		return &imageApplyAcquire{RecordID: rec.ID, Owner: owner, Key: key}, res, nil
	default:
		return nil, res, err
	}
}

func (s *Service) completeImageUndo(ctx context.Context, job *imageApplyAcquire, applicationID string) error {
	if s == nil || s.Idempotency == nil || job == nil {
		return nil
	}
	summary, _ := json.Marshal(map[string]string{"applicationId": applicationID})
	return s.Idempotency.Complete(ctx, job.RecordID, job.Owner, idempotency.CompleteResult{
		ResponseCode:    "AI_IMAGE_UNDO_SUCCESS",
		ResponseSummary: string(summary),
		ResourceType:    "product_image_application",
		ResourceID:      applicationID,
	})
}

func (s *Service) failImageUndo(ctx context.Context, job *imageApplyAcquire, code string, retryable bool) {
	if s == nil || s.Idempotency == nil || job == nil {
		return
	}
	_ = s.Idempotency.Fail(ctx, job.RecordID, job.Owner, code, retryable)
}
