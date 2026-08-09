# P7 Performance Targets

All targets are `development_baseline` only. P10 must recalibrate against real pre-production.

| Area | Target draft | Evidence required |
| --- | --- | --- |
| Read API | p50/p95/p99, error rate, concurrency | `docs/p7-load-test-report.json` |
| Write API | p50/p95/p99, idempotency conflict rate, DB error rate | load report + app metrics |
| Webhook | ACK p95, persist p95, lag p95, burst capacity | webhook load scenario |
| Worker | throughput, task p95, queue age, retry ratio, lease lost ratio | worker baseline |
| Database | query p95, slow query ratio, pool wait, lock wait, tx p95 | query plan report |
| Provider | provider p95, timeout ratio, 429 ratio, adaptive slowdown | mock provider test |

Reports must include average, p95, p99 and error rate, and must keep warm-up separate from formal samples.
