package collect

import (
	"net/http"
	"testing"
)

func TestCollectEngineRoutingErrorIsTerminal(t *testing.T) {
	err := &CollectEngineRoutingError{
		Code:       "COLLECT_ENGINE_DISABLED",
		Message:    "playwright collect engine is disabled",
		HTTPStatus: http.StatusServiceUnavailable,
	}
	if got := collectorErrorCode(err); got != "COLLECT_ENGINE_DISABLED" {
		t.Fatalf("unexpected routing error code: %q", got)
	}
	if !collectErrNonRetryable(err, nil, BatchSourcePolicy{}) {
		t.Fatal("disabled engine error must not be retried")
	}
	extras := collectorRejectExtras(err)
	if extras["collectorCode"] != "COLLECT_ENGINE_DISABLED" || extras["errorType"] != "collect_engine_disabled" {
		t.Fatalf("unexpected routing error extras: %#v", extras)
	}
}
