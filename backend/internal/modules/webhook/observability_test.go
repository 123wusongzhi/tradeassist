package webhook

import (
	"testing"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
)

func TestWebhookObservabilityRecordsMetrics(t *testing.T) {
	reg := metrics.NewRegistry("test")
	cat, err := metrics.RegisterCatalog(reg)
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{Metrics: cat}
	svc.ObserveWebhook("internal_test", "order.created", "request", "success", "", time.Millisecond)
	svc.ObserveWebhook("internal_test", "order.created", "persisted", "success", "", 0)
	cat.ObserveWebhookProcessed("internal_test", "order", "success", "", time.Millisecond, time.Millisecond)
	if got := reg.SnapshotValues()["webhook_requests_total"]; got == 0 {
		t.Fatalf("webhook_requests_total = %v", got)
	}
	if got := reg.SnapshotValues()["webhook_events_persisted_total"]; got == 0 {
		t.Fatalf("webhook_events_persisted_total = %v", got)
	}
	if got := reg.SnapshotValues()["webhook_events_processed_total"]; got == 0 {
		t.Fatalf("webhook_events_processed_total = %v", got)
	}
}
