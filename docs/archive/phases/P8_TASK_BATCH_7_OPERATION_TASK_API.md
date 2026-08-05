# P8 Task Batch 7 Operation Task API

Production Ready: false

Batch 7 completes the backend operation-task API loop for P8 Batch 1-6 domain capabilities. It does not complete Admin UI, P8 as a whole, or Production Ready.

## Completed tasks

- P8-501 Task CRUD / Query API: task create, list, detail, and cancel workflow endpoints.
- P8-502 Draft Edit API: draft creation, latest draft edit, and draft history endpoints.
- P8-503 Approve / Reject API: reviewer-controlled approval decision endpoints.
- P8-504 Execute / Retry API: safe execution and manual retry endpoints.
- P8-505 Attempts / Events API: safe attempt and audit event query endpoints.

## Architecture

The implementation reuses the existing Gin router, `/api/v1` authenticated group, `middleware.BearerAuthWithDB`, tenant context, admin principal resolution, operationtask services, and admin RBAC matrix. No second API framework or duplicated RBAC system is introduced.

Handlers only bind, validate, call the API service, and respond. Write paths go through `APIService`; handlers do not write through repositories directly.

## Authorization

Tenant and actor are derived from authenticated backend context. Bodies cannot override tenant, actor, reviewer role, admin flags, approval flags, or execution flags. RBAC uses existing operationtask permissions:

- `operationtask.audit.read` for read/history APIs
- `operationtask.execute` for create, draft edit, cancel, and execute APIs
- `operationtask.review` for approve/reject APIs
- `operationtask.retry` for manual retry APIs

## Safety controls

- Strict JSON binding rejects unknown write fields.
- `Idempotency-Key` is required for high-risk writes.
- Raw idempotency keys are not returned or persisted in audit metadata.
- Request ID is taken from trusted request context/header.
- Payloads are canonicalized and hashed.
- Sensitive metadata is redacted before response/persistence paths.
- Execute/retry responses do not expose published/listed semantics.
- Keyset or sequence pagination is used instead of offset pagination.

## Explicit non-production boundary

- `productionReady=false`
- `realCredentialsEnabled=false`
- `realPlatformWriteEnabled=false`
- `automaticPublishEnabled=false`
- `automaticListingEnabled=false`
- `humanConfirmationRequired=true`

Batch 7 does not enable real Douyin, OAuth, credentials, platform draft writes, automatic publish, automatic listing, background automatic retry, Admin UI, production gray, tag, release, or Production Ready. Final Production Acceptance remains deferred to P10.

## Verification artifacts

- API docs: `docs/P8_OPERATION_TASK_API.md`
- Evidence JSON: `docs/p8-task-batch-7-operation-task-api.json`
- Final gate: `scripts/p8-task-batch-7-final-gate.mjs`
- Fixture gate: `tests/gates/p8/task-batch-7.mjs`
- Final gate report: `docs/p8-task-batch-7-final-gate.json`
- Fixture report: `docs/p8-task-batch-7-fixture-report.json`
