# P4-V SQL Tenant Scope Report

Module and repository tenant-scope audit for Phase P4-V closure. Focuses on fixes applied to inventory, ordersync, productpublish, customerchat, and taskcenter, plus shared repository helpers.

## Status Banner

**P4-V Module Fixes Landed** · **repository.FindByID + ApplyTenantScope Standard** · **55 IDOR Cases Pass**

---

## Shared Infrastructure

| Package | File | Functions | Purpose |
| --- | --- | --- | --- |
| `repository` | `backend/internal/pkg/repository/tenant_scope.go` | `ApplyTenantScope`, `FindByID`, `DeleteByID`, `SystemFindByID` | Canonical tenant filter; system queries require audited context |
| `adminperm` | `backend/internal/pkg/adminperm/tenant_scope.go` | `ApplyTenantScope` | Gin-context tenant restriction for admin APIs |
| `tenantquery` | `backend/internal/pkg/tenantquery/scope.go` | `FindByIDTenant`, `FindByIDShopTenant`, `ScopeTenant`, `ScopeShopTenant` | Shop-joined and product-joined scopes |

### `FindByID` contract

```text
WHERE id = ? AND tenant_id = ?
```

Missing row or wrong tenant → `gorm.ErrRecordNotFound` (no cross-tenant leak).

### `SystemFindByID` guard

Requires system worker context; otherwise returns `ErrSystemContextRequired`. Verified by P4-V gate (`system-repository-naming` check).

---

## P4-V Module Fixes

### inventory

| Operation | Before (gap) | After (P4-V) | File |
| --- | --- | --- | --- |
| Task list | Possible cross-tenant list leak | `adminperm.ApplyTenantScope(c, tx)` on list query | `backend/internal/modules/inventory/queries.go` |
| Task get by ID | Direct ID lookup | `repository.FindByID(ctx, s.DB, &t, tenantID, id)` | `backend/internal/modules/inventory/queries.go` |
| Worker task load | Unscoped | `repository.FindByID(ctx, s.DB, &task, tenantID, taskID)` | `backend/internal/modules/inventory/queries.go` |

IDOR tests: `TestIDOR_InventoryTaskFindByIDCrossTenant`, `TestIDOR_InventoryListTasksExcludesOtherTenant`

---

### ordersync

| Operation | Before (gap) | After (P4-V) | File |
| --- | --- | --- | --- |
| Task list | Cross-tenant list risk | `adminperm.ApplyTenantScope(c, tx)` | `backend/internal/modules/ordersync/service.go` |
| Task get | Direct lookup | `repository.FindByID(c.Request.Context(), s.DB, &t, tid, id)` | `backend/internal/modules/ordersync/service.go` |
| Worker load | Unscoped | `repository.FindByID(..., &task, tid, taskID)` | `backend/internal/modules/ordersync/service.go` |

IDOR tests: `TestIDOR_OrderSyncTaskFindByIDCrossTenant`, `TestIDOR_OrderSyncListExcludesOtherTenant`

---

### productpublish

| Operation | Before (gap) | After (P4-V) | File |
| --- | --- | --- | --- |
| Task list | Cross-tenant list risk | `adminperm.ApplyTenantScope(c, tx)` | `backend/internal/modules/productpublish/service_queries.go` |
| Task get (service) | Direct lookup | `repository.FindByID(ctx, s.DB, &t, tenantID, taskID)` | `backend/internal/modules/productpublish/service_queries.go` |
| Task get (handler) | Direct lookup | `repository.FindByID(c.Request.Context(), s.DB, &task, tid, taskID)` | `backend/internal/modules/productpublish/service_queries.go` |

IDOR tests: `TestIDOR_ProductPublishTaskFindByIDCrossTenant`, `TestIDOR_ProductPublishListExcludesOtherTenant`

Shop scope tests: `TestShopScope_ProductPublishListOnlyGrantedShops`

---

### customerchat

| Operation | Before (gap) | After (P4-V) | File |
| --- | --- | --- | --- |
| Conversation list | Cross-tenant list risk | `adminperm.ApplyTenantScope(c, tx)` | `backend/internal/modules/customerchat/service.go` |
| Conversation get | Direct lookup | `repository.FindByID(c.Request.Context(), s.DB, &row, tid, id)` | `backend/internal/modules/customerchat/service.go` |

IDOR tests: `TestIDOR_CustomerConversationFindByIDCrossTenant`, `TestIDOR_CustomerChatListExcludesOtherTenant`

Shop scope tests: `TestShopScope_CustomerChatGetOtherShopDenied`, `TestShopScope_CustomerChatListOnlyGrantedShops`

---

### taskcenter

| Operation | Before (gap) | After (P4-V) | File |
| --- | --- | --- | --- |
| Alert / failure / collect lists | Cross-tenant aggregation | `applyTenantListFilter(q, p)` on all list paths | `backend/internal/modules/taskcenter/service_list.go`, `backend/internal/modules/taskcenter/service.go` |
| Alert get | Direct lookup | `repository.FindByID` (via IDOR tests on alert/failure models) | Covered by securitytests |

IDOR tests: `TestIDOR_TaskCenterAlertFindByIDCrossTenant`, `TestIDOR_TaskCenterFailureMarkFindByIDCrossTenant`, `TestIDOR_TaskCenterCollectTaskScopedDenied`, `TestIDOR_TaskCenterAlertScopedListExcludesOtherTenant`, `TestIDOR_TaskCenterFailureMarkScopedListExcludesOtherTenant`

