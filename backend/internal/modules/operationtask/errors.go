package operationtask

import (
	"errors"
	"strings"
)

const (
	ErrCodeNotFound                = "not_found"
	ErrCodeConflict                = "conflict"
	ErrCodeValidation              = "validation_error"
	ErrCodeTenantMismatch          = "tenant_mismatch"
	ErrCodeRevisionConflict        = "revision_conflict"
	ErrCodePermissionDenied        = "permission_denied"
	ErrCodeInvalidTransition       = "invalid_transition"
	ErrCodeStateConflict           = "state_conflict"
	ErrCodeDraftNotLatest          = "draft_not_latest"
	ErrCodeDraftVersionMismatch    = "draft_version_mismatch"
	ErrCodeDraftHashMismatch       = "draft_hash_mismatch"
	ErrCodeApprovalIdemConflict    = "approval_idempotency_conflict"
	ErrCodeDuplicateRequest        = "duplicate_request"
	ErrCodeDuplicateIdempotencyKey = "duplicate_idempotency_key"
	ErrCodeDuplicateDraftVersion   = "duplicate_draft_version"
	ErrCodeDuplicateApprovalIdem   = "duplicate_approval_idempotency"
	ErrCodeDuplicateExecutionIdem  = "duplicate_execution_idempotency"
	ErrCodeDuplicateAttemptNumber  = "duplicate_attempt_number"
	ErrCodeDuplicateErrorSequence  = "duplicate_error_sequence"
	ErrCodeDuplicateEventSequence  = "duplicate_event_sequence"
	ErrCodeImmutableRecord         = "immutable_record"
	ErrCodeReferenceMismatch       = "reference_mismatch"
	ErrCodeExecutionModeForbidden  = "execution_mode_forbidden"
	ErrCodeDraftBindingConflict    = "draft_binding_conflict"
	ErrCodeExecutionInProgress     = "execution_already_in_progress"
	ErrCodeIdemPayloadConflict     = "idempotency_payload_conflict"
	ErrCodeRetryLimitExceeded      = "retry_limit_exceeded"
	ErrCodeFinalizeConflict        = "finalize_conflict"
	ErrCodeResultUnknown           = "result_unknown"
)

var (
	ErrNotFound                = errors.New(ErrCodeNotFound)
	ErrConflict                = errors.New(ErrCodeConflict)
	ErrValidation              = errors.New(ErrCodeValidation)
	ErrTenantMismatch          = errors.New(ErrCodeTenantMismatch)
	ErrRevisionConflict        = errors.New(ErrCodeRevisionConflict)
	ErrPermissionDenied        = errors.New(ErrCodePermissionDenied)
	ErrInvalidTransition       = errors.New(ErrCodeInvalidTransition)
	ErrStateConflict           = errors.New(ErrCodeStateConflict)
	ErrDraftNotLatest          = errors.New(ErrCodeDraftNotLatest)
	ErrDraftVersionMismatch    = errors.New(ErrCodeDraftVersionMismatch)
	ErrDraftHashMismatch       = errors.New(ErrCodeDraftHashMismatch)
	ErrApprovalIdemConflict    = errors.New(ErrCodeApprovalIdemConflict)
	ErrDuplicateRequest        = errors.New(ErrCodeDuplicateRequest)
	ErrDuplicateIdempotencyKey = errors.New(ErrCodeDuplicateIdempotencyKey)
	ErrDuplicateDraftVersion   = errors.New(ErrCodeDuplicateDraftVersion)
	ErrDuplicateApprovalIdem   = errors.New(ErrCodeDuplicateApprovalIdem)
	ErrDuplicateExecutionIdem  = errors.New(ErrCodeDuplicateExecutionIdem)
	ErrDuplicateAttemptNumber  = errors.New(ErrCodeDuplicateAttemptNumber)
	ErrDuplicateErrorSequence  = errors.New(ErrCodeDuplicateErrorSequence)
	ErrDuplicateEventSequence  = errors.New(ErrCodeDuplicateEventSequence)
	ErrImmutableRecord         = errors.New(ErrCodeImmutableRecord)
	ErrReferenceMismatch       = errors.New(ErrCodeReferenceMismatch)
	ErrExecutionModeForbidden  = errors.New(ErrCodeExecutionModeForbidden)
	ErrDraftBindingConflict    = errors.New(ErrCodeDraftBindingConflict)
	ErrExecutionInProgress     = errors.New(ErrCodeExecutionInProgress)
	ErrIdemPayloadConflict     = errors.New(ErrCodeIdemPayloadConflict)
	ErrRetryLimitExceeded      = errors.New(ErrCodeRetryLimitExceeded)
	ErrFinalizeConflict        = errors.New(ErrCodeFinalizeConflict)
	ErrResultUnknown           = errors.New(ErrCodeResultUnknown)
)

func stableError(err error, fallback error) error {
	if err == nil {
		return nil
	}
	if fallback == nil {
		fallback = ErrConflict
	}
	return fallback
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "unique violation") ||
		strings.Contains(msg, "constraint failed") ||
		strings.Contains(msg, "sqlstate 23505")
}
