# P2.1 幂等接入矩阵

> Phase P2.1 将统一 `idempotency.Service` 接入关键生产写路径，并与任务行级租约（`tasklease`）协同防重复与防陈旧写入。  
> **状态说明**：`integrated` = 代码已接入并通过静态扫描；`partial` = 部分子路径或 HTTP 路由未就绪；`planned` = 键/文档已预留。

| 业务路径 | Scope | Key 模式 | 接入模块 | 幂等状态 | 任务租约 | 备注 |
| --- | --- | --- | --- | --- | --- | --- |
| 订单同步任务创建 | `order_sync` | `order-sync-job:{platform}:{shopId}:{mode}:{window}` | `ordersync/idempotency_create.go` | integrated | integrated (`ordersync/lease.go`) | 同输入重放返回已有 `order_sync_tasks` |
| 订单导入 / Upsert | `order_import` | `order-import:{platform}:{shopId}:{platformOrderId}` | `order/idempotency_import.go` | integrated | — | 配合 DB 平台订单唯一键 + 陈旧更新忽略 |
| 库存扣减 | `inventory` | `inventory-deduct:{orderId}:{orderItemId}:{skuId}` | `inventory/idempotency_deduct.go` | integrated | — | 配合 `business_event_key` 唯一索引 |
| 库存推送 | `inventory_push` | `inventory-push:{platform}:{shopId}:{skuId}:{stockVersion}` | `inventory/idempotency_push.go` | integrated | integrated (`inventory/lease.go`) | 同版本推送任务去重 |
| 刊登批次创建 | `publish` | `publish-batch:{shopId}:{draftId}:{version}` 或批次 hash | `productpublish/idempotency_batch.go` | integrated | — | 客户端可传 `idempotencyKey` |
| 刊登子任务入队 | `publish` | `publish-enqueue:{batchId}:{taskType}` | `productpublish/idempotency_batch.go` | integrated | integrated (`productpublish/lease.go`) | 防止重复 enqueue |
| 客服消息外发 | `customer_send` | `customer-send:{conversationId}:{clientMessageId}` | `customerchat/send_platform.go` | integrated | — | 平台发送前 Acquire；MVP 仍建议人工确认 |
| AI 文案批次创建 | `ai_text` | `ai-text-batch:{productId}:{version}:{op}:{inputHash}` | `aiproducttext/service.go` | integrated | — | 批次表 `idempotency_key` 双写 |
| AI 图片批次创建 | `ai_image` | `ai-image-batch:{productId}:{version}:{op}:{imageHash}` | `aiproductimage/service.go` | integrated | — | 同上 |
| Webhook 入站 ACK | `webhook` | `webhook:{platform}:{eventId}` | `webhook/service.go` + `RegisterPublic` | integrated | — | P2.2：公开 HTTP + process 键；平台业务适配后置 |
| AI 文案应用 | `ai_text` | `ai-text-apply:{batch}:{item}:{product}:{version}` | `aiproducttext/idempotency_apply.go` | integrated | — | **P2.2 closed**；见 [`AI_RESULT_APPLY_IDEMPOTENCY.md`](AI_RESULT_APPLY_IDEMPOTENCY.md) |
| AI 文案撤销 | `ai_text` | `ai-text-undo:{applicationId}:{version}` | `aiproducttext/idempotency_apply.go` | integrated | — | **P2.2 closed** |
| AI 图片应用 | `ai_image` | `ai-image-apply:{batch}:{item}:{product}:{version}:{slot}` | `aiproductimage/idempotency_apply.go` | integrated | — | **P2.2 closed** |
| AI 图片撤销 | `ai_image` | `ai-image-undo:{applicationId}:{version}` | `aiproductimage/idempotency_apply.go` | integrated | — | **P2.2 closed** |
| 库存补偿 | `inventory` | `inventory-compensate:…` | — | planned | — | Key 已预留，补偿路径待接入 |

## 任务租约矩阵（P2.1）

| 任务表 | `heartbeat_at` | `execution_id` | `lock_version` | `tasklease` 包 | Worker 续租 |
| --- | --- | --- | --- | --- | --- |
| `order_sync_tasks` | ✓ | ✓ | ✓ | ✓ | ✓ |
| `inventory_sync_tasks` | ✓ | ✓ | ✓ | ✓ | ✓ |
| `product_publish_tasks` | ✓ | ✓ | ✓ | ✓ | ✓ |
| `collect_tasks` | ✓ | ✓ | ✓ | ✓（P2.2 `TryClaimPendingOrRetrying`） | ✓ |
| `image_tasks` | ✓ | ✓ | ✓ | ✓（P2.2） | ✓ |
| `customer_message_sync_tasks` | ✓ | ✓ | ✓ | ✓（P2.2） | ✓ |

## 路由注入（`router.go`）

共享 `idempotencySvc` 已注入：

- `ordersync.Service`
- `order.Service`
- `inventory.Service`
- `productpublish.Service`
- `customerchat.Service`
- `aiproducttext.Service`
- `aiproductimage.Service`

`webhook.Service` 已接入幂等；P2.2 起 `webhook.RegisterPublic` 挂载公开 HTTP 接收（真实平台业务适配仍后置）。

> **P2.2 closed（2026-07-11）**：AI apply/undo、Webhook HTTP、collect/imagetask/customersync tasklease 收口。详见 [`P2_2_RELIABILITY_CLOSURE_MATRIX.md`](P2_2_RELIABILITY_CLOSURE_MATRIX.md)。

## 验证

```bash
node scripts/p2-1-domain-idempotency-check.mjs
node scripts/p2-2-reliability-closure-check.mjs
```

报告：`docs/P2_1_DOMAIN_IDEMPOTENCY_REPORT.md`、`docs/P2_2_RELIABILITY_CLOSURE_REPORT.md`。

## 相关文档

- [`IDEMPOTENCY_DESIGN.md`](IDEMPOTENCY_DESIGN.md) — 数据模型与 Service API
- [`DOMAIN_IDEMPOTENCY_INTEGRATION.md`](DOMAIN_IDEMPOTENCY_INTEGRATION.md) — 接入步骤
- [`TASK_LEASE_AND_HEARTBEAT_DESIGN.md`](TASK_LEASE_AND_HEARTBEAT_DESIGN.md) — 行级租约
- [`STALE_WORKER_PROTECTION.md`](STALE_WORKER_PROTECTION.md) — 陈旧 Worker 防护
- [`CONCURRENT_WRITE_SAFETY.md`](CONCURRENT_WRITE_SAFETY.md) — 并发写安全
- [`P2_2_RELIABILITY_CLOSURE_MATRIX.md`](P2_2_RELIABILITY_CLOSURE_MATRIX.md) — P2.2 收口
