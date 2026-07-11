# P4 Refresh Token Rotation

Opaque refresh token rotation with reuse detection and token family compromise handling.

## Status Banner

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**

---

## Design Goals

1. Refresh tokens are **never stored in plaintext** — only `token_hash`
2. Each refresh **rotates** the token (one-time use for active tokens)
3. **Reuse** of a rotated token indicates theft → revoke entire family
4. Rotation runs in a **DB transaction** with row-level locks

Implementation: `backend/internal/modules/auth/session_service.go` (`RotateRefresh`).

---

## Token Lifecycle States

| Status | Meaning |
| --- | --- |
| `active` | Valid for one refresh exchange |
| `rotated` | Already used; presenting again triggers reuse detection |
| `revoked` | Explicit logout or admin revoke |
| `expired` | Past `expires_at` |
| `reuse_detected` | (Reserved label in constants) |
| `compromised` | Family revoked due to reuse |

Constants: `backend/internal/modules/auth/models.go`.

---

## Rotation Flow

```text
POST /api/v1/auth/refresh
  → Read refresh from HttpOnly cookie OR JSON body (legacy mode)
  → hash = HashToken(refreshRaw, JWT_SECRET)
  → BEGIN TRANSACTION
  → SELECT auth_refresh_tokens WHERE token_hash = ? FOR UPDATE
  → Switch status:
       active     → continue
       rotated    → revokeTokenFamily(compromised) → ErrRefreshTokenReused
       revoked/compromised → ErrRefreshTokenRevoked
  → Check expires_at
  → Lock auth_sessions row; verify status = active
  → Verify user not disabled; check token_version
  → Create new refresh row (same token_family_id, parent_token_id = old.id)
  → Mark old row: status=rotated, replaced_by_token_id=new.id
  → Update session last_activity_at, ip_hash, user_agent_summary
  → Mint new access JWT
  → COMMIT
  → Return new access + refresh (cookie or body)
```

---

## Token Family

On login, a new `token_family_id` (UUID) is created. All rotated tokens in a session share this family ID.

`revokeTokenFamilyTx` marks all `active` and `rotated` tokens in the family as `compromised` (or specified status) with `revoke_reason`.

This ensures a leaked old refresh token cannot be replayed after rotation without invalidating the attacker's chain.

---

## Hashing

```text
token_hash = HashToken(opaqueToken, JWT_SECRET)
```

Uses `authutil.HashToken` — server secret participates in hash so DB leak alone is insufficient.

Opaque token generation: `authutil.NewOpaqueToken(32)` (256 bits entropy).

---

## Reuse Detection Scenario

```text
Legitimate client:  R1 → R2 → R3 (normal rotation)
Attacker steals R2 after client moved to R3:
  Attacker presents R2 (status=rotated)
  → revokeTokenFamily → all tokens compromised
  → Legitimate R3 also fails on next refresh
  → User must re-login (acceptable tradeoff for theft detection)
```

Error returned: `ErrRefreshTokenReused`.

---

## Logout & Revoke

| Action | Method |
| --- | --- |
| Single logout | `RevokeByRefreshToken` → revoke session + active refresh tokens |
| Revoke session | `RevokeSession` → owner-only |
| Revoke others | `RevokeOtherSessions` |
| Admin revoke all | `RevokeAllUserSessions` |

---

## API Surface

| Endpoint | Auth | Notes |
| --- | --- | --- |
| `POST /api/v1/auth/refresh` | Public | Rate limit config: `AUTH_REFRESH_RATE_LIMIT` (reserved) |

Handler: `SessionHandler.Refresh` in `sessions_handler.go`.

---

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `AUTH_REFRESH_TOKEN_TTL_DAYS` | 7 | New token expiry on each rotation |
| `AUTH_SESSION_MODE` | dev: legacy | Cookie vs body transport |
| `JWT_SECRET` | - | Hash pepper for refresh tokens |

---

## Database Indexes

- `token_hash` — UNIQUE (lookup on refresh)
- `token_family_id` — family revoke
- `session_id` — session cascade revoke
- `expires_at` — cleanup jobs (future)

---

## Known Limitations

- Refresh rate limiting env var exists but dedicated middleware not fully wired
- No refresh token binding to device fingerprint beyond IP hash summary
- Family revoke is aggressive (by design); UX may require re-login after parallel tab refresh races

---

## Deferred Verification

- [ ] Concurrent refresh race test (two tabs, same token)
- [ ] Metrics on `compromised` family count
- [ ] Automated integration test for reuse path (`session_service_test.go` extends)

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**
