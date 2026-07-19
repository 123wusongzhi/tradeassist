# P7-V2-R3B Host Isolation Validation Cleanup Summary

- Status: stopped_after_failed_final_gate_with_cleanup_passed
- Summarized at: 2026-07-19T07:48:42.264Z
- Validation matrix: p7v2-diag-host-isolation-validation-20260719061648
- Run order: B-C-C-B
- Run count: 4
- Matrix status: completed
- Final gate status: failed
- Failed checks: currentSelfMaterialRegressionCount, orderPositionEffectDetected, validForFormalPlan
- Valid for formal plan: false
- Next required action: Stop. Do not create a new formal plan until host isolation validation passes.

## Regression Evidence

- Baseline self material regression count: 0
- Current self material regression count: 2
- Order position effect detected: true
- Later run degradation detected: true

## Cleanup Evidence

- Cleanup preflight status: passed
- Runtime cleanup status: passed
- Runtime cleanup mode: check
- Runtime cleanup semantic gate passed: true
- Current formal residual count: 0
- Unknown database count: 0
- Unknown process count: 0
- Unknown connection count: 0
- Listener 18080 count: 0
- Destructive action count: 0
- Dropped databases: 0
- Action history preserved: true
- Check mode does not overwrite execute history: true
- Host guard status: passed
- Host guard failed count: 0

## Forbidden Artifacts

No formal plan, runtime freeze, formal pair, fifth run, tag, push, or release was created.
