package idempotency

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// ExecuteResult is returned when an idempotent operation finishes or replays.
type ExecuteResult struct {
	Decision       Decision
	Record         *Record
	ResourceID     string
	ReplaySummary  string
	AcquiredRecord *Record
}

// ExecuteFn runs business logic after a successful acquire.
type ExecuteFn func(ctx context.Context, recordID uuid.UUID) (CompleteResult, error)

// Execute acquires execution rights and runs fn when allowed.
func (s *Service) Execute(ctx context.Context, scope, key, requestHash, owner string, fn ExecuteFn) (*ExecuteResult, error) {
	if s == nil {
		return nil, fmt.Errorf("idempotency: unavailable")
	}
	res, err := s.Acquire(ctx, scope, key, requestHash, owner, DefaultLease)
	decision, rec, classifyErr := Classify(res, err)
	out := &ExecuteResult{Decision: decision, Record: rec}
	if rec != nil {
		out.ResourceID = rec.ResourceID
		out.ReplaySummary = rec.ResponseSummary
	}
	switch decision {
	case DecisionAlreadySucceeded:
		return out, nil
	case DecisionInProgress:
		return out, fmt.Errorf("%s: %w", ErrCodeInProgress, ErrInProgress)
	case DecisionKeyConflict, DecisionPermanentFailure:
		if classifyErr != nil {
			return out, classifyErr
		}
		return out, ErrKeyConflict
	case DecisionRetryAllowed, DecisionAcquired:
		if res == nil || res.Record == nil {
			return out, fmt.Errorf("idempotency: missing record")
		}
		out.AcquiredRecord = res.Record
		complete, runErr := fn(ctx, res.Record.ID)
		if runErr != nil {
			retryable := true
			_ = s.Fail(ctx, res.Record.ID, owner, runErr.Error(), retryable)
			return out, runErr
		}
		if err := s.Complete(ctx, res.Record.ID, owner, complete); err != nil {
			return out, err
		}
		out.ResourceID = complete.ResourceID
		out.ReplaySummary = complete.ResponseSummary
		out.Decision = DecisionAcquired
		return out, nil
	default:
		return out, classifyErr
	}
}
