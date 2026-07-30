# Restore Failed Runbook

Meaning: isolated restore job failed.

Impact: restore drill or recovery workflow is blocked.

Safety checks: confirm target is isolated and explicit.

Triage steps: inspect safety gate, backup verification, checksum, decrypt result and sanitized `pg_restore` summary.

Recovery steps: fix target DB or backup selection; rerun restore in an isolated environment.

Forbidden actions: do not retry against production; do not drop databases automatically.

Rollback boundary: failed restore does not imply application rollback.

Escalate when: verified backups cannot restore to a clean isolated database.

Verify: restore validation status is `passed`.
