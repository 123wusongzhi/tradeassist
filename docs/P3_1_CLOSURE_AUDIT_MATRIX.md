# P3.1 Closure Audit Matrix

Phase P3.1 — Douyin production adapter code closure (not final E2E acceptance).

| 能力 | 真实入口 | 业务服务 | 幂等键 | 事务边界 | 契约状态 | 故障恢复 | 缺口（前） | 修改结果 |
|------|----------|----------|--------|----------|----------|----------|------------|----------|
| 订单 Webhook | `webhook.HandleDouyinPlatformEvent` → `ordersync.DouyinOrderWebhookHandler` | `order.Service.UpsertPlatformOrder` | `order-import:{platform}:{shop}:{orderId}:{revision}` + `webhook-process` | DB tx upsert + idempotency Complete 外 | `douyin_order_webhook_events` fixture_verified | 陈旧事件 `ORDER_STALE_UPDATE_IGNORED` | Handler 未注入 | **已接入** |
| 订单轮询同步 | `ordersync.ProcessQueuedTask` | 同上 `UpsertPlatformOrders` source=polling | 同上 | 同上 | fixture_verified | 任务租约 + stale 忽略 | 与 Webhook 分离 | **共用 Upsert** |
| 订单乱序保护 | `order.isStalePlatformUpdate` | `importSyncedOrderWithIdempotency` | revision 键 | Complete 不写库 | — | 审计摘要 | 仅 lifecycle rank | **revision + updatedAt** |
| 抖店 IM | `douyinshop.PullMessages/SendMessage` | customer adapter | — | — | blocked_by_contract_verification | Gate 硬阻断 | 无统一 Gate | **ContractCapabilityGate** |
| 抖店品牌 | `douyinshop.GetBrandList` | publish precheck | — | — | blocked_by_contract_verification | 手工 brand ID | 无 config 项 | **Gate + configstatus** |
| Webhook 签名 | `webhook.DouyinVerifier` | ingest only | `webhook:{platform}:{eventId}` | 验签在 ingest | `douyin_webhook_signature_v1` fixture_verified | production 拒绝 fixture | 无版本门控 | **v1 + production gate** |
| AI 文案 apply | `aiproducttext.applyOneItem` | `product.applyAIContent` tx | `ai-text-apply` + `ai-product-apply` | tx 内 apply 记录；Complete 外 | — | `ReconcileTextApply` | Complete 间隙 | **已实现** |
| AI 图片 apply | `aiproductimage.applyOneItem` | image application tx | `ai-image-apply` | 同上 | — | `ReconcileImageApply` | Complete 间隙 | **已实现** |
| 平台草稿创建 | `productpublish.ProcessDouyinDraftTask` | `douyin_create` + idempotency | `douyin-product-draft-create` | 平台写 + 本地记录 | `douyin_product_draft_create` fixture_verified | `tryRecoverDouyinDraftFromPlatform` | 未知结果 | **已有 + 文档化** |
| Token 刷新 | `douyinshop.EnsureFreshAccessSingleflight` | `shop.persistOAuthTokenRefresh` | singleflight + lock | DB Save token | — | `token_version++` | version 未持久化 | **DB version 递增** |

**最终状态：** Phase P3 Fully Closed · Contract-Gated Capabilities Explicitly Isolated · Real Credential Verification Deferred
