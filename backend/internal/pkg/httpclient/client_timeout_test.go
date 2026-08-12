package httpclient

import (
	"testing"
	"time"
)

func TestLimitedStdHTTPConfigCanAlignResponseHeaderWithRequestBudget(t *testing.T) {
	t.Parallel()

	completionBudget := 3 * time.Minute
	aligned := limitedStdHTTPConfig(completionBudget, completionBudget)
	if aligned.RequestTimeout != completionBudget {
		t.Fatalf("request timeout = %v, want %v", aligned.RequestTimeout, completionBudget)
	}
	if aligned.ResponseHeaderTimeout != completionBudget {
		t.Fatalf("response header timeout = %v, want %v", aligned.ResponseHeaderTimeout, completionBudget)
	}

	legacy := limitedStdHTTPConfig(completionBudget, 0)
	if legacy.ResponseHeaderTimeout != 30*time.Second {
		t.Fatalf("zero override must preserve shared default, got %v", legacy.ResponseHeaderTimeout)
	}
}
