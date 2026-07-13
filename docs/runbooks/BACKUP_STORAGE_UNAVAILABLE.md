# Backup Storage Unavailable Runbook

Meaning: backup storage upload, read or metadata check failed.

Impact: local backup may exist without durable copy.

Safety checks: do not generate public URLs or paste private object keys.

Triage steps: check provider configuration, bucket privacy, prefix separation and credentials in settings.

Recovery steps: restore storage access, upload again, verify size and checksum.

Forbidden actions: do not switch production to local-only backup.

Rollback boundary: N/A.

Escalate when: storage is unavailable for more than one backup window.

Verify: upload succeeds and object metadata matches manifest.
