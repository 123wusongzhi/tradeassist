# Douyin Webhook Tenant Isolation

Webhook tenant isolation is enforced by carrying resolver output through the event and order chain.

- `webhook_events` stores `tenant_id`, `internal_shop_id`, `platform_shop_id`, `app_id`, and `binding_id`.
- Webhook event uniqueness is scoped by `platform + tenant_id + platform_shop_id + event_id`.
- Worker processing uses event row ID, so duplicate `eventId` values from different shops do not collide.
- `DouyinOrderWebhookHandler` validates resolved `internalShopId`, `tenantId`, and `platformShopId` before upsert.
- `UpsertPlatformOrder` accepts `TenantID` and writes it to synced orders.
- Existing order lookup during import includes `tenant_id`, `shop_id`, `platform`, and `external_order_id`.

`requestId` is not used as an authorization boundary. URL parameters are not used to decide tenant ownership.
