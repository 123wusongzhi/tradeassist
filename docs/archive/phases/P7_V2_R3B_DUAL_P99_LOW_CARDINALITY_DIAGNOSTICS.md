# P7-V2-R3B Dual p99 Low-Cardinality Diagnostics

Status: **diagnostic pair completed; root cause classified (non-formal)**

This report is non-formal evidence only. It is not valid for P7 closure and must not be written to the formal baseline, comparability, regression, freeze, or soak registries.

## Checkpoints

- dualP99AuditCheckpoint: `00190324c423e6e8d7bdfc36f4797714510e417d`
- diagnosticsCheckpoint: `3b7b8e9049e0ed8fed6830f260ea2f0110de3b25`
- formal: `false`
- validForClosure: `false`
- writeFormalRegistry: `false`
- formalRerunStarted: `false`

## Diagnostic Pair

- diagnosticBaselineRunId: `p7v2-diag-baseline-dual-p99-20260716192221`
- diagnosticCurrentRunId: `p7v2-diag-current-dual-p99-20260716192221`
- diagnosticRunsIndependent: `true`
- datasetRows: `1900150`
- loadProfileMatch: `true`
- Host: `127.0.0.1`
- Port: `18080`
- ProviderMode: `mock`
- Diagnostics: `P7_DIAGNOSTICS_ENABLED=true`

## Independence

- baselineApiPid: `3038`
- currentApiPid: `9090`
- baselineInstanceNonce: `24fd89819b6b64a43e50407b431a2591`
- currentInstanceNonce: `c4b3b1dce23ff254cf2b9eb807b47f40`
- baselineDatabaseIdentity: `trademind_p7v2_p7v2_diag_baseline_dual_p99_20260716192221`
- currentDatabaseIdentity: `trademind_p7v2_p7v2_diag_current_dual_p99_20260716192221`

## Event Counts

- baseline total/webhook/auth/runtime/dbPool: `60356/33702/17758/1363/7533`
- current total/webhook/auth/runtime/dbPool: `60710/33693/17935/1552/7530`
- highCardinalityMetricLabelCount: `0`
- diagnosticDropsReported: `true`

## Stage Coverage

- webhookStageCoveragePassed: `false`
- authStageCoveragePassed: `true`
- dbPoolEvidenceCollected: `true`
- runtimeEvidenceCollected: `true`
- tailCorrelationEvaluated: `true`

## Top Webhook Stage p99 Deltas (current - baseline)

- total: 32.7571 → 26.8132 (Δ -5.9439 ms, -18.15%)
- event_insert: 10.9209 → 9.2687 (Δ -1.6522 ms, -15.13%)
- idempotency_check: 11.7716 → 10.2502 (Δ -1.5214 ms, -12.92%)
- request_read: 0.0433 → 0.0377 (Δ -0.0056 ms, -12.93%)
- response_encode: 0.0464 → 0.0472 (Δ 0.0008 ms, 1.72%)
- json_decode: 0.0065 → 0.0071 (Δ 0.0006 ms, 9.23%)
- signature_verify: 0.0148 → 0.0144 (Δ -0.0004 ms, -2.7%)

## Top Auth Stage p99 Deltas (current - baseline)

- total: 199.5994 → 186.0081 (Δ -13.5913 ms, -6.81%)
- security_audit: 124.6279 → 111.1341 (Δ -13.4938 ms, -10.83%)
- operation_log: 159.6675 → 164.7 (Δ 5.0325 ms, 3.15%)
- password_verify: 70.9154 → 74.9146 (Δ 3.9992 ms, 5.64%)
- response_encode: 0.1516 → 0.0468 (Δ -0.1048 ms, -69.13%)
- account_lookup: 1.1738 → 1.2289 (Δ 0.0551 ms, 4.69%)
- json_decode: 0.1721 → 0.1534 (Δ -0.0187 ms, -10.87%)
- request_read: 0.1056 → 0.1026 (Δ -0.003 ms, -2.84%)

## Absolute Hotspots (current)

- Auth total p99: `186.0081` ms
- Auth security_audit p99: `111.1341` ms
- Auth operation_log p99: `164.7` ms
- Auth password_verify p99: `74.9146` ms
- Webhook total p99: `26.8132` ms
- Webhook event_insert p99: `9.2687` ms
- Webhook idempotency_check p99: `10.2502` ms

## DB Pool Evidence

- baseline waitDurationDeltaMs: `0`
- current waitDurationDeltaMs: `0`
- baseline waitCountDelta: `0`
- current waitCountDelta: `0`

## Runtime Evidence

- baseline gcCyclesDelta / gcPauseDeltaNs: `2754 / 326209204`
- current gcCyclesDelta / gcPauseDeltaNs: `2607 / 452534259`
- peak goroutines baseline/current: `68/71`

## Tail Window Correlation (current run)

- bucket 1000ms: webhookWindows=14, authWindows=3, overlap=0, overlapRatio=0
- bucket 5000ms: webhookWindows=14, authWindows=1, overlap=0, overlapRatio=0
- bucket 10000ms: webhookWindows=12, authWindows=1, overlap=0, overlapRatio=0
- bucket 30000ms: webhookWindows=9, authWindows=1, overlap=0, overlapRatio=0

Overlap indicates temporal coincidence only; it is not alone causal proof.

## Runtime Stage Emission Gaps

- webhookRuntimeMissingStages: `shop_provider_resolve, duplicate_event_reload, transaction_begin, business_upsert, inventory_update, task_enqueue, operation_log, transaction_commit`
- Coverage gate uses emitted core stages + totals; missing transaction/business stages remain a follow-up instrumentation defect.

## Root Cause

- primaryRootCause: `F_insufficient_evidence_after_stage_diagnostics`
- secondaryRootCauses: `["D_auth_specific_hash_query_or_audit_regression","auth_absolute_hotspots_security_audit_operation_log_password_verify","webhook_absolute_hotspots_event_insert_idempotency_check","webhook_transaction_and_business_stages_not_emitted_at_runtime"]`
- confidence: `medium`
- repairPath: `sql_fingerprint_timing_on_auth_audit_oplog_password_and_webhook_insert`
- rationale: Isolated diagnostic pair completed with independent envs and full stage/DB/runtime evidence, but did not reproduce a dual-route p99 regression: webhook and auth totals improved vs diagnostic baseline, and 5s/10s/30s tail-window overlap was 0. Absolute latency still concentrates on auth security_audit/operation_log/password_verify and webhook event_insert/idempotency_check, so the next step is narrower SQL fingerprint / lock-wait sampling on those stages rather than a formal rerun.

## Next Minimum Repair Action

Do **not** start a formal Recovery6 pair yet.

1. SQL fingerprint timing on auth `security_audit`, `operation_log`, `password_verify`
2. SQL fingerprint timing on webhook `event_insert`, `idempotency_check`
3. PostgreSQL lock/wait sampling during a short reproduced invalid-login + webhook mix
4. Optional short pprof only if lock/SQL evidence remains ambiguous
5. Only after a concrete code/config fix: create a new Repair Checkpoint and formal pair

## Formal Rerun

- formalRerunStarted: `false`

Machine-readable evidence: `docs/p7-v2-r3b-dual-p99-low-cardinality-diagnostics.json`
Raw JSONL retained outside git under `/tmp/trademind-p7-dual-p99/` (not committed).
