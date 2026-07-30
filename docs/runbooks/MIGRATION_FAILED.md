# Migration Failed Runbook

Meaning: release migration failed.

Impact: release must stop before deployment continues.

Safety checks: confirm pre-release backup exists and database was not automatically restored.

Triage steps: inspect advisory lock, migration logs, compatibility policy and schema state.

Recovery steps: keep application on previous compatible version; plan manual database recovery only if needed.

Forbidden actions: do not run destructive down migrations by default.

Rollback boundary: application rollback first; database recovery is separate.

Escalate when: schema is incompatible with previous application version.

Verify: application serves previous version and migration status is understood.
