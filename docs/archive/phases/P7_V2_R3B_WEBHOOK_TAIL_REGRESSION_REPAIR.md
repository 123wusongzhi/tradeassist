# P7-V2-R3B Webhook Tail Regression Repair

Status: **local repair verified; new formal pair still required**

## Failed Metrics

The frozen formal regression report has exactly two failed metrics:

| Metric | Aggregation | Baseline | Current | Absolute Delta | Relative Delta | Verdict |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| Webhook Ingestion | p95 | 19.3266438 | 23.091402000000002 | 3.764758200000003 | 0.19479627393971027 | failed_material_regression |
| Webhook Ingestion | p99 | 34.82735036999988 | 58.69811770000004 | 23.870767330000163 | 0.6854029111144307 | failed_material_regression |

`notComparableCount=0`, `invalidMetricCount=0`, `insufficientSampleCount=0`, and `summaryStatMissingCount=0`.

## Branch Mix

The frozen formal raw summaries do not contain branch path samples, random input seeds, or duplicate sequence hashes, so formal branch-mix comparability is **not proven** from frozen artifacts.

The non-formal diagnostic pair shows:

| Side | Normal Insert | Duplicate Conflict | Normal Ratio | Duplicate Ratio |
| --- | ---: | ---: | ---: | ---: |
| Baseline | 2511 | 0 | 1 | 0 |
| Current | 2510 | 0 | 1 | 0 |

This rules out a duplicate-conflict ratio increase in the diagnostic evidence, but it does not retroactively make the failed formal pair pass.

## Root Cause

`primaryRootCause=C_webhook_event_insert_or_idempotency_query_tail`

The selected minimal repair path is to remove the redundant successful normal-insert event reload, while preserving duplicate conflict reload with a fresh `Event` struct and explicit consistency-error handling.

## Verification

Local evidence records:

- `normalInsertQueryCount=1`
- `duplicatePathQueryCount=2`
- `idempotencySemanticsUnchanged=true`
- `transactionSemanticsUnchanged=true`
- `operationLogSemanticsUnchanged=true`
- `authRepairPreserved=true`
- `racePassed=true`
- `dataRaces=0`

No thresholds, SLOs, materiality floors, VUs, stages, request ratios, or dataset size were changed. Formal rerun has not started.

Machine-readable evidence: `docs/p7-v2-r3b-webhook-tail-regression-repair.json`.
