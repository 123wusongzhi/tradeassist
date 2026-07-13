# Capacity Threshold Reached

Meaning: one or more P7 capacity model thresholds are exceeded.

Check: API p95, DB wait, worker queue age, provider 429, memory, goroutines and dataset size.

Mitigate: apply route limits, reduce low-priority jobs and pause load generation.

Scale: follow capacity model recommendations and document threshold changes.

Forbidden: do not call isolated load evidence real production capacity validation.

Recovery: metrics stay below thresholds for the required observation window.
