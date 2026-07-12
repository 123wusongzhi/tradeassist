package tracing

import (
	"context"
	"testing"

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
