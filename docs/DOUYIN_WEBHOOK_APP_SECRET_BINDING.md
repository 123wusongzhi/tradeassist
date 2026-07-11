# Douyin Webhook App / Secret Binding

P3.2 treats existing `shops` and `shop_auth_tokens` as the current ShopBinding projection.

Binding path:

```text
App / appKey
-> shop_auth_tokens row
-> shops row
-> tenant_id + internal shop ID
```

Rules:

- Resolver accepts app key and optional binding ID, never secret plaintext.
- Secret values remain encrypted in `shop_auth_tokens` or platform settings and are not returned to the frontend.
- Logs and operation records must not include app secret, access token, refresh token, or raw webhook payload.
- If one app can map to multiple shops, platform shop ID must disambiguate.
- If binding ID is present but does not match the resolved shop, processing is rejected.

The current verifier still validates Douyin signatures through the platform app secret configured in settings. P3.2 prevents post-verification business handling from falling back to an arbitrary authorized shop.
