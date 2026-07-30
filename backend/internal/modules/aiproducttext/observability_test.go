package aiproducttext

import (
	"testing"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
)

func TestAITextObservabilityRecordsMetrics(t *testing.T) {
	reg := metrics.NewRegistry("test")
	cat, err := metrics.RegisterCatalog(reg)
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{Metrics: cat}
	svc.ObserveAIText("mock", OpTitle, "request", "success", "", time.Millisecond)
	svc.ObserveAIText("mock", OpTitle, "timeout", "timeout", "provider_timeout", time.Millisecond)
	if got := reg.SnapshotValues()["ai_text_requests_total"]; got == 0 {
		t.Fatalf("ai_text_requests_total = %v", got)
	}
	if got := reg.SnapshotValues()["ai_text_provider_timeouts_total"]; got == 0 {
		t.Fatalf("ai_text_provider_timeouts_total = %v", got)
	}
}
