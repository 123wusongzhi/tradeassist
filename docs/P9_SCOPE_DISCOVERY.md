# P9 Scope Discovery

Status: **P9 Canonical Scope Resolved**

This discovery was performed from the current `dev` branch and the real repository `HEAD`.

## P8 baseline

- `P8 Development Complete`
- `P8 Final Gate Passed`
- `P8 Production Ready: false`
- Final production acceptance remains deferred to P10

## Discovery head

```text
p9DiscoveryBaseHead=313726f310f4d76f5967ef169b9c197488f8bed4
```

## Search scope

Searches covered repo-wide P9 references, historical phase wording, roadmap language, and implementation evidence.

## Classified references

| Source | Classification | Why it matters |
| --- | --- | --- |
| `docs/provider.md` | current | Canonical Douyin Phase 9 scope text for inventory sync MVP, SKU binding calibration, and manual binding fallback. |
| `docs/api.md` | current | Public API contract for the Phase 9 / 9.1 / 9.2 Douyin endpoints. |
| `docs/PROGRESS.md` | current | Current project status keeps the P10 boundary preserved and now records the P9 planning realignment. |
| `docs/DOUYIN_E2E_CHECKLIST.md` | current | Acceptance checklist for the Douyin inventory sync chain. |
| `DEMO_CHECKLIST.md` | current | Demo checklist names Phase 9 inventory sync MVP and its SKU binding steps. |
| `backend/internal/providers/platform/douyinshop/product.go` | completed | Official OpenAPI comment for Phase 9.1 `product.detail`. |
| `backend/internal/providers/platform/douyinshop/inventory.go` | completed | Official OpenAPI comment for Phase 9 `sku.syncStock`. |
| `backend/internal/modules/productpublish/douyin_sku_manual_bind.go` | completed | Manual bind fallback error path for Phase 9.2. |
| `docs/DOUYIN_E2E_REPORT_TEMPLATE.md` | historical | Supporting template only, not a scope owner. |
| `docs/github-repo-presentation.md` | historical | Repo presentation guidance only, not a scope owner. |

## Canonical scope

Douyin Shop Phase 9 is the inventory sync MVP with SKU binding calibration and manual binding fallback.

- inventory sync uses the existing inventory orchestration
- SKU calibration uses `product.detail` with `show_draft=true`
- manual bind and unbind handle `ambiguous` and `unmatched` rows
- inventory sync does not guess missing platform SKU IDs
- P9-101 through P9-402 remain the planning / governance foundation
- P9-501 through P9-1105 are the product implementation range, but implementation is not started
- multi-warehouse, auto-replenish, scheduled auto sync, and P10-only capabilities stay out of scope

## Decision

```text
ownerScopeDecisionCreated=true
ownerScopeDecisionApproved=true
ownerScopeDecisionId=P9-OWNER-SCOPE-DECISION-20260728T0440Z
canonicalScopeResolved=true
scopeConfidence=high
planningFoundationCompleted=true
fullImplementationPlanCompleted=true
productImplementationStarted=false
```

## Boundary

```text
productionReady=false
realCredentialsEnabled=false
realPlatformNetworkEnabled=false
realPlatformReadEnabled=false
realPlatformWriteEnabled=false
automaticPublishEnabled=false
automaticListingEnabled=false
p10BoundaryPreserved=true
implementationStarted=false
p9ProductImplementationFileCount=0
```

## Forbidden before execution

- new P9 product code
- real Douyin credentials
- real platform writes
- automatic publish or listing
- P10-only capabilities

