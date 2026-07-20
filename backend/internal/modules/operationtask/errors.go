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
	ErrCodeDuplicateIdempotencyKey = "duplicate_idempotency_key"
	ErrCodeDuplicateDraftVersion   = "duplicate_draft_version"
)

var (
	ErrNotFound                = errors.New(ErrCodeNotFound)
	ErrConflict                = errors.New(ErrCodeConflict)
	ErrValidation              = errors.New(ErrCodeValidation)
	ErrTenantMismatch          = errors.New(ErrCodeTenantMismatch)
	ErrRevisionConflict        = errors.New(ErrCodeRevisionConflict)
	ErrDuplicateIdempotencyKey = errors.New(ErrCodeDuplicateIdempotencyKey)
	ErrDuplicateDraftVersion   = errors.New(ErrCodeDuplicateDraftVersion)
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
