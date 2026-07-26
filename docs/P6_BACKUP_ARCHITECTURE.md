# P6 Backup Architecture

Backup chain:

`Backup Policy -> Backup Job -> PostgreSQL logical dump -> Encryption -> Checksum -> Manifest -> Storage -> Verification -> Retention -> Restore Drill`

Implementation:

- Module: `backend/internal/modules/backup`.
- Runtime helpers: `backend/internal/pkg/backupruntime`.
- Tables: `backup_jobs`, `backup_artifacts`, `backup_verifications`, `backup_retention_holds`, `backup_object_inventories`.
- API: `/api/v1/ops/backups`.
- Task types reserved: `backup_database`, `backup_object_inventory`, `backup_verify`, `backup_retention_cleanup`.

Safety rules:

- Use argv arrays, never shell string concatenation.
- Database password is passed through controlled process environment and never logged.
- Backup IDs are stable random IDs and must not be metric labels.
- Real production object storage verification remains Deferred in P6.