---

## Adjacent Modules (verified in P4-V gate, pre-existing or extended)

| Module | Pattern | File | Gate check |
| --- | --- | --- | --- |
| webhook | `tenant_id = ?` on event processing | `backend/internal/modules/webhook/processor.go` | `webhook-tenant` |
| exportmod | `repository.FindByID` + `ApplyTenantScope` | `backend/internal/modules/exportmod/service.go` | `exportmod-tenant` |
| files | `repository.FindByID` on access/delete/scan | `backend/internal/modules/files/service.go`, `access.go`, `scan_worker.go` | IDOR file tests |
| product | `ApplyTenantScope` + `FindByID` | `backend/internal/modules/product/service.go` | IDOR product tests |
| order | `ApplyTenantScope` + `FindByID` | `backend/internal/modules/order/service.go` | IDOR order tests |
| shop | `ApplyTenantScope` + `FindByID` | `backend/internal/modules/shop/service.go` | IDOR + shop scope tests |
| securitymod | Rotation job `FindByID` tenant scoped | `backend/internal/securitytests/idor/idor_p42_modules_test.go` | IDOR rotation tests |

---

## Tables with `tenant_id` (P4.2 lineage, P4-V validated)

| Table / model | Scope mechanism | P4-V test coverage |
| --- | --- | --- |
| `inventory_sync_tasks` | `tenant_id` column + `FindByID` | IDOR inventory |
| `order_sync_tasks` | `tenant_id` column + `FindByID` | IDOR ordersync |
| `product_publish_tasks` | `tenant_id` column + `FindByID` | IDOR productpublish + shop scope |
| `customer_conversations` | `tenant_id` column + `FindByID` | IDOR customerchat + shop scope |
| `collect_tasks` | `tenant_id` via taskcenter filter | IDOR taskcenter |
| `task_failure_marks` | `tenant_id` + list filter | IDOR taskcenter |
| `export_jobs` | `FindByID` + list scope | IDOR export + shop scope |
| `files` | `FindByID` | IDOR files |
| `ai_product_text_batches` | `FindByID` | IDOR repository |
| `ai_product_image_batches` | `FindByID` | IDOR repository |
| `webhook_events` | Tenant filter in processor + `FindByID` | IDOR webhook |

Migration reference: P4.2 `migrateP42Security` (see `docs/P4_2_REPOSITORY_TENANT_COVERAGE.md`).

---

## Worker Query Pattern

```text
HTTP / worker message carries tenant_id
  → tasktenant.BeginWorker (where applicable)
  → security.WorkerSystemContext + ShopScope (shop-bound workers)
  → repository.FindByID(ctx, db, dest, tenantID, id)
  → OR adminperm.ApplyTenantScope(c, tx) for list APIs
```

File scan worker additionally uses `WHERE id = ? AND tenant_id = ?` on status transitions (`backend/internal/modules/files/scan_worker.go`).

---

## Audit Matrix

| Module | List scoped | Get by ID scoped | Worker scoped | IDOR test | Shop scope test | P4-V fix |
| --- | --- | --- | --- | --- | --- | --- |
| inventory | Yes | Yes | Yes | Yes | — | **Yes** |
| ordersync | Yes | Yes | Yes | Yes | — | **Yes** |
| productpublish | Yes | Yes | Yes | Yes | Yes | **Yes** |
| customerchat | Yes | Yes | — | Yes | Yes | **Yes** |
| taskcenter | Yes | Yes | Partial | Yes | — | **Yes** |
| webhook | Yes (processor) | Yes | Yes | Yes | N/A | Pre-existing |
| exportmod | Yes | Yes | Yes | Yes | Yes | Pre-existing |
| files | Yes | Yes | Yes | Yes | — | Pre-existing |
| product | Yes | Yes | — | Yes | — | Pre-existing |
| order | Yes | Yes | — | Yes | Yes | Pre-existing |
| shop | Yes | Yes | — | Yes | Yes | Pre-existing |

---

## Verification Commands

```bash
cd backend

# Full IDOR suite (55 cases)
go test ./internal/securitytests/idor/... -count=1

# Shop scope (21 cases)
go test ./internal/securitytests/shopscope/... -count=1

# P4-V gate (static tenant-scope checks)
node ../scripts/p4-v-security-closure-gate.mjs
```

**Last run:** All three commands exit 0 on Windows dev host.

---

## Production Notes

| Item | Status | Action |
| --- | --- | --- |
| `tenant_id=0` dev fallback | Warning in gate | Confirm `ResolveRequestTenantID` blocks `legacy_dev_zero` in production |
| Shop-only tables (no `tenant_id`) | Documented | Scope via shop join (`tenantquery.ScopeShopTenant`) |
| `SystemFindByID` usage | Audited naming | Code review required for each call site |

---

## Conclusion

**P4-V SQL tenant scope: PASS.**

Inventory, ordersync, productpublish, customerchat, and taskcenter now consistently use `ApplyTenantScope` for list paths and `repository.FindByID` for single-record access. Shared repository guards remain in place. Regression evidence: 55 IDOR + 21 shop scope automated cases passing.
