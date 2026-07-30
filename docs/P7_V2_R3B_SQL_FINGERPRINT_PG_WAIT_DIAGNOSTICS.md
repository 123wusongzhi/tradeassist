# P7-V2-R3B SQL Fingerprint / PG Wait Diagnostics

Status: **diagnostic pair completed; root cause classified (non-formal)**

This report is non-formal evidence only. It is not valid for P7 closure and must not be written to formal registries.

## Checkpoints

- dualP99AuditCheckpoint: `00190324c423e6e8d7bdfc36f4797714510e417d`
- parentCheckpoint / diagnosticsCheckpoint: `3b7b8e9049e0ed8fed6830f260ea2f0110de3b25`
- formal: `false`
- validForClosure: `false`
- formalRerunStarted: `false`

## Diagnostic Pair

- diagnosticBaselineRunId: `p7v2-diag-baseline-sql-fingerprint-20260716160124`
- diagnosticCurrentRunId: `p7v2-diag-current-sql-fingerprint-20260716160124`
- diagnosticRunsIndependent: `true`
- datasetRows: `1900150`
- fingerprintsMatch: `true`

## Coverage

- authStageCoveragePassed: `true`
- webhookStageCoveragePassed: `true`
- sqlFingerprintCoveragePassed: `true`
- pgWaitEvidenceCollected: `true`
- dbPoolEvidenceCollected: `true`

## Totals (current)

- Auth total p50/p95/p99/max: `6.4616/117.0614/252.2447/1147.2929`
- Webhook total p50/p95/p99/max: `12.1002/54.7287/70.2292/716.2667`

## Root Cause

- primaryRootCause: `B_auth_audit_or_operation_log_db_tail`
- secondaryRootCauses: `["auth_operation_log_and_security_audit_sql_hotspot","webhook_event_insert_idempotency_hotspot","auth_password_verify_present","auth_transaction_commit_present","db_pool_wait_not_elevated","pg_blocked_backends_not_elevated"]`
- confidence: `high`
- repairPath: `auth_operation_log_hash_chain_or_commit_path_minimal_fix`
- hotspotSnapshot: `{"authTotalP99":252.2447,"pwdP99":64.7192,"auditP99":157.8808,"webhookInsertP99":45.112,"commitP99":111.1494,"poolWait":false,"pgBlocked":false}`

## Security

- credentialLeakCount: `0`
- rawSqlParameterLeakCount: `0`
- highCardinalityLabelCount: `0`

## Formal Rerun

- formalRerunStarted: `false`

Machine-readable evidence: `docs/p7-v2-r3b-sql-fingerprint-pg-wait-diagnostics.json`
Raw JSONL retained outside git under `/tmp/trademind-p7-sql-fingerprint/` (not committed).
