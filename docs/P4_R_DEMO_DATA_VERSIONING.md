# Phase P4-R Demo Data Versioning

## Version

`DEMO_DATASET_VERSION=p4-r-v1`

## Stored Metadata

Seed reports now include:

- `demo_dataset_version`
- `demo_dataset_seeded_at`
- `demo_dataset_checksum`

## Business Keys

New P4-R product samples use `demo://p4-r-v1/product/<stable-key>` as `sourceUrl`. Repeat runs first reuse matching demo rows, then create only missing rows.

## Repeat Semantics

| Situation | Result |
| --- | --- |
| Latest demo data exists | `passed`, `unchanged`, exit 0 |
| Missing demo data created | `passed`, `created > 0`, exit 0 |
| Optional provider-dependent samples missing | `passed_with_warning`, exit 0 |
| Backend or credentials unavailable | `environment_blocked`, exit 2 |
| Unsafe structural conflict | `validation_conflict` or `manual_action_required` |
