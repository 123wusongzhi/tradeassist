package inventorysyncp9

import (
	"errors"
	"strings"
)

const (
	ErrCodeValidation                      = "validation_error"
	ErrCodeNotFound                        = "not_found"
	ErrCodeTenantMismatch                  = "tenant_mismatch"
	ErrCodeRevisionConflict                = "revision_conflict"
	ErrCodeStateConflict                   = "state_conflict"
	ErrCodeDuplicateExternalSKU            = "duplicate_external_sku"
	ErrCodeInvalidIdentifier               = "invalid_identifier"
	ErrCodeNormalizationFailed             = "normalization_failed"
	ErrCodeNoBindingCandidate              = "no_binding_candidate"
	ErrCodeMultipleBindingCandidates       = "multiple_binding_candidates"
	ErrCodeBindingConflict                 = "binding_conflict"
	ErrCodeBindingNotConfirmed             = "binding_not_confirmed"
	ErrCodeCalibrationPolicyInvalid        = "calibration_policy_invalid"
	ErrCodeCalibrationThresholdNotMet      = "calibration_threshold_not_met"
	ErrCodeManualReviewRequired            = "manual_review_required"
	ErrCodeManualBindingAlreadyPending     = "manual_binding_already_pending"
	ErrCodeManualBindingAlreadyResolved    = "manual_binding_already_resolved"
	ErrCodeCandidateLocalSKUNotFound       = "candidate_local_sku_not_found"
	ErrCodeCandidateLocalSKUTenantMismatch = "candidate_local_sku_tenant_mismatch"
	ErrCodePermissionDenied                = "permission_denied"
	ErrCodeIdempotencyPayloadConflict      = "idempotency_payload_conflict"
	ErrCodeImmutableRecord                 = "immutable_record"
	ErrCodeProviderNotRegistered           = "provider_not_registered"
	ErrCodeProviderCapabilityForbidden     = "provider_capability_forbidden"
	ErrCodeProductionCapabilityForbidden   = "production_capability_forbidden"
	ErrCodeProviderCursorInvalid           = "provider_cursor_invalid"
	ErrCodeProviderCursorLoop              = "provider_cursor_loop"
	ErrCodeProviderPageLimitExceeded       = "provider_page_limit_exceeded"
	ErrCodeProviderPageInvalid             = "provider_page_invalid"
	ErrCodeProviderRejected                = "provider_rejected"
	ErrCodeProviderTimeout                 = "provider_timeout"
	ErrCodeSyncRunAlreadyRunning           = "sync_run_already_running"
	ErrCodeSyncCancelled                   = "sync_cancelled"
)

var (
	ErrValidation                      = errors.New(ErrCodeValidation)
	ErrNotFound                        = errors.New(ErrCodeNotFound)
	ErrTenantMismatch                  = errors.New(ErrCodeTenantMismatch)
	ErrRevisionConflict                = errors.New(ErrCodeRevisionConflict)
	ErrStateConflict                   = errors.New(ErrCodeStateConflict)
	ErrDuplicateExternalSKU            = errors.New(ErrCodeDuplicateExternalSKU)
	ErrInvalidIdentifier               = errors.New(ErrCodeInvalidIdentifier)
	ErrNormalizationFailed             = errors.New(ErrCodeNormalizationFailed)
	ErrNoBindingCandidate              = errors.New(ErrCodeNoBindingCandidate)
	ErrMultipleBindingCandidates       = errors.New(ErrCodeMultipleBindingCandidates)
	ErrBindingConflict                 = errors.New(ErrCodeBindingConflict)
	ErrBindingNotConfirmed             = errors.New(ErrCodeBindingNotConfirmed)
	ErrCalibrationPolicyInvalid        = errors.New(ErrCodeCalibrationPolicyInvalid)
	ErrCalibrationThresholdNotMet      = errors.New(ErrCodeCalibrationThresholdNotMet)
	ErrManualReviewRequired            = errors.New(ErrCodeManualReviewRequired)
	ErrManualBindingAlreadyPending     = errors.New(ErrCodeManualBindingAlreadyPending)
	ErrManualBindingAlreadyResolved    = errors.New(ErrCodeManualBindingAlreadyResolved)
	ErrCandidateLocalSKUNotFound       = errors.New(ErrCodeCandidateLocalSKUNotFound)
	ErrCandidateLocalSKUTenantMismatch = errors.New(ErrCodeCandidateLocalSKUTenantMismatch)
	ErrPermissionDenied                = errors.New(ErrCodePermissionDenied)
	ErrIdempotencyPayloadConflict      = errors.New(ErrCodeIdempotencyPayloadConflict)
	ErrImmutableRecord                 = errors.New(ErrCodeImmutableRecord)
	ErrProviderNotRegistered           = errors.New(ErrCodeProviderNotRegistered)
	ErrProviderCapabilityForbidden     = errors.New(ErrCodeProviderCapabilityForbidden)
	ErrProductionCapabilityForbidden   = errors.New(ErrCodeProductionCapabilityForbidden)
	ErrProviderCursorInvalid           = errors.New(ErrCodeProviderCursorInvalid)
	ErrProviderCursorLoop              = errors.New(ErrCodeProviderCursorLoop)
	ErrProviderPageLimitExceeded       = errors.New(ErrCodeProviderPageLimitExceeded)
	ErrProviderPageInvalid             = errors.New(ErrCodeProviderPageInvalid)
	ErrProviderRejected                = errors.New(ErrCodeProviderRejected)
	ErrProviderTimeout                 = errors.New(ErrCodeProviderTimeout)
	ErrSyncRunAlreadyRunning           = errors.New(ErrCodeSyncRunAlreadyRunning)
	ErrSyncCancelled                   = errors.New(ErrCodeSyncCancelled)
)

func stableError(err error, fallback error) error {
	if err == nil {
		return nil
	}
	if fallback == nil {
		fallback = ErrStateConflict
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
