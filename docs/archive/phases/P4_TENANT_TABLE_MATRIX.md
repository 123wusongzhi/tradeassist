# P4 Tenant Table Matrix

Audit of core database tables for `tenant_id` column presence, indexing, query scoping, and isolation maturity.

## Status Banner

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**

---

## Summary

| Tier | Count | Meaning |
| --- | --- | --- |
| **A — Column + indexed** | 12+ | Model has `tenant_id`; used in auth or business logic |
| **B — Column, partial enforcement** | 3+ | Column exists; not all queries filter by tenant |
| **C — Missing tenant column** | Many task/AI tables | Single-tenant MVP default (`tenant_id=0`) |

Current MVP default tenant is **`0`** (global/single-tenant). Multi-tenant SaaS requires Tier A enforcement on all Tier C tables.

---

## Tier A — Auth & Security (Full Column)

| Table | Model | `tenant_id` | Index | Scoped Queries | Notes |
| --- | --- | --- | --- | --- | --- |
| `auth_sessions` | `auth.AuthSession` | ✓ NOT NULL default 0 | ✓ | By `user_id` | Session bound to admin user tenant |
| `auth_refresh_tokens` | `auth.AuthRefreshToken` | ✓ | ✓ | By session/family | Rotation preserves tenant |
| `auth_login_attempts` | `auth.AuthLoginAttempt` | ✓ | ✓ | By `account_key` | Lockout keys not tenant-namespaced yet |
| `auth_reauth_tokens` | `auth.AuthReauthToken` | ✓ | ✓ | By user/session | Step-up token (handler wiring partial) |
| `admin_users` | `admin.AdminUser` | ✓ | ✓ | Login lookup | JWT carries `tenant_id` from user |
| `operation_logs` | `operationlog.OperationLog` | ✓ | ✓ | List filter | Hash chain partitioned `t{tenant}:YYYY-MM-DD` |
| `files` | `files.FileRecord` | ✓ | ✓ | Partial | `security_status` + `scan_status` columns added |
| `settings` | `settings.Setting` | ✓ | Unique (tenant, group, item) | By tenant + group | Secrets encrypted per row |

---

## Tier A — Business Core

| Table | Model | `tenant_id` | Index | Scoped Queries | Notes |
| --- | --- | --- | --- | --- | --- |
| `products` | `product.Product` | ✓ default 0 | ✓ | Partial | Create accepts body tenant; list scoping incomplete |
| `shops` | `shop.Shop` | ✓ default 0 | ✓ | List + OAuth | Webhook resolver uses shop tenant |
| `orders` | `order.Order` | ✓ default 0 | ✓ | `ApplyStoreScope` | PII masked on read |
| `webhook_events` | `webhook.WebhookEvent` | ✓ | ✓ unique w/ shop | Shop resolver | Douyin tenant isolation documented separately |

---

## Tier B — Partial Enforcement

| Table / Area | Issue | Risk | Planned Fix |
| --- | --- | --- | --- |
| `products` list/get | May not always filter `tenant_id = JWT` | Cross-tenant IDOR | Add `TenantScopedQuery` in all product repos |
| `files` list | Tenant filter not guaranteed | File metadata leak | Filter by JWT tenant + owner |
| `settings` reads | Often `tenant_id=0` in code paths | Shared config in multi-tenant | Pass tenant from TenantContext |
| `collect_tasks` | No `tenant_id` column in model grep | Task data visible across tenants | Migration + scoped APIs |
| `ai_tasks` / `image_tasks` | Typically global | Task enumeration | Add tenant + scoped worker context |

---

## Tier C — Global / Single-Tenant Assumption

These modules operate with implicit `tenant_id=0` until multi-tenant migrations land:

- Collect batches and task events
- AI prompt templates (shared)
- Image tasks queue metadata
- Inventory sync jobs (scoped via shop, not tenant column)
- Idempotency keys (semantic keys, no tenant column)

Workers should use `security.WorkerTenantContext(tenantID, userID)` when processing tenant-bound jobs.

---

## TenantContext Propagation

```
JWT (tenant_id claim)
  → middleware.BearerAuthWithDB
  → ctxkey.TenantID + security.TenantContext
  → operationlog.Write (fallback tenant from ctx)
  → security.EnsureTenantMatch / TenantScopedQuery (opt-in)
```

Implementation: `backend/internal/pkg/security/tenant.go`, `backend/internal/middleware/jwt.go`.

### TenantContext Fields

| Field | Source |
| --- | --- |
| `TenantID` | JWT `tenant_id` |
| `UserID` | JWT `sub` |
| `SessionID` | JWT `session_id` |
| `Role` | Loaded via adminperm (when populated) |
| `Permissions` | Role matrix |
| `ShopScope` | Store grants for operator roles |
| `RequestID` | Trace ID |

---

## Hash Chain Partitioning

Audit entries partition by tenant and UTC date:

```text
chain_partition = "t{tenantID}:2006-01-02"
```

Verification API currently defaults to **tenant 0** in `securitymod.Service.VerifyAuditIntegrity`. Multi-tenant verify must iterate tenants in production.

Source: `backend/internal/modules/operationlog/hash_chain.go`.

---

## Webhook Tenant Resolution

```
POST /webhooks/:platform/:shopId
  → DBWebhookShopResolver
  → shop.TenantID copied to webhook_events.tenant_id
  → order upsert inherits tenant
```

See also `docs/DOUYIN_WEBHOOK_TENANT_ISOLATION.md`.

---

## Recommended Migration Order (Multi-Tenant)

1. `collect_tasks`, `collect_batches` — add `tenant_id`, backfill from creator admin
2. `ai_tasks`, `image_tasks` — add `tenant_id`, backfill from product linkage
3. Enforce `TenantScopedQuery` in product/files list handlers
4. Settings reads: tenant-aware with fallback to 0 for system keys
5. Per-tenant audit chain verification job

---

## Verification Queries (Manual)

```sql
-- Orphan products vs admin tenant (should be empty in strict mode)
SELECT p.id FROM products p
JOIN admin_users u ON p.created_by = u.id
WHERE p.tenant_id != u.tenant_id;

-- Sessions without matching user tenant
SELECT s.id FROM auth_sessions s
JOIN admin_users u ON s.user_id = u.id
WHERE s.tenant_id != u.tenant_id;
```

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**
