package securitymod

import (
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
)

func TestSecurityObservabilityRecordsMetrics(t *testing.T) {
	reg := metrics.NewRegistry("test")
	cat, err := metrics.RegisterCatalog(reg)
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{Metrics: cat}
	svc.ObserveSecurity("tenant", "tenant_access_denied", "failure", "warning")
	svc.ObserveSecurity("operationlog", "audit_chain_mismatch", "failure", "critical")
	if got := reg.SnapshotValues()["security_events_total"]; got == 0 {
		t.Fatalf("security_events_total = %v", got)
	}
	if got := reg.SnapshotValues()["audit_chain_mismatch_total"]; got == 0 {
		t.Fatalf("audit_chain_mismatch_total = %v", got)
	}
}
