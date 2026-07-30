# Douyin Webhook Shop Resolution

P3.2 resolves Douyin webhook ownership through `WebhookShopResolver` before any business processing.

Resolution order:

1. Verified payload platform shop ID.
2. Verified header platform shop ID when present.
3. App key plus platform shop ID mapped to one active shop auth binding.
4. Explicit `DOUYIN_WEBHOOK_TEST_SHOP_BINDING_ID` only in `development` / `test`, or `demo` when `ENABLE_DOUYIN_WEBHOOK_DEMO_FALLBACK=true`.

Rejected cases:

- Missing trusted shop ID: `DOUYIN_WEBHOOK_UNTRUSTED_SHOP_IDENTIFIER`.
- No binding: `DOUYIN_WEBHOOK_SHOP_NOT_RESOLVED`.
- Multiple candidate bindings: `DOUYIN_WEBHOOK_SHOP_AMBIGUOUS`.
- App or binding mismatch: `DOUYIN_WEBHOOK_APP_BINDING_MISMATCH` / `DOUYIN_WEBHOOK_SHOP_BINDING_MISMATCH`.
- Expired or revoked binding: `DOUYIN_WEBHOOK_AUTHORIZATION_EXPIRED` / `DOUYIN_WEBHOOK_BINDING_REVOKED`.

The handler does not select the first authorized shop and the worker does not re-resolve a shop from unordered database rows.
