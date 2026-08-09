# Phase P4-R Demo Data Seed Root Cause

## Finding

`seed-demo-data.ps1` previously appended demo products and used time-based idempotency keys for some batches. When validation saw missing optional samples, it returned `exit 2`, which made demo acceptance treat repeatable or environment-dependent gaps as a hard failure.

## Fix

- Added `DEMO_DATASET_VERSION=p4-r-v1`.
- Added structured JSON result with `passed`, `passed_with_warning`, `environment_blocked`, and counters.
- Added production/staging guard: `DEMO_SEED_FORBIDDEN_IN_PRODUCTION`.
- Added stable `demo://p4-r-v1/product/...` source keys for newly created demo products.
- Reused existing product rows by exact demo title/source key before creating new rows.
- Replaced time-based batch idempotency keys with deterministic P4-R keys.

## Exit Codes

| Exit | Meaning |
| --- | --- |
| 0 | passed, updated, unchanged, or passed_with_warning |
| 1 | code_failed |
| 2 | environment_blocked |
| 3 | validation_conflict |
| 4 | manual_action_required |

## Remaining Risk

Some historical demo rows were created before stable source keys existed. The script safely reuses them by exact known demo title and writes stable keys for new rows.
