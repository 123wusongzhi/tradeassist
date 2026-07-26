# P7 Backpressure Design

Backpressure policy:

- Reduce claim rate when DB/provider/storage is saturated.
- Delay `available_at` for retryable downstream failures.
- Return `429` or `503` with `Retry-After` for low-priority new work when capacity is exhausted.
- Keep webhook ACK/persist path short and never drop already acknowledged events.

Current code foundation covers HTTP 429 local rate limiting and bounded config. Worker backpressure execution remains a closure item.
