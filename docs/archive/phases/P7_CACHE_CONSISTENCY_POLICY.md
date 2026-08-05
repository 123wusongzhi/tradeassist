# P7 Cache Consistency Policy

Config foundation:

- `CACHE_ENABLED`
- `CACHE_DEFAULT_TTL_SECONDS`
- `CACHE_MAX_ENTRIES`
- `CACHE_SINGLEFLIGHT_ENABLED`

Rules:

- Cache only non-sensitive reusable data.
- Tenant and permission scope must be part of cache boundaries.
- Inventory correctness must not depend on cache.
- Permission changes require invalidation or short TTL.

Runtime cache implementation and singleflight coverage remain pending.
