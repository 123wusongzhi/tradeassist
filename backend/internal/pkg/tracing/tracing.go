package tracing

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds tracer settings.
type Config struct {
	Enabled       bool
	ServiceName   string
	Version       string
	Environment   string
	SampleRatio   float64
	ExportStdout  bool
	OTLPEndpoint  string
	ExportTimeout time.Duration
}

// Provider wraps OTel tracer provider with safe shutdown.
type Provider struct {
	cfg            Config
	provider       *sdktrace.TracerProvider
	tracer         trace.Tracer
	mu             sync.Mutex
	closed         bool
	exportBlocked  bool
	exportFailures int64
}

// Init creates and installs global tracer provider.
func Init(cfg Config) (*Provider, error) {
	p := &Provider{cfg: cfg}
	if !cfg.Enabled {
		otel.SetTracerProvider(trace.NewNoopTracerProvider())
		p.tracer = otel.Tracer(cfg.ServiceName)
		return p, nil
	}
	if cfg.SampleRatio <= 0 {
		cfg.SampleRatio = 0.1
	}
	if cfg.SampleRatio > 1 {
		cfg.SampleRatio = 1
	}
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.Version),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, err
	}
	var exporters []sdktrace.SpanExporter
	if cfg.ExportStdout {
		exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, err
		}
		exporters = append(exporters, exp)
	}
	if ep := strings.TrimSpace(cfg.OTLPEndpoint); ep != "" {
		// OTLP HTTP exporter deferred when dependency graph blocked; mark export blocked.
		p.exportBlocked = true
	}
	var spanExporter sdktrace.SpanExporter
	if len(exporters) == 1 {
		spanExporter = exporters[0]
	} else if len(exporters) > 1 {
		spanExporter = exporters[0]
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
	)
	if spanExporter != nil {
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(spanExporter),
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
		)
	}
	otel.SetTracerProvider(tp)
	p.provider = tp
	p.tracer = tp.Tracer(cfg.ServiceName)
	return p, nil
}

// Tracer returns the application tracer.
func (p *Provider) Tracer() trace.Tracer {
	if p == nil || p.tracer == nil {
		return otel.Tracer("trademind")
	}
	return p.tracer
}

// ExportBlocked reports OTLP environment blocked state.
func (p *Provider) ExportBlocked() bool {
	if p == nil {
		return true
	}
	return p.exportBlocked
}

// Shutdown flushes and shuts down tracer provider.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil || p.provider == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	return p.provider.Shutdown(ctx)
}

// StartSpan starts a child span with safe attributes only.
func StartSpan(ctx context.Context, tracer trace.Tracer, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if tracer == nil {
		tracer = otel.Tracer("trademind")
	}
	safe := sanitizeAttrs(attrs)
	return tracer.Start(ctx, name, trace.WithAttributes(safe...))
}

// EndSpan ends span with optional error type.
func EndSpan(span trace.Span, err error, errorType string) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, safeErrorType(errorType, err))
		if errorType != "" {
			span.SetAttributes(attribute.String("error.type", errorType))
		}
	}
	span.End()
}

func safeErrorType(errorType string, err error) string {
	if strings.TrimSpace(errorType) != "" {
		return errorType
	}
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "timeout"):
		return "timeout"
	case strings.Contains(msg, "circuit"):
		return "circuit_open"
	default:
		return "error"
	}
}

var forbiddenAttrKeys = []string{
	"authorization", "cookie", "token", "secret", "password", "api_key",
	"prompt", "signed_url", "raw_payload", "phone", "email", "address",
}

func sanitizeAttrs(attrs []attribute.KeyValue) []attribute.KeyValue {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]attribute.KeyValue, 0, len(attrs))
	for _, a := range attrs {
		k := strings.ToLower(string(a.Key))
		blocked := false
		for _, f := range forbiddenAttrKeys {
			if strings.Contains(k, f) {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		out = append(out, a)
	}
	return out
}

// ParseTraceParent parses W3C traceparent header.
func ParseTraceParent(raw string) (trace.TraceID, trace.SpanID, error) {
	raw = strings.TrimSpace(raw)
	parts := strings.Split(raw, "-")
	if len(parts) != 4 || parts[0] != "00" {
		return trace.TraceID{}, trace.SpanID{}, fmt.Errorf("invalid traceparent")
	}
	tid, err := trace.TraceIDFromHex(parts[1])
	if err != nil {
		return trace.TraceID{}, trace.SpanID{}, err
	}
	sid, err := trace.SpanIDFromHex(parts[2])
	if err != nil {
		return trace.TraceID{}, trace.SpanID{}, err
	}
	return tid, sid, nil
}

// FormatTraceParent formats W3C traceparent from span context.
func FormatTraceParent(sc trace.SpanContext) string {
	if !sc.IsValid() {
		return ""
	}
	return fmt.Sprintf("00-%s-%s-01", sc.TraceID().String(), sc.SpanID().String())
}

// LinkParent links consumer span to parent trace context stored as traceparent string.
func LinkParent(ctx context.Context, tracer trace.Tracer, name, traceParent string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	tid, sid, err := ParseTraceParent(traceParent)
	if err != nil {
		return StartSpan(ctx, tracer, name, attrs...)
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: tid,
		SpanID:  sid,
		Remote:  true,
	})
	ctx = trace.ContextWithSpanContext(ctx, sc)
	return StartSpan(ctx, tracer, name, attrs...)
}
