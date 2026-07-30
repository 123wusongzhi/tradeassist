# Backup Verification Failed Runbook

Meaning: checksum, manifest, encryption or `pg_restore --list` validation failed.

Impact: backup must not be used for restore.

Safety checks: keep failed artifact for investigation if policy allows.

Triage steps: compare artifact checksum, manifest checksum, encryption metadata and PostgreSQL client version.

Recovery steps: rerun backup; verify the new backup.

Forbidden actions: do not mark failed verification as passed manually.

Rollback boundary: N/A.

Escalate when: repeated verification failure occurs for new backups.

Verify: new backup verification status is `passed`.
