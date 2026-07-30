package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
)

func TestHTTPClientObservabilityRecordsProviderRequest(t *testing.T) {
	reg := metrics.NewRegistry("test")
	cat, err := metrics.RegisterCatalog(reg)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(DefaultConfig(), nil, 0)
	c.SetObservability(cat, "mock", "catalog")
	_, err = c.DoWithRetry(context.Background(), func(int) (*http.Request, error) {
		return http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := reg.SnapshotValues()["provider_requests_total"]; got == 0 {
		t.Fatalf("provider_requests_total = %v", got)
	}
	if got := reg.SnapshotValues()["provider_request_duration_seconds"]; got == 0 {
		t.Fatalf("provider_request_duration_seconds = %v", got)
	}
	_ = time.Second
}
