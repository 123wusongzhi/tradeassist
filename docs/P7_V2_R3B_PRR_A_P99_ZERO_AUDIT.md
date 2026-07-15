# P7-V2-R3B PRR-A P99 Zero Audit

```json
{
  "phase": "P7-V2-R3B-PRR-A",
  "status": "completed",
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
}
```
