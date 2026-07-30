# P4.1 Tenant Enforcement Audit

Phase P4.1 module-by-module audit of tenant scoping on CRUD paths, residual IDOR risk, and planned fixes.

## Status Banner

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**

---

## 概述

P4.1 introduces shared helpers in `backend/internal/pkg/repository/tenant_scope.go` and `backend/internal/pkg/adminperm/tenant_scope.go`, plus automated IDOR regression tests in `backend/internal/securitytests/idor/idor_test.go`.

**Legend**

| Symbol | Meaning |
| --- | --- |
| ✓ | Tenant enforced via `ApplyTenantScope` / `repository.FindByID` / `DeleteByID` |
| △ | Partial — secondary boundary (shop scope) or some paths missing |
| ✗ | Not enforced — IDOR risk in multi-tenant mode |
| N/A | No HTTP CRUD surface or inbound-only (webhook) |

**Risk levels:** **Critical** (cross-tenant read/write), **High** (list/enumeration or delete gap), **Medium** (worker/background or create stamping), **Low** (global-by-design).

---

## 审计矩阵

### Core business modules

| Module | Primary service | FindByID / Get | List | Update | Delete | Overall | Risk | Modification plan |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| **product** | `backend/internal/modules/product/service.go` | ✓ `Get` L335–357 via `repository.FindByID` + `EnsureProductVisible` | ✓ `List` L185–189 `adminperm.ApplyTenantScope` | ✓ `Update` L553–562 | ✓ `Delete` L620–627 `repository.DeleteByID` | **Enforced** | Low | AI sub-paths still gap (see below) |
| **order** | `backend/internal/modules/order/service.go` | ✓ `Get` / `loadDetailDTO` L770–777 | ✓ `List` L456–460 + `ApplyStoreScope` | ✓ `Update` L860–865 | ✗ `Delete` L1058–1066 raw `First`/`Delete` | **Partial** | **High** | Wire `Delete`, `findOrderBare`, item/shipment mutators through `repository.FindByID`; add IDOR tests |
| **shop** | `backend/internal/modules/shop/service.go` | ✓ `GetDetail` L338–343 | ✓ `List` L194 | ✓ `Update` L338–343 | ✓ `Delete` L393–397 | **Enforced** | Low | OAuth callback paths already stamp from shop row |
| **files** | `backend/internal/modules/files/service.go`, `access.go` | ✓ `Get`/`DeleteRecordByTenant` L363–368 | ✓ `List` L308 | N/A (metadata patch rare) | ✓ tenant-scoped delete | **Enforced** | Low | Upload stamps JWT tenant L100–115; object key prefix `t{tid}/` |
| **webhook** | `backend/internal/modules/webhook/service.go`, `shop_resolver.go` | N/A (public ingest) | N/A | N/A | N/A | **Resolver-bound** | Medium | Tenant copied from `shops.tenant_id` L250; idempotency keyed by tenant L170–238; no admin list API yet |
| **settings** | `backend/internal/modules/settings/service.go` | Per-row `tenant_id` in upsert L93–148 | ✗ `List` L30–35 returns all tenants | PUT accepts body `tenantId` L70–74 | N/A | **Partial** | **High** | Scope `List`/reads to JWT tenant; keep `tenant_id=0` rows as `system_global` (see migration doc) |
| **collect** | `backend/internal/modules/collect/service.go`, `batch.go` | ✗ `GetDTO` L787 | ✗ `List` L794–813 | ✗ worker paths | ✗ | **Open** | **Critical** | Add `tenant_id` column + migration; scope List/Get; stamp from creator `admin_users.tenant_id` |
| **taskcenter** | `backend/internal/modules/taskcenter/service_list.go`, `handler.go` | Aggregates foreign tables | ✗ no tenant filter on unified failure list | N/A | N/A | **Open** | **High** | Propagate `adminperm.ApplyTenantScope` into each `list*` helper or filter by linked shop/product tenant |
| **operationlog** | `backend/internal/modules/operationlog/service.go` | N/A | △ `List` L185–190 shop scope only | N/A | N/A | **Partial** | Medium | Add `ApplyTenantScope`; write path already stamps from ctx L92–95 |
| **productpublish** | `backend/internal/modules/productpublish/service_create.go` | Shop/product join checks | Via product/shop | Create validates tenant match L59–60 | — | **Partial** | Medium | Add explicit `ApplyTenantScope` on publication list/get |
| **ordersync** | `backend/internal/modules/ordersync/` | Worker | Worker | Worker | — | **Shop-scoped** | Medium | Tasks inherit tenant via shop; webhook handler filters `tenant_id` L91 in `douyin_order_webhook.go` |
| **inventory** | `backend/internal/modules/inventory/` | Shop-scoped queries | Shop-scoped | — | — | **Shop-only** | Medium | Add `tenant_id` column when multi-tenant inventory ships |
| **aitask** | `backend/internal/modules/aitask/model.go` | ✗ no column | ✗ | — | — | **Open** | Medium | Add `tenant_id`; backfill from `product_id` / `created_by` |
| **imagetask** | `backend/internal/modules/imagetask/model.go` | ✗ no column | ✗ | — | — | **Open** | Medium | Same as aitask |
| **securitymod** | `backend/internal/modules/securitymod/service.go`, `rotation.go` | System context | Rotation jobs per tenant row | Admin API | — | **System** | Low | `SystemFindByID` pattern; audit verify defaults tenant 0 L49 |
| **auth / admin** | `backend/internal/modules/auth/`, `admin/model.go` | Session owner checks | By `user_id` | — | — | **User-bound** | Low | JWT carries `tenant_id`; login loads `admin_users.tenant_id` |

