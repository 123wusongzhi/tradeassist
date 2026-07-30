# 抖店 OAuth 与 Token 生命周期

## OAuth 授权流程

```
1. 管理员发起授权
   POST /api/v1/shops/douyin/oauth/start
   → 生成 state（随机）
   → Redis SET oauth:douyin_shop:state:{state} = payload (10min TTL)
   → DB INSERT douyin_oauth_states (StateHash, ExpiresAt)
   → 返回授权 URL（含 service_id + state）

2. 用户在浏览器完成授权
   → 抖店回调 GET /api/v1/shops/douyin/oauth/callback?code=xxx&state=yyy

3. 回调处理
   → Redis GET state（快速路径）
   → DB ConsumedAt 一次性消费保护
   → ExchangeCode → AccessToken + RefreshToken
   → 持久化到 shop_auth_tokens（加密存储）
   → 更新 shops.auth_status = authorized
```

## State 保护

| 保护机制 | 实现 |
|---------|------|
| Redis TTL | 10 分钟过期 |
| DB ExpiresAt | 冗余过期检查 |
| ConsumedAt | 一次性使用，防止 replay |
| redirect_uri 白名单 | 与配置精确匹配 |

## Token 刷新

```
EnsureFreshAccess()
  → freshAccessToken() — 有效则直接返回
  → EnsureFreshAccessSingleflight() — dedup
    → ensureFreshAccessDirect()
      → refreshUsable() — refresh token 是否可用
      → RefreshToken API (token.refresh)
      → PersistRefreshedToken callback
      → 更新 Client 内存 token
```

## TokenVersion（P3 新增）

每次成功 refresh 后 token_version++。写入 shop_auth_tokens 前校验版本，防止多实例并发 refresh 导致旧版本覆盖新版本（DOUYIN_TOKEN_VERSION_CONFLICT）。

## ShopAuthToken 新增字段（P3）

| 字段 | 类型 | 说明 |
|-----|------|------|
| token_version | bigint | 单调递增版本号 |
| reauthorization_required | bool | 需要重新授权时置 true |
| last_refresh_error_code | varchar(128) | 最近一次刷新失败的错误码 |

## 错误码

| 错误码 | 含义 |
|-------|------|
| DOUYIN_AUTH_EXPIRED | token 已过期，需重新授权 |
| DOUYIN_TOKEN_REFRESH_FAILED | 刷新失败 |
| DOUYIN_TOKEN_VERSION_CONFLICT | 版本冲突，拒绝旧版本写入 |
| DOUYIN_TOKEN_REFRESH_IN_PROGRESS | 刷新进行中 |
| DOUYIN_REAUTHORIZATION_REQUIRED | 需要用户重新完成 OAuth |
| DOUYIN_OAUTH_STATE_MISSING | state 找不到 |
| DOUYIN_OAUTH_STATE_EXPIRED | state 已过期 |
| DOUYIN_OAUTH_STATE_ALREADY_USED | state 已被使用 |
| DOUYIN_OAUTH_REDIRECT_NOT_ALLOWED | redirect_uri 不在白名单 |
