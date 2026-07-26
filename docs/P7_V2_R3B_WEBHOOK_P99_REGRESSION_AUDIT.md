# P7-V2-R3B Webhook P99 Regression Audit

Status: **passed**

- Runtime freeze ID: `a39c1f26e709d612670525759d0d2badc9d54f6c508d98d0965b2a919a95d53b`
- Baseline run ID: `p7v2-baseline-r3b-recovery6-20260715153726`
- Current run ID: `p7v2-current-r3b-recovery6-20260715153726`
- Metric: `p7_webhook_ingestion_steady_duration`
- Baseline p99: 25.18280538000002 ms
- Current p99: 29.425643459999993 ms
- Absolute delta: 4.242838079999974 ms
- Relative delta: 0.16848154985026415
- Binding equal: true
- Primary root cause: `database_query_or_index_regression`
- Confidence: `medium`
- Repair path: `B`

## Evidence

- Formal pair integrity: passed
- Samples: baseline 2515, current 2514, minimum 100
- Distribution: p50 -0.17810900000000096 ms, p90 0.3238231000000038 ms, p95 0.4836825000000111 ms, p99 4.242838079999974 ms
- Runtime DB EXPLAIN: unavailable after required Stage 0 cleanup
- Minimum repair action: Skip the post-insert event reload when ON CONFLICT inserted a new row; keep the reload only for concurrent duplicate insert.

## Guardrails

Thresholds, materiality floors, SLOs, VUs, stages, and dataset size were not changed. No production resources, real providers, or real Douyin calls were used.
