# 库存一致性设计（P2）

> 本地库存以 **append-only 台账 + 订单效应表 + 版本化同步任务** 保证可审计、可重试、可去重。

## 核心表

### `inventory_change_logs`（台账）

只追加审计流水，记录每次本地库存变更：

| 字段 | 说明 |
| --- | --- |
| `change_type` | `manual_adjust` / `order_deduct` / `platform_sync` 等 |
| `before_stock` / `after_stock` / `delta` | 变更前后与差值 |
| `ref_order_id` / `ref_order_item_id` | 订单关联 |
| `business_event_key` | **全局唯一**（`uniqueIndex`），防重复记账 |

### `order_inventory_effects`（订单扣减效应）

每个 `(order_item_id, product_sku_id, effect_type)` 唯一（`ux_oie_item_sku_effect`）：

- `deduct` / `restore`；状态 `pending` / `success` / `failed` / `skipped`。
- 成功行关联 `inventory_change_log_id`。
- 已成功扣减的行再次请求 → **跳过**（幂等）。

## `business_event_key` 约定

与 `idempotency` key 对齐，写入台账前设置：

| 场景 | 推荐格式 |
| --- | --- |
| 订单扣减 | `inventory-deduct:{orderId}:{orderItemId}:{skuId}` |
| 订单回滚 | `inventory-restore:{orderId}:{orderItemId}:{skuId}` |
| 手工调整 | `inventory-adjust:{skuId}:{changeLogId}` 或时间戳批次号 |
| 平台推送 | `inventory-push:{shopId}:{skuId}:{stockVersion}` |

唯一约束保证 Worker 重试、API 重复提交不会产生第二条台账。

## 订单扣减流程

`DeductInventoryForOrder`：

1. 按订单行遍历，SKU 行级 `SELECT FOR UPDATE`。
2. 检查 `order_inventory_effects` 是否已有 `success`。
3. 库存不足 → 写 `failed` effect，不部分扣减（可配置负库存策略）。
4. 成功 → `inventory_change_logs` + 更新 `product_skus.stock` + `success` effect。

订单同步完成后可选自动扣减；失败进入订单异常工作台 `inventory_deduct_failed`。

## 版本同步（`stockVersion`）

出站 `inventory_sync_tasks`：

- `input.stockVersion` = `targetStock`（当前目标库存快照）。
- `idempotency.InventoryPush(shopId, skuId, stockVersion)` 作为业务幂等键。
- `lock_version` 参与 Worker claim，防止并发双推。

### Pending/Running 去重

批量创建同步任务时，`blockingPublicationSKUSet` 检测同 `publication_sku_id` 已有 pending/running 任务：

- 默认 **跳过** 并记 `duplicate_pending_running_task`；
- `force=true` 可强制新建（运维场景）。

抖店路径额外要求 SKU 绑定完成（`inventorySyncReady`）。

## 平台推送副作用

`ProcessQueuedTask` 成功/失败后 `appendChange` 写台账；Douyin 等指标经 `douyinmetrics` 计数。

## 与订单同步的边界

- 订单 upsert **不**自动改本地 SKU 库存（除显式策略触发扣减）。
- 库存推送 **不**回写订单状态；两侧通过 `order_inventory_effects` 关联可追溯。

## 验收

重复扣减 skipped；`business_event_key` 唯一防双记账；pending 推送默认去重。API 重试见 `POST /api/v1/inventory-sync/tasks/:id/retry`。
