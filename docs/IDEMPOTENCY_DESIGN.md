# 统一幂等设计（P2 / P2.1）

> Phase P2 引入跨模块幂等基础设施，避免重复执行产生副作用。实现位于 `backend/internal/modules/idempotency`。
> **Phase P2.1**：关键生产写路径已通过共享 `idempotency.Service` 完成接入（订单同步/导入、库存扣减/推送、刊登、客服外发、AI 批次、Webhook）；接入矩阵见 [`P2_1_IDEMPOTENCY_ADOPTION_MATRIX.md`](P2_1_IDEMPOTENCY_ADOPTION_MATRIX.md)，集成指南见 [`DOMAIN_IDEMPOTENCY_INTEGRATION.md`](DOMAIN_IDEMPOTENCY_INTEGRATION.md)。

## 数据模型：`idempotency_records`

| 字段 | 说明 |
| --- | --- |
| `scope` + `idempotency_key` | 业务域 + 稳定键，**联合唯一**（`ux_idempotency_scope_key`） |
| `request_hash` | 请求体 SHA-256，同键不同 payload 视为冲突 |
| `status` | 见下表 |
| `owner` | 当前持锁执行者（worker ID 或服务名） |
| `locked_until` | 处理中租约截止时间 |
| `expires_at` | 成功记录保留期（默认 7 天，供重放查询） |
| `resource_type` / `resource_id` | 成功后关联资源 |
| `response_code` / `response_summary` | 成功摘要，供 API 重放 |
| `error_code` / `retryable` | 失败分类 |

### 状态机

```text
pending → processing → succeeded
                    ↘ failed_retryable → processing（重试）
                    ↘ failed_permanent
processing（租约过期）→ expired（ReleaseExpired 清扫）
```

默认租约 **2 分钟**（`DefaultLease`）；完成记录 TTL **7 天**（`DefaultTTL`）。

## Service API

| 方法 | 用途 |
| --- | --- |
| `Acquire(ctx, scope, key, requestHash, owner, lease)` | 获取执行权；已成功返回 `Replay` + `OpError(IDEMPOTENCY_ALREADY_SUCCEEDED)` |
| `Heartbeat(ctx, recordID, owner, lease)` | 延长 processing 租约 |
| `Complete(ctx, recordID, owner, CompleteResult)` | 标记成功并释放租约 |
| `Fail(ctx, recordID, owner, errorCode, retryable)` | 标记可重试或永久失败 |
| `Get(ctx, scope, key)` | 查询最新记录 |
| `ReleaseExpired(ctx, limit)` | 将过期 processing / 超 TTL 记录标为 `expired` |

## P2.1 接入状态

| 状态 | 说明 |
| --- | --- |
| **已接入** | 订单同步任务、订单导入、库存扣减/推送、刊登批次/入队、客服外发、AI 文案/图片批次创建、Webhook 入站 |
| **预留** | AI 文案/图片 **应用**（`ai-text-apply` / `ai-image-apply`）、库存补偿 |
| **验证** | `node scripts/p2-1-domain-idempotency-check.mjs` → [`P2_1_DOMAIN_IDEMPOTENCY_REPORT.md`](P2_1_DOMAIN_IDEMPOTENCY_REPORT.md) |

`router.go` 将同一 `idempotencySvc` 注入 `ordersync`、`order`、`inventory`、`productpublish`、`customerchat`、`aiproducttext`、`aiproductimage`。

## Scope 与 Key 模式

Key 构造见 `scope.go` + `keys.go`，**不得嵌入密钥或 PII**：

| Scope | Key 模式 | 场景 | P2.1 |
| --- | --- | --- | --- |
| `order_sync` | `order-sync-job:{platform}:{shopId}:{mode}:{window}` | 同步任务创建 | ✓ |
| `order_import` | `order-import:{platform}:{shopId}:{platformOrderId}` | 单订单导入 | ✓ |
| `inventory` | `inventory-deduct:{orderId}:{orderItemId}:{skuId}` | 库存扣减 | ✓ |
| `inventory_push` | `inventory-push:{platform}:{shopId}:{skuId}:{stockVersion}` | 库存推送 | ✓ |
| `publish` | `publish-batch:…` / `publish-enqueue:…` | 刊登批次/入队 | ✓ |
| `customer_send` | `customer-send:{conversationId}:{clientMessageId}` | 客服外发 | ✓ |
| `ai_text` | `ai-text-batch:…` | AI 文案批次 | ✓ |
| `ai_image` | `ai-image-batch:…` | AI 图片批次 | ✓ |
| `webhook` | `webhook:{platform}:{eventId}` | Webhook 入站 | ✓ |

`HashRequest(payload []byte)` 对规范化请求体做 SHA-256。

## 错误码

| 代码 | 含义 | 建议处理 |
| --- | --- | --- |
| `IDEMPOTENCY_IN_PROGRESS` | 其他 worker 持锁处理中 | 轮询或返回 409 |
| `IDEMPOTENCY_KEY_CONFLICT` | 同键不同 payload，或永久失败 | 人工介入 |
| `IDEMPOTENCY_ALREADY_SUCCEEDED` | 已成功，可重放 `response_summary` | 返回缓存结果 |
| `IDEMPOTENCY_LEASE_LOST` | 租约丢失（Complete/Fail/Heartbeat） | 放弃本次写入 |
| `IDEMPOTENCY_RECORD_EXPIRED` | 记录已过期 | 使用新键或清理后重试 |

## 索引与迁移

P2 迁移（`migrate_p2.go`）创建表及索引：`ix_idempotency_status`、`ix_idempotency_locked_until`。

## 使用约定

1. 写操作前先 `Acquire`；长任务周期 `Heartbeat`。
2. 业务成功必须 `Complete`；失败按 `taskretry.Classify` 决定 `retryable`。
3. 客户端可选传幂等键；服务端必须用稳定业务语义生成 key，而非随机 UUID。
4. Webhook 与订单同步共享同一套 `idempotency_records` + 领域表双写防重。
5. 异步 Worker 须配合 `tasklease`（`execution_id` / `heartbeat_at` / `lock_version`），见 [`TASK_LEASE_AND_HEARTBEAT_DESIGN.md`](TASK_LEASE_AND_HEARTBEAT_DESIGN.md)。
