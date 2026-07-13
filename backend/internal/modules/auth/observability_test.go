package auth

import (
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
)

func TestAuthObservabilityRecordsMetrics(t *testing.T) {
	reg := metrics.NewRegistry("test")
	cat, err := metrics.RegisterCatalog(reg)
	if err != nil {
		t.Fatal(err)
	}
	svc := &SessionService{Metrics: cat}
	svc.ObserveAuth("login_attempt", "success", "attempt", "password")
	svc.ObserveAuth("refresh_reuse", "failure", "reuse_detected", "refresh_token")
	if got := reg.SnapshotValues()["auth_login_attempts_total"]; got == 0 {
		t.Fatalf("auth_login_attempts_total = %v", got)
	}
	if got := reg.SnapshotValues()["auth_refresh_reuse_detected_total"]; got == 0 {
		t.Fatalf("auth_refresh_reuse_detected_total = %v", got)
	}
}
