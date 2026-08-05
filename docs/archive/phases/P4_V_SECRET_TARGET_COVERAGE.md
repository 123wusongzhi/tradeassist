# P4-V Secret Target Coverage

Full coverage matrix for master-key rotation (`enc:v2` key ring) secret targets registered in Phase P4-V.

## Status Banner

**Two Registered Targets** · **Settings + Shop Auth Tokens** · **Legacy Format Re-encrypt Eligible** · **NOT Production Rotation Executed**

---

## Adapter Architecture

| Component | Path | Role |
| --- | --- | --- |
| `ReencryptTargetAdapter` | `backend/internal/modules/securitymod/secret_targets.go` | Count / list / re-encrypt contract |
| `AllReencryptTargets` | `backend/internal/modules/securitymod/secret_targets.go` | Registry returning both adapters |
| `aggregateSecretReferences` | `backend/internal/modules/securitymod/rotation_aggregate.go` | Verify/count scan across targets |
| `classifyCiphertext` | `backend/internal/modules/securitymod/secret_classify.go` | Active / needs-reencrypt / legacy / unknown |
| Rotation service | `backend/internal/modules/securitymod/rotation.go` | `CountSecretReferencesByKeyID`, `VerifyRotation`, `ProcessReencryptBatch` |
| Re-encrypt worker | `backend/internal/modules/securitymod/reencrypt_worker.go` | Background batch driver |

---

## Target 1: `settings_encrypted` (`SettingsSecretTarget`)

| Attribute | Value |
| --- | --- |
| Adapter name | `settings_encrypted` |
| Table | `settings` |
| Column | `item_value` |
| Row filter | `is_encrypted = true` |
| Tenant scope | Optional `ReencryptScope.TenantID` on list/count |
| Record ID format | Numeric `settings.id` |
| Re-encrypt guard | `WHERE id = ? AND item_value = ?` optimistic update |

### Covered secret categories (by `group_key` / `item_key`)

| Category | `group_key` | Sensitive `item_key` examples | Encrypted | Rotation adapter |
| --- | --- | --- | --- | --- |
| AI Provider keys | `ai` | `openai_api_key`, `openai_compatible_api_key`, `deepseek_api_key`, `qwen_api_key`, `api_key` (legacy fallback) | `is_encrypted=true` | **Yes** |
| Image Provider keys | `image` | `removebg_api_key`, `openai_image_api_key`, `comfyui_api_key` | `is_encrypted=true` | **Yes** |
| Storage secrets | `storage` | `s3_secret_access_key`, `cos_secret_key`, `oss_access_key_secret`, `s3_access_key_id`, `cos_secret_id`, `oss_access_key_id` | `is_encrypted=true` | **Yes** |
| Mail SMTP | `mail` | `smtp_password` | `is_encrypted=true` | **Yes** |
| Alert / Webhook | `alert_notify` | `webhook_url`, `webhook_secret`, `feishu_webhook_url`, `feishu_secret`, `wecom_webhook_url` | `is_encrypted=true` | **Yes** |
| Platform app secrets | `platform_tiktok` | `app_secret` | `is_encrypted=true` | **Yes** |
| Platform app secrets | `platform_douyin_shop` | `app_secret` | `is_encrypted=true` | **Yes** |
| Platform app secrets | `platform_shopee` | `partner_key` / `app_secret` (schema `appSecret`) | `is_encrypted=true` | **Yes** |
| Platform app secrets | `platform_lazada` | `app_secret` | `is_encrypted=true` | **Yes** |
| Platform app secrets | `platform_amazon` | `client_secret`, `refresh_token` (if stored encrypted) | `is_encrypted=true` | **Yes** |
| Platform app secrets | `platform_aliexpress` | `app_secret` | `is_encrypted=true` | **Yes** |
| Platform app secrets | `platform_shopify` | `api_secret_key` / shared secret fields marked sensitive | `is_encrypted=true` | **Yes** |
| Platform app secrets | `platform_woocommerce` | `consumerSecret` | `is_encrypted=true` | **Yes** |
| Platform app secrets | `platform_ebay` | sensitive OAuth fields | `is_encrypted=true` | **Yes** |
| Shop-level platform override | `shop_platform_*` | Per-shop `appSecret` override when `Sensitive: true` | `is_encrypted=true` | **Yes** |

Schema sources:

- `backend/internal/modules/settings/integration_schema.go`
- `backend/internal/providers/platform/app_config_presets.go`
- `backend/internal/modules/shop/platform_app_settings.go`

### Explicitly NOT in this target

| Location | Reason |
| --- | --- |
| `settings` rows with `is_encrypted=false` | Non-secret configuration |
| `admin_users.password_hash` | bcrypt one-way hash |
| JWT signing keys | Environment / config only |
| Refresh token hashes | One-way hash, not key-ring encrypted |

---

## Target 2: `shop_auth_tokens` (`ShopAuthTokenTarget`)

