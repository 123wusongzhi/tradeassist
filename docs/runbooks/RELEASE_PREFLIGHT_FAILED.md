# Release Preflight Failed Runbook

Meaning: release preflight rejected the candidate.

Impact: deployment must not continue.

Safety checks: confirm no traffic switch occurred.

Triage steps: inspect release root, artifact manifest, dependency manifest, config schema and health timeout.

Recovery steps: fix candidate or configuration; create a new release run.

Forbidden actions: do not skip preflight.

Rollback boundary: if no switch occurred, no rollback is needed.

Escalate when: candidate cannot pass preflight after fix.

Verify: preflight step passes in a new release run.
