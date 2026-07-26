package inventory

import (
	"testing"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
)

func TestInventoryObservabilityRecordsMetrics(t *testing.T) {
	reg := metrics.NewRegistry("test")
	cat, err := metrics.RegisterCatalog(reg)
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{Metrics: cat}
	svc.ObserveInventory("local", "adjust", "adjust", "success", "", 1, time.Millisecond)
	svc.ObserveInventory("douyin_shop", "push", "unknown_result", "unknown_result", "provider_timeout", 1, time.Millisecond)
	if got := reg.SnapshotValues()["inventory_adjustments_total"]; got == 0 {
		t.Fatalf("inventory_adjustments_total = %v", got)
	}
	if got := reg.SnapshotValues()["inventory_unknown_results_total"]; got == 0 {
		t.Fatalf("inventory_unknown_results_total = %v", got)
	}
}
