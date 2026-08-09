# P7 Capacity Model

Initial model dimensions:

- API request throughput and p95 latency
- DB connections, wait count and wait duration
- queue age and worker inflight
- provider wait and 429 ratio
- cache hit/miss and load failures
- memory, goroutine and GC trend

Capacity thresholds are not calibrated until medium dataset load evidence exists.
