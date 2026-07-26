# Database Pool Saturated

Meaning: DB pool wait count or wait duration grows continuously.

Check: open/in-use/idle connections, slow queries, long transactions, lock waits and worker inflight.

Mitigate: reduce worker concurrency, reject low-priority exports, shorten batch sizes and stop nonessential load tests.

Scale: increase pool only within DB capacity budget and after query plans are checked.

Forbidden: do not create new DB connections per request or remove tenant filters.

Recovery: wait duration returns to baseline and connection count stabilizes.
