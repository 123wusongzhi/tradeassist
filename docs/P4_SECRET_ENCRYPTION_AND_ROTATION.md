# P4 Secret Encryption & Key Rotation

APP_MASTER_KEY KeyRing (`enc:v2`), settings encryption, and rotation APIs.

## Status Banner

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**

---

## KeyRing enc:v2 Format

Ciphertext string format:

```text
enc:v2:{keyId}:{nonce_b64url}:{ciphertext_b64url}
```

- **Algorithm:** AES-256-GCM
- **Key material:** 32 bytes from hex, base64, or SHA-256 stretch of passphrase
- **Version ID:** `APP_MASTER_ACTIVE_KEY_ID` (default `default`)

Source: `backend/internal/pkg/crypto/keyring.go`.

---

## KeyRing Structure

```go
type KeyRing struct {
    ActiveID     string
    ActiveKey    []byte
    PreviousKeys map[string][]byte  // from JSON env
}
```

### Environment Variables

| Variable | Purpose |
| --- | --- |
| `APP_MASTER_KEY` | Legacy single key (fallback) |
| `APP_MASTER_ACTIVE_KEY_ID` | Active key identifier |
| `APP_MASTER_ACTIVE_KEY` | Active 32-byte material |
| `APP_MASTER_PREVIOUS_KEYS` | JSON map `{"old-id":"hexkey",...}` |

Production validation requires master key when `APP_ENV=production` (`validateAuthSecurity`).

Loader merges `APP_MASTER_ACTIVE_KEY` with root `MasterKey` in securitymod service.

---

## Encrypt / Decrypt

```go
kr, _ := crypto.NewKeyRing(activeID, activeKey, previousJSON)
cipher, _ := kr.Encrypt(plaintext)
plain, _ := kr.Decrypt(cipher)
```

- Empty plaintext → empty ciphertext
- Legacy format (base64 nonce+ciphertext without prefix) decrypts with active key only
- Unknown `keyId` in ciphertext → error

Helper: `crypto.IsEncrypted(value)` checks `enc:v2:` prefix.

---

## Settings Integration

Rows with `is_encrypted=true` in `settings` table store KeyRing ciphertext in `item_value`.

Affected groups (non-exhaustive):

- `ai` — API keys
- `storage` — secret keys
- `platform_*` — app secrets
- Email SMTP passwords

Frontend displays masked values; PUT with mask placeholder preserves existing secret.

Legacy `encrypt.Service` still exists; KeyRing is preferred path for new enc:v2 values.

---

## JWT vs Master Key Separation

| Secret | Purpose | Rotation doc |
| --- | --- | --- |
| `JWT_ACTIVE_SECRET` | Access token HMAC | [P4_JWT_KEY_ROTATION.md](./P4_JWT_KEY_ROTATION.md) |
| `APP_MASTER_ACTIVE_KEY` | Settings/shop secret encryption | This document |

Refresh token hashing uses `JWT_SECRET` — coordinate JWT secret rotation with session invalidation.

---

## Rotation API

Module: `backend/internal/modules/securitymod/`

| Endpoint | Permission | Action |
| --- | --- | --- |
| `POST /security/keys/rotation/prepare` | `security.key.rotate` | Dry-run: count encrypted settings rows |
| `GET /security/keys/rotation/status` | `security.key.rotate` | Same as prepare |
| `POST /security/keys/rotation/start` | `security.key.rotate` | Requires confirm phrase |

Confirm phrase: `ROTATE-KEYS-DRY-RUN` (reauth placeholder).

Response fields:

- `activeKeyId`
- `pendingReencrypt` — count of `settings WHERE is_encrypted=true`
- `previousKeyCount`

**Note:** P4 implements preparation/status only; batch re-encrypt job is operational follow-up.

---

## Rotation Runbook (Target State)

1. **Prepare:** Call rotation prepare API; note `pendingReencrypt`
2. **Deploy new key:** Set `APP_MASTER_PREVIOUS_KEYS` with old id/material; set new active
3. **Re-encrypt job:** Decrypt with old id, encrypt with new active (job TBD)
4. **Verify:** Sample decrypt settings test-* endpoints
5. **Remove previous keys** from env after all rows migrated

---

## Startup Validation

| Check | Error Code |
| --- | --- |
| Production without master key | `KEYRING_CONFIGURATION_INVALID` |
| Invalid previous keys JSON | Startup error from `NewKeyRing` |

---

## Threat Model

| Threat | Mitigation |
| --- | --- |
| DB leak | Ciphertext + external master key |
| Key compromise | Rotation + re-encrypt |
| Version confusion | Explicit keyId in ciphertext prefix |

---

## Deferred Verification

- [ ] Implement and test batch re-encrypt worker
- [ ] Verify legacy → enc:v2 migration on read/write
- [ ] HSM/KMS integration evaluation (post-MVP)

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**
