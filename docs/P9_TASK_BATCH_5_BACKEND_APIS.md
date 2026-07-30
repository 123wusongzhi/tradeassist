# P9 Task Batch 5 — Backend APIs

Status: **Completed locally; PostgreSQL deferred to P9 final development closure**

Batch 5 delivers the protected backend API surface for P9 inventory synchronization. It is fixture/mock-backed only and does not add Admin UI, real Douyin credentials, OAuth, network access, inventory mutation, workers, cron, ticker, queue consumers, or automatic retry.

## Delivered surface

Base path: `/api/v1/inventory-sync`

- Run APIs: create, keyset list, detail, safe statistics/error summary, and guarded manual rerun.
- Snapshot APIs: run-scoped list/detail, binding-result filtering, immutable observed quantities, and signed keyset cursors.
- Binding/calibration APIs: safe binding/candidate projections, calibration history, binding history, and controlled recalibration that creates a new immutable calibration version.
- Manual binding APIs: pending/status list, detail with immutable decisions, revision-checked confirm/reject.
- History/audit APIs: run history projection and tenant-scoped audit timeline with allowlisted metadata.

All write routes require an `Idempotency-Key`; request IDs come from the trusted request context. JSON bodies use the shared strict binder (`application/json`, bounded body, unknown-field rejection, one JSON value). DTOs are explicit allowlists and never serialize credentials, raw cursors, idempotency hashes, internal checkpoints, or ORM models.

## Security and consistency

- Tenant and actor identity are loaded from the existing authenticated context.
- Existing P9 RBAC permissions and domain services are reused; denied writes are rejected before provider resolution or persistence.
- Signed keyset pagination is used for every list endpoint; offset/page pagination is not exposed.
- Cross-tenant resources resolve as not found.
- Recalibration is idempotent and versioned; prior calibration rows remain immutable.
- No `PATCH` or `DELETE` routes are registered for immutable snapshots, calibrations, or audit records.

## Verification

The uncached full backend suite, targeted race tests for inventory sync and Admin permissions, backend builds, targeted `go vet`, API contracts, architecture boundaries, sensitive-diff scan, environment check, strict UI-copy check, Batch 1–5 gates, and B5-01 through B5-21 fixtures passed. The architecture boundary ratchet reports zero new violations.

Two repository-level orchestrators remain blocked by pre-existing baseline/tooling issues: `pnpm quality:backend` stops at 454 historical Go files reported by `gofmt -l` (all Batch 5 Go files are formatted), and `pnpm architecture:affected` stops when Vitest imports the existing `.mjs` architecture helper with `SyntaxError: Invalid or unexpected token` (the independent architecture boundary check passes).

`go test ./internal/modules/inventorysyncp9`, `go test ./internal/modules/operationtask`, `go test ./internal/pkg/httpapi`, and the backend module suite passed. Batch 1–4 fixtures and gates passed. `TEST_DATABASE_URL` is unset, so PostgreSQL integration is `not_run` and remains a final-closure blocker.

Git state is intentionally uncommitted and unstaged on `dev`; no commit, push, tag, or branch creation was performed.

See also: [`p9-task-batch-5-backend-apis.json`](p9-task-batch-5-backend-apis.json), [`P9_TASK_BATCH_5_BACKEND_APIS_GATE.md`](P9_TASK_BATCH_5_BACKEND_APIS_GATE.md).

## PostgreSQL Revalidation

The original Batch evidence remains unchanged: PostgreSQL was not run because `TEST_DATABASE_URL` was unset. The Batch was revalidated later against an isolated PostgreSQL test database.

```text
initialPostgresVerification.status=not_run
initialPostgresVerification.reason=TEST_DATABASE_URL_not_set
postgresRevalidation.status=passed
postgresRevalidation.evidencePath=docs/P9_POSTGRESQL_INTEGRATION_CLOSURE.md
postgresRevalidation.gateReportPath=docs/P9_POSTGRESQL_INTEGRATION_CLOSURE_GATE.md
postgresRevalidation.verifiedAt=2026-07-30T07:47:32.008Z
postgresRevalidation.runId=p9pg-20260730074632-3b1bbb38
postgresRevalidation.runtimeSummarySha256=bf2427b1a6e2961b19298afbf1d77784ef12333cbe465a410d96a441d77f23f6
currentPostgresIntegrationStatus=passed
```
