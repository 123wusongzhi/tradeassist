# P7-V2-R3B Fast-Close R2 Final Report

Status: **incomplete**

- Failed step: formal baseline
- Run ID: `p7v2-baseline-r3b-recovery5-20260715091700`
- Command: `pnpm p7-v2:baseline -- --run-id p7v2-baseline-r3b-recovery5-20260715091700`
- Exit code: 1
- Failed field: `stages[0].targetVUs`
- Baseline artifact: not created
- Current: not started
- Regression, soak, demo, stability, race, cleanup, and final gates: not run

The minimum repair is to include an explicit `targetVUs` in every canonical load-profile stage. This changes harness semantics, so it requires a new runtime freeze and a new Recovery5 baseline/current pair.

Tag remains deferred. This is not Production Ready; final production acceptance remains deferred to P10.
# P7-V2-R3B Fast Close R2 Final Report

Status: **passed**

- Production Ready: false
- Tag deferred: true
