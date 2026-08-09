# P8 Task Batch 8 Admin Operation Task Center

Production Ready: false

Batch 8 completes the Admin UI surface for the P8 operation-task loop by consuming the Batch 7 `/api/v1/operation-tasks` API. It does not complete P8 as a whole, does not enable real platform writes, and does not authorize production release behavior.

## Completed tasks

- P8-601 Task List: Admin list page with status/platform/taskType filters, backend keyset cursor pagination, safe empty/error/loading states, and no fabricated total count.
- P8-602 Task Detail: detail page that loads task summary, drafts, attempts, and audit events from backend APIs and ignores stale refresh races.
- P8-603 Draft Preview / Edit: safe JSON editor, JSON validation, frontend-only diff against the visible payload, and draft version metadata display.
- P8-604 Approval Actions: approve/reject dialogs bound to backend-returned latest draft version/hash and task revision.
- P8-605 Execution / Retry Actions: execute, manual retry, and cancel dialogs with safe adapter modes only and one idempotency key per explicit user action.
- P8-606 Audit Timeline: ordered event timeline with safe metadata allowlist and request ID display for support.

## Architecture

The implementation reuses the existing Umi Max Admin stack, Ant Design components, `TmPageContainer`, `TmProTable`, shared UI primitives, URL state helpers, menu access wiring, and the existing API request envelope. A dedicated `admin/src/services/operationTasks.ts` contract layer aligns to the Batch 7 backend DTOs and uses the existing request helpers.

Frontend components do not call repositories, do not duplicate backend operation-task state-machine logic, and do not bypass the service layer with raw `fetch` or `axios` calls.

## Authorization and actions

Menu visibility is wired to `operationtask.audit.read`. Operation buttons are enabled only from backend-returned `allowedActions`:

- `canEditDraft`
- `canApprove`
- `canReject`
- `canExecute`
- `canRetry`
- `canCancel`

The frontend does not make final authorization decisions. Backend RBAC, task state, revision checks, draft version checks, payload hash checks, and idempotency remain the final security boundary.

## Safety controls

- All write requests send `Idempotency-Key` through the operation-task service layer.
- Dangerous dialogs use loading/disabled state during requests to reduce duplicate submissions.
- Conflict and mismatch errors trigger a refresh instead of resubmitting stale payloads.
- List and timeline payloads avoid raw full payload/error/metadata display.
- Sensitive-looking keys are redacted before JSON preview/diff display.
- Audit metadata uses an explicit frontend allowlist.
- Payloads are rendered through JSON components, not as HTML.
- Historical draft payload diff is not fabricated because the current backend draft summary DTO does not return full historical draft payloads.

## Explicit non-production boundary

- `productionReady=false`
- `realCredentialsEnabled=false`
- `realPlatformWriteEnabled=false`
- `automaticPublishEnabled=false`
- `automaticListingEnabled=false`
- `humanConfirmationRequired=true`

Batch 8 does not implement real Douyin API, OAuth, real credentials, real platform draft writes, automatic publish, automatic listing, background automatic retry, scheduled execution, production queue workers, production gray rollout, tag, release, or Production Ready. Final Production Acceptance remains deferred to P10.

## Verification artifacts

- Evidence JSON: `docs/p8-task-batch-8-admin-operation-task-center.json`
- Final gate: `scripts/p8-task-batch-8-final-gate.mjs`
- Fixture gate: `tests/gates/p8/task-batch-8.mjs`
- Final gate report: `docs/p8-task-batch-8-final-gate.json`
- Fixture report: `docs/p8-task-batch-8-fixture-report.json`
