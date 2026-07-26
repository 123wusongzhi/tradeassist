# P5.2 HTTP Client Observability

`backend/internal/pkg/httpclient.Client.DoWithRetry` records provider logical requests through the shared `metrics.Catalog`.

- Logical request success/failure increments `provider_requests_total` once.
- Physical retries increment `provider_request_retries_total`.
- Timeout, rate limit and circuit-open outcomes use controlled result/error classes.
- Raw URL, query, headers, Authorization, Cookie and Signature values are not labels.

Verification: `go test ./internal/pkg/httpclient`.
