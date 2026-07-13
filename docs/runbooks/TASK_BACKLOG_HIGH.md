# Task Backlog High

Meaning: queue age or pending task count keeps growing.

Check: worker health, downstream provider 429, DB wait, retries, dead letters and tenant/shop concentration.

Mitigate: reduce low-priority task intake, increase delay for retryable failures and reserve capacity for critical tasks.

Scale: raise workers only within queue and DB capacity budgets.

Forbidden: do not start unbounded goroutines or queues.

Recovery: queue age decreases and inflight returns to normal after backlog drains.
