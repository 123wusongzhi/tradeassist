package douyinshop_test

import (
	"net/http"
	"testing"

	douyinshop "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

func TestRetryAfterSeconds_NumericHeader(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "30")
	secs := douyinshop.RetryAfterSeconds(h)
	if secs != 30 {
		t.Errorf("expected 30, got %d", secs)
	}
}

func TestRetryAfterSeconds_MissingHeader(t *testing.T) {
	h := http.Header{}
	secs := douyinshop.RetryAfterSeconds(h)
	if secs != 0 {
		t.Errorf("expected 0, got %d", secs)
	}
}

func TestRetryAfterSeconds_NilHeader(t *testing.T) {
	secs := douyinshop.RetryAfterSeconds(nil)
	if secs != 0 {
		t.Errorf("expected 0, got %d", secs)
	}
}

func TestRetryAfterSeconds_HTTPDate(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "Wed, 21 Oct 2015 07:28:00 GMT")
	secs := douyinshop.RetryAfterSeconds(h)
	// Non-numeric → approximated as 60s
	if secs != 60 {
		t.Errorf("expected 60 for HTTP-date format, got %d", secs)
	}
}

func TestEnrichRateLimitError_AddsRetryAfter(t *testing.T) {
	e := douyinshop.NewError(douyinshop.CodeDouyinRateLimited, "rate limited", "", "", "")
	h := http.Header{}
	h.Set("Retry-After", "45")
	enriched := douyinshop.EnrichRateLimitError(e, h)
	if enriched.RetryAfter != 45 {
		t.Errorf("expected RetryAfter=45, got %d", enriched.RetryAfter)
	}
}

func TestNewUnknownResultError(t *testing.T) {
	err := douyinshop.NewUnknownResultError("product.addV2", "req-123")
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if err.Code != douyinshop.CodeDouyinUnknownResult {
		t.Errorf("expected code %s, got %s", douyinshop.CodeDouyinUnknownResult, err.Code)
	}
	if !err.UnknownResult {
		t.Error("expected UnknownResult=true")
	}
	if err.SafeRetry {
		t.Error("expected SafeRetry=false for unknown result")
	}
	if !err.ManualReviewRequired {
		t.Error("expected ManualReviewRequired=true")
	}
}
