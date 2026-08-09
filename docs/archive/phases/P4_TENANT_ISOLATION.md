# P4 Tenant Isolation

Tenant scoping model, context propagation, and enforcement helpers for multi-tenant readiness.

## Status Banner

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**

---

## Current State

TradeMind MVP runs primarily as **single-tenant** (`tenant_id = 0`). Phase P4 adds:

1. **`tenant_id` on auth and core business tables**
2. **JWT claim `tenant_id`** on every access token
3. **`TenantContext`** attached to Gin and `context.Context`
4. **Authorization helpers** for tenant and shop matching
5. **Webhook shop resolver** copying tenant from shop record

Full query-level isolation is **partial** — see gaps below.

---

## TenantContext

```go
type TenantContext struct {
    TenantID    int64
    UserID      uuid.UUID
    SessionID   uuid.UUID
    Role        string
    Permissions []string
    ShopScope   []uuid.UUID
    RequestID   string
}
```

| Function | Purpose |
| --- | --- |
| `BuildTenantContext(...)` | From Gin auth keys |
| `SetGin` / `FromGin` | Request-scoped access |
| `WithTenantContext` / `FromContext` | Service/worker propagation |
| `WorkerTenantContext(tenantID, userID)` | Background jobs |

Source: `backend/internal/pkg/security/tenant.go`.

---

## Propagation Chain

```text
AdminUser.tenant_id
  → LoginSessionResult / JWT tenant_id claim
  → middleware: ctxkey.TenantID + security.SetGin
  → operationlog.Write: row.TenantID from ctx if unset
  → Optional: EnsureTenantMatch / TenantScopedQuery
```

Middleware reference: `backend/internal/middleware/jwt.go`.

---

## Enforcement Helpers

### TenantScopedQuery

```go
security.TenantScopedQuery(tx, tenantID)
// Adds WHERE tenant_id = ? when tenantID > 0
```

Note: **No filter when tenantID is 0** — preserves single-tenant behavior.

### EnsureTenantMatch

```go
security.EnsureTenantMatch(ctx, resourceTenantID)
// Returns ErrTenantAccessDenied if ctx tenant != resource
```

### AuthorizationService.RequireTenant

Same check via `GinAuthorizer` for handler-level enforcement.

Source: `backend/internal/pkg/security/authorize.go`.

---

## Shop-Level Isolation (Secondary Boundary)

For operator roles, **shop grants** provide finer isolation than tenant alone:

```text
Tenant
  └── Shop A (grant)
  └── Shop B (no grant) ← denied via EnsureStoreVisible
```

Admin role bypasses shop grants (`AllowedStoreIDs()` returns nil = all).

Implementation: `backend/internal/pkg/adminperm/principal.go`, order/inventory services.

---

## Webhook Tenant Binding

Public webhooks do not use JWT. Tenant is resolved from shop:

```text
POST /webhooks/douyin/:shopId
  → DBWebhookShopResolver.Lookup(shopId)
  → shop.TenantID → webhook_event.tenant_id → order upsert
```

See `docs/DOUYIN_WEBHOOK_TENANT_ISOLATION.md`.

---

## Tables with tenant_id

See [P4_TENANT_TABLE_MATRIX.md](./P4_TENANT_TABLE_MATRIX.md) for full audit.

**Strong:** auth_*, admin_users, settings, shops, orders, products, files, operation_logs, webhook_events.

**Weak / missing:** collect_tasks, ai_tasks, image_tasks (no column yet).

---

## Product Publish Cross-Check

`productpublish` validates product vs publish record tenant:

```go
if prod.TenantID != 0 && row.TenantID != 0 && prod.TenantID != row.TenantID {
    // reject
}
```

Source: `backend/internal/modules/productpublish/service_create.go`.

---

## Settings Tenant Model

`settings` table unique index: `(tenant_id, group_key, item_key)`.

Most code paths still read/write `tenant_id = 0` for system configuration. Multi-tenant settings require passing TenantContext tenant into `settings.Service` calls.

---

## Risk Register

| Risk | Severity | Mitigation Status |
| --- | --- | --- |
| IDOR on product UUID | High | Partial — column exists, filter incomplete |
| Collect task cross-tenant | High | Open — no tenant column |
| Global settings leak | Medium | By design for MVP single tenant |
| Worker job wrong tenant | Medium | Use WorkerTenantContext (manual) |

---

## Recommended Enforcement Pattern (New Code)

```go
tc := security.FromGin(c)
if tc == nil {
    // 401
}
q := security.TenantScopedQuery(db, tc.TenantID)
if err := security.EnsureTenantMatch(c.Request.Context(), resource.TenantID); err != nil {
    security.Deny(c, err)
    return
}
```

---

## Deferred Verification

- [ ] Multi-tenant fixture tests (tenant 1 vs tenant 2)
- [ ] Migration adding tenant_id to task tables
- [ ] Lint rule: handlers must call TenantScopedQuery on list endpoints

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**
