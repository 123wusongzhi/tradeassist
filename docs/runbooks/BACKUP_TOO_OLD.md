# Backup Too Old Runbook

Meaning: latest successful backup exceeds `BACKUP_MAX_AGE_HOURS`.

Impact: RPO draft target may not be met.

Safety checks: confirm environment and storage provider before acting.

Triage steps: check scheduler, last backup job, storage upload result and retention cleanup history.

Recovery steps: run a manual backup and verify it.

Forbidden actions: do not lower max age to silence the alert without owner approval.

Rollback boundary: N/A.

Escalate when: manual backup cannot pass verification.

Verify: `backup_last_success_timestamp` updates and backup age returns below threshold.
