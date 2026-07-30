# P5.1 OTLP Export Test Report

Implemented test:

- `backend/internal/pkg/tracing/tracing_test.go`
- `TestHTTPExporterSendsSpanToMockCollector`

Coverage:

- Starts a local mock HTTP collector.
- Creates and ends a sampled span.
- Flushes through tracer shutdown.
- Confirms the mock collector receives an export request.

Deferred:

- Real OTLP collector.
- Production collector credentials, TLS policy, routing, and retention.
