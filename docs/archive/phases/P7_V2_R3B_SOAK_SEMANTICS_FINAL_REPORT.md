# P7-V2 R3B Soak Semantics Final Report

Status: incomplete

- Failed stage: Stage B
- Failed step: Formal Baseline dataset execution
- Failed command: `pnpm p7-v2:dataset`
- Exit code: 1
- Runtime freeze ID: `af5952c5d7f3858eb69beb631b212f03dc04e5cf7712a4e2d2b35d3103ac3c21`
- Runtime freeze still valid: true
- Baseline run ID: `p7v2-baseline-r3b-recovery6-20260715150913`
- Current run ID: `p7v2-current-r3b-recovery6-20260715150913`
- Soak run ID: `p7v2-soak-r3b-recovery6-20260715150913`

The Stage A semantics fix and scoped gate passed. Stage B stopped at dataset execution because the dataset report ended with `status=dry_run_passed`, `dryRun=true`, `insertedRows=0`, and `actualRows=0` instead of the required medium dataset execution.

Downstream baseline/current/comparability/regression/soak/demo/race/final gates were not executed.
