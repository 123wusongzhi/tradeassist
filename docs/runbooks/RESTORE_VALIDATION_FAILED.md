# Restore Validation Failed Runbook

Meaning: restore completed but integrity validation failed.

Impact: restored environment cannot be trusted.

Safety checks: keep validation report sanitized.

Triage steps: compare migration version, tenant count, shop count, RBAC rows, object inventory and audit chain.

Recovery steps: rerun validation; if still failing, choose another verified backup.

Forbidden actions: do not promote this restore to production readiness.

Rollback boundary: N/A.

Escalate when: multiple verified backups fail validation.

Verify: Tenant / Shop / RBAC / Audit checks pass.
