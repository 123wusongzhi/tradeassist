# P8 Task Batch 1 Domain Persistence and Repository

Status: **completed**

```text
batchId=P8-TASK-BATCH-1
baseCheckpoint=ea356d8077722e2f94c6215fe10c7d4f6e53fde5
phase=P8
phaseStatus=In Progress
productionReady=false
```

## Scope

Completed tasks:

- `P8-101` Operation Task Model
- `P8-102` Platform Draft Model
- `P8-106` Migrations and Repository Tests

Not implemented in this batch:

- Approval records or approval service
- Execution attempts, execution errors, execution service, or retry orchestration
- Operation task event or audit model
- P8 API
- Admin UI
- Real platform writes, automatic publish, or automatic listing

## Conventions

| Item | Decision |
| --- | --- |
| Domain model convention | GORM module models under `backend/internal/modules/operationtask` |
| Repository convention | Module-local GORM repositories with stable domain errors |
| Migration convention | `AutoMigrate` plus P8 hardening in `operationtask.Migrate` |
| ID convention | Shared UUID v4 model base (`model.HardDeleteBase`) |
| Tenant isolation convention | Every repository read/write binds `tenant_id`; draft creation verifies task tenant |
| Timestamp convention | GORM `CreatedAt` / `UpdatedAt` UTC timestamps |
| JSON convention | `gorm.io/datatypes.JSON` with PostgreSQL `jsonb` type tags |
| Error convention | Stable sentinel errors: `not_found`, `validation_error`, `tenant_mismatch`, `revision_conflict`, `duplicate_idempotency_key`, `duplicate_draft_version` |
| Test database convention | Real GORM database integration tests using SQLite in-memory for local deterministic execution; no SQL mock layer |

## Schema Evidence

### `operation_tasks`

Columns:

- `id` UUID primary key
- `tenant_id` not null
- `source_type` not null
- `source_reference`
- `task_type` not null
- `platform` not null
- `title` not null
- `summary`
- `payload` jsonb not null
- `status` not null
- `priority` not null
- `idempotency_key`
- `revision` not null, default `1`, check `revision >= 1`
- `created_by`
- `updated_by`
- `created_at` not null
- `updated_at` not null

Constraints and indexes:

- Primary key: `id`
- Partial unique index: `tenant_id + idempotency_key` where idempotency key is non-empty
- Index: `tenant_id + status + updated_at`
- Index: `tenant_id + platform + status + updated_at`
- Index: `tenant_id + task_type + created_at`
- Index: `tenant_id + source_type + source_reference`

Index rationale:

- `tenant_id + status + updated_at`: task queue/review lists by status
- `tenant_id + platform + status + updated_at`: platform-scoped review and draft worklists
- `tenant_id + task_type + created_at`: task-type history and diagnostics
- `tenant_id + source_type + source_reference`: source reconciliation and duplicate-source lookup

### `platform_drafts`

Columns:

- `id` UUID primary key
- `tenant_id` not null
- `operation_task_id` not null
- `platform` not null
- `adapter_mode` not null
- `draft_version` not null, default `1`, check `draft_version >= 1`
- `payload` jsonb not null
- `payload_hash` not null
- `status` not null
- `change_reason`
- `created_by`
- `updated_by`
- `created_at` not null
- `updated_at` not null

Constraints and indexes:

- Primary key: `id`
- Foreign key: `operation_task_id -> operation_tasks.id`, `ON DELETE RESTRICT`
- Unique index: `tenant_id + operation_task_id + draft_version`
- Check: `adapter_mode IN ('mock','sandbox','local_draft_only')`
- Check: SHA-256 lowercase hex `payload_hash` format on PostgreSQL
- Index: `tenant_id + operation_task_id + draft_version DESC`
- Index: `tenant_id + status + updated_at`
- Index: `tenant_id + platform + status`

Index rationale:

- `tenant_id + operation_task_id + draft_version DESC`: latest/versioned draft lookups
- `tenant_id + status + updated_at`: review worklists by draft status
- `tenant_id + platform + status`: platform-scoped draft lists

## Repository Methods

Operation Task:

- `Create`
- `GetByID`
- `GetByIdempotencyKey`
- `List`
- `UpdateRevision`

Platform Draft:

- `CreateVersion`
- `GetByID`
- `GetVersion`
- `GetLatest`
- `ListVersions`

## Validation Evidence

```text
tenantIsolationPassed=true
idempotencyTestsPassed=true
revisionTestsPassed=true
draftVersionTestsPassed=true
concurrencyTestsPassed=true
currentBatchRaceStatus=passed
migrationTestsPassed=true
repositoryTestsPassed=true
```

Covered tests:

- Operation Task create/read by tenant and ID
- Tenant isolation on task reads
- Tenant-scoped idempotency uniqueness
- Multiple empty idempotency keys
- Revision update and revision conflict
- List filters and cursor pagination
- JSON payload validation and secret-key rejection
- Platform Draft create version
- Missing task and tenant mismatch
- Duplicate draft version
- Version increment/list/latest
- Payload hash validation
- Adapter mode rejection for `production`, `real_write`, `auto_publish`
- Foreign-key delete restriction
- Concurrent idempotency conflict
- Concurrent draft version conflict

## Plan Deviation

The approved JSON plan lists `P8-106` dependencies on `P8-103`, `P8-104`, and `P8-105`, while the same plan and owner batch instructions limit Batch 1 to `P8-101`, `P8-102`, and `P8-106` and explicitly forbid implementing `P8-103` through `P8-105`.

Decision for this batch:

```text
planDeviation=P8-106 dependency list includes later WS-02 tasks not allowed in Batch 1
decisionRequired=false_for_batch_1
batchInterpretation=P8-106 covers migrations and repository tests for P8-101 and P8-102 only
```

## Boundary

```text
businessServiceImplemented=false
approvalServiceImplemented=false
executionServiceImplemented=false
apiImplemented=false
adminUiImplemented=false
platformWriteImplemented=false

realCredentialsEnabled=false
realPlatformWriteEnabled=false
automaticPublishEnabled=false
automaticListingEnabled=false
humanConfirmationRequired=true
productionReady=false
```

Draft status decision:

```text
draftStatusDecisionSource=batch fallback minimal set because P8 execution plan does not define independent PlatformDraft status values
```

P8 remains **In Progress**. P7 deferred performance and P10 production boundary remain preserved.
