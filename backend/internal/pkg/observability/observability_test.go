package observability

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/pkg/logging"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tracing"
)

func TestInitLocalMode(t *testing.T) {
	obs, err := Init(Config{
		Enabled:        true,
		Mode:           "local",
		MetricsEnabled: true,
		TracingEnabled: false,
		Logger: logging.Config{
			Format:      "json",
			Level:       "info",
			Service:     "test",
			Environment: "test",
		},
		Tracer: tracing.Config{Enabled: false, ServiceName: "test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if obs.Metrics == nil || obs.Catalog == nil {
		t.Fatal("metrics expected")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = obs.Shutdown(ctx)
}
