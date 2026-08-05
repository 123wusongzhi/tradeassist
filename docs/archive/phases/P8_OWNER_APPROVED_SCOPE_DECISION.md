# P8 Owner Approved Scope Decision

Status: **approved**

```text
decisionId=P8-OWNER-SCOPE-DECISION-20260720
sourceType=product_owner_decision
sourceDate=2026-07-20
phase=P8
canonical=true
productionReady=false
```

## Canonical Scope

P8 is **Operations Task Center, Draft Orchestration and Human Review Loop MVP**.

The phase establishes a complete development loop from AI/rule operation suggestion to operation task, local platform draft, human review, controlled draft write, execution result, failure handling, and audit trail.

## Required Boundary

```text
Human-in-the-loop
Draft-first
No automatic publish
No automatic listing
No production credentials
```

Douyin is limited to mock or sandbox mode in P8. Other platforms are `local_draft_only`.

## Historical Phase 8

The historical Douyin Phase 8 order sync MVP is already completed and is not reused as the current post-P7 P8 scope.

## P10 Reserved

Real credentials, real platform writes, production gray release, production tag, production release, Production Ready, and final performance/capacity acceptance remain reserved for P10.
