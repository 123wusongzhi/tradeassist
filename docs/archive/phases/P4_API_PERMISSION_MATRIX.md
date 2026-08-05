# P4 API Permission Matrix

Maps major HTTP routes to authentication requirements, RBAC permission keys, tenant scope, and shop scope as implemented in Phase P4.

## Status Banner

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**

---

## Legend

| Column | Description |
| --- | --- |
| **Auth** | `Public` = no Bearer token; `Bearer` = JWT required; `Bearer+Session` = JWT + active session row when `secure_session` |
| **Permission** | `adminperm` key checked in handler or via helper; `-` = authenticated only |
| **Tenant** | How `tenant_id` is applied: `JWT`, `Resource`, `Global(0)`, `Resolver` |
| **Shop** | Store scope: `All` (admin), `Grant`, `ApplyStoreScope`, `EnsureStoreVisible`, `-` |

Roles: **admin** (all permissions), **operator** (subset), **readonly** (read-only subset). See `backend/internal/pkg/adminperm/matrix.go`.

---

## Authentication & Session

| Method | Route | Auth | Permission | Tenant | Shop |
| --- | --- | --- | --- | --- | --- |
| POST | `/api/v1/auth/login` | Public | - | On create | - |
| POST | `/api/v1/auth/register` | Public | - | On create | - |
| POST | `/api/v1/auth/send-email-code` | Public | - | - | - |
| POST | `/api/v1/auth/refresh` | Public (cookie/body) | - | From token | - |
| GET | `/api/v1/auth/profile` | Bearer+Session | - | JWT | - |
| POST | `/api/v1/auth/logout` | Bearer+Session | - | JWT | - |
| GET | `/api/v1/auth/sessions` | Bearer+Session | - | JWT | - |
| DELETE | `/api/v1/auth/sessions/:id` | Bearer+Session | - | JWT (owner) | - |
| POST | `/api/v1/auth/sessions/revoke-others` | Bearer+Session | - | JWT (owner) | - |
| POST | `/api/v1/auth/logout-all` | Bearer+Session | - | JWT (owner) | - |
| GET | `/api/v1/security/overview` | Bearer | `settings.manage` | JWT | - |

---

## Security Administration

| Method | Route | Auth | Permission | Tenant | Shop |
| --- | --- | --- | --- | --- | --- |
| POST | `/api/v1/security/keys/rotation/prepare` | Bearer | `security.key.rotate` | Global(0) | - |
| GET | `/api/v1/security/keys/rotation/status` | Bearer | `security.key.rotate` | Global(0) | - |
| POST | `/api/v1/security/keys/rotation/start` | Bearer | `security.key.rotate` | Global(0) | - |
| GET | `/api/v1/security/audit/integrity/status` | Bearer | `audit.read` | Global(0) | - |
| POST | `/api/v1/security/audit/integrity/verify` | Bearer | `audit.read` | Global(0) | - |

---

## Settings & Configuration

| Method | Route | Auth | Permission | Tenant | Shop |
| --- | --- | --- | --- | --- | --- |
| GET | `/api/v1/settings` | Bearer | `settings.manage` | Global(0) default | - |
| PUT | `/api/v1/settings` | Bearer | `settings.manage` | Global(0) default | - |
| POST | `/api/v1/settings/test-*` | Bearer | `settings.manage` | Global(0) | - |
| GET | `/api/v1/config-status/*` | Bearer | `config.read` (implicit) | JWT | - |

---

## Files & Storage

| Method | Route | Auth | Permission | Tenant | Shop |
| --- | --- | --- | --- | --- | --- |
| POST | `/api/v1/files/upload` | Bearer | - | JWT (on record) | - |
| GET | `/api/v1/files` | Bearer | - | Partial | - |
| DELETE | `/api/v1/files/:id` | Bearer | - | Resource | - |
| GET | `/static/*` | Public | - | - | - |

---

## Products & AI

