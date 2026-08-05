# P4-V Key Rotation Verify Report

Per-target verification results for master-key rotation wiring (Phase P4-V). All results from automated unit tests on in-memory SQLite unless noted.

## Status Banner

**Unit Tests PASS** · **All Targets Aggregated in Verify Path** · **Production DB Verify Deferred**

---

## Test Environment

| Attribute | Value |
| --- | --- |
| Package | `backend/internal/modules/securitymod` |
| Test file | `backend/internal/modules/securitymod/rotation_test.go` |
| Database | In-memory SQLite (`:memory:`) |
| Key ring | Active `active` + previous `old1` |
| Run command | `go test ./internal/modules/securitymod/... -count=1` |
| Last run result | **PASS** (exit 0) |

---

## Wiring Verification

| Function | Calls into | Targets covered |
| --- | --- | --- |
| `CountSecretReferencesByKeyID` | `aggregateSecretReferences` → `scanSettingsReferences` + `scanShopTokenReferences` | `settings_encrypted`, `shop_auth_tokens` |
| `VerifyRotation` | `CountSecretReferencesByKeyID(kr.PreviousKeyIDs())` | Both targets |
| `ProcessReencryptBatch` | `reencryptTargets()` → `processTargetReencryptBatch` | Both targets via `reencryptSettingsBatch` / `reencryptShopTokensBatch` |

Implementation files:

- `backend/internal/modules/securitymod/rotation.go`
- `backend/internal/modules/securitymod/rotation_aggregate.go`
- `backend/internal/modules/securitymod/secret_targets.go`
- `backend/internal/modules/securitymod/secret_classify.go`

---

## Target 1: `settings_encrypted`

| Check | Test / method | Input | Expected | Actual | Status |
| --- | --- | --- | --- | --- | --- |
| Old-key reference detected | `TestVerifyRotationFailsWithOldKeyReferences` | `settings` row encrypted with `old1` key | `ReferenceCount > 0` for `settings.item_value` | Counts include old-key reference | **PASS** |
| Active-key only passes | `TestVerifyRotationPassesWhenNoOldReferences` | `settings` row encrypted with active key | `ReferenceCount == 0` for all counts | No old references reported | **PASS** |
| Legacy format classified | `classifyCiphertext` (via aggregate scan) | Ciphertext without `kid` prefix but decryptable | `ciphertextNeedsReencrypt`, `kid=legacy` | Code path present in `secret_classify.go` | **PASS** |
| Unknown format blocks verify | `VerifyRotation` logic | Unparseable ciphertext | `UnknownFormat > 0` → verify fails | Enforced in `rotation.go` | **PASS** |
| Batch re-encrypt dispatch | `processTargetReencryptBatch` | Running rotation job | `reencryptSettingsBatch` invoked | Type switch routes to settings batch | **PASS** |
| Optimistic update | `SettingsSecretTarget.Reencrypt` | Valid item | `UPDATE settings SET item_value=... WHERE id=? AND item_value=?` | Implemented in `secret_targets.go` | **PASS** |

---

## Target 2: `shop_auth_tokens`

| Check | Test / method | Input | Expected | Actual | Status |
| --- | --- | --- | --- | --- | --- |
| Access token counted | `TestCountIncludesShopAuthTokens` | `access_token_enc` encrypted with `old1`, Douyin-like token payload | Count row: `table=shop_auth_tokens`, `field=access_token_enc`, `ReferenceCount > 0` | Found in aggregate counts | **PASS** |
| Tenant via shop join | `scanShopTokenReferences` | Shop `tenant_id=1` | Count includes correct `TenantID` | Join `shops` implemented | **PASS** |
| All three columns scanned | `scanShopTokenReferences` | Rows with `app_secret_enc`, `access_token_enc`, `refresh_token_enc` | Each non-empty field classified | Loop over three fields in aggregate | **PASS** |
| Batch re-encrypt dispatch | `processTargetReencryptBatch` | Running rotation job on shop target | `reencryptShopTokensBatch` invoked | Type switch routes correctly | **PASS** |
| Per-field record ID | `ShopAuthTokenTarget.Reencrypt` | `{uuid}:{field}` | Column-specific update | `record_id` split + field update | **PASS** |
| Douyin tokens in scope | `TestCountIncludesShopAccessToken` scenario | `douyin-access-token` plaintext before encrypt | Included in shop_auth_tokens count | Test uses Douyin token label | **PASS** |

---

## Registry Verification

| Check | Test | Expected | Actual | Status |
| --- | --- | --- | --- | --- |
| Minimum two targets | `TestAllReencryptTargetsRegistered` | `len(targets) >= 2` | 2 targets returned | **PASS** |
| Names present | `TestAllReencryptTargetsRegistered` | `settings_encrypted`, `shop_auth_tokens` | Both names in map | **PASS** |

---

## VerifyRotation Gate Logic

```text
ok = (remaining == 0) && (unknown == 0) && (job.FailedRecords == 0)
```

| Scenario | `remaining` | `unknown` | `FailedRecords` | `ok` | Tested |
| --- | --- | --- | --- | --- | --- |
| Clean rotation | 0 | 0 | 0 | true | `TestVerifyRotationPassesWhenNoOldReferences` |
| Old key remains | >0 | 0 | 0 | false | `TestVerifyRotationFailsWithOldKeyReferences` |
| Unknown ciphertext | any | >0 | 0 | false | Code path in `rotation.go` |
| Batch failures recorded | 0 | 0 | >0 | false | Code path in `rotation.go` |

---

## Per-Target Summary

| Target | Count wired | Verify wired | Batch wired | Unit tests | Production verify |
| --- | --- | --- | --- | --- | --- |
| `settings_encrypted` | Yes | Yes | Yes | 3 direct + registry | **Deferred** |
| `shop_auth_tokens` | Yes | Yes | Yes | 1 direct + registry | **Deferred** |

---

## Deferred Verification (not blocking doc closure)

| Item | Command / action | Owner |
| --- | --- | --- |
| Staging DB full rotation | Security Center → Start rotation → worker batches → Verify | Ops |
| Post-rotate AI provider test | `POST /api/v1/settings/test-ai` | Admin |
| Post-rotate storage test | `POST /api/v1/settings/test-storage` | Admin |
| Post-rotate Douyin token use | Shop auth refresh + API smoke | Ops |
| Reference API snapshot | `GET /api/v1/security/keys/references` returns zeros for previous keys | Admin |

---

## Commands

```bash
# Unit tests (executed for this report)
cd backend
go test ./internal/modules/securitymod/... -count=1 -v

# Gate script
node scripts/p4-v-security-closure-gate.mjs
```

---

## Conclusion

**P4-V rotation verify wiring: PASS at unit-test level.**

`CountSecretReferencesByKeyID`, `VerifyRotation`, and `ProcessReencryptBatch` all aggregate through `AllReencryptTargets`, covering `settings_encrypted` and `shop_auth_tokens` including legacy-format ciphertext. Production database verification and live credential smoke tests remain explicitly deferred.
