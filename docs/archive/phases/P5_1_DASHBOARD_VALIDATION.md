# P5.1 Dashboard Validation

Added:

- `deploy/observability/dashboards/alerts-and-slo.json`

The dashboard references real metric names from the catalog:

- `telemetry_export_failures_total`
- `telemetry_dropped_items_total`
- `slo_compliance_ratio`
- `slo_error_budget_remaining_ratio`
- `slo_burn_rate`
- `db_connections_open`
- `db_connections_in_use`

No datasource secret or production endpoint is embedded.
