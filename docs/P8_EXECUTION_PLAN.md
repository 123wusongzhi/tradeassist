# P8 Execution Plan

Status: **approved**

```text
phase=P8
planVersion=1
baseCheckpoint=ef537bf1d19c670f831e71a8c3c6fa7cbf1bc8ca
canonicalScopeDecisionId=P8-OWNER-SCOPE-DECISION-20260720
productionReady=false
```

## Goal

Build the Operations Task Center, Draft Orchestration and Human Review Loop MVP without production credentials, real platform writes, automatic publish, or automatic listing.

## Workstreams

| Workstream | Scope | Task Count |
| --- | --- | ---: |
| WS-01 | Scope, plan and gate | 4 |
| WS-02 | Domain model and database | 6 |
| WS-03 | State machine and services | 6 |
| WS-04 | Platform draft adapters | 5 |
| WS-05 | Permission and audit | 4 |
| WS-06 | API | 5 |
| WS-07 | Admin UI | 6 |
| WS-08 | Integration and closure | 5 |

Total tasks: **41**.

## Dependency Order

```text
WS-01
-> WS-02
-> WS-03
-> WS-04 + WS-05
-> WS-06
-> WS-07
-> WS-08
```

WS-04 and WS-05 may run in parallel only after the WS-02 model work and the core WS-03 state-machine service work are complete.

## First Batch

After the plan checkpoint is created and the worktree is clean, the first implementation batch is limited to:

```text
P8-101 Operation Task Model
P8-102 Platform Draft Model
P8-106 Migrations and Repository Tests
```

The first batch must not implement real adapters, real platform writes, automatic publish, Admin approval execution, or production release behavior.

## Batch 1 Status

```text
P8-101=completed
P8-102=completed
P8-106=completed
batchEvidence=docs/P8_TASK_BATCH_1_DOMAIN_PERSISTENCE_AND_REPOSITORY.md
batchEvidenceJson=docs/p8-task-batch-1-domain-persistence-and-repository.json
phaseStatus=In Progress
productionReady=false
```

Plan deviation recorded for Batch 1:

```text
planDeviation=P8-106 dependency list includes P8-103/P8-104/P8-105, but Batch 1 scope explicitly forbids implementing those later tasks.
decisionRequired=false_for_batch_1
```

## Batch 2 Status

```text
P8-103=completed
P8-104=completed
P8-105=completed
batchEvidence=docs/P8_TASK_BATCH_2_APPROVAL_EXECUTION_AUDIT_PERSISTENCE.md
batchEvidenceJson=docs/p8-task-batch-2-approval-execution-audit-persistence.json
workingBranch=dev
implementationCommitted=true
checkpointStatus=created
phaseStatus=In Progress
productionReady=false
```

Plan deviation recorded for Batch 2:

```text
planDeviation=batch_2_uses_existing_sqlite_gorm_integration_test_convention
decisionRequired=false_for_batch_2
```

## Batch 3 Status

```text
P8-201=completed
P8-202=completed
P8-203=completed
batchEvidence=docs/P8_TASK_BATCH_3_STATE_DRAFT_APPROVAL_SERVICES.md
batchEvidenceJson=docs/p8-task-batch-3-state-draft-approval-services.json
workingBranch=dev
implementationCommitted=false
checkpointStatus=not_created_by_owner_instruction
phaseStatus=In Progress
productionReady=false
```

Batch 3 adds only state-machine, draft-version, and approval domain services. It does not start execution orchestration, retry service, API, Admin UI, platform adapters, real platform writes, automatic publish, or automatic listing.

## Batch 4 Status

```text
P8-204=completed
P8-205=completed
P8-206=completed
batchEvidence=docs/P8_TASK_BATCH_4_EXECUTION_RETRY_IDEMPOTENCY_SERVICES.md
batchEvidenceJson=docs/p8-task-batch-4-execution-retry-idempotency-services.json
workingBranch=dev
implementationCommitted=false
checkpointStatus=not_created_by_owner_instruction
phaseStatus=In Progress
productionReady=false
```

Batch 4 adds only execution orchestration, failure classification, manual retry, and execution idempotency domain services. It does not start API, Admin UI, platform adapter productization, real platform writes, automatic publish, automatic listing, or background automatic retry.

## Required Planning Gates

```bash
pnpm p7:conditional-close-gate
pnpm p8:entry-gate
pnpm test:p8-plan
pnpm p8:plan-gate
```

## Production Boundary

```text
realCredentialsEnabled=false
realPlatformWriteEnabled=false
automaticPublishEnabled=false
automaticListingEnabled=false
humanConfirmationRequired=true
p7DeferredPerformancePreserved=true
p10ProductionBoundaryPreserved=true
productionReady=false
```
