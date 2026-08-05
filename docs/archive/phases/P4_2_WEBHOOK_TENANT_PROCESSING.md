# P4.2 Webhook Tenant Processing

Tenant and shop scoping for durable webhook ingest and async processing.

## Status Banner

**Webhook Tenant Scoped** · **Shop-Keyed Idempotency** · **NOT Production Ready**

---

## Ingest trust boundary

```text
POST /api/v1/webhooks/:platform/:shopId
  → signature / timestamp verify
  → WebhookShopResolver (shops.tenant_id)
  → webhook_events row: tenant_id, internal_shop_id, platform_shop_id
  → idempotency key: platform + tenant_id + platform_shop_id + event_id
```

Tenant is **never** taken from client headers; it is copied from the resolved shop row.

---

## Async processing paths

| Entry | Tenant gate |
| --- | --- |
| `ProcessEventByRowID` | `RequireTaskTenant` + `WHERE id = ? AND tenant_id = ?` |
| `ProcessEvent` | `RequireTaskTenant` + composite unique lookup |
| `ProcessQueuedEvents` | Per-row `BeginWorker(tenant_id, shop_id, "webhook_process")` |
| `ProcessEventByID` | Uses row already loaded (worker must pass scoped context) |

File: `backend/internal/modules/webhook/processor.go`

---

## Worker loop

`webhook/worker.go` polls `status=queued` events. `ProcessQueuedEvents`:

1. Skips rows with `tenant_id <= 0`
2. Builds worker context with optional `internal_shop_id` as shop scope
3. Delegates to `ProcessEventByRowID` under scoped context

---

## Platform handler

`handlePlatformEvent` routes `douyin_shop` / `douyin` to `HandleDouyinPlatformEvent`; other platforms noop-mark processed.

Order upsert inherits tenant from webhook event row (P3.1/P3.2 alignment).

---

## Indexes (P3.2 / P4.2)

- `ux_webhook_shop_event` — `(platform, tenant_id, platform_shop_id, event_id)`
- `ix_webhook_shop_scope` — lookup by platform + tenant + platform_shop_id

---

## Test notes

- WH-01 / WH-02 covered in `docs/P4_IDOR_TEST_MATRIX.md`
- Automated tenant webhook tests deferred to expanded IDOR matrix
