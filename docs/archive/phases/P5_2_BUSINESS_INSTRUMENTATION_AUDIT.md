# P5.2 Business Instrumentation Audit

Status: code-level audit passed, real environment telemetry verification deferred.

| Module | Real entry | Metrics | Trace / log boundary | Test | Status |
| --- | --- | --- | --- | --- | --- |
| httpclient | `Client.DoWithRetry` | `provider_requests_total`, duration, retry, timeout, rate limit | no URL/header/token labels | `backend/internal/pkg/httpclient/observability_test.go` | verified |
| webhook | `Service.Ingest`, `processEventRow` | `webhook_requests_total`, persisted, processed, lag, duplicate | event ID/shop ID not labels | `backend/internal/modules/webhook/observability_test.go` | verified |
| ordersync | `Service.ProcessQueuedTask` | runs, received, created, updated, failures | order ID/content not labels | `backend/internal/modules/ordersync/observability_test.go` | verified |
| inventory | `AdjustSKUStock`, `ProcessQueuedTask` | adjust, push, failure, unknown result | SKU ID/stock value not labels | `backend/internal/modules/inventory/observability_test.go` | verified |
| ai-text | `CreateBatch`, `runOneItem`, apply paths | request, timeout, failure, batch, apply | prompt/response not telemetry | `backend/internal/modules/aiproducttext/observability_test.go` | verified |
| ai-image | `CreateBatch`, `runOneItem`, `failItem` | request, provider timeout, failure, batch | prompt/image URL/object key not telemetry | `backend/internal/modules/aiproductimage/observability_test.go` | verified |
| file-scan | `EnqueueSecurityScan`, `processScanPayload` | enqueue, claim, queue age, result | filename/object key not labels | `backend/internal/modules/files/observability_test.go` | verified |
| security | `VerifyAuditIntegrity`, `ObserveSecurity` facade | security events, tenant denied, audit mismatch | tenant/user/IP not labels | `backend/internal/modules/securitymod/observability_test.go` | verified |
| auth | `Login`, `CreateSession`, `RotateRefresh`, revoke paths | login, refresh, refresh reuse, sessions | credential/token/session ID not labels | `backend/internal/modules/auth/observability_test.go` | verified |

`registered` is not treated as `instrumented`; each row above has a business/service/worker call path plus a non-zero metric test.
