# P8 Task Batch 4 Execution, Retry and Idempotency Services

Status: **completed**

```text
batchId=P8-TASK-BATCH-4
baseBranch=dev
baseCheckpoint=ce497e1a55f4540abff3717e58138768232ab5b7
phase=P8
phaseStatus=In Progress
productionReady=false
changesCommitted=false
checkpointCreated=false
```

## Scope

Completed tasks:

- `P8-204` Execution Orchestrator
- `P8-205` Retry and Failure Service
- `P8-206` Idempotency Protection

Not implemented in this batch:

- Platform draft adapter productization
- P8 API
- Admin UI
- Queue worker or background automatic retry
- Real platform adapter, real platform write, automatic publish, or automatic listing

## Domain Services

Implemented in `backend/internal/modules/operationtask`:

- `ExecutionOrchestrator`
- `DraftExecutionPort`
- `ExecutionFailureClassifier`
- `ManualRetryService`
- Service-level execution idempotency coordination

The execution port is a domain interface used by tests and future safe adapters. Batch 4 does not implement a production platform client.

## Execution Flow

Execution is split into three stages:

1. Prepare in a short database transaction.
2. Call `DraftExecutionPort.ExecuteDraft` outside the database transaction.
3. Finalize success or failure in a new short database transaction.

Prepare locks the task, verifies approved state, latest draft, latest approved record, draft version and payload hash binding, execution authorizer, safe execution mode, active attempt conflicts, and idempotency state. Success finalize writes the result, moves the task to `draft_written`, and appends `draft_written`. Failure finalize writes `ExecutionError`, moves the task to `execution_failed`, and appends `execution_failed`.

## Failure and Retry

`ExecutionFailureClassifier` supports:

```text
validation_error
permission_denied
state_conflict
adapter_unavailable
provider_timeout
provider_rejected
idempotency_conflict
internal_error
```

Only explicit manual retry is implemented. `ManualRetryService` requires `execution_failed`, latest failed attempt, latest retryable execution error, execution authorizer pass through the orchestrator, current latest draft approval binding, and a finite retry limit.

```text
maxManualRetryAttempts=3
manualRetryOnly=true
automaticRetryImplemented=false
```

Full secret redaction remains deferred:

```text
fullSecretRedactionDeferredTo=P8-404
executionErrorSafeRecordingPresent=true
```

## Idempotency

Execution idempotency uses tenant, task, operation, request/idempotency key, and draft version/hash binding through `ExecutionAttempt`.

Duplicate behavior:

| Existing attempt | Behavior |
| --- | --- |
| queued/running | Return `in_progress`; do not call the port again |
| succeeded | Return original safe result; do not call the port again |
| failed retryable | Preserve failure; retry only through explicit manual retry |
| failed final | Return original failure; do not execute again |

If the same idempotency key is reused with a changed draft version or payload hash, the service returns `idempotency_payload_conflict`.

## Validation Evidence

```text
executionOrchestratorPresent=true
executionFailureClassifierPresent=true
manualRetryServicePresent=true
idempotencyProtectionPresent=true
executionAuthorizerRequired=true
executionDefaultAllow=false
approvalLatestDraftBindingEnforced=true
approvalDraftVersionBindingEnforced=true
approvalDraftHashBindingEnforced=true
executionPortCalledOutsideTransaction=true
executionPrepareAtomic=true
executionSuccessFinalizeAtomic=true
executionFailureFinalizeAtomic=true
duplicateExecutionPrevented=true
concurrentExecutionPrevented=true
idempotencyPayloadConflictDetected=true
manualRetryOnly=true
automaticRetryWorkerPresent=false
retryLimitPresent=true
nonRetryableErrorsBlocked=true
transactionTestsPassed=true
rollbackTestsPassed=true
idempotencyTestsPassed=true
concurrencyTestsPassed=true
racePassed=true
dataRaces=0
```

Covered Go tests:

- Approved task execution success and status/event consistency.
- Pending review, missing authorizer, and forbidden execution mode rejection without port calls.
- Port call observed after prepare transaction commits.
- Failure classification and safe execution error recording.
- Manual retry creates a new attempt and preserves the old attempt/error.
- Non-retryable failures and missing retry limit are blocked.
- Duplicate idempotency returns in-progress or original success without another port call.
- Same idempotency key with changed payload returns conflict.
- Classifier retryability matrix and safe message/details filtering.

## Gate Compatibility

```text
batch1GateBackwardCompatible=true
batch1GateWeakened=false
batch2GateBackwardCompatible=true
batch2GateWeakened=false
batch3GateBackwardCompatible=true
batch3GateWeakened=false
```

Batch 1/2/3 gates remain separate and are not weakened by Batch 4.

## Source Status

```text
currentBranch=dev
headDetached=false
baseCheckpoint=ce497e1a55f4540abff3717e58138768232ab5b7
changesCommitted=false
implementationCommitted=false
checkpointCreated=false
p8TaskBatch4Checkpoint=notCreated
batch4StartBackupDirectory=D:\project-backups\trademind-p8-before-batch4-20260721015515
batch4StartPatchSha256=E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855
```

## Boundary

```text
apiImplemented=false
adminUiImplemented=false
productionPlatformAdapterImplemented=false
realPlatformWriteImplemented=false
automaticRetryImplemented=false
automaticPublishImplemented=false
automaticListingImplemented=false

realCredentialsEnabled=false
realPlatformWriteEnabled=false
automaticPublishEnabled=false
automaticListingEnabled=false
humanConfirmationRequired=true
productionReady=false
```

P8 remains **In Progress**. Batch 5 / platform draft boundary work is not started. P7 deferred performance and P10 production boundary remain preserved.
