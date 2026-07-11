# P4 JWT Key Rotation

JWT access token signing with `kid` header and graceful verification during key rotation.

## Status Banner

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**

---

## Overview

Access tokens are HS256 JWTs with:

- **`kid` header** — identifies signing key version
- **Active + previous key** — verify tokens signed before rotation during grace window
- **Session binding** — `session_id`, `tenant_id`, `token_version` in claims

Source: `backend/internal/modules/auth/jwt_access.go`.

---

## KeySet Structure

```go
type KeySet struct {
    ActiveID       string
    ActiveSecret   []byte
    PreviousID     string
    PreviousSecret []byte
    GraceUntil     time.Time
}
```

Built by `BuildKeySet(cfg)` from environment configuration.

---

## Configuration

| Variable | Purpose |
| --- | --- |
| `JWT_ACTIVE_KEY_ID` | Current `kid` (default `default`) |
| `JWT_ACTIVE_SECRET` | Current HMAC secret (falls back to `JWT_SECRET`) |
| `JWT_PREVIOUS_KEY_ID` | Previous `kid` during rotation |
| `JWT_PREVIOUS_SECRET` | Previous HMAC secret |
| `JWT_ROTATION_GRACE_MINUTES` | Accept previous key until this many minutes from process start (default 60) |

Legacy path: if `JWT_ACTIVE_SECRET` empty, uses root `JWT_SECRET`.

---

## Minting

`MintAccessToken(cfg, ks, input)`:

1. TTL from `cfg.AccessTokenTTL()` (default 15 minutes in secure mode)
2. Random `jti` via opaque token
3. Claims: `sub`, `tenant_id`, `session_id`, `token_version`, `typ=access`
4. Header: `kid = ks.ActiveID`
5. Sign with `ks.ActiveSecret`

---

## Verification

`ParseAccessToken(cfg, ks, tokenStr)`:

1. Read `kid` from JWT header (default to active if missing)
2. Select secret:
   - `kid == ActiveID` → active secret
   - `kid == PreviousID` → previous secret **if** `now <= GraceUntil`
   - Unknown kid → reject
3. Validate claims: non-empty `sub`, `typ == access`

Used by `BearerAuthWithDB` middleware on every authenticated request.

---

## Rotation Procedure (Operational)

### Phase 1 — Prepare

1. Generate new secret (`JWT_ACTIVE_SECRET` candidate)
2. Assign new `JWT_ACTIVE_KEY_ID` (e.g. `2026-07-v2`)
3. Keep current secret as `JWT_PREVIOUS_*`

### Phase 2 — Deploy

1. Set env:
   ```text
   JWT_PREVIOUS_KEY_ID=<old-id>
   JWT_PREVIOUS_SECRET=<old-secret>
   JWT_ACTIVE_KEY_ID=<new-id>
   JWT_ACTIVE_SECRET=<new-secret>
   JWT_ROTATION_GRACE_MINUTES=60
   ```
2. Restart all API instances (grace timer starts per process)
3. New logins receive tokens with new `kid`
4. Old tokens validate until grace expires

### Phase 3 — Complete

1. After grace + max access TTL, remove `JWT_PREVIOUS_*`
2. Redeploy
3. Old tokens fully invalid

---

## Interaction with Session Revocation

JWT validation alone is insufficient:

- `BearerAuthWithDB` also calls `ValidateSessionAccess` when `session_id` claim present
- `token_version` mismatch invalidates access token even with valid signature
- Session revoke immediately blocks refresh; access expires at TTL

---

## Legacy Mode

`LegacyMintToken` / `LegacyParseToken` support tokens without session binding (`session_id` empty, `tenant_id=0`) for `legacy_local_storage` dev mode.

**Not permitted in staging/production** per `validateAuthSecurity()`.

---

## Security Overview API

`GET /api/v1/security/overview` returns (non-secret):

- `jwtActiveKeyId`
- `accessTokenTTLMinutes`

Requires `settings.manage`.

---

## Threat Model

| Threat | Mitigation |
| --- | --- |
| Secret leak | Rotate active key; shorten access TTL |
| Stale tokens after rotation | Previous key + grace window |
| Algorithm confusion | Parser restricts to HS256 only |

---

## Limitations

- Symmetric HS256 only (no RS256/JWKS yet)
- Grace window is per-process start time, not cluster-coordinated
- No automated rotation scheduler; manual ops procedure
- Refresh token hash still uses `JWT_SECRET` — rotate refresh handling when rotating JWT secret

---

## Deferred Verification

- [ ] Blue/green rotation drill in staging
- [ ] Monitor 401 spike during rotation window
- [ ] Document coordination for multi-instance grace alignment

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**
