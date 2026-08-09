# P6 Backup Retention

Retention supports:

- daily
- weekly
- monthly
- manual hold
- legal hold as a data model only

Configured by:

- `BACKUP_RETENTION_DAILY`
- `BACKUP_RETENTION_WEEKLY`
- `BACKUP_RETENTION_MONTHLY`

Cleanup rules:

- Never delete running backups.
- Never delete backups referenced by restore validation.
- Manual hold prevents automatic deletion.
- Deletion must be audited and retryable.
- Do not use plain "created before X" as the only deletion rule.

