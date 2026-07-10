package ordersync

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
)

const errOrderSyncInProgress = "ORDER_SYNC_IN_PROGRESS"

func syncWindowKey(snap syncInputSnapshot) string {
	parts := []string{
		snap.Mode,
		snap.Start,
		snap.End,
		snap.Cursor,
	}
	return fmt.Sprintf("%s|%s|%s|%s", parts[0], parts[1], parts[2], parts[3])
}

type syncJobAcquire struct {
	RecordID uuid.UUID
	Owner    string
	Key      string
}

func (s *Service) acquireSyncJob(ctx context.Context, platform string, shopID uuid.UUID, snap syncInputSnapshot, inputJSON []byte, forceNew bool, owner string) (*syncJobAcquire, *idempotency.AcquireResult, error) {
	if s == nil || s.Idempotency == nil {
		return nil, nil, nil
	}
	window := syncWindowKey(snap)
	key := idempotency.OrderSyncJob(platform, shopID.String(), snap.Mode, window)
	if forceNew {
		key = key + ":force:" + uuid.New().String()
	}
	res, err := s.Idempotency.Acquire(ctx, idempotency.ScopeOrderSync, key, idempotency.HashRequest(inputJSON), owner, idempotency.DefaultLease)
	decision, rec, _ := idempotency.Classify(res, err)
	switch decision {
	case idempotency.DecisionAlreadySucceeded:
		return nil, res, nil
	case idempotency.DecisionInProgress:
		return nil, res, fmt.Errorf("%s", errOrderSyncInProgress)
	case idempotency.DecisionKeyConflict, idempotency.DecisionPermanentFailure:
		return nil, res, fmt.Errorf("ORDER_SYNC_KEY_CONFLICT")
	case idempotency.DecisionAcquired, idempotency.DecisionRetryAllowed:
		if rec == nil && res != nil {
			rec = res.Record
		}
		if rec == nil {
			return nil, res, fmt.Errorf("idempotency: missing record")
		}
		return &syncJobAcquire{RecordID: rec.ID, Owner: owner, Key: key}, res, nil
	default:
		return nil, res, err
	}
}

func (s *Service) completeSyncJobCreate(ctx context.Context, job *syncJobAcquire, taskID uuid.UUID) error {
	if s == nil || s.Idempotency == nil || job == nil {
		return nil
	}
	summary, _ := json.Marshal(map[string]string{"taskId": taskID.String()})
	return s.Idempotency.Complete(ctx, job.RecordID, job.Owner, idempotency.CompleteResult{
		ResponseCode:    "ORDER_SYNC_TASK_CREATED",
		ResponseSummary: string(summary),
		ResourceType:    "order_sync_task",
		ResourceID:      taskID.String(),
	})
}

func (s *Service) failSyncJobCreate(ctx context.Context, job *syncJobAcquire, code string, retryable bool) {
	if s == nil || s.Idempotency == nil || job == nil {
		return
	}
	_ = s.Idempotency.Fail(ctx, job.RecordID, job.Owner, code, retryable)
}

func (s *Service) resolveExistingSyncTask(ctx context.Context, res *idempotency.AcquireResult) (*TaskDTO, error) {
	if res == nil {
		return nil, fmt.Errorf("%s", errOrderSyncInProgress)
	}
	rid := res.ResourceID
	if rid == "" && res.Record != nil {
		rid = res.Record.ResourceID
	}
	if rid == "" {
		return nil, fmt.Errorf("%s", errOrderSyncInProgress)
	}
	tid, err := uuid.Parse(rid)
	if err != nil {
		return nil, fmt.Errorf("%s", errOrderSyncInProgress)
	}
	out, err := s.GetDTO(ctx, tid)
	if err != nil {
		return nil, fmt.Errorf("%s", errOrderSyncInProgress)
	}
	return &out, nil
}
