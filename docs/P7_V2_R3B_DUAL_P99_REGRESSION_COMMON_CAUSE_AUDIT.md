# P7-V2 R3B Dual p99 Regression Common Cause Audit

Machine-readable evidence: `docs/p7-v2-r3b-dual-p99-regression-common-cause-audit.json`

Status: **completed**

This is an audit-stage result only. It does not mark P7-V2, P7 closure, soak, demo, stability, race, cleanup, or final gates as passed.

## Pair

| Field | Value |
| --- | --- |
| Baseline run | `p7v2-baseline-r3b-recovery6-20260716082252` |
| Current run | `p7v2-current-r3b-recovery6-20260716082252` |
| Runtime freeze | `db08320b4e37b1cb11f1ad3182ad7ce09730906e7aaa912230f1c7b82279116f` |
| Baseline artifact SHA-256 | `6edf07eceea10be9b059aeb55cd4d5e679734bde955e6f56ceabced5af2f2512` |
| Current artifact SHA-256 | `7ed88fe13a9b556fd7b43ec0ef603c3699fd098eb59f1500d5696f03ba7c61e5` |
| Pair integrity | passed |
| Comparability | passed |
| Regression | failed, 2 failed metrics |

## Failure-State Cleanup

Cleanup used scoped P7-V2 resource ownership checks.

| Field | Value |
| --- | ---: |
| currentFormalResidualCount | 0 |
| unknownDatabaseCount | 0 |
| unknownProcessCount | 0 |
| listener18080Count | 0 |
| historicalFrozenArtifactDeletedCount | 0 |
| historicalEvidenceDeletedCount | 0 |
| unknownResourceDeletedCount | 0 |

The failed pair is retained as historical audit evidence and is not valid for regression rerun, soak, or closure.

## Distribution Audit

Frozen k6 artifacts contain aggregate summary metrics only. Request-level raw samples, timestamps, p25/p75/p97/p98/p99.5, and standard deviation are unavailable and were not fabricated.

| Metric | Baseline p50 | Baseline p95 | Baseline p99 | Current p50 | Current p95 | Current p99 | Verdict |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| Webhook Ingestion | 13.055998 | 17.7284625 | 27.7955604 | 12.895408 | 18.1907232 | 52.21840064 | failed_material_regression |
| Auth Invalid Login | 5.136534 | 7.73351665 | 16.88928389 | 5.221208 | 8.9023304 | 140.05195838 | failed_material_regression |

Tail morphology: `tail_only_spike` for both failed metrics. The medians and p95 values stayed close to baseline while p99 widened sharply.

## Runtime And Database Audit

The failed pair does not include enough frozen runtime evidence to prove a specific shared cause:

- no request-level tail timestamps;
- no DB pool counters or wait timing;
- no transaction commit timing;
- no `pg_stat_activity` / `pg_locks` snapshot during tail windows;
- no GC pause alignment data;
- no CPU saturation or run-queue data;
- no per-stage timing for webhook or auth invalid-login paths.

The audit can say the shared DB/runtime hypothesis is plausible, but it cannot truthfully prove it.

## Dependency Matrix

| Dependency | Webhook | Auth Invalid Login | Current abnormal evidence |
| --- | ---: | ---: | --- |
| PostgreSQL Pool | yes | yes | not recorded |
| Transaction Commit | yes | yes | not recorded |
| Audit Logging | no | yes | not recorded |
| File Logger | no | no | not recorded |
| Redis/Rate Limit | no | no | not used by failed pair path |
| CPU/GC | yes | yes | not recorded |
| Worker Queue | no | no | not used by synchronous failed pair metric |

## Root Cause

Primary root cause: `F_insufficient_evidence`

Confidence: `medium`

Secondary causes:

- `B_shared_database_pool_or_transaction_regression_possible`
- `D_auth_specific_hash_query_or_audit_regression_possible`
- `C_webhook_specific_query_or_lock_regression_less_likely_after_query_budget_fixture`

Recommended repair path: `F_low_cardinality_diagnostics_before_formal_rerun`

Minimum repair action: add fixed low-cardinality diagnostic timing for webhook ingestion stages, auth invalid-login stages, DB pool wait, transaction commit, and runtime snapshots. Do not write diagnostic output into the formal registry until a new checkpoint, runtime freeze, and formal pair are created.

## Required Next Formal Work

- New checkpoint: required
- New runtime freeze: required
- New formal pair: required
- Formal rerun: not started
- Soak/demo/stability/race/final gates: not executed

Phase P7-V2 remains incomplete. P7 Development Closure remains incomplete. Tag remains deferred. Not Production Ready.
