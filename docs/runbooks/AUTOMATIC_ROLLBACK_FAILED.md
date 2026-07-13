# Automatic Rollback Failed Runbook

Meaning: application rollback did not complete after a failed release.

Impact: service may be degraded or unavailable.

Safety checks: avoid destructive database actions.

Triage steps: inspect current/previous link, Nginx config, systemd unit and release rollback row.

Recovery steps: manually restore previous app version and config.

Forbidden actions: do not restore database automatically.

Rollback boundary: application artifacts and routing only.

Escalate when: previous version cannot be started.

Verify: previous version is healthy and release state is `rolled_back`.
