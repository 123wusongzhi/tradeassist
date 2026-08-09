# P4.2 Secret Reencrypt Execution

Background worker and batch processing for master key rotation.

## Status Banner

**Reencrypt Worker Running** · **Batch Size 50** · **Real Rotation Dry-Run Recommended Before Prod**

---

## Components

| Piece | Location |
| --- | --- |
| Worker | `securitymod/reencrypt_worker.go` — `StartReencryptWorker` |
| Batch logic | `securitymod/rotation.go` — `ProcessReencryptBatch` |
| Targets | `securitymod/secret_targets.go` |
| Models | `key_rotation_jobs`, `key_rotation_item_failures` |
| Registry type | `worker.TypeSecuritySecretReencrypt` |

---

## Worker loop

```text
every 5s:
  SELECT key_rotation_jobs WHERE status=running AND dry_run=false LIMIT 5
  FOR EACH job:
    ProcessReencryptBatch(jobId, batchSize=50)
```

Context: `security.WorkerSystemContext(0, uuid.Nil, "security_secret_reencrypt")` — global system scope.

Started from `cmd/server/main.go` when `secSvc != nil`.

---

## Rotation lifecycle (API-driven)

1. `POST /security/keys/rotation/prepare` — confirm phrase `ROTATE-KEYS-DRY-RUN`
2. `POST /security/keys/rotation/start` — confirm phrase `ROTATE-KEYS-START`, creates `KeyRotationJob`
3. Worker processes batches until `processed_records >= total_records`
4. `POST /security/keys/rotation/:id/verify` — post-rotation verification
5. `GET /security/keys/references` — per-target reference counts by `kid`

Statuses: `prepared`, `running`, `paused`, `dry_run_completed`, `completed`, `failed`.

---

## Batch write safety

- Decrypt with `KeyRing` (supports previous keys)
- Re-encrypt with active key
- Update uses `WHERE id = ? AND item_value = ?` (optimistic on ciphertext)
- Failures recorded in `key_rotation_item_failures` with error codes

---

## Operational notes

1. Run **dry-run** first (`dry_run=true` job) and review `GET .../progress`
2. Pause via `POST .../pause` before maintenance window if needed
3. Monitor `security_reencrypt_batch_done` / `security_reencrypt_batch_failed` logs
4. No automatic rollback — keep previous key in `APP_MASTER_KEY_RING` until verify passes

---

## Deferred

- Per-tenant rotation jobs (current scope is `global`)
- Automated CI rotation test with real Postgres fixtures
