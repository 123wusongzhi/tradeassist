# 并发写安全（P2.1）

> 多实例 API + 多 Worker 部署下，TradeMind 通过 **DB 唯一约束、乐观锁、任务 execution 身份、统一幂等** 四层组合保证关键写路径安全。

## 四层模型

```text
┌─────────────────────────────────────────────────────────┐
│ L4  idempotency_records (scope + key, request_hash)      │  业务语义防重 + 重放
├─────────────────────────────────────────────────────────┤
│ L3  task execution (execution_id + lock_version)         │  单任务单 Worker 写终态
├─────────────────────────────────────────────────────────┤
│ L2  领域唯一键 (platform order id, business_event_key…)   │  最后一道 DB 防线
├─────────────────────────────────────────────────────────┤
│ L1  事务 + SELECT FOR UPDATE (幂等 Acquire)              │  记录级串行化
└─────────────────────────────────────────────────────────┘
```

## 各层职责

### L1：幂等 Acquire 事务锁

`idempotency.Service.Acquire` 在 `scope + idempotency_key` 上使用 `SELECT … FOR UPDATE`，保证并发请求对同一键串行决策（processing / succeeded / conflict）。

### L2：领域唯一约束

| 表 / 索引 | 保护对象 |
| --- | --- |
| `ux_idempotency_scope_key` | 幂等记录 |
| 平台订单唯一（按 shop + platform_order_id） | 订单导入 |
| `ux_inventory_change_business_event_key` | 库存变更台账 |
| `product_publish_batches.idempotency_key` | 刊登批次 |
| `webhook_events (platform, event_id)` | Webhook |

P2.1 迁移：`migrate_p2_1.go` 增加 `business_event_key` 部分唯一索引。

### L3：任务 execution 身份

Claim 时递增 `lock_version` 并分配新 `execution_id`。终态更新必须匹配当前 execution，否则 `tasklease.ErrLeaseLost`。

适用：**订单同步、库存同步、商品刊登** Worker（P2.1 已接 `tasklease`）。

### L4：业务幂等 Complete 摘要

成功后写入 `response_summary` / `resource_id`，重复请求返回 **相同业务结果** 而非重复副作用。

## 典型并发场景

### 双 API 同时创建订单同步任务

1. 两者 `Acquire(order_sync, order-sync-job:…)`；
2. 一个 `Acquired`，另一个 `InProgress` 或 `AlreadySucceeded`；
3. 返回已有 task DTO，不创建第二条任务。

### 双 Worker 消费同一库存同步任务

1. `TryClaim` 仅一个成功；
2. 失败者 `RowsAffected=0`，放弃执行；
3. 成功者续租并在 `finishInventorySyncTask` 带 execution 条件。

### 订单导入 + 库存扣减重试

1. 导入：`order_import` 键 + 平台订单 unique；
2. 扣减：`inventory-deduct` 键 + `business_event_key`；
3. 重试导入 → 重放 summary；重试扣减 → `AlreadySucceeded` 跳过。

### 客服双发同一 clientMessageId

`customer-send:{conversationId}:{clientMessageId}` → 第二次 Acquire 重放或 `InProgress`。

## 开发检查清单

- [ ] 新写路径是否定义稳定 scope/key？
- [ ] 是否在 router 注入 `Idempotency`？
- [ ] 成功/失败是否调用 `Complete` / `Fail`？
- [ ] 异步任务是否 `TryClaim` + `StartRenewal` + 条件 finish？
- [ ] 是否有领域 unique 作为兜底？
- [ ] 是否运行 `node scripts/p2-1-domain-idempotency-check.mjs`？

## 测试

- `idempotency/concurrency_test.go` — 并发 Acquire
- `tasklease/lease_test.go` — 双 Worker claim 互斥

## 已知 partial 区域

- `collect` / `image` / `customer_message_sync`：DB 列已备，Worker 仍部分 legacy lease（见 [`P2_1_IDEMPOTENCY_ADOPTION_MATRIX.md`](P2_1_IDEMPOTENCY_ADOPTION_MATRIX.md)）。
- AI 应用（apply）路径：键已预留，幂等待后续阶段。

## 相关文档

- [`IDEMPOTENCY_DESIGN.md`](IDEMPOTENCY_DESIGN.md)
- [`DOMAIN_IDEMPOTENCY_INTEGRATION.md`](DOMAIN_IDEMPOTENCY_INTEGRATION.md)
- [`MULTI_INSTANCE_SAFETY.md`](MULTI_INSTANCE_SAFETY.md)
