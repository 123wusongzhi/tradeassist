# 抖店订单 Webhook 统一 Upsert 设计

## 统一入口

```go
order.Service.UpsertPlatformOrder(ctx, PlatformOrderUpsertInput)
```

`source` 取值：`polling` | `webhook` | `manual_sync` | `reconciliation`

## 链路

```text
Webhook Receiver → 验签 → Ingest → ProcessEvent
  → HandleDouyinPlatformEvent → MapDouyinOrderWebhookEvent
  → UpsertPlatformOrder → MatchOrderItemsForOrder（副作用，幂等保护）
```

轮询同步：

```text
ordersync.ProcessQueuedTask → ToSyncedPayloads → UpsertPlatformOrders(source=polling)
```

## 唯一键

`shop_id + platform + external_order_id`（租户字段预留于 `Order.TenantID`）

## 禁止

- Webhook Handler 直接写 orders 表
- 绕过 idempotency.Service
- Webhook 与轮询各写一套逻辑
## P3.2 Tenant/Shop Scope

Order webhooks must use resolver output already stored on the webhook event. `HandleDouyinOrderEvent` validates `tenantId`, `internalShopId`, and `platformShopId` before unified upsert. `UpsertPlatformOrder` receives both `TenantID` and `PlatformShopID`; raw order summaries may include non-sensitive platform shop identifiers, but never secrets or tokens.

The webhook handler must not write `orders` directly, bypass `idempotency.Service`, fork polling/webhook upsert logic, or infer the shop from "the only authorized shop".
