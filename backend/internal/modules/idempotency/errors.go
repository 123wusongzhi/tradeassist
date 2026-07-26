package idempotency

import "errors"

// Error codes for idempotency operations.
const (
	ErrCodeInProgress       = "IDEMPOTENCY_IN_PROGRESS"
	ErrCodeKeyConflict      = "IDEMPOTENCY_KEY_CONFLICT"
	ErrCodeAlreadySucceeded = "IDEMPOTENCY_ALREADY_SUCCEEDED"
	ErrCodeLeaseLost        = "IDEMPOTENCY_LEASE_LOST"
	ErrCodeRecordExpired    = "IDEMPOTENCY_RECORD_EXPIRED"
)

var (
	ErrInProgress       = errors.New(ErrCodeInProgress)
	ErrKeyConflict      = errors.New(ErrCodeKeyConflict)
	ErrAlreadySucceeded = errors.New(ErrCodeAlreadySucceeded)
	ErrLeaseLost        = errors.New(ErrCodeLeaseLost)
	ErrRecordExpired    = errors.New(ErrCodeRecordExpired)
)

// OpError wraps idempotency business errors with optional record reference.
type OpError struct {
	Code     string
	Message  string
	RecordID string
	Record   *Record
}

func (e *OpError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Code + ": " + e.Message
	}
	return e.Code
}

func newOpErr(code, msg string, rec *Record) *OpError {
	id := ""
	if rec != nil {
		id = rec.ID.String()
	}
	return &OpError{Code: code, Message: msg, RecordID: id, Record: rec}
}
