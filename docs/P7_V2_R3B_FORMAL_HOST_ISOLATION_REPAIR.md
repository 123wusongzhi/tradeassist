# P7-V2-R3B Formal Host Isolation V2 Repair

Status: **passed**

- Formal host isolation version: `2`
- Lifecycle schema version: `2`
- Lifecycle step sequence hash: `6bc90c4ab3fd38a932a69cf7bb1054db014baaa8e5f88735ea96fe9ac52a5ba8`
- Primary root cause: `A_formal_harness_repeatability_or_order_bias_defect`
- Primary harness sub-root cause: `A6_multiple_harness_isolation_factors`
- Secondary harness sub-root causes: `A2_postgres_checkpoint_autovacuum_or_shared_instance_bias, A3_cache_and_warmup_asymmetry, A4_host_scheduler_cpu_or_io_contention, A5_baseline_current_lifecycle_asymmetry`
- Confidence: `medium`
- PostgreSQL isolation mode: `dedicated_ephemeral_postgres_instance_per_run`
- Formal pair started: false
- Validation matrix started: false

This repair records the host/order isolation contract and binding evidence. It does not modify Auth, Webhook, Operation Log, Security Audit, password verification, event insert, idempotency, business transactions, thresholds, SLOs, VUs, stages, duration, dataset size, input sequence, or branch mix.
