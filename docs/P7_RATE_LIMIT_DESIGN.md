# P7 Rate Limit Design

Implemented foundation:

- `pkg/ratelimit.LocalLimiter` with token bucket.
- Hashed limiter keys to avoid storing clear PII.
- Gin middleware returns `429` and `Retry-After`.
- Production cannot disable rate limiting without explicit approval env.

Remaining closure:

- Redis distributed limiter.
- Route group policies.
- Auth-specific account/IP/session limits.
- Webhook burst policies and management UI.
