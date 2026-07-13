# Cache Stampede

Meaning: cache miss/load failure spikes cause downstream pressure.

Check: TTL, entry count, singleflight status, backend latency and permission/tenant boundaries.

Mitigate: shorten high-risk cache paths, enable singleflight, use negative caching only for safe non-sensitive data.

Scale: increase cache entries only within memory budget.

Forbidden: do not cache sensitive or permission-scoped data without tenant/user boundary.

Recovery: hit rate improves and downstream latency returns to baseline.
