# P4.2 Secret Target Coverage

Re-encrypt target adapters for master key rotation (`enc:v2` key ring).

## Status Banner

**Two Secret Targets Registered** · **Tenant-Scoped Listing** · **NOT Production Ready**

---

## Adapter interface

`ReencryptTargetAdapter` in `backend/internal/modules/securitymod/secret_targets.go`:

| Method | Purpose |
| --- | --- |
| `Name()` | Target identifier |
| `CountByKeyID` | Count ciphertext rows still on old `kid` |
| `ListByKeyID` | Paginated cursor listing |
| `Reencrypt` | Decrypt with key ring → re-encrypt with active key |

---

## Registered targets

### 1. `settings_encrypted` (`SettingsSecretTarget`)

| Field | Detail |
| --- | --- |
| Table | `settings` |
| Filter | `is_encrypted = true` |
| Scope | Optional `tenant_id` when `ReencryptScope.TenantID > 0` |
| Columns | `item_value` (AES-GCM ciphertext with `kid` prefix) |

Covers: AI API keys, storage secrets, platform app secrets, webhook secrets, etc.

### 2. `shop_auth_tokens` (`ShopAuthTokenTarget`)

| Field | Detail |
| --- | --- |
| Table | `shop_auth_tokens` |
| Encrypted columns | `app_secret_enc`, `access_token_enc`, `refresh_token_enc` |
| Scope | When `TenantID > 0`, `shop_id IN (SELECT id FROM shops WHERE tenant_id = ?)` |
| Record ID | `{shop_token_uuid}:{field_name}` |

---

## Registry

```go
func AllReencryptTargets(db *gorm.DB, kr *crypto.KeyRing) []ReencryptTargetAdapter
```

Returns both adapters. Rotation progress API (`GET /security/keys/references`) aggregates counts per target.

---

## Not yet covered (backlog)

| Location | Notes |
| --- | --- |
| `admin_users.password_hash` | bcrypt — not key-ring encrypted |
| JWT signing keys | Env/config only |
| Refresh token hashes | One-way hash, not re-encrypted |
| `operation_logs` | No secret columns |

---

## API permissions

All rotation endpoints require `adminperm.PermSecurityKeyRotate` (admin role).