| Method | Route | Auth | Permission | Tenant | Shop |
| --- | --- | --- | --- | --- | --- |
| GET | `/api/v1/products` | Bearer | `product.view` | Partial | - |
| POST | `/api/v1/products` | Bearer | `product.write` | Body/JWT | - |
| GET/PUT/DELETE | `/api/v1/products/:id` | Bearer | view/write | Resource | - |
| POST | `/api/v1/products/:id/ai/*` | Bearer | `ai_text.apply` / `ai_image.apply` | Resource | - |
| POST | `/api/v1/products/:id/apply-ai-*` | Bearer | `ai_text.apply` | Resource | - |

---

## Collect

| Method | Route | Auth | Permission | Tenant | Shop |
| --- | --- | --- | --- | --- | --- |
| POST | `/api/v1/collect/tasks` | Bearer | - | **Gap** | - |
| GET | `/api/v1/collect/tasks` | Bearer | - | **Gap** | - |
| POST | `/api/v1/collect/tasks/:id/retry` | Bearer | `task.retry` | **Gap** | - |
| POST | `/api/collector/providers/*/open-login-browser` | Bearer | - | - | - |

---

## Shops & Platform

| Method | Route | Auth | Permission | Tenant | Shop |
| --- | --- | --- | --- | --- | --- |
| GET | `/api/v1/shops` | Bearer | `store.view` | Resource | Grant filter |
| POST | `/api/v1/shops` | Bearer | `store.operate` | JWT | - |
| GET/PUT/DELETE | `/api/v1/shops/:id` | Bearer | view/operate | Resource | `EnsureStoreVisible` |
| PUT | `/api/v1/platform/settings/:platform` | Bearer | `settings.manage` | Global(0) | - |
| GET | `/api/v1/shops/oauth/douyin/callback` | Public | OAuth state | Resolver | Shop bind |

---

## Orders & Inventory

| Method | Route | Auth | Permission | Tenant | Shop |
| --- | --- | --- | --- | --- | --- |
| GET | `/api/v1/orders` | Bearer | `order.view` | Resource | `ApplyStoreScope` |
| GET | `/api/v1/orders/:id` | Bearer | `order.view` | Resource | `EnsureStoreVisible` |
| PUT | `/api/v1/orders/:id` | Bearer | `order.operate` | Resource | Shop check |
| GET | `/api/v1/inventory/*` | Bearer | `inventory.view` | Via shop | Grant |

---

## Admin Users

| Method | Route | Auth | Permission | Tenant | Shop |
| --- | --- | --- | --- | --- | --- |
| GET | `/api/v1/admin/users` | Bearer | `user.manage` | JWT | - |
| POST | `/api/v1/admin/users` | Bearer | `user.manage` | JWT | - |
| PUT | `/api/v1/admin/users/:id/store-permissions` | Bearer | `user.manage` | JWT | Grant write |

---

## Webhooks (Public)

| Method | Route | Auth | Permission | Tenant | Shop |
| --- | --- | --- | --- | --- | --- |
| POST | `/api/v1/webhooks/:platform/:shopId` | Signature | - | Resolver | Shop ID in path |

---

## Operation Logs

| Method | Route | Auth | Permission | Tenant | Shop |
| --- | --- | --- | --- | --- | --- |
| GET | `/api/v1/operation-logs` | Bearer | `operationlog.view` | JWT filter | Optional |

---

## P4 Permission Keys (New)

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

Source: `backend/internal/pkg/adminperm/matrix.go`.

---

## Middleware Stack (All `/api/v1` Authed Routes)

1. `SecurityHeaders` — baseline response headers
2. `CSRFProtection` — Origin/Referer check when `AUTH_SESSION_MODE=secure_session`
3. `BearerAuthWithDB` — JWT parse, session validation, TenantContext

Source: `backend/cmd/server/main.go`, `backend/internal/middleware/jwt.go`.

---

## Gaps & Follow-ups

- Collect and several task tables lack consistent `tenant_id` enforcement at API layer.
- Not every handler calls `RequirePermissionGin`; some rely on implicit authentication only.
- Full permission matrix automation tests are deferred to P10 verification.

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**
