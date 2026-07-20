# P8 Task Batch 2 Approval, Execution and Audit Persistence

Status: **completed**

```text
batchId=P8-TASK-BATCH-2
baseCheckpoint=73e2ea3ec0b749d607da0e919ad71b29cef73c3d
p8PlanCheckpoint=ea356d8077722e2f94c6215fe10c7d4f6e53fde5
p8TaskBatch2Checkpoint=null
checkpointStatus=not_created_by_owner_instruction
currentBranch=dev
headDetached=false
devBaseCheckpoint=ecce1814a5df89bc066a2ab70cc9d74305857286
changesCommitted=false
commitRequired=false
workingTreeDirty=true
phase=P8
phaseStatus=In Progress
productionReady=false
```

## Scope

Completed tasks:

- `P8-103` Approval Record
- `P8-104` Execution Attempt and Error
- `P8-105` Task Event Audit Model

Not implemented in this batch:

- Task state machine
- Approval service
- Execution orchestrator
- Retry or failure service
- P8 API
- Admin UI
- Real platform writes, automatic publish, or automatic listing

## Persistence

Models:

- `ApprovalRecord`
- `ExecutionAttempt`
- `ExecutionError`
- `OperationTaskEvent`

Tables:

- `approval_records`
- `execution_attempts`
- `execution_errors`
- `operation_task_events`

Repository methods:

- `ApprovalRecord.CreateDecision`
- `ApprovalRecord.GetByID`
- `ApprovalRecord.GetByIdempotencyKey`
- `ApprovalRecord.GetLatestByTask`
- `ApprovalRecord.ListByTask`
- `ExecutionAttempt.CreateAttempt`
- `ExecutionAttempt.GetByID`
- `ExecutionAttempt.GetByIdempotencyKey`
- `ExecutionAttempt.GetByAttemptNumber`
- `ExecutionAttempt.ListByTask`
- `ExecutionAttempt.UpdateLifecycle`
- `ExecutionError.AppendError`
- `ExecutionError.GetByID`
- `ExecutionError.ListByAttempt`
- `ExecutionError.GetLatestByAttempt`
- `OperationTaskEvent.AppendEvent`
- `OperationTaskEvent.GetByID`
- `OperationTaskEvent.GetBySequence`
- `OperationTaskEvent.ListByTask`
- `OperationTaskEvent.GetLatestByTask`

## Constraints and Indexes

Constraints:

- Approval records bind tenant, task, draft, draft version, draft payload hash, reviewer, decision, and idempotency key.
- Execution attempts bind tenant, task, draft, approval record, attempt number, adapter mode, approved draft version/hash, executed draft version/hash, and revision.
- Execution errors bind tenant, execution attempt, sequence, category, retryability, sanitized message, details JSON, and occurrence time.
- Operation task events bind tenant, task, sequence, event type, actor type, optional draft/version, metadata JSON, and occurrence time.
- Approval records, execution errors, and operation task events are append-only through repository API, GORM hooks, and SQLite/PostgreSQL immutable triggers.

Indexes:

- `idx_approval_records_task_created`: task approval timeline by tenant and task.
- `idx_approval_records_draft_created`: draft approval timeline by tenant and draft.
- `idx_approval_records_task_decision_created`: latest approval/rejection lookup by tenant, task, and decision.
- `ux_approval_records_task_idempotency`: tenant/task scoped approval idempotency.
- `idx_execution_attempts_task_attempt`: stable attempt ordering by tenant, task, and attempt number.
- `ux_execution_attempts_task_attempt`: tenant/task attempt number uniqueness.
- `ux_execution_attempts_task_idempotency`: tenant/task scoped execution idempotency.
- `idx_execution_errors_attempt_sequence`: stable error ordering by tenant, attempt, and sequence.
- `ux_execution_errors_attempt_sequence`: tenant/attempt error sequence uniqueness.
- `idx_operation_task_events_task_sequence`: stable event timeline ordering by tenant, task, and sequence.
- `ux_operation_task_events_task_sequence`: tenant/task event sequence uniqueness.

## Validation Evidence

```text
approvalImmutable=true
executionErrorImmutable=true
taskEventImmutable=true

tenantIsolationPassed=true
approvalIdempotencyPassed=true
executionIdempotencyPassed=true

attemptNumberConcurrencyPassed=true
errorSequenceConcurrencyPassed=true
eventSequenceConcurrencyPassed=true

migrationTestsPassed=true
repositoryTestsPassed=true
concurrencyTestsPassed=true
racePassed=true
dataRaces=0
```

Covered tests:

- Approval create, reject, tenant isolation, task/draft mismatch, draft version mismatch, payload hash mismatch, idempotency, cross-tenant idempotency reuse, latest lookup, and immutability.
- Execution attempt create, missing approval, approval/draft mismatch, tenant isolation, attempt-number conflict, idempotency, revision CAS, adapter mode rejection, payload hash validation, and stable list order.
- Execution error append, missing attempt, tenant isolation, sequence conflict, category validation, retryability persistence, details JSON round-trip, sensitive content rejection, stable ordering, and immutability.
- Operation task event append, continuous sequences, duplicate sequence conflict, tenant isolation, event type validation, actor validation, metadata JSON round-trip, timeline ordering, keyset pagination, and immutability.
- Concurrent approval idempotency, attempt number conflict, execution idempotency, error sequence conflict, and task event sequence conflict.

## Plan Deviation

The task instructions prefer real PostgreSQL integration tests for this batch. The already accepted Batch 1 convention records the current project test database convention as real GORM integration tests using SQLite in-memory with no SQL mock layer. This batch follows that committed convention and records the difference here.

```text
planDeviation=batch_2_uses_existing_sqlite_gorm_integration_test_convention
decisionRequired=false_for_batch_2
postgresConstraintHardeningPresent=true
```

## Source Status

```text
workingBranch=dev
implementationCommitted=false
changesCommitted=false
workingTreeDirty=true
batch2BackupDirectory=D:\project-backups\trademind-p8-batch2-20260720205605
batch2TrackedPatchSha256=5D7E8D8F91E0C309F1DB3C90A32BC218DF7F4CC05E46B2FDF8523B38FF269C92
devEnvironmentCheckStatus=passed
dockerCliAvailable=false
postgresReachable=true
redisReachable=true
```

Full secret redaction remains deferred:

```text
fullSecretRedactionDeferredTo=P8-404
```

## Boundary

```text
stateMachineServiceImplemented=false
approvalServiceImplemented=false
executionOrchestratorImplemented=false
retryServiceImplemented=false

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

P8 remains **In Progress**. P7 deferred performance and P10 production boundary remain preserved.
