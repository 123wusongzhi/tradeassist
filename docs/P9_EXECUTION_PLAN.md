# P9 Execution Plan

Status: **P9 Execution Plan Ready**

```text
phase=P9
planVersion=2
ownerScopeDecisionId=P9-OWNER-SCOPE-DECISION-20260728T0440Z
canonicalPhaseName=Douyin Shop inventory sync MVP with SKU binding calibration and manual binding fallback
planningFoundationPreserved=true
historicalPlanningTaskIdsPreserved=true
productImplementationWorkstreamCount=7
productImplementationBatchCount=7
productImplementationStarted=true
productionReady=false
```

## Goal

Build the Douyin Shop inventory sync MVP with SKU binding calibration and manual binding fallback, while keeping the P10 boundary intact.

## Planning and Governance Foundation

The existing `P9-101` through `P9-402` tasks remain the planning / governance foundation. They are completed and preserved as evidence, not reused as product implementation tasks.

### WS-00 Scope Discovery and Owner Decision

| Task ID | Task Name | Status | Deliverables |
| --- | --- | --- | --- |
| `P9-101` | Capture the live P8 closure baseline and repository HEAD | completed | live HEAD capture, P8 baseline snapshot |
| `P9-102` | Classify all repo P9 references | completed | reference classification table, historical evidence map |

### WS-01 Entry and Boundary Gates

| Task ID | Task Name | Status | Deliverables |
| --- | --- | --- | --- |
| `P9-201` | Lock the canonical Douyin P9 scope | completed | canonical scope decision lock, P10 boundary summary |
| `P9-202` | Record the P10 boundary and explicit non-goals | completed | non-goals list, production boundary summary |

### WS-02 Canonical Plan and Evidence

| Task ID | Task Name | Status | Deliverables |
| --- | --- | --- | --- |
| `P9-301` | Add the P9 entry gate and fixture test | completed | entry gate script, entry gate fixtures |
| `P9-302` | Add the P9 plan gate and fixture test | completed | plan gate script, plan gate fixtures |

### WS-03 Planning Closure

| Task ID | Task Name | Status | Deliverables |
| --- | --- | --- | --- |
| `P9-401` | Publish discovery and plan artifacts in docs | completed | discovery docs, plan docs, gate reports |
| `P9-402` | Refresh the docs index and progress summary | completed | docs index update, progress summary update |

## PostgreSQL Integration Baseline

Status: **Passed**

```text
postgresIntegrationStatus=passed
postgresIntegrationPassed=true
postgresIntegrationDeferredTo=null
p9FinalClosureBlocker=false
postgresIntegrationEvidence=docs/P9_POSTGRESQL_INTEGRATION_CLOSURE.md
postgresIntegrationGateStatus=passed
P9 Product Batch 1–5 Completed
P9 Product Batch 6 Ready to Start
P9 Product Batch 6 Started=false
productionReady=false
```

## Product Implementation Workstreams

### WS-05 Domain and Persistence

| Task ID | Task Name | Batch | Status | Deliverables |
| --- | --- | --- | --- | --- |
| `P9-501` | Inventory Sync Run Model | 1 | completed | InventorySyncRun, status, cursor/checkpoint |
| `P9-502` | Inventory Snapshot Model | 1 | completed | InventorySnapshotItem, payload hash, immutable history |
| `P9-503` | SKU Binding Model | 1 | completed | SKUBinding, binding source, optimistic revision |
| `P9-504` | SKU Binding Calibration Model | 1 | completed | SKUBindingCalibration, score breakdown, reason codes |
| `P9-505` | Manual Binding Fallback Model | 1 | completed | ManualBindingRequest, resolution, audit trail |
| `P9-506` | Migration, Repository and Persistence Verification | 1 | completed | forward migrations, repositories, race tests |

### WS-06 Calibration Services

| Task ID | Task Name | Batch | Status | Deliverables |
| --- | --- | --- | --- | --- |
| `P9-601` | SKU Identifier Normalization | 2 | completed | SKU code normalization, barcode normalization |
| `P9-602` | Exact Identifier Matching | 2 | completed | exact code match, exact barcode match |
| `P9-603` | Candidate Scoring and Explainability | 2 | completed | score breakdown, reason codes, candidate ordering |
| `P9-604` | Calibration Threshold Policy | 2 | completed | high-confidence threshold, low-confidence policy |
| `P9-605` | Manual Binding Fallback Service | 2 | completed | create request, approve/reject, audit |
| `P9-606` | Calibration Service Verification | 2 | completed | concurrency tests, race tests, gate evidence |

