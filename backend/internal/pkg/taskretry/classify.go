package taskretry

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
)

// Error classification codes.
const (
	CodeTimeout              = "timeout"
	CodeNetworkError         = "network_error"
	CodeProvider5xx          = "provider_5xx"
	CodeRateLimited          = "rate_limited"
	CodeTemporaryUnavailable = "temporary_unavailable"
	CodeLeaseExpired         = "lease_expired"
	CodeRedisTemporary       = "redis_temporary_failure"

	CodeInvalidRequest       = "invalid_request"
	CodePermissionDenied     = "permission_denied"
	CodeInvalidCredentials   = "invalid_credentials"
	CodeUnsupportedOperation = "unsupported_operation"
	CodeValidationFailed     = "validation_failed"
	CodeResourceNotFound     = "resource_not_found"
	CodeIdempotencyConflict  = "idempotency_conflict"
	CodeBusinessRuleRejected = "business_rule_rejected"

	CodeCredentialRefreshRequired = "credential_refresh_required"
)

// Classification describes retry behavior for an error.
type Classification struct {
	Code      string
	Retryable bool
	Message   string
}

// Classify maps errors and HTTP status to retry classification.
func Classify(err error, httpStatus int) Classification {
	if err == nil {
		return Classification{Code: "", Retryable: false}
	}
	msg := err.Error()
	lower := strings.ToLower(msg)

	switch {
	case strings.Contains(lower, "credential_refresh"), strings.Contains(lower, "auth expired"), strings.Contains(lower, "token expired"):
		return Classification{Code: CodeCredentialRefreshRequired, Retryable: false, Message: msg}
	case strings.Contains(lower, "idempotency"), strings.Contains(lower, "duplicate"):
		return Classification{Code: CodeIdempotencyConflict, Retryable: false, Message: msg}
	case strings.Contains(lower, "permission denied"), strings.Contains(lower, "forbidden"):
		return Classification{Code: CodePermissionDenied, Retryable: false, Message: msg}
	case strings.Contains(lower, "not found"):
		return Classification{Code: CodeResourceNotFound, Retryable: false, Message: msg}
	case strings.Contains(lower, "validation"), strings.Contains(lower, "invalid"):
		return Classification{Code: CodeValidationFailed, Retryable: false, Message: msg}
	case strings.Contains(lower, "lease lost"), strings.Contains(lower, "lease expired"):
		return Classification{Code: CodeLeaseExpired, Retryable: true, Message: msg}
	case strings.Contains(lower, "rate limit"), strings.Contains(lower, "429"):
		return Classification{Code: CodeRateLimited, Retryable: true, Message: msg}
	case strings.Contains(lower, "timeout"), errors.Is(err, context.DeadlineExceeded):
		return Classification{Code: CodeTimeout, Retryable: true, Message: msg}
	case strings.Contains(lower, "connection"), strings.Contains(lower, "network"), strings.Contains(lower, "eof"):
		return Classification{Code: CodeNetworkError, Retryable: true, Message: msg}
	case strings.Contains(lower, "redis"):
		return Classification{Code: CodeRedisTemporary, Retryable: true, Message: msg}
	}

	if httpStatus == http.StatusTooManyRequests {
		return Classification{Code: CodeRateLimited, Retryable: true, Message: msg}
	}
	if httpStatus >= 500 && httpStatus < 600 {
		return Classification{Code: CodeProvider5xx, Retryable: true, Message: msg}
	}
	if httpStatus >= 400 && httpStatus < 500 {
		return Classification{Code: CodeInvalidRequest, Retryable: false, Message: msg}
	}
	return Classification{Code: CodeTemporaryUnavailable, Retryable: true, Message: msg}
}

// ParseRetryAfter parses Retry-After header value (seconds or HTTP-date).
func ParseRetryAfter(v string) (seconds int, ok bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	if n, err := strconvAtoi(v); err == nil && n > 0 {
		return n, true
	}
	if t, err := http.ParseTime(v); err == nil {
		sec := int(time.Until(t).Seconds())
		if sec > 0 {
			return sec, true
		}
	}
	return 0, false
}

func strconvAtoi(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errors.New("not int")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
