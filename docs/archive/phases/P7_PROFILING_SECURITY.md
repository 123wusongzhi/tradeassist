# P7 Profiling Security

Config foundation:

- `PPROF_ENABLED`
- `PPROF_INTERNAL_ONLY`

Production guard:

- `PPROF_ENABLED=true` with `PPROF_INTERNAL_ONLY=false` is rejected in production.

No public pprof route was added in this pass. Profiling execution API remains pending and must not expose arbitrary paths or raw request bodies.
