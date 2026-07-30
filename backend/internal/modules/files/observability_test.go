package files

import (
	"testing"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
)

func TestFileScanObservabilityRecordsMetrics(t *testing.T) {
	reg := metrics.NewRegistry("test")
	cat, err := metrics.RegisterCatalog(reg)
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{Metrics: cat}
	svc.ObserveFileScan("basic", "enqueue", "queued", "image", 0)
	svc.ObserveFileScan("basic", "result", "rejected", "image", time.Millisecond)
	if got := reg.SnapshotValues()["file_scan_tasks_total"]; got == 0 {
		t.Fatalf("file_scan_tasks_total = %v", got)
	}
	if got := reg.SnapshotValues()["file_scan_results_total"]; got == 0 {
		t.Fatalf("file_scan_results_total = %v", got)
	}
}