---

## Product 子路径缺口

| Path | File | Issue | Risk | Plan |
| --- | --- | --- | --- | --- |
| AI title optimize | `product/ai_title.go` L146–150 | `First(&p, "id = ?", productID)` without tenant | **Critical** | Replace with `repository.FindByID` + `EnsureProductVisible` |
| AI description / apply | `product/ai_description.go` (same pattern) | Same | **Critical** | Same |
| Collect import draft | `product/service.go` `importDraftCore` L711–712 | Hardcodes `TenantID: 0` | **Medium** | Pass worker/request tenant from collect task scope |
| Create body override | `product/service.go` `Create` L297–303 | Body `tenantId` ignored — JWT wins | ✓ OK | Covered by `TestIDOR_ProductCreateStampsTenant` |

---

## Order 子路径缺口

| Path | File | Issue | Risk | Plan |
| --- | --- | --- | --- | --- |
| `findOrderBare` | `order/service.go` L1130–1138 | Shop scope only, no `tenant_id` filter | **High** | Use `repository.FindByID` before `EnsureStoreVisible` |
| `Delete` | `order/service.go` L1058–1066 | Unscoped delete | **Critical** | `repository.DeleteByID` + tenant from Gin |
| Nested item/shipment CRUD | `order/service.go` via `findOrderBare` | Inherits gap | **High** | Fix shared helper once |
| Platform upsert | `order/sync_platform.go`, `platform_upsert.go` | ✓ resolves tenant from shop L144–156 | Low | Keep; marks unresolved payload tenant via shop lookup |

---

## Webhook 与 Worker

```text
POST /webhooks/:platform/:shopId
  → shop_resolver.go Lookup
  → service.go sets ev.TenantID = resolved.TenantID
  → order upsert inherits tenant
```

Worker tasks should use `backend/internal/pkg/tasktenant/context.go` (`RequireTaskTenant`, `EnsureResourceTenantMatch`) when processing tenant-bound jobs.

---

## 自动化验证

| Asset | Path | Coverage |
| --- | --- | --- |
| IDOR regression tests | `backend/internal/securitytests/idor/idor_test.go` | Product (4), Order (3), Shop (3), Files (4), missing-tenant, create stamping — **20+ cases** |
| Manual matrix | `docs/P4_IDOR_TEST_MATRIX.md` | Broader backlog; update statuses after P4.1 |

Run: `cd backend && go test ./internal/securitytests/idor/...`

---

## 改造优先级

1. **P0** — `order.Delete` + `findOrderBare`; product AI paths (`ai_title.go`, `ai_description.go`)
2. **P1** — `collect` tenant column + API scope; `settings.List` tenant filter
3. **P2** — `taskcenter` unified list; `operationlog.List` tenant filter; `aitask` / `imagetask` schema
4. **P3** — `product/importDraftCore` tenant stamping; inventory tenant column

---

## 相关文档

- `docs/P4_1_REPOSITORY_TENANT_ENFORCEMENT.md` — helper usage patterns
- `docs/P4_1_TENANT_DATA_MIGRATION.md` — `tenant_id=0` backfill strategy
- `docs/P4_TENANT_ISOLATION.md` — TenantContext and propagation
- `docs/P4_IDOR_TEST_MATRIX.md` — manual QA matrix
