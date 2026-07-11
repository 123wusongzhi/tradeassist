# P4 Sensitive Data Classification

Data classification tiers, storage rules, and access controls for TradeMind Phase P4.

## Status Banner

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**

---

## Classification Tiers

| Tier | Label | Examples | Storage | Log | API Response |
| --- | --- | --- | --- | --- | --- |
| **T1** | Public | Product titles, public image URLs | Plaintext DB | Allowed | Full |
| **T2** | Internal | Task status, config non-secrets | Plaintext DB | Allowed (no secrets) | Full for authed users |
| **T3** | Confidential | Admin email/phone, order addresses | Plaintext DB; masked on read | Hashed/summary only | Masked default |
| **T4** | Secret | API keys, OAuth tokens, APP_MASTER_KEY | KeyRing `enc:v2` in settings | **Never** | Masked `sk-****abcd` |
| **T5** | Credential | Password hashes, refresh token hash | bcrypt / SHA hash | **Never** | Never returned |

---

## T4 Secrets Inventory

| Asset | Location | Encryption |
| --- | --- | --- |
| AI Provider API Key | `settings` (`ai` group) | `is_encrypted=true`, KeyRing |
| Storage secret key | `settings` (`storage`) | KeyRing |
| Platform app secret | `settings` (`platform_*`) | KeyRing |
| Shop access/refresh token | `shops` encrypted columns | Encrypter service |
| JWT signing secret | Env `JWT_ACTIVE_SECRET` | Env only (not DB) |
| APP master key | Env `APP_MASTER_ACTIVE_KEY` | Env only |

Settings model: `backend/internal/modules/settings/model.go`.

---

## T3 PII Fields

| Domain | Fields | Default Display |
| --- | --- | --- |
| Admin user | email, phone, display_name | Profile API full for self |
| Order | customer_phone, customer_email | Masked on detail (`maskDetailPII`) |
| Session | ip_hash, user_agent_summary | Hashed/truncated only |
| Operation log | username, ip_hash | Truncated UA; IP hashed |

Masking utilities: `backend/internal/pkg/security/pii.go`.

| Function | Output Example |
| --- | --- |
| `MaskPhone` | `138****5678` |
| `MaskEmail` | `u***@example.com` |
| `MaskName` | `李**` |
| `MaskAddress` | Province/city + `****` |
| `MaskIP` | `192.168.***.***` |

---

## PII Permission Keys

| Permission | Capability |
| --- | --- |
| `pii.read_masked` | Default for all roles (operator/readonly/admin) |
| `pii.read_full` | Admin only — full phone/email |
| `pii.export` | Admin only — bulk export (future) |

Source: `adminperm/matrix.go`.

Current enforcement: order detail masking is unconditional; full-read permission gating is incremental.

---

## Auth Tables — Sensitive Fields

| Table | Field | Classification | Notes |
| --- | --- | --- | --- |
| auth_refresh_tokens | token_hash | T5 | Never expose |
| auth_sessions | ip_hash | T3 derivative | One-way hash |
| auth_login_attempts | account_key | T3 | Lowercased account |
| auth_reauth_tokens | token_hash | T5 | Single-use |

---

## Files — Security Metadata

| Field | Values | Purpose |
| --- | --- | --- |
| `security_status` | `pending_scan` (default), future: `clean`, `blocked` | Antivirus pipeline hook |
| `scan_status` | `pending_scan` | Async scan job hook |
| `tenant_id` | int64 | Isolation |

Model: `backend/internal/modules/files/model.go`.

---

## Logging Rules (Mandatory)

**Never log:**

- Full API keys, JWT secrets, master keys
- Refresh/access tokens (raw)
- OAuth access/refresh tokens
- Passwords or password hashes
- Cookie values
- Full customer PII in webhook debug

Project rules: `.cursorrules`, `SECURITY.md`.

---

## AI & Task Audit

AI tasks record provider, model, prompt_code, token counts — **not** full prompts with embedded secrets in production logs.

Order AI context uses minimal PII subset comment in `order/service.go`.

---

## Retention (Policy — Not Automated)

| Data | Suggested Retention | P4 Status |
| --- | --- | --- |
| operation_logs | 365 days | Stored indefinitely |
| auth_login_attempts | 30 days | Manual purge TBD |
| auth_sessions revoked | 90 days | Manual purge TBD |
| webhook raw payload | Configurable | Stored in event table |

---

## Deferred Verification

- [ ] Data inventory scan across all models
- [ ] Enforce `pii.read_full` on order/admin APIs
- [ ] DLP review on export endpoints
- [ ] GDPR erasure procedure (not MVP)

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**
