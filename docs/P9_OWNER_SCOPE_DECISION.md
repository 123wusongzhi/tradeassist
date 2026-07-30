# P9 Owner Scope Decision

Status: **approved**

```text
decisionId=P9-OWNER-SCOPE-DECISION-20260728T0440Z
decisionStatus=approved
approvedByRole=Owner
approvalSource=explicit_owner_instruction
sourceDate=2026-07-28
canonicalScopeResolved=true
scopeConfidence=high
productionReady=false
```

## Canonical Scope

Douyin Shop inventory sync MVP with SKU binding calibration and manual binding fallback.

## Business Objective

Lock the authoritative P9 scope and keep future work from drifting into unapproved product changes.

## User Value

Maintainers get one high-confidence P9 definition, a machine-checkable gate, and a documented P10 boundary.

## In Scope

- inventory sync run records
- remote SKU inventory snapshots
- local and remote SKU binding
- SKU identifier normalization
- explainable candidate matching
- low-confidence human confirmation
- binding conflict human handling
- manual binding fallback
- fixture / mock Douyin inventory provider
- inventory sync orchestration
- P9 RBAC and audit
- backend APIs
- Admin binding workspace
- integration fixtures and tests
- non-production E2E
- P9 development closure

## Out of Scope

- real Douyin OAuth
- real access token / refresh token
- real shop credentials
- real Douyin API calls
- real inventory reads or writes
- automatic publish
- automatic listing
- production gray release
- production tag
- production release
- final production acceptance

## Safety Boundary

```text
realCredentialsEnabled=false
realPlatformNetworkEnabled=false
realPlatformReadEnabled=false
realPlatformWriteEnabled=false
automaticInventoryMutationEnabled=false
automaticPublishEnabled=false
automaticListingEnabled=false
productionReady=false
finalProductionAcceptancePhase=P10
```

