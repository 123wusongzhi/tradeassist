# P7 Performance Architecture

P7 adds a layered foundation without changing the MVP product boundary.

Core layers:

- Config guards: P7 env variables load through `config.P7Config`; production blocks performance-test mode, dataset generation and public pprof.
- Database capacity: `database.Open` applies bounded pool settings from env.
- Pagination governance: `pkg/pagination` defines bounded offset and cursor scope checks.
- HTTP rate limiting: `pkg/ratelimit` and `middleware.RateLimit` provide local token bucket protection with hashed keys.
- Evidence gates: `scripts/p7-performance-capacity-check.mjs` and `scripts/p7-performance-regression-gate.mjs` separate code foundation from real validation evidence.

Distributed Redis limiting, full cursor migration, medium dataset materialization, load test and soak test remain closure items.
