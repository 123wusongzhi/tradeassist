# P4.1 Repository Tenant Enforcement

Patterns for request-scoped tenant isolation at the GORM query layer (Phase P4.1).

## Status Banner

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**

---

## 概述

P4.1 centralizes tenant filtering in two packages:

| Package | Role | Primary consumers |
| --- | --- | --- |
| `backend/internal/pkg/adminperm/tenant_scope.go` | HTTP handler layer — reads **trusted** tenant from Gin (`ctxkey.TenantID`) | `product`, `order`, `shop`, `files` services |
| `backend/internal/pkg/repository/tenant_scope.go` | Data access layer — CRUD helpers + optional config resolver | Same modules; workers via explicit `tenantID int64` |

Lower-level filter: `security.TenantScopedQuery` in `backend/internal/pkg/security/authorize.go` L131–137.

---

## 信任边界

```text
JWT tenant_id claim
  → middleware/jwt.go ResolveRequestTenantID (staging/prod: must be > 0)
  → ctxkey.TenantID + security.TenantContext
  → adminperm.TenantIDFromGin  OR  repository.RequireTenantID
  → WHERE tenant_id = ?
```

| Source | Function | Trust model |
| --- | --- | --- |
| Access token | `adminperm.TenantIDFromGin` | **Trusted** — set only by auth middleware |
| Request body `tenantId` | — | **Untrusted** — services must ignore (e.g. `product.Create` stamps JWT) |
| Dev/demo fallback | `config.ResolveRequestTenantID` | **Config-gated** — forbidden in staging/production |
| Webhook | `webhook/shop_resolver.go` | Tenant from DB shop row, not client input |
| Worker | `tasktenant.BuildWorkerContext` | Tenant from task payload; require `> 0` |
| System / rotation | `repository.SystemFindByID` | Requires `security.SystemFromContext` |

---

## adminperm 模式

File: `backend/internal/pkg/adminperm/tenant_scope.go`

### TenantIDFromGin

```go
tid, err := adminperm.TenantIDFromGin(c)
// Returns errTenantContextMissing if ctxkey.TenantID missing or <= 0
```

Errors: `backend/internal/pkg/adminperm/tenant_errors.go` → `TENANT_CONTEXT_MISSING`.

### ApplyTenantScope

```go
scoped, tid, err := adminperm.ApplyTenantScope(c, tx)
// scoped = tx.Where("tenant_id = ?", tid)
```

**Use for:** list endpoints and any query builder starting in a handler.

**Example:** `product/service.go` List L185–189, `order/service.go` List L456–460, `files/service.go` List L308.

---

## repository 模式

File: `backend/internal/pkg/repository/tenant_scope.go`

### ScopeTenant / ApplyTenantScope

```go
q := repository.ScopeTenant(tx, tenantID)
// Delegates to security.TenantScopedQuery
```

`ApplyTenantScope(c, cfg, tx)` combines `RequireTenantID` (with optional `config.TenantResolver` dev fallback) + `ScopeTenant`. Prefer **adminperm** in HTTP handlers; use **repository** when you already hold `tenantID` or need `ConfigAdapter`.

### FindByID

```go
err := repository.FindByID(ctx, tx, &dest, tenantID, id)
// SELECT … WHERE tenant_id = ? AND id = ?
```

Cross-tenant ID → `gorm.ErrRecordNotFound` (no leak via 403 vs 404 distinction at DB layer).

### UpdateByID

```go
err := repository.UpdateByID(ctx, tx, model, tenantID, id, updates)
// RowsAffected == 0 → ErrRecordNotFound
```

### DeleteByID

```go
n, err := repository.DeleteByID(ctx, tx, model, tenantID, id)
```

### SystemFindByID（特权路径）

```go
err := repository.SystemFindByID(ctx, tx, &dest, id)
// Requires security.SystemFromContext — no tenant filter
```

For key rotation, cross-tenant batch jobs, etc. Every call site must be auditable. Worker helper: `security.WorkerSystemContext` in `backend/internal/pkg/security/system.go`.

