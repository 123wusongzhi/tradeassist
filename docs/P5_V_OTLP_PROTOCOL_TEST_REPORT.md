# P5-V OTLP Protocol Test Report

Phase: P5-V
Status: passed at package level

## Tests

| Test | Coverage | Result |
| --- | --- | --- |
| `TestHTTPExporterSendsStandardOTLPToMockCollector` | Mock Collector parses standard OTLP/HTTP JSON request, checks method/path/content type, resource/scope/span structure, parent/child span, status, typed attributes, and sensitive field filtering | passed |
| `TestHTTPExporterRetriesRetryableStatus` | `500` triggers one bounded retry and then succeeds | passed |
| `TestHTTPExporterDoesNotRetryClientStatus` | `400` is non-retryable and records export failure | passed |
| `TestBuildOTLPTraceExportRequestFixtureShape` | generated request contains standard `resourceSpans` / `scopeSpans` shape | passed |
| `TestGoldenOTLPFixtureParses` | `testdata/valid_otlp_trace.json` parses under strict decoder and satisfies field assertions | passed |

## Command

```text
go test ./internal/pkg/tracing/...
```

Result: passed.

## Sensitive Test Values

The test payload checks that values such as `TEST_ACCESS_TOKEN_UNIQUE`, `TEST_REFRESH_TOKEN_UNIQUE`, `TEST_APP_SECRET_UNIQUE`, `TEST_COOKIE_UNIQUE`, `TEST_PHONE_UNIQUE`, `TEST_EMAIL_UNIQUE`, `TEST_SIGNED_URL_UNIQUE`, and `TEST_OBJECT_KEY_UNIQUE` do not appear in the exported OTLP request.
