# P7-V2-R3B Soak Failure Decision

Status: **blocked**

- Primary root cause: `soak_metric_schema_mismatch`
- Confidence: `high`
- Repair path: `C`
- Current pair reusable: false
- New runtime freeze required: true
- New baseline/current required: true/true

Minimum repair action: Repair soak load metric semantics so soak emits the formal route-level steady metrics, repair evaluator missing-metric classification and wrapper cooldown finalization, then run fixtures, create a new runtime freeze, and execute a full new Recovery6 baseline/current/comparability/regression/soak chain.
