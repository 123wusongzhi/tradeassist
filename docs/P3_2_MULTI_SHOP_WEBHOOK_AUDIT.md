# P3.2 Multi-Shop Webhook Audit

This audit records the code-level closure for multi-shop Douyin webhook routing. It is a production capability development checkpoint, not final real-credential acceptance.

| Capability | Current implementation | Trusted input | Database binding | Tenant boundary | Fallback behavior | Production risk | Change result |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Shop resolution | `webhook.DBWebhookShopResolver` | Verified body/header shop ID, app key, optional binding ID | `shops` + `shop_auth_tokens` | Resolver returns `tenantId` and `internalShopId` | Explicit dev/test/demo only | Wrong-shop writes | Closed |
| App binding | Resolver filters `shop_auth_tokens.app_key` | `client_key` / app headers after signature verification | Token row as binding projection | Bound to matched shop row | No implicit app fallback | App/shop mismatch | Closed |
| Secret binding | Resolver accepts binding ID, not secret plaintext | Header binding ID when available | `shop_auth_tokens.id` | Bound to matched shop row | No secret fallback | Secret leakage / mismatch | Closed |
| Event persistence | `webhook_events` stores tenant/shop/app/binding fields | Resolver output only | `ux_webhook_shop_event` | Unique key includes tenant/shop | Not persisted when unresolved | Cross-shop event dedupe | Closed |
| Async processing | Worker calls `ProcessEventByID` | Persisted event row | Event row ID | No re-guess in worker | None | Worker picks wrong row | Closed |
| Order upsert | `DouyinOrderWebhookHandler` uses resolved event fields | Event metadata from resolver | Shop lookup includes tenant | `UpsertPlatformOrder.TenantID` | Legacy single-shop fallback removed | Cross-tenant order write | Closed |
| Production fallback | `config.Validate` rejects fallback vars | Environment profile | Config fields | staging/production fail-fast | Forbidden | Hidden single-shop fallback | Closed |
| Tests | Focused Go tests cover multi-shop and ambiguity | Fixtures only | SQLite isolated DSN | Per-test DB | Explicit only | Regression | Covered |

Real Douyin credentials, real webhook delivery, final acceptance, tag creation, gray release, and Production Ready marking remain deferred.
