# P5-V Final Verification Audit

Phase: P5-V
Status: in progress

## Scope

P5-V is a verification and closure gate. It replaces the P5.2 custom HTTP span JSON payload with a standard OTLP/HTTP JSON TraceService export payload, keeps real telemetry backend verification deferred, and does not expand business observability scope.

## Audit Matrix

| Check | Current implementation | Verification | Closure status |
| --- | --- | --- | --- |
| HTTP endpoint | `POST /v1/traces` via `normalizeEndpoint` | `TestHTTPExporterSendsStandardOTLPToMockCollector` | passed |
| Content-Type | `application/json` | Mock Collector header assertion | passed |
| Request shape | `resourceSpans.resource.scopeSpans.scope.spans` | JSON decoder with `DisallowUnknownFields` | passed |
| Trace ID | 16-byte hex string | test length assertion | passed |
| Span ID | 8-byte hex string | test length assertion | passed |
| Parent span | `parentSpanId` on child span | parent/child assertion | passed |
| Resource | `service.name`, version, environment attributes | resource attribute assertion | passed |
| Scope | instrumentation scope name/version | scope assertion | passed |
| Status | Go SDK status mapped to OTLP status codes | error span assertion | passed |
| Attributes | OTLP `KeyValue` / `AnyValue` | typed attribute assertion | passed |
| Sensitive values | token, cookie, secret, PII-like keys dropped | payload string scan | passed |
| Retry | 429/5xx retry with bounded attempts | `TestHTTPExporterRetriesRetryableStatus` | passed |
| 4xx classification | non-retryable client status | `TestHTTPExporterDoesNotRetryClientStatus` | passed |
| Queue | OTel SDK batch queue with bounded size | config + code scan | passed |
| Shutdown | OTel SDK shutdown flush with caller timeout | tracing shutdown tests | passed |

## Remaining Verification Gates

- Linux race matrix: pending until WSL2/Linux CI execution.
- Frontend and collector checks: pending until `pnpm check:dev`, `pnpm check:ui-copy --strict`, `pnpm build:admin`, and `pnpm build:collector` complete.
- Demo acceptance runs: pending until two complete `pnpm demo:auto-acceptance` runs pass.
- Real telemetry backend, external alert channels, production SLO validation, real Douyin credential E2E, production gray release, tag creation: deferred by phase boundary.
