# 统一幂等设计（P2）

> Phase P2 引入跨模块幂等基础设施，避免重复执行产生副作用。实现位于 `backend/internal/modules/idempotency`。

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

## Scope 与 Key 模式

Key 构造见 `keys.go`，**不得嵌入密钥或 PII**：

| Scope 示例 | Key 模式 | 场景 |
| --- | --- | --- |
| `webhook` | `webhook:{platform}:{eventId}` | Webhook 入站 |
| （调用方自定） | `order-sync:{platform}:{shopId}:{platformOrderId}` | 订单同步 |
| | `inventory-deduct:{orderId}:{orderItemId}:{skuId}` | 库存扣减 |
| | `inventory-push:{shopId}:{skuId}:{stockVersion}` | 库存推送 |
| | `customer-send:{conversationId}:{clientMessageId}` | 客服外发 |
| | `publish-draft:{shopId}:{productDraftId}:{publishVersion}` | 刊登草稿 |
| | `ai-text-batch:{productId}:{contentHash}:{op}` | AI 文案批次 |
| | `ai-image-batch:{productId}:{imageHash}:{op}` | AI 图片批次 |

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
