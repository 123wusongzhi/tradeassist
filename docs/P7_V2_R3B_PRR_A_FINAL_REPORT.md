# P7-V2-R3B PRR-A Final Report

```json
{
  "phase": "P7-V2-R3B-PRR-A",
  "status": "diagnosis_completed_execution_still_blocked",
  "artifacts": {
    "baselineRunId": "p7v2-baseline-r3b-recovery3-20260715-131400",
    "baselineSha256": "a04a39d4aa5e1cf8a951e9195af894e2a03df185c360e7652e4eacd7770aeaa9",
    "currentRunId": "p7v2-current-r3b-recovery3-20260715-131400",
    "currentSha256": "19aaa21b4094ee14147621b47b7370003b9a4dbaec12b24f64a32a66162af4c5",
    "integrityPassed": true,
    "modified": false
  },
  "regression": {
    "status": "failed",
    "p95FailedMetricCount": 3,
    "p99ZeroViolationCount": 9,
    "recalculated": false
  },
  "p95Audit": {
    "expected": 3,
    "audited": 3,
    "unclassified": 0,
    "results": [
      {
        "scenario": "Task List",
        "routeId": "GET /api/v1/task-center/failures?pageSize=20",
        "metric": "p95",
        "metricFamily": "latency",
        "direction": "lower_is_better",
        "unit": "ms",
        "baselinePresent": true,
        "baselineValue": 49.463726399999985,
        "baselineSampleCount": 2517,
        "currentPresent": true,
        "currentValue": 60.92983575,
        "currentSampleCount": 2506,
        "relativeThreshold": 0.1,
        "materialityFloor": {
          "type": "absolute_delta",
          "value": 2,
          "unit": "ms"
        },
        "absoluteSlo": true,
        "absoluteDelta": 11.466109350000018,
        "relativeDelta": 0.2318084419535367,
        "relativeExceeded": true,
        "materialityExceeded": true,
        "finalVerdict": "failed_material_regression",
        "reason": "failed_material_regression",
        "baseline": {
          "durationMetric": "p7_task_list_duration",
          "requestMetric": "p7_task_list_requests",
          "rawMetricPresent": true,
          "rawMetricPath": "metrics.p7_task_list_duration",
          "p50": 39.186513,
          "p90": 46.5005442,
          "p95": 49.463726399999985,
          "p99": null,
          "max": 605.430919,
          "avg": 41.20294366626933,
          "min": 34.122739,
          "requestCount": 2517,
          "throughput": 2.0681580632506837,
          "sampleCount": 2517
        },
        "current": {
          "durationMetric": "p7_task_list_duration",
          "requestMetric": "p7_task_list_requests",
          "rawMetricPresent": true,
          "rawMetricPath": "metrics.p7_task_list_duration",
          "p50": 41.5421975,
          "p90": 57.3636745,
          "p95": 60.92983575,
          "p99": null,
          "max": 118.835347,
          "avg": 45.15478018914609,
          "min": 34.7724,
          "requestCount": 2506,
          "throughput": 2.0394004451851444,
          "sampleCount": 2506
        },
        "distribution": {
          "broadlyShifted": true,
          "p95Only": false,
          "throughputDeclined": true,
          "comparablePercentiles": [
            "p50",
            "p90",
            "p95"
          ]
        },
        "classification": "statistical_variance_insufficient_evidence",
        "primaryRootCause": "statistical_variance_insufficient_evidence",
        "secondaryContributors": [
          "database_state_asymmetry",
          "environmental_variance"
        ],
        "confidence": "low",
        "supportingEvidence": [
          "Raw p95 changed from 49.463726399999985 ms to 60.92983575 ms.",
          "Request counts are 2517 and 2506; ratio=0.9956.",
          "Runtime/load/dataset/config fingerprints match, while the independent runs use different database identities.",
          "Distribution analysis: broadlyShifted=true; p95Only=false; throughputDeclined=true."
        ],
        "contradictingEvidence": [
          "No frozen query plan, slow-query trace, lock metric, connection-pool metric, or database statistics snapshot is available."
        ],
        "missingEvidence": [
          "EXPLAIN (ANALYZE, BUFFERS) from the original databases",
          "PostgreSQL lock/pool/auto-vacuum evidence",
          "steady-window-only samples",
          "host CPU, memory, disk, GC, goroutine, worker, and Redis telemetry"
        ],
        "recommendedAction": "Run a diagnostic Recovery4 with deterministic database-state preparation and capture plans, pool wait, locks, and process telemetry; do not alter runtime until that evidence exists."
      },
      {
        "scenario": "Webhook Ingestion",
        "routeId": "POST /api/v1/webhooks/internal-test/ping",
        "metric": "p95",
        "metricFamily": "latency",
        "direction": "lower_is_better",
        "unit": "ms",
        "baselinePresent": true,
        "baselineValue": 16.161770999999998,
        "baselineSampleCount": 2517,
        "currentPresent": true,
        "currentValue": 65.24907179999998,
        "currentSampleCount": 2505,
        "relativeThreshold": 0.1,
        "materialityFloor": {
          "type": "absolute_delta",
          "value": 2,
          "unit": "ms"
        },
        "absoluteSlo": true,
        "absoluteDelta": 49.08730079999998,
        "relativeDelta": 3.0372476382693447,
        "relativeExceeded": true,
        "materialityExceeded": true,
        "finalVerdict": "failed_material_regression",
        "reason": "failed_material_regression",
        "baseline": {
          "durationMetric": "p7_webhook_ingestion_duration",
          "requestMetric": "p7_webhook_ingestion_requests",
          "rawMetricPresent": true,
          "rawMetricPath": "metrics.p7_webhook_ingestion_duration",
          "p50": 12.365287,
          "p90": 15.1407296,
          "p95": 16.161770999999998,
          "p99": null,
          "max": 383.647828,
          "avg": 13.457343122367899,
          "min": 6.423312,
          "requestCount": 2517,
          "throughput": 2.0681580632506837,
          "sampleCount": 2517
        },
        "current": {
          "durationMetric": "p7_webhook_ingestion_duration",
          "requestMetric": "p7_webhook_ingestion_requests",
          "rawMetricPresent": true,
          "rawMetricPath": "metrics.p7_webhook_ingestion_duration",
          "p50": 13.989027,
          "p90": 52.5013432,
          "p95": 65.24907179999998,
          "p99": null,
          "max": 727.02577,
          "avg": 23.058940007584848,
          "min": 5.971259,
          "requestCount": 2505,
          "throughput": 2.0385866381439692,
          "sampleCount": 2505
        },
        "distribution": {
          "broadlyShifted": true,
          "p95Only": false,
          "throughputDeclined": true,
          "comparablePercentiles": [
            "p50",
            "p90",
            "p95"
          ]
        },
        "classification": "statistical_variance_insufficient_evidence",
        "primaryRootCause": "statistical_variance_insufficient_evidence",
        "secondaryContributors": [
          "database_state_asymmetry",
          "environmental_variance"
        ],
        "confidence": "low",
        "supportingEvidence": [
          "Raw p95 changed from 16.161770999999998 ms to 65.24907179999998 ms.",
          "Request counts are 2517 and 2505; ratio=0.9952.",
          "Runtime/load/dataset/config fingerprints match, while the independent runs use different database identities.",
          "Distribution analysis: broadlyShifted=true; p95Only=false; throughputDeclined=true."
        ],
        "contradictingEvidence": [
          "No frozen query plan, slow-query trace, lock metric, connection-pool metric, or database statistics snapshot is available."
        ],
        "missingEvidence": [
          "EXPLAIN (ANALYZE, BUFFERS) from the original databases",
          "PostgreSQL lock/pool/auto-vacuum evidence",
          "steady-window-only samples",
          "host CPU, memory, disk, GC, goroutine, worker, and Redis telemetry"
        ],
        "recommendedAction": "Run a diagnostic Recovery4 with deterministic database-state preparation and capture plans, pool wait, locks, and process telemetry; do not alter runtime until that evidence exists."
      },
      {
        "scenario": "Auth/Security",
        "routeId": "aggregated: POST /api/v1/auth/login invalid-login + POST /api/v1/webhooks/internal-test/ping invalid-signature",
        "metric": "p95",
        "metricFamily": "latency",
        "direction": "lower_is_better",
        "unit": "ms",
        "baselinePresent": true,
        "baselineValue": 6.64309025,
        "baselineSampleCount": 2376,
        "currentPresent": true,
        "currentValue": 33.26310325,
        "currentSampleCount": 2366,
        "relativeThreshold": 0.1,
        "materialityFloor": {
          "type": "absolute_delta",
          "value": 2,
          "unit": "ms"
        },
        "absoluteSlo": true,
        "absoluteDelta": 26.620013,
        "relativeDelta": 4.007173167638359,
        "relativeExceeded": true,
        "materialityExceeded": true,
        "finalVerdict": "failed_material_regression",
        "reason": "failed_material_regression",
        "baseline": {
          "durationMetric": "p7_auth_security_duration",
          "requestMetric": "p7_auth_security_requests",
          "rawMetricPresent": true,
          "rawMetricPath": "metrics.p7_auth_security_duration",
          "p50": 2.8080160000000003,
          "p90": 5.4750084999999995,
          "p95": 6.64309025,
          "p99": null,
          "max": 496.809155,
          "avg": 4.739357391835016,
          "min": 0.277938,
          "requestCount": 2376,
          "throughput": 1.9523017712688218,
          "sampleCount": 2376
        },
        "current": {
          "durationMetric": "p7_auth_security_duration",
          "requestMetric": "p7_auth_security_requests",
          "rawMetricPresent": true,
          "rawMetricPath": "metrics.p7_auth_security_duration",
          "p50": 2.9893205,
          "p90": 10.2759655,
          "p95": 33.26310325,
          "p99": null,
          "max": 207.478328,
          "avg": 6.958282500422652,
          "min": 0.314856,
          "requestCount": 2366,
          "throughput": 1.9254674594206114,
          "sampleCount": 2366
        },
        "distribution": {
          "broadlyShifted": true,
          "p95Only": false,
          "throughputDeclined": true,
          "comparablePercentiles": [
            "p50",
            "p90",
            "p95"
          ]
        },
        "classification": "metric_tag_aggregation_bug",
        "primaryRootCause": "metric_tag_aggregation_bug",
        "secondaryContributors": [
          "database_state_asymmetry"
        ],
        "confidence": "high",
        "supportingEvidence": [
          "tests/load/p7v2-baseline.js adds both POST /api/v1/auth/login invalid-login latency and POST /api/v1/webhooks/internal-test/ping invalid-signature latency to p7_auth_security_duration.",
          "Raw p95 changed from 6.64309025 ms to 33.26310325 ms.",
          "Request counts are 2376 and 2366; ratio=0.9958.",
          "Runtime/load/dataset/config fingerprints match, while the independent runs use different database identities."
        ],
        "contradictingEvidence": [
          "The aggregated p95 remains a real end-to-end timing distribution, so aggregation alone does not prove no runtime-state contribution."
        ],
        "missingEvidence": [
          "Route-specific percentile summaries for the two constituent routes",
          "EXPLAIN (ANALYZE, BUFFERS) from the original databases",
          "PostgreSQL lock/pool/auto-vacuum evidence",
          "steady-window-only samples",
          "host CPU, memory, disk, GC, goroutine, worker, and Redis telemetry"
        ],
        "recommendedAction": "Split Auth/Security into route-specific latency trends for invalid login and invalid webhook signature; retain aggregate only as a non-gating diagnostic metric, then rerun Recovery4 because metric collection changes."
      }
    ]
  },
  "p99ZeroAudit": {
    "expected": 9,
    "audited": 9,
    "unclassified": 0,
    "results": [
      {
        "scenario": "Product List",
        "metric": "p99",
        "rawMetricPath": "metrics.p7_product_list_duration; metrics.p7_product_list_duration",
        "rawMetricPresent": true,
        "rawCount": {
          "baseline": 5050,
          "current": 5022
        },
        "rawSamples": {
          "baseline": 5050,
          "current": 5022
        },
        "rawP95": {
          "baseline": 1.83321675,
          "current": 2.2349422
        },
        "rawP99": {
          "baseline": null,
          "current": null
        },
        "rawMax": {
          "baseline": 8.635688,
          "current": 18.236591
        },
        "parserInput": {
          "baseline": null,
          "current": null
        },
        "parserOutput": 0,
        "classification": "summary_stat_missing",
        "zeroIsReal": false,
        "missingWasConvertedToZero": true,
        "sampleCountSufficient": true,
        "policyRequirement": "p99 latency requires a present, non-zero summary statistic and at least 100 samples per side.",
        "recommendedFix": "Evaluator-only: preserve missing as null/not_comparable and emit summary_stat_missing; do not coerce absent p(99) to zero. A future harness remediation must ensure k6 exports p(99) for scenario trends."
      },
      {
        "scenario": "Order List",
        "metric": "p99",
        "rawMetricPath": "metrics.p7_order_list_duration; metrics.p7_order_list_duration",
        "rawMetricPresent": true,
        "rawCount": {
          "baseline": 5039,
          "current": 5018
        },
        "rawSamples": {
          "baseline": 5039,
          "current": 5018
        },
        "rawP95": {
          "baseline": 2.1656071,
          "current": 2.486808049999997
        },
        "rawP99": {
          "baseline": null,
          "current": null
        },
        "rawMax": {
          "baseline": 11.351183,
          "current": 29.492035
        },
        "parserInput": {
          "baseline": null,
          "current": null
        },
        "parserOutput": 0,
        "classification": "summary_stat_missing",
        "zeroIsReal": false,
        "missingWasConvertedToZero": true,
        "sampleCountSufficient": true,
        "policyRequirement": "p99 latency requires a present, non-zero summary statistic and at least 100 samples per side.",
        "recommendedFix": "Evaluator-only: preserve missing as null/not_comparable and emit summary_stat_missing; do not coerce absent p(99) to zero. A future harness remediation must ensure k6 exports p(99) for scenario trends."
      },
      {
        "scenario": "Inventory List",
        "metric": "p99",
        "rawMetricPath": "metrics.p7_inventory_list_duration; metrics.p7_inventory_list_duration",
        "rawMetricPresent": true,
        "rawCount": {
          "baseline": 2518,
          "current": 2508
        },
        "rawSamples": {
          "baseline": 2518,
          "current": 2508
        },
        "rawP95": {
          "baseline": 2.6609939000000002,
          "current": 3.02498295
        },
        "rawP99": {
          "baseline": null,
          "current": null
        },
        "rawMax": {
          "baseline": 432.78508,
          "current": 16.007952
        },
        "parserInput": {
          "baseline": null,
          "current": null
        },
        "parserOutput": 0,
        "classification": "summary_stat_missing",
        "zeroIsReal": false,
        "missingWasConvertedToZero": true,
        "sampleCountSufficient": true,
        "policyRequirement": "p99 latency requires a present, non-zero summary statistic and at least 100 samples per side.",
        "recommendedFix": "Evaluator-only: preserve missing as null/not_comparable and emit summary_stat_missing; do not coerce absent p(99) to zero. A future harness remediation must ensure k6 exports p(99) for scenario trends."
      },
      {
        "scenario": "Task List",
        "metric": "p99",
        "rawMetricPath": "metrics.p7_task_list_duration; metrics.p7_task_list_duration",
        "rawMetricPresent": true,
        "rawCount": {
          "baseline": 2517,
          "current": 2506
        },
        "rawSamples": {
          "baseline": 2517,
          "current": 2506
        },
        "rawP95": {
          "baseline": 49.463726399999985,
          "current": 60.92983575
        },
        "rawP99": {
          "baseline": null,
          "current": null
        },
        "rawMax": {
          "baseline": 605.430919,
          "current": 118.835347
        },
        "parserInput": {
          "baseline": null,
          "current": null
        },
        "parserOutput": 0,
        "classification": "summary_stat_missing",
        "zeroIsReal": false,
        "missingWasConvertedToZero": true,
        "sampleCountSufficient": true,
        "policyRequirement": "p99 latency requires a present, non-zero summary statistic and at least 100 samples per side.",
        "recommendedFix": "Evaluator-only: preserve missing as null/not_comparable and emit summary_stat_missing; do not coerce absent p(99) to zero. A future harness remediation must ensure k6 exports p(99) for scenario trends."
      },
      {
        "scenario": "Webhook Event List",
        "metric": "p99",
        "rawMetricPath": "metrics.p7_webhook_event_list_duration; metrics.p7_webhook_event_list_duration",
        "rawMetricPresent": true,
        "rawCount": {
          "baseline": 2518,
          "current": 2508
        },
        "rawSamples": {
          "baseline": 2518,
          "current": 2508
        },
        "rawP95": {
          "baseline": 1.7774906000000001,
          "current": 1.9382421500000004
        },
        "rawP99": {
          "baseline": null,
          "current": null
        },
        "rawMax": {
          "baseline": 11.199545,
          "current": 8.426164
        },
        "parserInput": {
          "baseline": null,
          "current": null
        },
        "parserOutput": 0,
        "classification": "summary_stat_missing",
        "zeroIsReal": false,
        "missingWasConvertedToZero": true,
        "sampleCountSufficient": true,
        "policyRequirement": "p99 latency requires a present, non-zero summary statistic and at least 100 samples per side.",
        "recommendedFix": "Evaluator-only: preserve missing as null/not_comparable and emit summary_stat_missing; do not coerce absent p(99) to zero. A future harness remediation must ensure k6 exports p(99) for scenario trends."
      },
      {
        "scenario": "Operation Log List",
        "metric": "p99",
        "rawMetricPath": "metrics.p7_operation_log_list_duration; metrics.p7_operation_log_list_duration",
        "rawMetricPresent": true,
        "rawCount": {
          "baseline": 2518,
          "current": 2507
        },
        "rawSamples": {
          "baseline": 2518,
          "current": 2507
        },
        "rawP95": {
          "baseline": 2.61021845,
          "current": 2.845904499999999
        },
        "rawP99": {
          "baseline": null,
          "current": null
        },
        "rawMax": {
          "baseline": 41.467707,
          "current": 728.821943
        },
        "parserInput": {
          "baseline": null,
          "current": null
        },
        "parserOutput": 0,
        "classification": "summary_stat_missing",
        "zeroIsReal": false,
        "missingWasConvertedToZero": true,
        "sampleCountSufficient": true,
        "policyRequirement": "p99 latency requires a present, non-zero summary statistic and at least 100 samples per side.",
        "recommendedFix": "Evaluator-only: preserve missing as null/not_comparable and emit summary_stat_missing; do not coerce absent p(99) to zero. A future harness remediation must ensure k6 exports p(99) for scenario trends."
      },
      {
        "scenario": "Webhook Ingestion",
        "metric": "p99",
        "rawMetricPath": "metrics.p7_webhook_ingestion_duration; metrics.p7_webhook_ingestion_duration",
        "rawMetricPresent": true,
        "rawCount": {
          "baseline": 2517,
          "current": 2505
        },
        "rawSamples": {
          "baseline": 2517,
          "current": 2505
        },
        "rawP95": {
          "baseline": 16.161770999999998,
          "current": 65.24907179999998
        },
        "rawP99": {
          "baseline": null,
          "current": null
        },
        "rawMax": {
          "baseline": 383.647828,
          "current": 727.02577
        },
        "parserInput": {
          "baseline": null,
          "current": null
        },
        "parserOutput": 0,
        "classification": "summary_stat_missing",
        "zeroIsReal": false,
        "missingWasConvertedToZero": true,
        "sampleCountSufficient": true,
        "policyRequirement": "p99 latency requires a present, non-zero summary statistic and at least 100 samples per side.",
        "recommendedFix": "Evaluator-only: preserve missing as null/not_comparable and emit summary_stat_missing; do not coerce absent p(99) to zero. A future harness remediation must ensure k6 exports p(99) for scenario trends."
      },
      {
        "scenario": "Provider Mock Flow",
        "metric": "p99",
        "rawMetricPath": "metrics.p7_provider_mock_flow_duration; metrics.p7_provider_mock_flow_duration",
        "rawMetricPresent": true,
        "rawCount": {
          "baseline": 2513,
          "current": 2500
        },
        "rawSamples": {
          "baseline": 2513,
          "current": 2500
        },
        "rawP95": {
          "baseline": 0.7729612,
          "current": 0.9230273
        },
        "rawP99": {
          "baseline": null,
          "current": null
        },
        "rawMax": {
          "baseline": 5.309032,
          "current": 11.290137
        },
        "parserInput": {
          "baseline": null,
          "current": null
        },
        "parserOutput": 0,
        "classification": "summary_stat_missing",
        "zeroIsReal": false,
        "missingWasConvertedToZero": true,
        "sampleCountSufficient": true,
        "policyRequirement": "p99 latency requires a present, non-zero summary statistic and at least 100 samples per side.",
        "recommendedFix": "Evaluator-only: preserve missing as null/not_comparable and emit summary_stat_missing; do not coerce absent p(99) to zero. A future harness remediation must ensure k6 exports p(99) for scenario trends."
      },
      {
        "scenario": "Auth/Security",
        "metric": "p99",
        "rawMetricPath": "metrics.p7_auth_security_duration; metrics.p7_auth_security_duration",
        "rawMetricPresent": true,
        "rawCount": {
          "baseline": 2376,
          "current": 2366
        },
        "rawSamples": {
          "baseline": 2376,
          "current": 2366
        },
        "rawP95": {
          "baseline": 6.64309025,
          "current": 33.26310325
        },
        "rawP99": {
          "baseline": null,
          "current": null
        },
        "rawMax": {
          "baseline": 496.809155,
          "current": 207.478328
        },
        "parserInput": {
          "baseline": null,
          "current": null
        },
        "parserOutput": 0,
        "classification": "summary_stat_missing",
        "zeroIsReal": false,
        "missingWasConvertedToZero": true,
        "sampleCountSufficient": true,
        "policyRequirement": "p99 latency requires a present, non-zero summary statistic and at least 100 samples per side.",
        "recommendedFix": "Evaluator-only: preserve missing as null/not_comparable and emit summary_stat_missing; do not coerce absent p(99) to zero. A future harness remediation must ensure k6 exports p(99) for scenario trends."
      }
    ]
  },
  "decision": {
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
  },
  "execution": {
    "runtimeModified": false,
    "loadScriptsModified": false,
    "metricCollectionModified": false,
    "regressionPolicyModified": false,
    "loadExecuted": false,
    "regressionRecalculated": false,
    "soakExecuted": false,
    "demoExecuted": false
  },
  "production": {
    "resourcesAccessed": false,
    "realProviderCalls": 0,
    "realDouyinWrites": 0,
    "tagCreated": false,
    "productionReady": false
  },
  "issues": [
    "P95 root cause remains unproven without Recovery4 diagnostic state telemetry; Runtime remediation is not authorized by current evidence."
  ]
}
```
