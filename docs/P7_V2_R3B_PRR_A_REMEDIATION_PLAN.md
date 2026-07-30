# P7-V2-R3B PRR-A Remediation Plan

```json
{
  "phase": "P7-V2-R3B-PRR-A",
  "status": "completed",
  "nextPhase": "P7-V2-R3B-PRR-REPRO",
  "evaluatorOnlyRemediationRequired": true,
  "runtimeRemediationRequired": false,
  "harnessStateRemediationRequired": true,
  "diagnosticReproductionRequired": true,
  "recovery3RawArtifactsReusable": true,
  "recovery4Required": true,
  "rationale": "All nine p99 findings are evaluator extraction defects caused by missing p(99) summary values being converted to zero. The three p95 failures are genuine raw p95 differences, but the frozen evidence cannot distinguish database/execution-state asymmetry from runtime degradation. Auth/Security also aggregates two routes. Therefore evaluator-only recalculation may repair p99 semantics but cannot clear the p95 failures; a diagnostic Recovery4 is required before runtime remediation.",
  "recommendations": [
    {
      "targetArea": "evaluator parser",
      "targetFileOrModule": "scripts/p7-v2-lib.mjs metric()",
      "observedProblem": "Absent summary values are returned as 0.",
      "proposedChange": "Return null/undefined for missing statistics and require callers to classify missing latency summaries.",
      "expectedEffect": "Prevent missing p99 values from becoming valid-looking zero latency.",
      "risk": "Existing report formatting must handle null.",
      "validationMethod": "PRR-A fixtures plus evaluator-only recalculation of Recovery3 artifacts.",
      "requiresNewBaseline": false,
      "requiresNewCurrent": false
    },
    {
      "targetArea": "harness metric export",
      "targetFileOrModule": "tests/load/p7v2-baseline.js / k6 summary configuration",
      "observedProblem": "Scenario trend summaries contain p50/p90/p95/max but omit p99.",
      "proposedChange": "Capture scenario p99 explicitly and retain route-specific trend labels.",
      "expectedEffect": "Makes p99 latency semantically evaluable.",
      "risk": "Changes metric collection semantics.",
      "validationMethod": "Recovery4 diagnostic output must contain p(99) for all required trends.",
      "requiresNewBaseline": true,
      "requiresNewCurrent": true
    },
    {
      "targetArea": "execution-state harness",
      "targetFileOrModule": "P7-V2 environment/bootstrap scripts",
      "observedProblem": "Baseline and Current used different database identities without frozen stats/cache/pool/worker evidence.",
      "proposedChange": "Add deterministic state preparation and read-only state snapshots before each run.",
      "expectedEffect": "Separates execution-state variance from runtime changes.",
      "risk": "Longer setup and more evidence artifacts.",
      "validationMethod": "Diagnostic Recovery4 captures matching preparation outcomes, plans, locks, pool waits, and host telemetry.",
      "requiresNewBaseline": true,
      "requiresNewCurrent": true
    },
    {
      "targetArea": "Auth/Security metric tagging",
      "targetFileOrModule": "tests/load/p7v2-baseline.js securityNegativePhase",
      "observedProblem": "Invalid-login and invalid-webhook timing are merged into one gating trend.",
      "proposedChange": "Emit independent trends and evaluate each route separately; retain aggregate only diagnostically.",
      "expectedEffect": "Makes the p95 regression attributable to a specific path.",
      "risk": "Changes metric collection semantics.",
      "validationMethod": "Recovery4 has route-specific p50/p90/p95/p99 and status distributions.",
      "requiresNewBaseline": true,
      "requiresNewCurrent": true
    }
  ]
}
```
