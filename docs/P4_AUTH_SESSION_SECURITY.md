# P4 Auth & Session Security

Server-side session model, login protection, and access token binding for TradeMind admin authentication.

## Status Banner

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**

---

## Overview

Phase P4 replaces long-lived JWT-only auth with:

1. **Short-lived access JWT** (session-bound, optional `kid`)
2. **Opaque refresh tokens** stored hashed, rotated on each use
3. **Server-side sessions** (`auth_sessions`) with revoke support
4. **Login guard** rate limiting and account lockout
5. **Configurable session modes** via `AUTH_SESSION_MODE`

---

## Session Modes

| Mode | Env Var | Refresh Storage | Access Storage | Production |
| --- | --- | --- | --- | --- |
| `secure_session` | Default / `AUTH_SESSION_MODE=secure_session` | HttpOnly cookie (`authcookie`) | Bearer header / memory | **Required** in staging/prod |
| `legacy_local_storage` | Explicit local compatibility opt-in | JSON response body | localStorage (frontend) | **Forbidden** in staging/prod |

Validation: `config.validateAuthSecurity()` rejects legacy mode when `APP_ENV` is staging or production.

Source: `backend/internal/config/auth_config.go`.

---

## Data Model

### auth_sessions

| Field | Security Note |
| --- | --- |
| `status` | `active`, `revoked`, `expired` |
| `ip_hash` | SHA-based IP fingerprint, not raw IP |
| `user_agent_summary` | Truncated UA string |
| `tenant_id` | Copied from admin user |
| `revoke_reason` | Audit trail for logout/admin revoke |

### auth_refresh_tokens

| Field | Security Note |
| --- | --- |
| `token_hash` | Unique; never store raw refresh token |
| `token_family_id` | Links rotation chain for reuse detection |
| `parent_token_id` / `replaced_by_token_id` | Lineage graph |
| `status` | See rotation doc |

### auth_login_attempts

Tracks failures per **account key** and **account|ip_hash** composite for dual-dimension lockout.

### auth_reauth_tokens

Short-lived hashed tokens for high-risk operations (e.g. key rotation confirm). Model exists; handler integration is incremental.

Source: `backend/internal/modules/auth/models.go`.

---

## Login Flow

```text
POST /api/v1/auth/login
  → LoginGuard.CheckAllowed(account, ip)
  → Verify password (bcrypt via admin.CheckPassword)
  → LoginGuard.ClearFailures on success
  → Transaction: create auth_sessions + auth_refresh_tokens (family)
  → MintAccessToken (tenant_id, session_id, token_version)
  → Set refresh cookie if secure_session
  → Return access token + expiry
```

Failure paths increment `auth_login_attempts` without distinguishing invalid user vs password (generic `ErrInvalidCredentials`).

Source: `backend/internal/modules/auth/session_service.go` (`CreateSession`).

---

## Login Guard

| Setting | Env Var | Default |
| --- | --- | --- |
| Max attempts | `AUTH_LOGIN_MAX_ATTEMPTS` | 5 |
| Window | `AUTH_LOGIN_WINDOW_MINUTES` | 15 |
| Lock duration | `AUTH_ACCOUNT_LOCK_MINUTES` | 30 |

Behavior:

- Counters reset if last failure outside window
- Lock applies when `failed_count >= max_attempts`
- Checks both normalized account and `account|ip_hash` keys
- Returns `ErrAccountTemporarilyLocked` when locked

Password policy: `IsWeakPassword()` rejects common passwords and enforces `AUTH_PASSWORD_MIN_LENGTH` (default 8). In production, bootstrap password cannot be reused.

Source: `backend/internal/modules/auth/login_guard.go`.

---

## Access Token Binding

Access JWT claims (`AccessClaims`):

| Claim | Purpose |
| --- | --- |
| `sub` | Admin user UUID |
| `tenant_id` | Tenant scope |
| `session_id` | Binds token to server session |
| `token_version` | Invalidates tokens on password reset |
| `typ` | Must be `access` |
| Header `kid` | Signing key version |

Middleware `BearerAuthWithDB`:

1. Parse JWT with active/previous keys
2. If `session_id` present → `ValidateSessionAccess`
3. Populate `TenantContext`

Source: `backend/internal/modules/auth/jwt_access.go`, `backend/internal/middleware/jwt.go`.

---

## Session Management API

| Endpoint | Action |
| --- | --- |
| `GET /auth/sessions` | List up to 50 sessions for current user |
| `DELETE /auth/sessions/:id` | Revoke one session (owner only) |
| `POST /auth/sessions/revoke-others` | Revoke all except current |
| `POST /auth/logout-all` | Revoke all sessions + clear cookie |
| `POST /auth/logout` | Revoke by refresh token |
| `GET /security/overview` | Config summary (`settings.manage`) |

Revoke cascades: session → active refresh tokens marked `revoked`.

Operation log actions: `session_revoke`, `session_revoke_others`, `logout_all`.

Source: `backend/internal/modules/auth/sessions_handler.go`.

---

## Cookie Security (secure_session)

When `AUTH_SECURE_COOKIE=true` (default in staging/prod):

- HttpOnly refresh cookie
- Secure flag in production
- SameSite from `AUTH_COOKIE_SAME_SITE` (default `Lax`)
- Optional `AUTH_COOKIE_DOMAIN`

CSRF: write requests with cookies require matching `Origin`/`Referer` against `ADMIN_PUBLIC_URL` / `API_PUBLIC_URL`. Local-only profiles (`development`, `demo`, `performance`, and `test`) additionally accept origins that parse to an actual loopback host (`localhost`, `127.0.0.0/8`, or `::1`) so the secure-session default remains usable for local Admin access; staging, production, and unknown profiles never receive this fallback.

Source: `backend/internal/pkg/security/headers.go`, `backend/internal/pkg/authcookie/`.

---

## Configuration Reference

| Variable | Purpose |
| --- | --- |
| `AUTH_SESSION_MODE` | `secure_session` \| `legacy_local_storage` |
| `AUTH_ACCESS_TOKEN_TTL_MINUTES` | Access JWT lifetime (default 15) |
| `AUTH_REFRESH_TOKEN_TTL_DAYS` | Refresh lifetime (default 7) |
| `AUTH_SECURE_COOKIE` | Cookie Secure flag |
| `AUTH_LOGIN_MAX_ATTEMPTS` | Lockout threshold |
| `AUTH_PASSWORD_MIN_LENGTH` | Minimum password length |
| `JWT_ACTIVE_KEY_ID` / `JWT_ACTIVE_SECRET` | Signing key (see JWT rotation doc) |

Loader: `loadAuthConfig()` in `auth_config.go`.

---

## Threat Model (Implemented Mitigations)

| Threat | Mitigation |
| --- | --- |
| Stolen refresh token | Rotation + reuse detection revokes family |
| Stolen access token | Short TTL; session revoke; token_version bump |
| Brute force login | LoginGuard lockout |
| Session fixation | New session + family on each login |
| Disabled user access | Status check on login and refresh |

---

## Deferred Verification

- Load test concurrent refresh from multiple devices
- Cookie attribute audit in real browser matrix
- Reauth token enforcement on all sensitive handlers
- Automated session fixation regression tests

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**
