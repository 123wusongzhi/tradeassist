# P7-C2 Final Closure Report

Status: incomplete

Passed: 17
Failed: 17

## Failed Checks

- mandatory-partial-zero: mandatoryPartial=11
- product-pagination-passed: product=failed
- order-pagination-passed: order=failed
- inventory-pagination-passed: inventory=failed
- task-pagination-passed: task=failed
- webhook-pagination-passed: webhook=failed
- operationLog-pagination-passed: operationLog=failed
- cursor-tamper-rejected: tamper rejected must be true
- wrong-version-rejected: wrong version rejected must be true
- cross-tenant-rejected: cross tenant rejected must be true
- cross-shop-rejected: cross shop rejected must be true
- deep-offset-rejected: deep offset rejected must be true
- query-plan-passed: queryPlan=failed
- query-plan-no-large-seq-scan: unintendedLargeTableSeqScan=null
- query-plan-no-disk-spill: unresolvedDiskSpill=null
- nplusone-passed: nplusone=failed
- nplusone-no-linear-growth: linear query growth must be false