### WS-07 Inventory Sync Orchestration

| Task ID | Task Name | Batch | Status | Deliverables |
| --- | --- | --- | --- | --- |
| `P9-701` | Inventory Provider Port | 3 | completed | provider interface, cursor contract |
| `P9-702` | Douyin Fixture / Mock Inventory Provider | 3 | completed | deterministic fixture provider, success/error scenarios |
| `P9-703` | Inventory Sync Orchestrator | 3 | completed | sync run creation, snapshot capture, stats |
| `P9-704` | Binding Resolution Pipeline | 3 | completed | confirmed binding priority, automatic candidate flow |
| `P9-705` | Sync Failure and Manual Rerun | 3 | completed | failure classification, safe rerun, manual retry |
| `P9-706` | Sync Orchestration Verification | 3 | completed | task tests, cursor tests, race tests |

### WS-08 Permission, Audit and Safety

| Task ID | Task Name | Batch | Status | Deliverables |
| --- | --- | --- | --- | --- |
| `P9-801` | Inventory Sync RBAC | 4 | completed | view/run/binding/audit permissions |
| `P9-802` | Inventory Sync Audit Events | 4 | completed | run start/completion events, binding events |
| `P9-803` | Inventory Metadata Redaction | 4 | completed | provider metadata redaction, safe error details |
| `P9-804` | Provider and Production Boundary Guard | 4 | completed | fake provider guard, real boundary guard |

### WS-09 Backend APIs

| Task ID | Task Name | Batch | Status | Deliverables |
| --- | --- | --- | --- | --- |
| `P9-901` | Sync Run APIs | 5 | completed | create/list/detail/status/statistics |
| `P9-902` | Inventory Snapshot APIs | 5 | completed | list/filter/cursor pagination |
| `P9-903` | SKU Binding and Calibration APIs | 5 | completed | bindings, candidates, recalibration, history |
| `P9-904` | Manual Binding APIs | 5 | completed | pending list, detail, approve, reject |
| `P9-905` | Sync History and Audit APIs | 5 | completed | run history, error summary, audit timeline |

### WS-10 Admin UI

| Task ID | Task Name | Batch | Status | Deliverables |
| --- | --- | --- | --- | --- |
| `P9-1001` | Inventory Sync Dashboard | 6 | planned | sync run list, status, statistics, filters |
| `P9-1002` | Sync Run Detail | 6 | planned | run stats, inventory snapshot, unresolved items |
| `P9-1003` | SKU Calibration Workspace | 6 | planned | remote SKU, local candidate, score details |
| `P9-1004` | Manual Binding Workspace | 6 | planned | manual approve/reject, conflict hints, search |
| `P9-1005` | Binding History and Audit | 6 | planned | binding history, calibration history, audit timeline |
| `P9-1006` | Admin UX Verification | 6 | planned | i18n, loading, empty, error, responsive checks |

### WS-11 Integration and Closure

| Task ID | Task Name | Batch | Status | Deliverables |
| --- | --- | --- | --- | --- |
| `P9-1101` | Integration Fixtures | 7 | planned | success, low-confidence, conflict, manual binding, failure |
| `P9-1102` | API / Admin E2E | 7 | planned | fixture sync, candidate calibration, manual binding |
| `P9-1103` | Platform Boundary Final Gate | 7 | planned | real credential guard, real network guard |
| `P9-1104` | P9 Final Development Gate | 7 | planned | task completion, acceptance criteria, quality gates |
| `P9-1105` | P9 Development Closure Evidence | 7 | planned | closure markdown, closure JSON, P10 reservation |

## Batch Summary

| Batch | Batch Name | Task IDs |
| --- | --- | --- |
| 1 | Inventory Sync, SKU Binding Calibration and Manual Fallback / Domain and Persistence Foundation | `P9-501` - `P9-506` |
| 2 | Calibration and Manual Binding Services | `P9-601` - `P9-606` |
| 3 | Fixture Inventory Sync Orchestration | `P9-701` - `P9-706` |
| 4 | Permissions, Audit and Safety | `P9-801` - `P9-804` |
| 5 | Backend APIs | `P9-901` - `P9-905` |
| 6 | Admin Inventory Sync and Binding Center | `P9-1001` - `P9-1006` |
| 7 | Integration, Final Gates and Development Closure | `P9-1101` - `P9-1105` |

