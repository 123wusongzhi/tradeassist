# Webhook HTTP Receiver Design (P2.2)

> Public HTTP ingest for platform webhooks with fast ACK and async DB-poll processing.  
> Foundation only: unknown platforms use **noop** handlers. **非 Production Ready** · not Douyin/real-platform E2E.

## Route

```text
POST /api/v1/webhooks/:platform/:eventType
```

- Registered via `webhook.RegisterPublic` on the public `/api/v1` group (**no JWT**).
- Handler: `webhook.Handler.Receive`.

## Request constraints

| Rule | Implementation |
| --- | --- |
| Content-Type | Must be `application/json` |
| Body limit | `http.MaxBytesReader` + `Service.MaxPayloadBytes` (from `WEBHOOK_MAX_BODY_KB`, default **512**) |
| Signature | `SignatureVerifier` registry (see signature doc) |
| Timestamp | Header skew vs `MaxClockSkew` (`WEBHOOK_MAX_CLOCK_SKEW_SECONDS`, default **300**) |
| JSON | Body must be valid JSON after read |

## Fast ACK

After signature + timestamp + persist:

```json
{
  "code": 0,
  "message": "accepted",
  "data": {
    "eventId": "...",
    "status": "queued|...",
    "duplicate": false
  }
}
```

- Persist failure → **non-2xx** (no success ACK).
- Duplicate event → `duplicate: true` with existing status (still ACK OK).

## Ingest path

1. Resolve `eventId` from payload fields, else SHA-256 of body.
2. Short-circuit if `(platform, event_id)` already exists.
3. `idempotency.Acquire(ScopeWebhook, webhook:{platform}:{eventId}, HashRequest(body), ...)`.
4. Insert `webhook_events` with `ON CONFLICT (platform, event_id) DO NOTHING`, status `queued`.
5. `Complete` idempotency record (`WEBHOOK_RECEIVED` / `WEBHOOK_DUPLICATE`).

Async process key: `webhook-process:{platform}:{eventId}` (`WebhookProcess`).

## Async worker

- `webhook.StartWorker` polls DB on `WEBHOOK_WORKER_INTERVAL_SECONDS` (default 3s).
- Each tick: `ProcessQueuedEvents(limit)` → claim `queued`/`received`/`failed_retryable` → `ProcessEvent`.
- `ProcessEvent` runs `handlePlatformEvent` (noop for unknown platforms) then marks `processed`.

No Redis BRPOP for webhook MVP; durability is the `webhook_events` table.

## Config (env)

| Variable | Role |
| --- | --- |
| `WEBHOOK_MAX_BODY_KB` | Body size cap |
| `WEBHOOK_MAX_CLOCK_SKEW_SECONDS` | Timestamp window |
| `WEBHOOK_ENABLE_TEST_VERIFIER` | Register `internal-test` HMAC (dev/test only) |
| `WEBHOOK_WORKER_INTERVAL_SECONDS` | Poll interval |

## Out of scope

Real Douyin / TikTok / Shopee business adapters, production gray release, Production Ready claims.

## Related

- [`WEBHOOK_SIGNATURE_AND_REPLAY_PROTECTION.md`](WEBHOOK_SIGNATURE_AND_REPLAY_PROTECTION.md)
- [`IDEMPOTENCY_DESIGN.md`](IDEMPOTENCY_DESIGN.md)
