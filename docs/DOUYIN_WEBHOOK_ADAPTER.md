# 抖店 Webhook 适配器设计

## 签名验证算法

来源：抖店开放平台 Webhook 文档

```
signature = SHA1( appSecret + rawBody )  →  hex lowercase
```

验证头：优先 `X-Douyin-Signature`，备选 `X-Sign`

实现：`douyinshop/webhook_sign.go` `DouyinSignatureVerifier`

## 事件格式

### 标准 Douyin 格式
```json
{"event": "order_created", "client_key": "xxx", "content": {...}}
```

### jinritemai 数组格式
```json
[{"tag": "100", "msg_id": "xxx", "data": {...}}]
```

### Tag → EventType 映射

| Tag | EventType |
|-----|-----------|
| 0 | health_check（安全 ACK，不处理） |
| 100 | order_created |
| 101 | order_paid |
| 102 | order_shipped |
| 103 | order_completed |
| 104 | order_cancelled |
| 200 | inventory_alert（P3 只记录日志） |
| 300 | product_status_changed（P3 只记录日志） |
| 未知 | unknown:{tag}（安全 ACK + 警告日志） |

## 注册方式

`api/router.go` 在 `webhook.NewRegistry` 后从 `platform_douyin_shop.app_secret` 加载 secret：

```go
webhookRegistry.Register("douyin_shop", webhook.NewDouyinVerifier(appSecret))
webhookRegistry.Register("douyin", webhook.NewDouyinVerifier(appSecret))
```

若 secret 为空，verifier 已注册但 Verify 返回 `CodeVerifierNotConfigured`。

## 事件处理路径

```
webhook.Handler.Receive
  → webhook.Service.Ingest (持久化 + idempotency)
  → webhook.Service.ProcessEvent (异步)
    → handlePlatformEvent
      → Service.HandleDouyinPlatformEvent
        → ParseDouyinWebhookEnvelope / ParseJinriteimaiPushEnvelope
          → douyinEventDispatcher.DispatchDouyinEvent
            → OrderEventHandler.HandleDouyinOrderEvent (P3 placeholder)
```

## 未知事件处理原则

未知 tag/event 必须安全 ACK（返回 200），不得静默丢弃——记录 slog.Warn 日志。
## P3.2 Multi-Shop Routing

Douyin webhook business handling is shop-scoped. After signature verification and JSON validation, the handler extracts `client_key` / app ID, platform shop ID, and optional binding ID, then resolves them through `shops` + `shop_auth_tokens`. The persisted event carries `tenant_id`, `internal_shop_id`, `platform_shop_id`, `app_id`, and `binding_id`.

The resolver rejects missing, ambiguous, mismatched, expired, or revoked bindings. Staging and production reject `DOUYIN_WEBHOOK_TEST_SHOP_BINDING_ID` and `ENABLE_DOUYIN_WEBHOOK_DEMO_FALLBACK` at config validation time.
