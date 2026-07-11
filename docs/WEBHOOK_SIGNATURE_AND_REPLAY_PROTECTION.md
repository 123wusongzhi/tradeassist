# Webhook Signature and Replay Protection (P2.2)

> Authenticity and replay controls for `POST /api/v1/webhooks/:platform/:eventType`.  
> **非 Production Ready**. Production must not bypass signature verification.

## SignatureVerifier

```go
type SignatureVerifier interface {
  Verify(ctx context.Context, input VerifyInput) error
}
```

- `Registry` maps `platform → SignatureVerifier`.
- `Registry.Verify` fails with `WEBHOOK_VERIFIER_NOT_CONFIGURED` when no verifier is registered.
- Handler **requires** a non-nil registry; missing verifiers → unauthorized.

### Headers (normalized)

| Purpose | Headers |
| --- | --- |
| Signature | `X-Webhook-Signature` or `X-TradeMind-Signature` |
| Timestamp | `X-Webhook-Timestamp` or `X-TradeMind-Timestamp` |
| Nonce | `X-Webhook-Nonce` or `X-TradeMind-Nonce` (reserved for platform adapters) |

Hex signatures may use `sha256=` / `v1=` prefixes (stripped before compare).

### Test verifier (`internal-test`)

- HMAC-SHA256 over `{unix}.{rawBody}` with constant-time compare.
- Registered only when `WEBHOOK_ENABLE_TEST_VERIFIER=true` **and** `APP_ENV` is development/test.
- Config load **forces** test verifier off in production regardless of the env flag.
- Production request with platform `internal-test` → `WEBHOOK_SIGNATURE_BYPASS_FORBIDDEN`.

## Timestamp skew

- `Service.ValidateTimestamp` / `MaxClockSkew` (default 300s from `WEBHOOK_MAX_CLOCK_SKEW_SECONDS`).
- Out-of-window → `WEBHOOK_TIMESTAMP_EXPIRED`.
- Missing timestamp when required by verifier → `WEBHOOK_TIMESTAMP_MISSING`.

## Replay protection

Layers:

1. **Idempotency**: `webhook:{platform}:{eventId}` via `idempotency.Service.Acquire`.
2. **DB unique**: `(platform, event_id)` with `ON CONFLICT DO NOTHING`.
3. **Early read**: existing row → ACK `duplicate: true` without re-processing side effects.
4. **Async**: `webhook-process:{platform}:{eventId}` so process is also single-flight.

Same event redelivered by the platform therefore ACKs as duplicate and does not double-apply business handlers.

## Production bypass forbidden

| Rule | Enforcement |
| --- | --- |
| No anonymous ingest without verifier | Handler fails if `Verifiers` nil / platform missing |
| No production test bypass | `CodeSignatureBypassForbidden` for `internal-test` in production |
| Test secret never for real platforms | `TestHMACSecret` is internal-test only |

## Logging / secrets

- Handler does **not** log raw request bodies.
- Worker logs counts / errors only.
- Persist stores `payload_hash` + truncated `raw_summary` (≤200 chars) — not full secrets in logs.
- Never log API keys, tokens, or signature secrets.

## Related

- [`WEBHOOK_HTTP_RECEIVER_DESIGN.md`](WEBHOOK_HTTP_RECEIVER_DESIGN.md)
- Code: `backend/internal/modules/webhook/signature.go`, `service.go`, `handler.go`
