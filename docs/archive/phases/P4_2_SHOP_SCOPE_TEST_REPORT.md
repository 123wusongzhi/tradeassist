# P4.2 Shop Scope Test Report

Automated shop authorization scope regression tests (Phase P4.2).

## Status Banner

**5 Automated Cases Passing (SQLite)** · **Closure Target 20+ Not Yet Met** · **NOT Penetration Test Passed**

---

## Test package

- Path: `backend/internal/securitytests/shopscope/shop_scope_test.go`
- Run: `go test ./internal/securitytests/shopscope/... -count=1`
- Tenant: single tenant `42`, two shops A and B
- Roles: `admin`, `operator`, `readonly` via `adminperm.Principal`

---

## Implemented cases (5)

| Test | Role | Grants | Expected |
| --- | --- | --- | --- |
| `TestShopScope_OperatorCannotReadOtherShop` | operator | shop A only | Cannot `GetDetail` shop B |
| `TestShopScope_OperatorCannotUpdateOtherShop` | operator | shop A only | Cannot `Update` shop B |
| `TestShopScope_OperatorListOnlyOwnShops` | operator | shop A only | Can access shop A |
| `TestShopScope_ReadonlyCannotUpdateShop` | readonly | shop A | Update denied |
| `TestShopScope_AdminCanAccessAllShopsInTenant` | admin | all | Can access shop B |

---

## Enforcement layer

- `adminperm` store grants on Gin context
- `shop.Service` calls `EnsureStoreVisible` / operate permission checks
- `order.Service` uses `ApplyStoreScope` on list paths (tested indirectly via P4.1 order tests)

---

## Closure gap (20+ target)

**Recommended additions (not yet implemented):**

| Area | Cases |
| --- | --- |
| Order list/get | Operator sees only granted shops |
| Order update | Readonly denied |
| Export jobs | Shop filter on `exportmod` |
| Product list with shop filter | Operator scoped products |
| Customer conversations | Shop-bound operator |
| Inventory sync tasks | Shop scope on task list |
| Operation logs | Store grant filter |
| Dashboard todos | Shop-scoped KPI |
| Product publish tasks | Shop denial |
| Webhook events | N/A (no admin list) |

---

## Result

| Metric | Value |
| --- | --- |
| Automated cases | **5** |
| Closure target | **20+** |
| Static scan status | **warning** (below target, above minimum 5) |

**Expand shop scope matrix before production multi-store rollout.**
