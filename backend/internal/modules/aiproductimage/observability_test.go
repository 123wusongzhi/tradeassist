package aiproductimage

import (
	"testing"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
)

func TestAIImageObservabilityRecordsMetrics(t *testing.T) {
	reg := metrics.NewRegistry("test")
	cat, err := metrics.RegisterCatalog(reg)
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{Metrics: cat}
	svc.ObserveAIImage("mock", OpWhiteBackground, "request", "success", "", time.Millisecond)
	svc.ObserveAIImage("mock", OpWhiteBackground, "timeout", "timeout", "provider_timeout", time.Millisecond)
	if got := reg.SnapshotValues()["ai_image_requests_total"]; got == 0 {
		t.Fatalf("ai_image_requests_total = %v", got)
	}
	if got := reg.SnapshotValues()["ai_image_provider_timeouts_total"]; got == 0 {
		t.Fatalf("ai_image_provider_timeouts_total = %v", got)
	}
}
