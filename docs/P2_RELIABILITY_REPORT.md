# P2 Reliability Scan Report

Generated: 2026-07-12T01:41:37.087Z

**Overall:** passed (22 passed, 0 warnings, 0 failed)

| ID | Status | Message |
| --- | --- | --- |
| idempotency-module | passed | idempotency module exists |
| idempotency-unique | passed | idempotency unique constraint |
| p1-storage-failfast | passed | P1 local storage fail-fast in Validate() |
| task-lease-collect/model.go | passed | collect/model.go has lease fields |
| task-lease-ordersync/model.go | passed | ordersync/model.go has lease fields |
| task-lease-inventory/model.go | passed | inventory/model.go has lease fields |
| dead-letter-status | passed | dead_letter status exists |
| retry-classify | passed | retryable error classification |
| retry-after | passed | Retry-After parsing |
| circuit-breaker | passed | circuit breaker implementation |
| cors-middleware | passed | CORS middleware with wildcard guard |
| migration-lock | passed | PostgreSQL migration advisory lock |
| provider-health | passed | Provider HealthCheck registry |
| customer-client-msg-id | passed | customer clientMessageId field |
| inventory-event-key | passed | inventory business event key |
| webhook-idempotency | passed | webhook idempotency base |
| doc-docs/IDEMPOTENCY_DESIGN.md | passed | docs/IDEMPOTENCY_DESIGN.md exists |
| doc-docs/TASK_RELIABILITY_DESIGN.md | passed | docs/TASK_RELIABILITY_DESIGN.md exists |
| doc-docs/PROVIDER_RESILIENCE_DESIGN.md | passed | docs/PROVIDER_RESILIENCE_DESIGN.md exists |
| doc-docs/CORS_PRODUCTION_GUIDE.md | passed | docs/CORS_PRODUCTION_GUIDE.md exists |
| doc-docs/MIGRATION_LOCK_DESIGN.md | passed | docs/MIGRATION_LOCK_DESIGN.md exists |
| log-redact | passed | URL redaction in httpclient |
