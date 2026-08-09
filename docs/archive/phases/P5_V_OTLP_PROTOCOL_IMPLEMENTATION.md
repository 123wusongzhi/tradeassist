# P5-V OTLP Protocol Implementation

Phase: P5-V
Protocol: OTLP/HTTP JSON
Endpoint: `/v1/traces`
Content-Type: `application/json`

## Implementation

- `backend/internal/pkg/tracing/tracing.go` now builds a TraceService export request with `resourceSpans`, `resource`, `scopeSpans`, `scope`, and `spans`.
- Span fields include `traceId`, `spanId`, optional `parentSpanId`, `name`, `kind`, `startTimeUnixNano`, `endTimeUnixNano`, `attributes`, `events`, and `status`.
- Trace and span IDs are exported as 16-byte and 8-byte hex strings.
- Timestamps are exported as Unix nanoseconds in string form.
- Go SDK `codes.Ok` and `codes.Error` are mapped to OTLP status codes `1` and `2`.
- Attributes are emitted as OTLP `KeyValue` / `AnyValue` JSON, including string, bool, int, double, and array values.
- Sensitive attribute keys are dropped before export. Event names and status messages are also guarded against sensitive terms.

## Failure Safety

- The exporter retries retryable status codes only (`429` and `5xx`) with a bounded retry count.
- Client errors such as `400` are classified as non-retryable.
- Export failure callbacks increment telemetry failure/drop counters through the observability facade.
- Batch queue size, batch size, retry max, and timeout are bounded by config.
- Export failure does not change the original business response path.

## Boundary

Mock Collector verification proves protocol compatibility at code level. It is not a real production telemetry backend. Real OpenTelemetry Collector, Prometheus, Grafana, external alert channel, and production SLO validation remain deferred.
