# 抖店订单同步设计

## API

- 列表：`order.searchList`（分页，page/size，unix 时间戳）
- 详情：`order.orderDetail`（shop_order_id）

## 同步游标（DouyinSyncCursor）

每个店铺 + syncType 维护一个游标行：

```
(shop_id, sync_type) UNIQUE
fields: cursor, window_start, window_end, last_success_at, version
```

`UpsertDouyinCursor`：使用 ON CONFLICT 确保 version 只能向前推进（不回退）。

## 同步策略

| 模式 | 说明 |
|------|------|
| window | 按时间窗口拉取，每次最多 maxPages 页 |
| cursor | 基于上次 cursor 增量拉取 |

## 字段映射

| Douyin 字段 | PlatformOrder 字段 | 说明 |
|------------|-------------------|------|
| order_id | ExternalOrderID | 平台订单 ID |
| order_status | Status | 通过 MapOrderStatus 映射 |
| pay_amount | TotalAmount | 分为单位，转换为元 |
| create_time | OrderedAt | Unix 秒 |

## 错误处理

| 错误类型 | 处理 |
|---------|------|
| DOUYIN_ORDER_RATE_LIMITED | 等待 Retry-After 后重试 |
| DOUYIN_AUTH_EXPIRED | 触发 token 刷新 / 重新授权 |
| DOUYIN_ORDER_PARSE_FAILED | 记录 raw，跳过当前订单 |
