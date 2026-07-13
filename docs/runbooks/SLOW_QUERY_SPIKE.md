# Slow Query Spike

Meaning: slow query ratio exceeds P7 baseline.

Check: recent SQL plans, missing indexes, changed filters, sort spill, count query cost and N+1 patterns.

Mitigate: reduce high-cost query rate, lower export concurrency and route users to cursor pagination where available.

Scale: add or adjust indexes only after real query-plan evidence.

Forbidden: do not run `EXPLAIN ANALYZE` against production load automatically.

Recovery: slow query ratio and DB p95 return to baseline.
