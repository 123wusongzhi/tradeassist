# WAL Archive Interrupted Runbook

Meaning: WAL continuity check detected missing or invalid segment metadata.

Impact: PITR target window may be unavailable.

Safety checks: do not log WAL storage secrets or private object paths.

Triage steps: inspect archive command, segment timeline, checksum and retention.

Recovery steps: repair archive pipeline and confirm continuity from a new base backup.

Forbidden actions: do not claim PITR readiness until continuity is verified.

Rollback boundary: database recovery remains manual.

Escalate when: WAL archive gap overlaps required recovery window.

Verify: continuity check passes.
