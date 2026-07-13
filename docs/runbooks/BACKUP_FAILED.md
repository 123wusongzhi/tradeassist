# Backup Failed Runbook

Meaning: a scheduled or manual backup did not complete.

Impact: latest recovery point may be older than expected.

Safety checks: do not expose DSN, object path, token, cookie or master key in tickets.

Triage steps: inspect backup job status, sanitized error summary, database availability, storage availability and command timeout.

Recovery steps: fix dependency, rerun backup, then run verification.

Forbidden actions: do not run restore from an unverified backup; do not copy production data locally.

Rollback boundary: no database rollback is implied.

Escalate when: two consecutive backup windows fail or backup age exceeds policy.

Verify: backup job completed, verification passed, alert recovered.
