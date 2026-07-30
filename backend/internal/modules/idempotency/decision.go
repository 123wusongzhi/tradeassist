package idempotency

import (
	"errors"
)

// Decision describes how callers should react to an Acquire attempt.
type Decision string

const (
	DecisionAcquired         Decision = "acquired"
	DecisionAlreadySucceeded Decision = "already_succeeded"
	DecisionInProgress       Decision = "in_progress"
	DecisionRetryAllowed     Decision = "retry_allowed"
	DecisionKeyConflict      Decision = "key_conflict"
	DecisionPermanentFailure Decision = "permanent_failure"
)

// Classify maps Acquire output to a stable decision for business handlers.
func Classify(res *AcquireResult, err error) (Decision, *Record, error) {
	if err == nil {
		if res != nil && res.Acquired {
			return DecisionAcquired, res.Record, nil
		}
		if res != nil && res.Replay {
			return DecisionAlreadySucceeded, res.Record, nil
		}
		return DecisionAcquired, nil, nil
	}
	var op *OpError
	if errors.As(err, &op) && op != nil {
		rec := op.Record
		switch op.Code {
		case ErrCodeAlreadySucceeded:
			return DecisionAlreadySucceeded, rec, nil
		case ErrCodeInProgress:
			return DecisionInProgress, rec, nil
		case ErrCodeKeyConflict:
			if rec != nil && rec.Status == StatusFailedPermanent {
				return DecisionPermanentFailure, rec, nil
			}
			return DecisionKeyConflict, rec, err
		case ErrCodeRecordExpired:
			return DecisionRetryAllowed, rec, nil
		default:
			return DecisionKeyConflict, rec, err
		}
	}
	return DecisionKeyConflict, nil, err
}

// OwnerFromRequest builds a stable owner id from request metadata.
func OwnerFromRequest(requestID, fallback string) string {
	if requestID != "" {
		return requestID
	}
	return fallback
}
