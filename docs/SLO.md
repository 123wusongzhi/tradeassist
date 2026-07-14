# TradeMind Service Level Objectives (SLO)

This document captures the conservative performance thresholds used by the P7-V2 harness.
These values are fixed for Phase P7 closure and must not be lowered to pass verification.

## API SLO

| Metric | Target |
| --- | --- |
| Overall `http_req_failed` | < 1% |
| Overall 5xx | < 0.2% |
| Overall timeout | < 0.2% |

## Core Read List Endpoints

Applies to product, order, inventory, task, webhook event, and operation log list APIs.

| Metric | Target |
| --- | --- |
| p95 latency | <= 800 ms |
| p99 latency | <= 1500 ms |

## Internal Lightweight Endpoints

| Metric | Target |
| --- | --- |
| p95 latency | <= 500 ms |
| p99 latency | <= 1000 ms |

## Controlled Write Endpoints

| Metric | Target |
| --- | --- |
| p95 latency | <= 1200 ms |
| p99 latency | <= 2500 ms |

## Performance Regression

When comparing Initial Controlled Baseline vs Independent Current Load Run:

| Metric | Allowed Degradation |
| --- | --- |
| p95 | <= 10% |
| p99 | <= 15% |
| throughput | <= 10% |
| error rate increase | <= 0.2 percentage points |
| timeout increase | <= 0.1 percentage points |
| peak RSS increase | <= 15% |
| heap growth increase | <= 15% |
| DB pool wait duration increase | <= 20% |
| queue peak depth increase | <= 20% |

Absolute SLO thresholds above still apply during regression comparison.

## Soak Test

| Metric | Target |
| --- | --- |
| steady window | >= 30 continuous minutes |
| goroutine end | <= goroutine start + 10% |
| heap end | <= steady median heap + 15% |
| connection growth | no sustained growth |
| queue growth | no sustained growth |
| cache entries | bounded |
| limiter registry entries | bounded |

## Source

- Harness defaults: `scripts/p7-v2-lib.mjs` (`DEFAULT_SLO`, `REGRESSION_THRESHOLDS`)
- k6 thresholds: `tests/load/lib/thresholds.js`
- Regression gate: `scripts/p7-v2-performance-regression.mjs`
