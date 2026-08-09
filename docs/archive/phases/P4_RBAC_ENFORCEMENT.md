# P4 RBAC Enforcement

Role-based access control via permission keys, principal resolution, and shop-scoped grants.

## Status Banner

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**

---

## Architecture

```text
JWT (sub, tenant_id)
  → adminperm.LoadPrincipal(c, db)
  → Principal { UserID, Role, Permissions, StoreGrants }
  → HasPermission(role, perm) OR CanViewStore(shopID)
  → security.GinAuthorizer / RequirePermissionGin
```

Sources:

- `backend/internal/pkg/adminperm/matrix.go` — permission keys & role matrix
- `backend/internal/pkg/adminperm/principal.go` — principal model
- `backend/internal/pkg/security/authorize.go` — AuthorizationService

---

## Roles

| Role | Constant | Scope |
| --- | --- | --- |
| Admin | `admin` | All permissions; all shops |
| Operator | `operator` | Operational subset; granted shops only |
| Readonly | `readonly` | Read-only subset; granted shops only |

Normalization via `normalizeRole()` in adminperm package.

---

## Permission Keys

### Business Permissions

| Key | Description |
| --- | --- |
| `product.view` / `product.write` | Product drafts |
| `ai_text.apply` / `ai_image.apply` | Apply AI results |
| `publish.create_draft` | Platform publish drafts |
| `order.view` / `order.operate` | Orders |
| `inventory.view` / `inventory.operate` | Inventory |
| `customer.view` / `customer.operate` | Customer chat |
| `store.view` / `store.operate` | Shops |
| `task.retry` | Retry async tasks |
| `settings.manage` | System settings |
| `user.manage` | Admin users |
| `operationlog.view` | Audit log read |

### P4 Security Permissions

| Key | admin | operator | readonly |
| --- | --- | --- | --- |
| `security.session.manage` | ✓ | ✓ | - |
| `security.key.rotate` | ✓ | - | - |
| `audit.read` | ✓ | ✓ | ✓ |
| `audit.export` | ✓ | - | - |
| `pii.read_masked` | ✓ | ✓ | ✓ |
| `pii.read_full` | ✓ | - | - |
| `pii.export` | ✓ | - | - |
| `config.read` | ✓ | ✓ | ✓ |
| `config.manage` | ✓ | - | - |

---

## AuthorizationService

Interface (`pkg/security/authorize.go`):

| Method | Behavior |
| --- | --- |
| `RequirePermission(ctx, perm)` | Deny if role lacks key |
| `RequireTenant(ctx, tenantID)` | Deny if JWT tenant mismatch |
| `RequireShopAccess(ctx, shopID)` | Deny if shop not in grants (non-admin) |
| `RequireSensitivePermission(ctx, perm)` | Permission + reauth hook (handler-level) |

Helpers:

- `RequirePermissionGin(c, db, perm)` — handler shortcut
- `Deny(c, err)` — standardized 401/403 responses

Error codes: `AUTHENTICATION_REQUIRED`, `PERMISSION_DENIED`, `TENANT_ACCESS_DENIED`, `SHOP_ACCESS_DENIED`.

---

## Shop Scope

`StoreGrant` binds operator/readonly users to specific shop UUIDs.

| Helper | Usage |
| --- | --- |
| `Principal.CanViewStore(shopID)` | Single shop check |
| `Principal.AllowedStoreIDs()` | nil = admin (all shops) |
| `adminperm.ApplyStoreScope(c, db, tx, "shop_id")` | SQL scope for lists |
| `adminperm.EnsureStoreVisible(c, db, shopID)` | Single resource guard |

Example: order service applies store scope on list queries and visibility on detail.

---

## Handler Enforcement Patterns

### Pattern A — Explicit permission check

```go
if !adminperm.RequirePermission(c, h.DB, adminperm.PermSettingsManage) {
    return
}
```

Used in: security overview, securitymod handlers, settings.

### Pattern B — Domain helper

```go
if !adminperm.CanWriteOrders(c, h.Svc.DB) { ... }
```

Used in: order mutations.

### Pattern C — Authentication only

Some routes (e.g. files upload) require Bearer auth but no explicit permission key yet.

---

## Sensitive Operations

Key rotation requires:

1. Permission `security.key.rotate`
2. JSON body `confirmPhrase: "ROTATE-KEYS-DRY-RUN"`

Reauth tokens (`auth_reauth_tokens`) model exists for future step-up on:

- Settings secret changes
- User password reset by admin
- PII full read/export

---

## Deny Responses

| Error | HTTP | Code |
| --- | --- | --- |
| Not authenticated | 401 | `UNAUTHORIZED` |
| Permission denied | 403 | `PERMISSION_DENIED` |
| Tenant mismatch | 403 | `FORBIDDEN` |
| Shop denied | 403 | `STORE_PERMISSION_DENIED` |

User-facing message for permission deny: `无权限执行此操作`.

---

## Gaps

- Not all modules use `RequirePermissionGin` consistently
- `TenantContext.Permissions` not always populated in middleware (loaded on demand via adminperm)
- Fine-grained resource ACLs (per-product) not implemented
- Automated RBAC matrix tests deferred

---

## Deferred Verification

- [ ] Permission regression test suite per route
- [ ] Operator role penetration (attempt admin-only endpoints)
- [ ] Store grant boundary tests across two shops

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**