## Batch 4 Completion Artifact

Batch 4 permissions, audit, redaction, and safety boundary work is completed locally.

```text
batch4TaskIdsExactlyMatch=true
batch4TaskCount=4
batch4Completed=true
batch4GatePassed=true
modulePath=backend/internal/modules/inventorysyncp9
existingRBACReused=true
existingAuditInfrastructureReused=true
existingSecretRedactorReused=true
duplicateSecurityFrameworkCreated=false
postgresIntegrationStatus=not_run
postgresIntegrationPassed=false
postgresIntegrationDeferredTo=P9_Final_Development_Closure
p9FinalClosureBlocker=true
productionReady=false
p9Complete=false
```

Evidence:

- [`P9_TASK_BATCH_4_PERMISSIONS_AUDIT_SAFETY.md`](P9_TASK_BATCH_4_PERMISSIONS_AUDIT_SAFETY.md)
- [`p9-task-batch-4-permissions-audit-safety.json`](p9-task-batch-4-permissions-audit-safety.json)
- [`P9_TASK_BATCH_4_PERMISSIONS_AUDIT_SAFETY_GATE.md`](P9_TASK_BATCH_4_PERMISSIONS_AUDIT_SAFETY_GATE.md)
- [`p9-task-batch-4-permissions-audit-safety-gate.json`](p9-task-batch-4-permissions-audit-safety-gate.json)

## Batch 5 Completion Artifact

Batch 5 backend APIs are completed locally on `dev`. The surface is fixture/mock-only and keeps the P10 production boundary intact.

```text
batch5TaskIdsExactlyMatch=true
batch5TaskCount=5
batch5Completed=true
batch5GatePassed=true
apiImplemented=true
strictJSONBodyLimit=true
keysetPagination=true
allowedActionsImplemented=true
controlledRecalibrationHistory=true
adminUiImplemented=false
realCredentialsEnabled=false
realNetworkEnabled=false
workerImplemented=false
postgresIntegrationStatus=not_run
postgresIntegrationPassed=false
postgresIntegrationDeferredTo=P9_Final_Development_Closure
p9FinalClosureBlocker=true
productionReady=false
p9Complete=false
```

Evidence:

- [`P9_TASK_BATCH_5_BACKEND_APIS.md`](P9_TASK_BATCH_5_BACKEND_APIS.md)
- [`p9-task-batch-5-backend-apis.json`](p9-task-batch-5-backend-apis.json)
- [`P9_TASK_BATCH_5_BACKEND_APIS_GATE.md`](P9_TASK_BATCH_5_BACKEND_APIS_GATE.md)
- [`p9-task-batch-5-backend-apis-gate.json`](p9-task-batch-5-backend-apis-gate.json)

Next product batch is `P9-1001` through `P9-1006` Admin Inventory Sync and Binding Center, status `planned`.

## Batch 1 Scope Artifact

Batch 1 is the only immediately actionable product batch.

```text
batch1TaskIdsExactlyMatch=true
batch1TaskCount=6
batch1ScopeCreated=true
batch1ScopeReady=true
batch1ScopeGatePassed=true
```

## Required Gates

```bash
pnpm test:p9-entry
pnpm p9:entry-gate
pnpm test:p9-plan
pnpm p9:plan-gate
pnpm test:p9-task-batch-1-scope
pnpm p9:task-batch-1-scope-gate
pnpm test:p9-task-batch-1-domain-persistence
pnpm p9:task-batch-1-domain-persistence-gate
pnpm test:p9-task-batch-2-sku-calibration
pnpm p9:task-batch-2-sku-calibration-gate
pnpm test:p9-task-batch-3-sync-orchestration
pnpm p9:task-batch-3-sync-orchestration-gate
pnpm test:p9-task-batch-4-permissions-audit-safety
pnpm p9:task-batch-4-permissions-audit-safety-gate
```

## P10 Boundary

- real credentials
- real platform writes
- automatic publish
- automatic listing
- production gray release
- production tag
- production release
- final production acceptance

