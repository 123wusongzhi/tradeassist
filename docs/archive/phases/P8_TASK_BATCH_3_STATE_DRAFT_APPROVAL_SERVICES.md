# P8 Task Batch 3 State, Draft and Approval Services

Status: **completed**

```text
batchId=P8-TASK-BATCH-3
baseBranch=dev
baseCheckpoint=2d148f3acd43bf56d485a6fabf655c7ea05a8865
phase=P8
phaseStatus=In Progress
productionReady=false
changesCommitted=false
checkpointCreated=false
```

## Scope

Completed tasks:

- `P8-201` Task State Machine
- `P8-202` Draft Version Service
- `P8-203` Approval Service

Not implemented in this batch:

- Execution orchestrator
- Retry or failure service
- Execution idempotency service
- Platform draft adapter interface
- Local or Douyin platform adapters
- P8 API
- Admin UI
- Queue worker
- Real platform writes, automatic publish, or automatic listing

## Domain Services

Implemented in `backend/internal/modules/operationtask`:

- `TaskStateMachine`
- `TaskTransitionService`
- `DraftVersionService`
- `ApprovalService`
- Canonical JSON payload hash version `1` using SHA-256

The services only modify the local database domain state. They do not call external platform APIs and do not create publish/listing behavior.

## State Machine

The allowed transition matrix is the approved P8 canonical set:

```text
suggested -> draft_preparing | cancelled
draft_preparing -> pending_review | cancelled
pending_review -> approved | rejected | cancelled
approved -> pending_review | execution_queued | cancelled
execution_queued -> executing | cancelled
executing -> draft_written | execution_failed
execution_failed -> execution_queued | cancelled
```

Terminal states:

```text
rejected
draft_written
cancelled
```

`pending_review -> executing` and same-status transitions are rejected with `invalid_transition`.

## Draft Version Service

Draft versions are append-only. The service computes payload hashes from canonical JSON and never trusts a caller-provided hash.

Initial draft creation:

- Validates task tenant and expected revision.
- Requires the task to already be in `draft_preparing`.
- Creates `DraftVersion=1`.
- Updates the task to `pending_review`.
- Appends `draft_generated` and `review_requested` events in the same transaction.

Subsequent edits:

- Read the latest draft version.
- Create `latestVersion+1`.
- Keep prior draft rows unchanged.
- If the task was `approved`, move it back to `pending_review` and append `review_requested`.
- Use tenant/task/idempotency key to avoid duplicate draft versions.

## Approval Service

`ApprovalService` supports only human `Approve` and `Reject`.

Approval preconditions:

- Task belongs to tenant.
- Task status is `pending_review`.
- Target draft is the latest draft.
- Draft version and payload hash match the latest draft.
- Reviewer is authorized through `ApprovalAuthorizer`.
- Missing authorizer defaults to deny.
- Request ID and idempotency key are required.

Approval/rejection writes `ApprovalRecord`, updates task status/revision, and appends an event in one transaction.

RBAC integration remains deferred:

```text
rbacConcreteIntegrationDeferredTo=P8-401
authorizationPortEnforced=true
defaultAllow=false
```

## Validation Evidence

```text
stateTransitionMatrixTested=true
invalidTransitionsRejected=true
canonicalJsonHashVersion=1
draftPayloadHashComputedByService=true
draftVersionsAppendOnly=true
approvedDraftEditRequiresReapproval=true
approvalLatestDraftBindingPresent=true
approvalDraftVersionBindingPresent=true
approvalDraftHashBindingPresent=true
approvalAuthorizerRequired=true
approvalDefaultAllow=false
transactionTestsPassed=true
rollbackTestsPassed=true
idempotencyTestsPassed=true
concurrencyTestsPassed=true
racePassed=true
dataRaces=0
devEnvironmentCheckStatus=passed
dockerCliAvailable=false
dockerCheckRequired=false
dockerCheckMode=optional_local_fallback
dockerCheckResult=skipped_because_docker_cli_unavailable_and_local_postgres_redis_available
```

Covered Go tests:

- Canonical transition matrix and invalid transitions.
- Terminal state, approval requirement, and execution eligibility predicates.
- Canonical JSON hash stability for key order, whitespace, nested values, arrays, booleans, null, strings, and number forms such as `1`, `1.0`, and `1e0`.
- Task transition event atomicity and rollback on event append failure.
- Draft initial creation, next version creation, append-only preservation, idempotency, cross-tenant rejection, and approved-edit reapproval.
- Approval default deny, latest draft/version/hash binding, idempotency, reject reason, and approve/reject concurrency.

## Gate Compatibility

```text
batch1GateBackwardCompatible=true
batch1GateWeakened=false
batch2GateBackwardCompatible=true
batch2GateWeakened=false
```

Batch 1/2 gates remain separate and are not weakened by Batch 3.

## Source Status

```text
currentBranch=dev
headDetached=false
baseCheckpoint=2d148f3acd43bf56d485a6fabf655c7ea05a8865
changesCommitted=false
implementationCommitted=false
checkpointCreated=false
p8TaskBatch3Checkpoint=notCreated
batch3StartBackupDirectory=D:\project-backups\trademind-p8-before-batch3-20260720132036
batch3StartPatchSha256=E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855
devEnvironmentCheckStatus=passed
dockerCliAvailable=false
dockerCheckRequired=false
whyCheckDevPassedWithoutDockerCli=check_dev_contract_allows_local_postgres_redis_when_docker_cli_is_unavailable
```

## Boundary

```text
executionOrchestratorImplemented=false
retryServiceImplemented=false
executionIdempotencyServiceImplemented=false

apiImplemented=false
adminUiImplemented=false
platformAdapterImplemented=false
platformWriteImplemented=false

realCredentialsEnabled=false
realPlatformWriteEnabled=false
automaticPublishEnabled=false
automaticListingEnabled=false
humanConfirmationRequired=true
productionReady=false
```

P8 remains **In Progress**. Batch 4 is not started. P7 deferred performance and P10 production boundary remain preserved.
