# Rate Limit Rejection Spike

Meaning: 429 responses spike above normal traffic shape.

Check: route group, IP/user/tenant dimensions, auth failures, webhook source and attack indicators.

Mitigate: keep fail-closed or local fallback, adjust route policy only through approved config and protect login endpoints.

Scale: use Redis distributed limiting for multi-instance consistency.

Forbidden: do not disable all rate limits without explicit production approval.

Recovery: rejection rate normalizes and legitimate traffic succeeds.
