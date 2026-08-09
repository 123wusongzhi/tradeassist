# P6 Backup / Release / DR Audit

Phase P6 scope is code-level and isolated-environment readiness. It does not mark Production Ready, does not create tags, and does not execute real production restore or traffic switching.

| Capability | Current implementation | Data source | Entry | Permission | Encryption | Verification | Retention | Rollback | Risk | P6 result |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| PostgreSQL logical backup | `backup.Service` builds argv-only `pg_dump` commands | PostgreSQL configured in env | `/api/v1/ops/backups` | `backup.create` | AES-GCM chunked file encryption | SHA-256 + manifest + `pg_restore --list` foundation | daily / weekly / monthly + hold | N/A | real object storage deferred | code foundation ready |
| Backup manifest | `backup.Manifest` stored as JSON | backup job + artifact | backup job detail | `backup.read` | key id + wrapped data key only | manifest checksum | with backup | N/A | manifest must not include secrets | ready |
| Restore safety gate | `restore.Service.safetyGate` | verified backup record | `/api/v1/ops/restores` | `restore.execute` | decrypt only after gate | restore validation record | referenced backups protected | no automatic DB restore | isolated target required | ready |
| PITR foundation | `pkg/backupruntime` target-time and WAL continuity checks | WAL inventory metadata | docs + tests | ops | no secret output | fake WAL test | design only | manual | real PITR drill deferred | ready |
| Release state machine | `release.Service` | release run rows | `/api/v1/ops/releases` | `release.execute` | no secrets in manifest | preflight + steps | keep count config | application rollback only | no DB rollback | ready |
| DR drill status | `disasterrecovery.Service` | drill rows | `/api/v1/ops/dr/status` | `dr.read` / `dr.execute` | N/A | isolated drill report | N/A | manual runbook | real DR deferred | code foundation ready |
| Observability | P6 metrics added to shared catalog | Prometheus registry | `/internal/metrics` | internal guard | N/A | low-cardinality labels | N/A | alerts + runbooks | real backend deferred | ready |

Confirmed facts:

- Database: PostgreSQL by default, via `DB_DRIVER=postgres`.
- Migration: `RunMigrateWithLock` uses PostgreSQL advisory lock.
- Secret encryption: P4 `encrypt.Service` wraps per-backup data keys.
- Storage provider: existing provider layer remains source of truth; real backup object storage verification is deferred.
- systemd and Nginx templates live under `deploy/systemd/` and `deploy/nginx/`.
- Existing worker shutdown remains in `backend/cmd/server/main.go`; P6 release does not directly kill workers.

