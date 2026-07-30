# P5.2 Span Export Protocol Report

protocol: custom HTTP JSON span exporter
endpoint path: `/v1/traces`
content_type: `application/json`
payload encoding: JSON
compression: none
resource attributes: service name/version/environment are configured on the SDK resource
scope attributes: SDK tracer scope
trace_id encoding: lowercase hex string
span_id encoding: lowercase hex string
parent_span_id encoding: lowercase hex string
status encoding: string status code
event encoding: not exported by the custom payload
retry policy: SDK batch processor only; exporter errors are fail-safe
queue limit: SDK batch queue size 1024
shutdown timeout: configured by OTel SDK shutdown context
collector compatibility: custom mock collector compatible, standard OTLP collector compatibility pending

Status: Custom HTTP Span Exporter Ready. Distributed Tracing Export Compatibility Pending.

This exporter must not be represented as standard OTLP/HTTP until it emits the official TraceService Export request format.

## P5-V Update

P5-V replaces this P5.2 custom payload with a standard OTLP/HTTP JSON TraceService export request. See `docs/P5_V_OTLP_PROTOCOL_IMPLEMENTATION.md` and `docs/P5_V_OTLP_PROTOCOL_TEST_REPORT.md` for the current protocol implementation and Mock Collector parsing tests.
