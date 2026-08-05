# P7 Large Dataset Matrix

Status: matrix_defined, large_data_validation_not_executed.

| Dataset | Required dimensions | Current evidence |
| --- | --- | --- |
| Products | tenant/shop volume, filters, sort, index, p50/p95/p99 | pending |
| Orders | tenant/shop volume, filters, sort, index, p50/p95/p99 | pending |
| Inventory | tenant/shop volume, filters, sort, index, p50/p95/p99 | pending |
| Tasks | type/status/age, queue age, p50/p95/p99 | pending |
| Webhooks | platform/shop/status, burst and lag | pending |
| Operation logs | tenant/action/resource, audit query cost | pending |
| Files | security status, upload/export memory | pending |
| Backups/Releases | status/date list cost | pending |

`scripts/p7-generate-dataset.mjs` can produce a guarded dry-run plan. Medium data generation is not yet passed.
