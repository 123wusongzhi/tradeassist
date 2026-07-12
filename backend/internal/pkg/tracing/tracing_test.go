package tracing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

func TestInitNoopTracer(t *testing.T) {
	p, err := Init(Config{Enabled: false, ServiceName: "test"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, span := StartSpan(context.Background(), p.Tracer(), "test", attribute.String("authorization", "TEST_ACCESS_TOKEN_UNIQUE"))
	EndSpan(span, nil, "")
	if span == nil {
		t.Fatal("expected span")
	}
	_ = ctx
}

func TestParseTraceParentInvalid(t *testing.T) {
	if _, _, err := ParseTraceParent("invalid"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSanitizeAttrsDropsSecrets(t *testing.T) {
	attrs := sanitizeAttrs([]attribute.KeyValue{
		attribute.String("app.module", "webhook"),
		attribute.String("access_token", "TEST_ACCESS_TOKEN_UNIQUE"),
	})
	for _, a := range attrs {
		if stringsContains(string(a.Key), "token") {
			t.Fatal("token attr should be dropped")
		}
	}
}

func TestHTTPExporterSendsSpanToMockCollector(t *testing.T) {
	var received atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/traces" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		received.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	var exported atomic.Int64
	p, err := Init(Config{
		Enabled:       true,
		ServiceName:   "test",
		SampleRatio:   1,
		OTLPEndpoint:  srv.URL,
		ExportTimeout: time.Second,
		OnExportOK: func(n int) {
			exported.Add(int64(n))
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, span := StartSpan(context.Background(), p.Tracer(), "mock-export", attribute.String("access_token", "TEST_ACCESS_TOKEN_UNIQUE"))
	EndSpan(span, nil, "")
	_ = ctx
	shCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := p.Shutdown(shCtx); err != nil {
		t.Fatal(err)
	}
	if received.Load() == 0 || exported.Load() == 0 {
		t.Fatalf("expected mock collector to receive span, received=%d exported=%d", received.Load(), exported.Load())
	}
}

func stringsContains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) > 0 && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
