# Disaster Recovery Runbook

Meaning: coordinated recovery workflow for major outage or data-loss event.

Impact: service restoration requires controlled operator actions.

Safety checks: identify environment, freeze writes if needed, preserve evidence, and never expose secrets.

Triage steps: determine whether application rollback, backup restore or PITR is required.

Recovery steps: prefer application rollback when schema is compatible; use verified backup/PITR only with high-risk approval.

Forbidden actions: no production restore without explicit approval; no unverified backup; no public backup links.

Rollback boundary: application rollback and database recovery are separate decisions.

Escalate when: RPO/RTO target cannot be met or tenant isolation is uncertain.

Verify: health checks, tenant isolation, RBAC, audit chain, object inventory and customer-facing smoke checks pass.
