# 订单同步可靠性设计（P2）

> 订单同步幂等依赖 **数据库唯一约束 + Upsert 语义 + Webhook 共享规则**，实现位于 `ordersync`、`order/sync_platform.go` 与 Phase 10.2 索引迁移。

## 唯一键与索引

### 订单头（PostgreSQL 部分唯一索引）

```sql
CREATE UNIQUE INDEX ux_orders_shop_platform_ext_order
 ON orders (shop_id, platform, external_order_id)
 WHERE external_order_id IS NOT NULL AND external_order_id <> '' AND deleted_at IS NULL;
```

业务语义：**同一店铺 + 平台 + 平台订单号** 仅一条本地订单。

### 订单行

```sql
CREATE UNIQUE INDEX ux_order_items_order_ext_item
 ON order_items (order_id, external_item_id)
 WHERE external_item_id IS NOT NULL AND external_item_id <> '';
```

迁移前执行重复数据检查；若存在重复组则 **阻塞迁移** 并指向 `DOUYIN_DUPLICATE_DATA_REPAIR.md`。

## Upsert 流程

`order.Service.UpsertSyncedOrders(shopID, platform, payloads)`：

1. 按 `(shop_id, platform, external_order_id)` 查找现有行。
2. **不存在** → `Create` 订单 + `replaceSyncedChildren`（items/shipments）。
3. **存在** → 更新头字段（状态、金额、时间戳、`raw_data`）；**保留运营备注** `remark`。
4. 子行按 `external_item_id` 对齐替换，避免重复插入。

返回指标：`created` / `updated` / `success` / `failed`；`ordersync` 任务 output 含 `upsertSuccess`、`upsertFailed`。

## 幂等 Key（统一幂等服务）

```text
idempotency.OrderSync(platform, shopID, platformOrderID)
→ order-sync:{platform}:{shopId}:{platformOrderId}
```

用于跨请求防重：重复同步同一平台订单不产生第二条 `orders` 行。

## 订单同步任务可靠性

- 队列：`ORDER_SYNC_QUEUE_*`；Worker DB 租约见 `ordersync/lease.go`。
- 分页：`partial_success` 时仅重试失败页（`RetryPages`）。
- 同步后可选 `DeductInventoryForOrder`；扣减失败记日志，不阻塞订单 upsert 成功计数。
- 平台 Provider 错误映射为 `DOUYIN_ORDER_*` 等码，进入失败任务中心。

## Webhook 共享规则

Webhook 入站（`webhook.Service.Ingest`）与订单同步共用防重思想：

| 层 | 机制 |
| --- | --- |
| 领域表 | `webhook_events (platform, event_id)` 唯一；`ON CONFLICT DO NOTHING` |
| 幂等表 | `scope=webhook`，`key=webhook:{platform}:{eventId}` |
| ACK | 重复事件 `duplicate=true`，快速返回已存 `status` |

无 `eventId` 时用 payload SHA-256 派生。Payload 上限 **1 MB**。

后续异步处理订单类 webhook 时，应使用与 `UpsertSyncedOrders` 相同的 `external_order_id` 键空间。

## 任务状态与断点

- `partial_success`：`pageErrors` 记录失败页；`hasMore` 且达 `maxPages` 亦为部分成功。
- Checkpoint 字段：`totalFetched`、`createdOrders`、`updatedOrders`、`nextPage` 等。
- 重试 API：`POST /api/v1/order-sync/tasks/:id/retry`（须 `SafeRetry`）。

## 验收要点

1. 同一 `external_order_id` 连续同步两次 → 仅一条订单，`updated` 计数增加。
2. 并发双 Worker 同步同一订单 → 唯一索引 + 事务保证无重复行。
3. Webhook 重复投递 → `duplicate=true`，不重复写 `webhook_events`。
4. 迁移前故意留重复数据 → 启动 fail-fast 并给出 sample IDs。
