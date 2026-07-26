# P5.1 OTLP Dependency Resolution

The official OTLP HTTP exporter dependency currently triggers a `google.golang.org/genproto` ambiguous import conflict in this repository. To keep the runtime buildable, P5.1 uses a lightweight internal `sdktrace.SpanExporter` implementation in `backend/internal/pkg/tracing/tracing.go`.

Behavior:

- Posts sanitized span JSON to `OTEL_EXPORTER_OTLP_ENDPOINT` with `/v1/traces` normalization.
- Uses bounded OTel batch processor settings.
- Records export success, export failures, and dropped telemetry through the observability facade.
- Fails safely: exporter errors do not block business requests or server startup.

This is code-level telemetry export verification only. Real collector verification remains deferred.
