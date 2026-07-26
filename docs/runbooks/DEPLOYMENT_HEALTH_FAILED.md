# Deployment Health Failed Runbook

Meaning: candidate failed readiness or smoke checks.

Impact: traffic must remain on previous version or roll back to it.

Safety checks: confirm database recovery is not triggered.

Triage steps: inspect readiness endpoint, Nginx test, systemd status, release steps and recent logs.

Recovery steps: run application rollback; keep failed candidate for investigation.

Forbidden actions: do not switch traffic after failed health check.

Rollback boundary: application config and symlink only.

Escalate when: rollback also fails.

Verify: previous version passes readiness and smoke checks.
