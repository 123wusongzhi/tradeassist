package douyinshop

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// NewUnifiedHTTPDoer builds a standard *http.Client HTTPDoer with configurable timeout.
// The douyinshop HTTPDoer interface expects Do(*http.Request) (*http.Response, error).
// For advanced circuit-breaker support, wrap with a custom adapter.
func NewUnifiedHTTPDoer(timeout time.Duration) HTTPDoer {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &http.Client{Timeout: timeout}
}

// RetryAfterSeconds parses the Retry-After response header into seconds.
// Returns 0 if the header is absent or unparseable.
func RetryAfterSeconds(h http.Header) int64 {
	if h == nil {
		return 0
	}
	val := strings.TrimSpace(h.Get("Retry-After"))
	if val == "" {
		return 0
	}
	if n, err := strconv.ParseInt(val, 10, 64); err == nil && n > 0 {
		return n
	}
	// HTTP-date format — approximate as 60 seconds
	return 60
}

// EnrichRateLimitError attaches RetryAfter seconds from response header to *Error.
func EnrichRateLimitError(e *Error, h http.Header) *Error {
	if e == nil {
		return e
	}
	secs := RetryAfterSeconds(h)
	if secs > 0 {
		e.RetryAfter = secs
	}
	return e
}

// NewUnknownResultError creates an error indicating the outcome of a write is unknown
// (e.g. timeout after sending the request). Operators must manually verify.
func NewUnknownResultError(method, requestID string) *Error {
	e := NewError(CodeDouyinUnknownResult,
		"抖店接口调用超时，写入结果未知，请人工核查平台草稿箱后再重试",
		"", "timeout_after_write", requestID)
	e.UnknownResult = true
	e.SafeRetry = false
	e.ManualReviewRequired = true
	e.ErrorClass = ErrorClassUnknownResult
	_ = method
	return e
}
