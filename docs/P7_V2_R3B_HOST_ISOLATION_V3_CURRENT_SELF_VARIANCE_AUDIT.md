# P7-V2-R3B Host Isolation V3 Current Self Variance Audit

Status: **passed**

- Failed validation matrix: `p7v2-diag-host-isolation-validation-20260719061648`
- Current self failed metrics: `2`
- Binary binding passed: `true`
- Input binding passed: `true`
- Branch mix binding passed: `true`
- Lifecycle sequence match: `true`
- PostgreSQL isolation mode: `dedicated_ephemeral_postgres_instance_per_run`
- Quiet window predictive readiness passed: `false`
- Primary root cause: `V3_E_quiet_window_not_predictive_of_measurement_stability`
- Repair path: `predictive_host_stability_barrier`
- Business runtime change required: `false`
- Formal plan allowed: `false`

The V2 matrix is retained as failed diagnostic evidence. The observed quiet window passed before C1/C2 measurement, but it did not prove measurement-window stability and the matrix lacks tail-window time-series evidence. V3 therefore adds a predictive host stability barrier and richer PostgreSQL identity evidence before any fresh matrix run.
