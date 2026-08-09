# P7-V2-R3B Refreeze Final Report

Status: **failed**

- Failed stage: `Stage H - Baseline Immediate Freeze`
- Failed step: `runtime freeze revalidation after baseline report, frozen artifact, and registry writes`
- Failed command: `pnpm p7-v2:r3b:runtime-freeze-revalidate -- --write`
- Exit code: `1`
- Runtime freeze ID: `057f7285831caac4f52d9bffec5559115954dda6f76407249fbf7d9b94b70d00`
- Baseline run ID: `p7v2-baseline-r3b-recovery6-20260715112749`
- Baseline artifact SHA-256: `5fe8002a2491c3a7bca09dee19386d120b807e6e4b8d0295de1461ed86c6e7a5`
- Current run ID not executed: `p7v2-current-r3b-recovery6-20260715112749`
- Dataset executed: `true`
- k6 executed: `true`
- Artifact created: `true`
- Registry active entry modified: `true`
- Runtime freeze still valid: `false`
- Revalidation error: `a planned Recovery6 manifest with unique run IDs is required before runtime freeze`
- 18080 listeners after stop: `0`
- Residual database: `trademind_p7v2_p7v2_baseline_r3b_recovery6_20260715112749`

Minimum repair action:

Make Runtime Freeze revalidation independent of mutable manifest status after formal execution, or explicitly allow `baseline_frozen` / `current_frozen` post-execution statuses while rebuilding only immutable freeze inputs. After repairing, create a new runtime freeze and new Recovery6 plan before any further formal run.

Downstream steps not executed: Current, Current Freeze, Comparability, Regression, Soak, Demo, Stability, Race, Final Gates.

No tag was created. Production Ready was not declared.
