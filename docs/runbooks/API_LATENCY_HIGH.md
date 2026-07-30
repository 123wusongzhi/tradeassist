# API Latency High

Meaning: API p95 or p99 exceeds the P7 development baseline.

Check: error rate, DB wait, slow queries, rate-limit rejection, provider latency and recent deploys.

Mitigate: enable conservative rate limits, pause low-priority exports, reduce worker prefetch and route heavy traffic away from degraded paths.

Scale: add API capacity only after DB pool and downstream saturation are ruled out.

Forbidden: do not disable auth, RBAC, tenant scope, audit or idempotency to improve latency.

Recovery: p95 and error rate return below baseline for two sample windows.
