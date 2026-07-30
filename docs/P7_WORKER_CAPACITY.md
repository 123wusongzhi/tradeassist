# P7 Worker Capacity

P7 config introduces generic bounded worker controls:

- `WORKER_CONCURRENCY_DEFAULT`
- `WORKER_QUEUE_CAPACITY`
- `WORKER_MAX_INFLIGHT`
- `WORKER_PREFETCH`
- `WORKER_SHUTDOWN_TIMEOUT_SECONDS`

Existing workers already use explicit per-domain concurrency envs. Full P7 closure still requires bounded queue/inflight enforcement per worker type, fairness testing and backlog recovery evidence.