| Attribute | Value |
| --- | --- |
| Adapter name | `shop_auth_tokens` |
| Table | `shop_auth_tokens` |
| Encrypted columns | `app_secret_enc`, `access_token_enc`, `refresh_token_enc` |
| Tenant resolution | `JOIN shops ON shops.id = shop_auth_tokens.shop_id` → `shops.tenant_id` |
| Tenant scope filter | `shop_id IN (SELECT id FROM shops WHERE tenant_id = ?)` when scoped |
| Record ID format | `{shop_token_uuid}:{field_name}` |
| Re-encrypt guard | `WHERE id = ? AND {field} = ?` per-column optimistic update |

### Covered token types

| Platform / use | Column | Notes | Rotation adapter |
| --- | --- | --- | --- |
| Douyin Shop OAuth | `access_token_enc`, `refresh_token_enc` | Includes Douyin access/refresh tokens | **Yes** |
| Douyin Shop app secret override | `app_secret_enc` | Per-shop secret when overridden | **Yes** |
| TikTok Shop OAuth | `access_token_enc`, `refresh_token_enc` | Platform token storage | **Yes** |
| TikTok app secret override | `app_secret_enc` | Per-shop override | **Yes** |
| Shopee / Lazada / other authorized shops | All three columns | Same table contract | **Yes** |

Provider references:

- `backend/internal/providers/platform/douyinshop/token.go`
- `backend/internal/modules/shop/service.go` (token persistence)

---

## Ciphertext Classification (all targets)

| Status | Condition | Rotation behavior |
| --- | --- | --- |
| `ciphertextActive` | `kid == kr.ActiveID` | Skipped during batch |
| `ciphertextNeedsReencrypt` | `kid` in `PreviousKeyIDs` **or** legacy decryptable without `kid` prefix | Counted + re-encrypted |
| `ciphertextUnknown` | Cannot parse `kid` and cannot decrypt | Counted as `UnknownFormat`; blocks `VerifyRotation` |
| `ciphertextEmpty` | Blank column | Skipped |

Legacy path: values decryptable by `KeyRing` but lacking `enc:v2` `kid` prefix are tagged `kid=legacy` and treated as **needs re-encrypt**.

---

## Registry

```go
// backend/internal/modules/securitymod/secret_targets.go
func AllReencryptTargets(db *gorm.DB, kr *crypto.KeyRing) []ReencryptTargetAdapter {
    return []ReencryptTargetAdapter{
        &SettingsSecretTarget{DB: db, KR: kr},
        &ShopAuthTokenTarget{DB: db, KR: kr},
    }
}
```

---

## API Surface

| Endpoint / method | Permission | Aggregates targets |
| --- | --- | --- |
| `GET /api/v1/security/keys/references` | `adminperm.PermSecurityKeyRotate` | Yes — via `CountSecretReferencesByKeyID` |
| `POST /api/v1/security/keys/rotation/:id/verify` | `adminperm.PermSecurityKeyRotate` | Yes — via `VerifyRotation` |
| Background re-encrypt worker | System context | Yes — via `ProcessReencryptBatch` |

Handler: `backend/internal/modules/securitymod/handler.go`

---

## Coverage Summary Matrix

| Secret type | Storage | Target adapter | Count | List | Reencrypt | Verify |
| --- | --- | --- | --- | --- | --- | --- |
| AI API keys | `settings.item_value` | `settings_encrypted` | Yes | Yes | Yes | Yes |
| Image API keys | `settings.item_value` | `settings_encrypted` | Yes | Yes | Yes | Yes |
| Storage credentials | `settings.item_value` | `settings_encrypted` | Yes | Yes | Yes | Yes |
| SMTP password | `settings.item_value` | `settings_encrypted` | Yes | Yes | Yes | Yes |
| Alert webhook URL/secret | `settings.item_value` | `settings_encrypted` | Yes | Yes | Yes | Yes |
| Platform `app_secret` (all platforms) | `settings.item_value` | `settings_encrypted` | Yes | Yes | Yes | Yes |
| Shop `app_secret_enc` | `shop_auth_tokens` | `shop_auth_tokens` | Yes | Yes | Yes | Yes |
| Shop `access_token_enc` | `shop_auth_tokens` | `shop_auth_tokens` | Yes | Yes | Yes | Yes |
| Shop `refresh_token_enc` | `shop_auth_tokens` | `shop_auth_tokens` | Yes | Yes | Yes | Yes |
| Legacy encrypt format | Either target | Both | Yes (as `legacy`) | Yes | Yes | Yes |
| Admin password hash | `admin_users` | — | No | No | No | N/A |
| JWT keys | Env | — | No | No | No | N/A |

---

## Backlog (post P4-V)

| Item | Priority | Notes |
| --- | --- | --- |
| Production rotation dry-run on staging DB | High | Deferred real-env verification |
| Douyin live credential verify after rotation | High | `pnpm demo:auto-acceptance` |
| Additional targets if new encrypted columns added | Medium | Register in `AllReencryptTargets` |

---

## Verification Commands

```bash
cd backend
go test ./internal/modules/securitymod/... -count=1
node ../scripts/p4-v-security-closure-gate.mjs
```

**Result:** Both targets registered and wired into count, verify, and batch re-encrypt paths. No secret values are logged or stored in this document.
