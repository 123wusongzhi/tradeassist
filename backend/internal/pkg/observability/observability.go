package observability

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/trademind-ai/trademind/backend/internal/pkg/logging"
	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tracing"
)

// Config bundles observability settings.
type Config struct {
	Enabled         bool
	Mode            string
	Environment     string
	MetricsEnabled  bool
	MetricsPath     string
	MetricsInternal bool
	TracingEnabled  bool
	AlertingEnabled bool
	Logger          logging.Config
	Tracer          tracing.Config
}

// Observability is the unified facade for logs, metrics and traces.
type Observability struct {
	Config  Config
	Logger  logging.Logger
	Metrics *metrics.Registry
	Catalog *metrics.Catalog
	Tracer  *tracing.Provider
	started time.Time
	mu      sync.RWMutex
	ready   float64
}

// Init wires logger, metrics registry and tracer.
func Init(cfg Config) (*Observability, error) {
	o := &Observability{
		Config:  cfg,
		Logger:  logging.New(cfg.Logger),
		started: time.Now().UTC(),
	}
	if cfg.MetricsEnabled {
		ns := cfg.Logger.Service
		if ns == "" {
			ns = "trademind"
		}
		o.Metrics = metrics.NewRegistry(ns)
		cat, err := metrics.RegisterCatalog(o.Metrics)
		if err != nil {
			return nil, err
		}
		o.Catalog = cat
	}
	if cfg.TracingEnabled {
		tp, err := tracing.Init(cfg.Tracer)
		if err != nil {
			return nil, err
		}
		o.Tracer = tp
	} else {
		tp, _ := tracing.Init(tracing.Config{Enabled: false, ServiceName: cfg.Tracer.ServiceName})
		o.Tracer = tp
	}
	o.setReadiness(1)
	return o, nil
}

func (o *Observability) setReadiness(v float64) {
	o.mu.Lock()
	o.ready = v
	o.mu.Unlock()
	if o.Metrics != nil && o.Catalog != nil {
		g, _ := o.Metrics.Gauge("app_readiness", "Application readiness", "component")
		if g != nil {
			g.WithLabelValues("core").Set(v)
		}
		ts, _ := o.Metrics.Gauge("app_start_timestamp", "Application start timestamp")
		if ts != nil {
			ts.WithLabelValues().Set(float64(o.started.Unix()))
		}
	}
}

// Shutdown flushes telemetry.
func (o *Observability) Shutdown(ctx context.Context) error {
	if o == nil {
		return nil
	}
	if o.Tracer != nil {
		return o.Tracer.Shutdown(ctx)
	}
	return nil
}

// MetricsHandler returns prometheus HTTP handler.
func (o *Observability) MetricsHandler() http.Handler {
	if o == nil || o.Metrics == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("metrics disabled"))
		})
	}
	return promhttp.HandlerFor(o.Metrics.Prometheus(), promhttp.HandlerOpts{
		Timeout: 10 * time.Second,
	})
}

// RecordTelemetryExportFailure increments export failure metric.
func (o *Observability) RecordTelemetryExportFailure() {
	if o != nil && o.Catalog != nil && o.Catalog.TelemetryExportFailures != nil {
		o.Catalog.TelemetryExportFailures.Inc()
	}
}

// RecordTelemetryDropped increments dropped telemetry items.
func (o *Observability) RecordTelemetryDropped(n int) {
	if o != nil && o.Catalog != nil && o.Catalog.TelemetryDroppedItems != nil && n > 0 {
		o.Catalog.TelemetryDroppedItems.Add(float64(n))
	}
}
