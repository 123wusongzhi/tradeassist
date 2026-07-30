# P4 IDOR Test Matrix

Insecure Direct Object Reference (IDOR) test cases for Phase P4 security validation. Use as manual QA and future automated security test backlog.

## Status Banner

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**

---

## Test Setup

| Actor | Tenant | Role | Shops |
| --- | --- | --- | --- |
| **UserA** | 0 (or T1) | admin | All |
| **UserB** | 0 (or T2) | operator | Shop-B only |
| **UserC** | 0 (or T2) | readonly | Shop-B only |

Prepare:

- Product `P1` owned by UserA tenant
- Order `O1` on Shop-B
- File `F1` uploaded by UserA
- Session `S1` belonging to UserB
- Shop `SHOP-A` not granted to UserB

Obtain JWT tokens for each actor via login API.

---

## Auth & Session IDOR

| ID | Endpoint | Attack | Expected | Implementation Status |
| --- | --- | --- | --- | --- |
| AUTH-01 | `DELETE /auth/sessions/:id` | UserA deletes UserB session UUID | 404 / not found | ✓ Owner check (`sess.UserID != userID`) |
| AUTH-02 | `GET /auth/sessions` | Token without session binding (legacy) | List own only | ✓ Filters by user_id |
| AUTH-03 | Refresh with revoked session | Reuse refresh after logout | 401 revoked | ✓ |
| AUTH-04 | Access JWT after session revoke | Use access until TTL | 401 on next request if session checked | ✓ BearerAuthWithDB |

---

## Product IDOR

| ID | Endpoint | Attack | Expected | Status |
| --- | --- | --- | --- | --- |
| PRD-01 | `GET /products/:id` | UserB reads UserA product UUID | 403/404 | ⚠ Partial — verify tenant filter |
| PRD-02 | `PUT /products/:id` | UserB modifies P1 | Denied | ⚠ Partial |
| PRD-03 | `DELETE /products/:id` | UserB deletes P1 | Denied | ⚠ Partial |
| PRD-04 | `POST /products` | UserB sets `tenantId` to other tenant | Ignored or denied | ⚠ Review body trust |
| PRD-05 | `POST /products/:id/ai/*` | AI ops on foreign product | Denied | ⚠ Partial |

---

## File IDOR

| ID | Endpoint | Attack | Expected | Status |
| --- | --- | --- | --- | --- |
| FIL-01 | `GET /files` | UserB lists all files | Only own/tenant | ⚠ Gap |
| FIL-02 | `DELETE /files/:id` | UserB deletes F1 | 404/403 | ⚠ Gap |
| FIL-03 | `/static/{objectKey}` | Guess object key path | No directory traversal | ✓ Path checks in upload |

---

## Order & Shop IDOR

| ID | Endpoint | Attack | Expected | Status |
| --- | --- | --- | --- | --- |
| ORD-01 | `GET /orders/:id` | UserB reads order on ungranted shop | 403 | ✓ EnsureStoreVisible |
| ORD-02 | `GET /orders` | UserB lists orders | Only Shop-B | ✓ ApplyStoreScope |
| ORD-03 | `PUT /orders/:id` | UserC (readonly) updates order | 403 | ✓ CanWriteOrders |
| SHP-01 | `GET /shops/:id` | UserB reads Shop-A | 403 | ✓ Store visibility |
| SHP-02 | `PUT /shops/:id/auth` | UserB updates Shop-A tokens | 403 | ✓ Operate permission + visibility |

---

## Collect IDOR

| ID | Endpoint | Attack | Expected | Status |
| --- | --- | --- | --- | --- |
| COL-01 | `GET /collect/tasks/:id` | UserB reads UserA task | Denied | ❌ Gap — no tenant scope |
| COL-02 | `POST /collect/tasks/:id/retry` | Retry foreign task | Denied | ❌ Gap |

---

## Settings & Secrets IDOR

| ID | Endpoint | Attack | Expected | Status |
| --- | --- | --- | --- | --- |
| SET-01 | `GET /settings` | Operator without settings.manage | 403 | ✓ Permission check |
| SET-02 | `PUT /settings` | Inject other tenant settings | Stays tenant 0 | ⚠ Single-tenant |
| SEC-01 | `POST /security/keys/rotation/start` | Operator calls rotation | 403 | ✓ admin only perm |

---

## Admin User IDOR

| ID | Endpoint | Attack | Expected | Status |
| --- | --- | --- | --- | --- |
| ADM-01 | `GET /admin/users/:id` | Operator reads user list | 403 | ✓ user.manage |
| ADM-02 | `PUT /admin/users/:id/store-permissions` | Grant self Shop-A | Denied or audited | ✓ + op log |

---

## Webhook IDOR

| ID | Endpoint | Attack | Expected | Status |
| --- | --- | --- | --- | --- |
| WH-01 | POST webhook wrong shopId | Event for shop not matching signature context | Reject / no write | ✓ Resolver + verifier |
| WH-02 | Replay same event id | Duplicate processing | Idempotent skip | ✓ Idempotency layer |

---

## Tenant Boundary (Multi-Tenant Future)

When `tenant_id > 0` fixtures exist:

| ID | Scenario | Expected |
| --- | --- | --- |
| TEN-01 | User T1 JWT accesses T2 product UUID | 403 TENANT_ACCESS_DENIED |
| TEN-02 | User T1 lists products | Only T1 rows |
| TEN-03 | Webhook shop in T2, forged T1 header | Tenant from shop, not header |

---

## Test Execution Notes

1. Record HTTP status, response `code`, and body message
2. Verify operation_logs entry for denied sensitive attempts (where applicable)
3. Do **not** use production credentials or real PII
4. Automate with integration tests in CI (deferred to P10)

---

## Priority Fixes from Matrix

1. **COL-*** — Add `tenant_id` to collect tasks + handler scoping
2. **FIL-*** — Tenant + owner filter on file list/delete
3. **PRD-*** — Mandatory `EnsureTenantMatch` on all product by-ID routes

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**
