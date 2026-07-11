# P4 Security Audit Matrix

Phase P4 documents **code-level security foundations** implemented in TradeMind backend. This matrix maps modules to auth, permissions, tenant scope, risks, and remediation status.

## Status Banner

| Label | Meaning |
| --- | --- |
| **Security Foundation Implemented** | Controls exist in code, models, middleware, or services |
| **Real Environment Security Verification Deferred** | Staging/production penetration, load, and credential rotation drills not yet executed |
| **NOT Production Ready / NOT Penetration Test Passed** | Do not treat P4 as a production security sign-off |

---

## Module Audit Matrix

| Module | Entry Points | Auth | Permission | Tenant | Shop Scope | Risk (Residual) | Fix / Mitigation |
| --- | --- | --- | --- | --- | --- | --- | --- |
| **auth** | `POST /auth/login`, `/refresh`, `/logout`, session APIs | JWT access + refresh rotation; `LoginGuard` lockout | Public login; authed session mgmt | `tenant_id` on sessions/tokens | N/A | Credential stuffing, session fixation | Rate limit + lockout; secure cookie mode; reuse detection |
| **auth (JWT)** | Bearer middleware on `/api/v1/*` | HS256 + `kid`; session binding | N/A (transport) | Claims `tenant_id` | N/A | Key compromise, token theft | Key rotation grace; short access TTL; session revoke |
| **adminperm / RBAC** | All authed handlers | Principal from JWT + DB | Role matrix (`adminperm/matrix.go`) | Implicit via user | `StoreGrant` filtering | Over-privileged operator | Sensitive perms admin-only; shop scope enforced in order/product paths |
| **securitymod** | `/security/keys/*`, `/audit/integrity/*` | Bearer | `security.key.rotate`, `audit.read` | Tenant 0 default in verify | N/A | Mis-rotation, audit tampering | Confirm phrase on rotation; hash chain verify API |
| **settings** | `GET/PUT /settings`, test-* | Bearer | `settings.manage` | `settings.tenant_id` column | N/A | Secret leakage in logs/responses | Encrypted fields via KeyRing; masked API responses |
| **files** | `/files/upload`, `/files`, `/files/:id` | Bearer | Implicit authed | `files.tenant_id` | N/A | Malware upload, path traversal | Image decode validation; `security_status`; object key sanitize |
| **product** | `/products/*` | Bearer | `product.view/write`, AI apply | `products.tenant_id` | Partial | IDOR across tenants | Tenant column present; full query scoping incomplete on some paths |
| **collect** | `/collect/*`, `/collector/*` | Bearer | Task retry / implicit | **Gap**: collect tasks lack `tenant_id` | N/A | Cross-tenant task visibility | Add tenant column + scoped queries (future) |
| **shop** | `/shops/*`, OAuth callbacks | Bearer / public callback | `store.view/operate` | `shops.tenant_id` | OAuth state binding | Token theft on callback | Encrypted shop tokens; public callback routes isolated |
| **order** | `/orders/*` | Bearer | `order.view/operate` | `orders.tenant_id` | `ApplyStoreScope` | PII exposure | Default PII masking on detail; shop visibility check |
| **inventory** | `/inventory/*` | Bearer | `inventory.view/operate` | Inherited via shop/product | Store scope | Stale stock writes | Idempotency keys; shop-scoped queries |
| **operationlog** | `/operation-logs` | Bearer | `operationlog.view` | `operation_logs.tenant_id` | Optional `shop_id` | Log injection / tampering | Hash chain (`prev_hash`, `entry_hash`); immutable rows |
| **webhook** | Public `POST /webhooks/*` | Signature verifier | N/A | `webhook_events.tenant_id` via shop resolver | Shop binding | Replay, wrong tenant routing | Idempotency + clock skew; Douyin verifier |
| **aiproductimage / safedownload** | Internal image fetch | Worker context | N/A | Worker tenant ctx | N/A | SSRF to internal networks | `pkg/safedownload` blocks private IPs/metadata |
| **middleware** | Global on Gin engine | BearerAuthWithDB | Populates TenantContext | JWT + session validate | Partial in authorizer | CSRF on cookie mode | `CSRFProtection` + `SecurityHeaders` in `main.go` |
| **config** | Startup validation | N/A | N/A | N/A | N/A | Insecure prod config | `validateAuthSecurity()` blocks legacy session in staging/prod |

---

## Auth Session Tables

| Table | Purpose | Tenant Column | Hashing / Secrets |
| --- | --- | --- | --- |
| `auth_sessions` | Server-side session lifecycle | Yes | IP hashed (`IPHash`) |
| `auth_refresh_tokens` | Rotation lineage | Yes | Token stored as hash only |
| `auth_login_attempts` | Lockout counters | Yes | Account + account\|IP keys |
| `auth_reauth_tokens` | High-risk step-up (reserved) | Yes | Token hash; single-use flag |

Implementation: `backend/internal/modules/auth/models.go`, `session_service.go`, `login_guard.go`.

---

## Cross-Cutting Controls

| Control | Location | Status |
| --- | --- | --- |
| TenantContext | `pkg/security/tenant.go` | Implemented |
| AuthorizationService | `pkg/security/authorize.go` | Implemented |
| KeyRing enc:v2 | `pkg/crypto/keyring.go` | Implemented |
| PII masking utilities | `pkg/security/pii.go` | Implemented |
| Upload validation | `pkg/security/upload.go` | Implemented (also partial in `files/service.go`) |
| Audit hash chain | `operationlog/hash_chain.go` | Implemented |
| Config status P4 items | `configstatus/p4_status.go` | Implemented |

---

## Verification Checklist (Deferred)

- [ ] End-to-end refresh rotation under concurrent clients
- [ ] JWT key rotation with zero-downtime grace window in staging
- [ ] Master key rotation dry-run + re-encrypt batch job
- [ ] IDOR test suite across tenant boundaries (see `P4_IDOR_TEST_MATRIX.md`)
- [ ] OWASP ASVS L2 mapping for auth/session
- [ ] External penetration test report

---

## Related Documents

- [P4_API_PERMISSION_MATRIX.md](./P4_API_PERMISSION_MATRIX.md)
- [P4_TENANT_TABLE_MATRIX.md](./P4_TENANT_TABLE_MATRIX.md)
- [P4_AUTH_SESSION_SECURITY.md](./P4_AUTH_SESSION_SECURITY.md)

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**
