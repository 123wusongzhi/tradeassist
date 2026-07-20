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
