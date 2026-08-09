# P5-V OTLP Dependency Matrix

Phase: P5-V
Status: standard OTLP/HTTP JSON implemented without adding the official OTLP exporter dependency.

## Current Dependencies

| Dependency | Version | Reason |
| --- | --- | --- |
| `go.opentelemetry.io/otel` | `v1.34.0` | API, attributes, status codes, global tracer provider |
| `go.opentelemetry.io/otel/sdk` | `v1.34.0` | resource, tracer provider, batch span processor |
| `go.opentelemetry.io/otel/trace` | `v1.34.0` | trace/span context propagation |
| `go.opentelemetry.io/otel/exporters/stdout/stdouttrace` | `v1.34.0` | development stdout trace exporter |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` | not used | avoided to keep the repository free of prior `genproto` ambiguous import risk |
| `google.golang.org/genproto/googleapis/rpc` | not needed directly | `go mod why` reports the main module does not need this package |

## Verification Commands

```text
go mod why go.opentelemetry.io/otel
go mod why go.opentelemetry.io/otel/sdk
go mod why go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
go mod why google.golang.org/genproto/googleapis/rpc
```

## Result

The P5-V exporter emits a standard OTLP/HTTP JSON TraceService export request through the existing internal `sdktrace.SpanExporter`. This keeps the buildable P5.2 dependency state and avoids introducing a second tracing SDK or a conflicting `genproto` version combination.