---

## 推荐用法

### Handler service（HTTP）

```go
func (s *Service) Get(c *gin.Context, id uuid.UUID) (*DTO, error) {
    tid, err := adminperm.TenantIDFromGin(c)
    if err != nil {
        return nil, err
    }
    var row Model
    if err := repository.FindByID(c.Request.Context(), s.DB, &row, tid, id); err != nil {
        return nil, err
    }
    // Optional secondary boundary:
    if err := adminperm.EnsureStoreVisible(c, s.DB, row.ShopID); err != nil {
        return nil, err
    }
    return toDTO(&row), nil
}
```

### List query

```go
tx := s.DB.WithContext(ctx).Model(&Model{})
if scoped, _, err := adminperm.ApplyTenantScope(c, tx); err != nil {
    return nil, err
} else {
    tx = scoped
}
// Then ApplyStoreScope / ApplyProductScope if needed
```

### Worker / webhook

```go
scope := tasktenant.TaskScope{TenantID: ev.TenantID, ShopID: shopID}
if err := tasktenant.RequireTaskTenant(scope.TenantID); err != nil {
    return err
}
ctx := tasktenant.BuildWorkerContext(scope, actorID, "order.webhook.upsert")
// Pass scope.TenantID into repository.FindByID — do not use Gin
```

---

## 与 shop scope 的组合

Tenant isolation is **primary**; shop grants are **secondary** (operator roles):

| Helper | File | Purpose |
| --- | --- | --- |
| `ApplyStoreScope` | `adminperm/context.go` L90+ | Filter orders/shops by grant |
| `EnsureStoreVisible` | `adminperm/context.go` | Single-resource shop check |
| `ApplyProductScope` | `adminperm/product_scope.go` | Product list via store linkage |
| `EnsureProductVisible` | `adminperm/product_scope.go` | Product get/update guard |

Apply order: **tenant first**, then shop/product scope.

---

## 反模式（P4.1 待清理）

| Anti-pattern | Example | Fix |
| --- | --- | --- |
| Raw `First(&row, "id = ?", id)` in handler | `order/service.go` `findOrderBare`, `Delete` | `repository.FindByID` / `DeleteByID` |
| Raw `First` on product for AI | `product/ai_title.go` L146–150 | `FindByID` + visibility check |
| Trust body `tenantId` | — | Stamp from `TenantIDFromGin` only |
| `TenantScopedQuery` with `tenantID=0` in prod | Legacy single-tenant | Require JWT tenant > 0 in staging/prod |
| System query without `SystemContext` | Ad-hoc cross-tenant SELECT | `SystemFindByID` + audit |

---

## 错误码

| Error | Code | HTTP (typical) |
| --- | --- | --- |
| `security.ErrTenantContextMissing` | `TENANT_CONTEXT_MISSING` | 401/403 |
| `security.ErrTenantAccessDenied` | `TENANT_ACCESS_DENIED` | 403 |
| `adminperm` missing tenant | `TENANT_CONTEXT_MISSING` | 403 |
| `security.ErrSystemContextRequired` | `SYSTEM_CONTEXT_REQUIRED` | 500 / job fail |
| Production fallback | `PRODUCTION_TENANT_FALLBACK_FORBIDDEN` | 403 |

Defined in `backend/internal/pkg/security/errors.go`, `backend/internal/config/tenant_config.go`.

---

## 测试

Automated cross-tenant cases: `backend/internal/securitytests/idor/idor_test.go`.

Pattern for new tests:

1. Seed resource in `tenantB`
2. Build Gin ctx with `ctxkey.TenantID = tenantA`
3. Assert `Get`/`Update`/`Delete`/`List` fails or excludes foreign rows

---

## 相关文档

- `docs/P4_1_TENANT_ENFORCEMENT_AUDIT.md` — per-module status
- `docs/P4_1_TENANT_DATA_MIGRATION.md` — legacy `tenant_id=0` backfill
- `docs/P4_TENANT_ISOLATION.md` — TenantContext propagation
