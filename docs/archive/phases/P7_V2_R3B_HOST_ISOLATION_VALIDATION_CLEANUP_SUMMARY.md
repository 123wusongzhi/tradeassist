# P7-V2-R3B Host Isolation V3 Validation Cleanup Summary

- Status: blocked_on_benchmark_environment_repeatability_with_cleanup_passed
- Summarized at: 2026-07-20T06:41:57Z
- Validation matrix: p7v2-diag-host-isolation-v3-validation-20260720054828
- Run order: B-C-C-B
- Completed slots: B1, C1
- Run count: 2 / 4
- Matrix status: invalid_incomplete
- Final gate status: failed
- Final gate failed count: 8
- Failed checks: validationMatrixRunCount, allFixedSlotsCompleted, allRunsIndependent, postgresProcessIdentityDistinct, postgresDataDirectoryDistinct, postgresPortDistinct, postgresWalDirectoryDistinct, validForFormalPlan
- Failed step: C2
- Failure: dataset post-build barrier failed
- Measurement started: false
- Valid for formal plan: false
- Next required action: Stop; require a dedicated benchmark host before any new formal plan/runtime freeze/formal pair.

## Validation Evidence

- Baseline self material regression count: 0
- Current self material regression count: 0
- Order position effect detected: false
- Later run degradation detected: false
- Host state mismatch count: 0
- Predictive host stability failure count: 0
- Quiet window failure count: 0
- Dataset barrier failure count: 0
- Binary/input/branch-mix binding: passed
- PostgreSQL isolation mode: dedicated_ephemeral_postgres_instance_per_run

## Completed Run Isolation

- B1: pid 1066, port 15433, data dir /tmp/tm-p7hi/6a562ee07a3c/B1/pgdata, WAL dir /tmp/tm-p7hi/6a562ee07a3c/B1/pgdata/pg_wal, cluster 6038119120bef37df90aaaf345c6cf1ffaa9a0e38f665480a1ba6a69cd1bee4f
- C1: pid 9524, port 15434, data dir /tmp/tm-p7hi/6a562ee07a3c/C1/pgdata, WAL dir /tmp/tm-p7hi/6a562ee07a3c/C1/pgdata/pg_wal, cluster f9db831d0fd7d9d34c771d40462edf2dc9e54a84081cca7db0d2e740f3ca0999

## Cleanup Evidence

- Cleanup scope: current V3 host-isolation validation dedicated resources only
- /tmp/tm-p7hi directory exists: false
- Current matrix ephemeral PostgreSQL directory count: 0
- Diagnostic database count: 0
- Diagnostic connection count: 0
- Listener 18080 count: 0
- Validation PostgreSQL listener count: 0
- Related process count: 0
- Destructive action count: 0
- Dropped databases: 0
- Action history preserved: true

## Forbidden Artifacts

No formal plan, runtime freeze, formal pair, fifth run, tag, push, or release was created.

## Bound

Do not create V4/V5 inside this phase. The minimum next action is a dedicated benchmark host and a fresh bounded checkpoint before any full fixed-order validation matrix.
