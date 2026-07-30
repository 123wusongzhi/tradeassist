package ordersync

import (
	"testing"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
)

func TestOrderSyncObservabilityRecordsMetrics(t *testing.T) {
	reg := metrics.NewRegistry("test")
	cat, err := metrics.RegisterCatalog(reg)
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{Metrics: cat}
	svc.ObserveOrder("douyin_shop", "polling", "run", "success", "", 1, time.Millisecond, 0)
	svc.ObserveOrder("douyin_shop", "polling", "created", "success", "", 2, 0, 0)
	if got := reg.SnapshotValues()["order_sync_runs_total"]; got == 0 {
		t.Fatalf("order_sync_runs_total = %v", got)
	}
	if got := reg.SnapshotValues()["order_sync_orders_created_total"]; got == 0 {
		t.Fatalf("order_sync_orders_created_total = %v", got)
	}
}
