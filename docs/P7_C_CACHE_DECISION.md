# P7-C Cache Decision

Status: cache_required_implemented

Decision: `cache_required`

Evidence:

- CACHE_* configuration is present in .env.example, .env.production.example and backend/internal/config/p7_config.go.
- P7-V audit marks cache TTL and entry bound as implemented and several cache capabilities as partial.
- backend/internal/pkg/cache now provides TTL, max entries, invalidation, negative cache and singleflight primitives.

Package: `backend/internal/pkg/cache`

Test package: `backend/internal/pkg/cache/...`
