# P9 Task Batch 1 Scope

Status: **Batch 1 Scope Ready**

```text
schemaVersion=1
phase=P9
batch=1
batchId=P9-TASK-BATCH-1
ownerScopeDecisionId=P9-OWNER-SCOPE-DECISION-20260728T0440Z
canonicalScopeResolved=true
scopeConfidence=high
scopeReady=true
implementationStarted=false
productCodeChanged=false
p10BoundaryPreserved=true
productionReady=false
```

## Canonical Batch

Inventory Sync, SKU Binding Calibration and Manual Fallback.

## Canonical Product Scope

Domain and persistence foundation only.

## Included Work

- domain models
- persistence migrations
- repositories
- constraints
- persistence tests

## Explicitly Excluded Work

- calibration service
- inventory sync orchestrator
- worker
- HTTP API
- Admin UI
- real provider access
- real OAuth
- real credentials
- real inventory reads
- real inventory writes

## Task IDs

| Task ID | Task Name | Status | Notes |
| --- | --- | --- | --- |
| `P9-501` | Inventory Sync Run Model | planned | sync run record and checkpoint model |
| `P9-502` | Inventory Snapshot Model | planned | snapshot item model and immutable history |
| `P9-503` | SKU Binding Model | planned | binding record and revision model |
| `P9-504` | SKU Binding Calibration Model | planned | calibration record and score breakdown |
| `P9-505` | Manual Binding Fallback Model | planned | manual request and audit trail model |
| `P9-506` | Migration, Repository and Persistence Verification | planned | forward migrations, repositories, race tests |

## Required Evidence

- `docs/P9_EXECUTION_PLAN.md`
- `docs/p9-execution-plan.json`
- `docs/P9_TASK_BATCH_1_SCOPE_GATE.md`
- `docs/p9-task-batch-1-scope-gate.json`

## Safety Boundary

```text
realCredentialsEnabled=false
realPlatformNetworkEnabled=false
realPlatformReadEnabled=false
realPlatformWriteEnabled=false
automaticPublishEnabled=false
automaticListingEnabled=false
productionReady=false
```

This artifact only authorizes the Batch 1 domain and persistence scope. It does not authorize calibration service, sync orchestration, API, Admin UI, or production work.
