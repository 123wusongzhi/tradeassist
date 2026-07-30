# Export Backlog

Meaning: export jobs accumulate or memory rises during export.

Check: export concurrency, row count, bytes, temp files, DB p95 and client cancellation.

Mitigate: reduce `EXPORT_MAX_CONCURRENT`, reject new low-priority exports and lower batch size.

Scale: add worker capacity only after DB and storage budget checks.

Forbidden: do not reuse normal list APIs for unlimited export.

Recovery: backlog drains and memory peak stays within budget.
