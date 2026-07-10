package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
)

const errInventoryPushInProgress = "INVENTORY_PUSH_IN_PROGRESS"

type pushAcquire struct {
	RecordID uuid.UUID
	Owner    string
	Key      string
}

func pushRequestHash(platform string, shopID, skuID, pubSkuID uuid.UUID, target int) string {
	payload, _ := json.Marshal(map[string]any{
		"platform":         platform,
		"shopId":           shopID.String(),
		"skuId":            skuID.String(),
		"publicationSkuId": pubSkuID.String(),
		"targetStock":      target,
	})
	return idempotency.HashRequest(payload)
}

func pushOwner(admin *uuid.UUID) string {
	if admin != nil && *admin != uuid.Nil {
		return admin.String()
	}
	return "inventory-push"
}

func (s *Service) acquireInventoryPush(ctx context.Context, platform string, shopID, skuID, pubSkuID uuid.UUID, target int, admin *uuid.UUID) (*pushAcquire, *idempotency.AcquireResult, error) {
	if s == nil || s.Idempotency == nil {
		return nil, nil, nil
	}
	key := idempotency.InventoryPush(platform, shopID.String(), skuID.String(), strconv.Itoa(target))
	owner := pushOwner(admin)
	hash := pushRequestHash(platform, shopID, skuID, pubSkuID, target)
	res, err := s.Idempotency.Acquire(ctx, idempotency.ScopeInventoryPush, key, hash, owner, idempotency.DefaultLease)
	decision, rec, _ := idempotency.Classify(res, err)
	switch decision {
	case idempotency.DecisionAlreadySucceeded:
		return nil, res, nil
	case idempotency.DecisionInProgress:
		return nil, res, fmt.Errorf("%s", errInventoryPushInProgress)
	case idempotency.DecisionKeyConflict, idempotency.DecisionPermanentFailure:
		return nil, res, fmt.Errorf("INVENTORY_PUSH_KEY_CONFLICT")
	case idempotency.DecisionAcquired, idempotency.DecisionRetryAllowed:
		if rec == nil && res != nil {
			rec = res.Record
		}
		if rec == nil {
			return nil, res, fmt.Errorf("idempotency: missing record")
		}
		return &pushAcquire{RecordID: rec.ID, Owner: owner, Key: key}, res, nil
	default:
		return nil, res, err
	}
}

func (s *Service) completeInventoryPush(ctx context.Context, job *pushAcquire, taskID uuid.UUID) error {
	if s == nil || s.Idempotency == nil || job == nil {
		return nil
	}
	summary, _ := json.Marshal(map[string]string{"taskId": taskID.String()})
	return s.Idempotency.Complete(ctx, job.RecordID, job.Owner, idempotency.CompleteResult{
		ResponseCode:    "INVENTORY_PUSH_TASK_CREATED",
		ResponseSummary: string(summary),
		ResourceType:    "inventory_sync_task",
		ResourceID:      taskID.String(),
	})
}

func (s *Service) failInventoryPush(ctx context.Context, job *pushAcquire, code string, retryable bool) {
	if s == nil || s.Idempotency == nil || job == nil {
		return
	}
	_ = s.Idempotency.Fail(ctx, job.RecordID, job.Owner, code, retryable)
}
