# 任务租约与心跳设计（P2.1）

> P2.1 引入共享包 `backend/internal/pkg/tasklease`，在 P2 既有 `locked_by` / `locked_until` / `lock_version` 基础上增加 **执行身份** 与 **心跳时间戳**，防止多 Worker 陈旧写入。

## 设计目标

1. **单任务单执行者**：同一 `task_id` 在 `running` 状态最多一个有效 execution。
2. **可检测租约丢失**：Worker 写结果前必须验证仍持有当前 `execution_id` + `lock_version`。
3. **可恢复**：租约或心跳过期后，其他 Worker 可接管（Takeover）或 Reaper 标记失败/重试。
4. **与业务幂等正交**：任务租约管 **队列消费**；`idempotency.Service` 管 **业务副作用**。

## 数据字段

各异步任务表（P2.1 起）扩展：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `locked_by` | string | Worker 实例 ID |
| `locked_until` | timestamptz | 租约截止时间 |
| `lock_version` | int | 每次 claim / takeover **+1**，乐观锁 |
| `heartbeat_at` | timestamptz | 最近一次续租时间 |
| `execution_id` | uuid (varchar36) | 本次 claim 生成的执行身份 |

迁移：`database/migrate_p2_1.go`（PostgreSQL `ADD COLUMN IF NOT EXISTS` + 索引）。

## API（`tasklease` 包）

| 函数 | 用途 |
| --- | --- |
| `TryClaim` | 从 `pending` 原子 claim 为 `running`，分配新 `execution_id`，递增 `lock_version`，写 `heartbeat_at` |
| `RenewHeartbeat` | 按 `execution_id` + `lock_version` 延长 `locked_until` 并刷新 `heartbeat_at` |
| `ValidateLease` | 写结果前校验仍持有有效租约 |
| `StartRenewal` | 后台 goroutine，每 `TTL/3`（最小 5s）调用 `RenewHeartbeat` |
| `TakeoverExpired` | 租约与心跳均过期时由新 Worker 接管 |

Claim 条件（简化）：

```text
status = pending AND (locked_by IS NULL OR locked_until < now)
```

Renew / Validate 条件：

```text
status = running
AND locked_by = worker
AND execution_id = ?
AND lock_version = ?
AND locked_until >= now
```

## 模块接入（P2.1）

| 模块 | 文件 | 默认 TTL 来源 |
| --- | --- | --- |
| 订单同步 | `ordersync/lease.go` | `ORDER_SYNC_TASK_TIMEOUT_SECONDS` |
| 库存同步 | `inventory/lease.go` | `INVENTORY_SYNC_TASK_TIMEOUT_SECONDS` |
| 商品刊登 | `productpublish/lease.go` | `PRODUCT_PUBLISH_TASK_TIMEOUT_SECONDS` |

典型 Worker 流程：

```text
BRPOP queue
  → TryClaim(taskID)
  → StartRenewal(...)   // defer stop()
  → 执行业务（可并行 idempotency.Acquire）
  → ValidateLease / finish*Task(updates)  // WHERE execution_id + lock_version
  → 更新 status=succeeded|failed
```

`finish*Task` 使用条件更新；`RowsAffected == 0` 时返回 `tasklease.ErrLeaseLost`，**不得**继续写平台或库存。

## 与 Worker Registry 的关系

- **任务行级**：`heartbeat_at` / `locked_until` — 单条任务执行权。
- **进程级**：`worker_instances.last_heartbeat_at` — 运维可见 Worker 存活（见 `TASK_RELIABILITY_DESIGN.md`）。

两者互补：进程崩溃但任务租约未过期时，任务仍受 DB 租约保护；租约过期后 Reaper 或新 Worker 可恢复。

## 配置

相关 env 见 `.env.example`：

- `ORDER_SYNC_TASK_TIMEOUT_SECONDS`
- `INVENTORY_SYNC_TASK_TIMEOUT_SECONDS`
- `PRODUCT_PUBLISH_TASK_TIMEOUT_SECONDS`
- `WORKER_REAPER_ENABLED` / `WORKER_REAPER_INTERVAL_SECONDS`
- `WORKER_LEGACY_RUNNING_TIMEOUT_SECONDS`

## 后续模块迁移

`collect_tasks`、`image_tasks`、`customer_message_sync_tasks` 已在 P2.1 迁移中增加列；Worker 仍可使用模块内 legacy lease 逻辑，后续可统一迁移至 `tasklease` 包（见接入矩阵 `partial` 行）。

## 相关文档

- [`STALE_WORKER_PROTECTION.md`](STALE_WORKER_PROTECTION.md)
- [`CONCURRENT_WRITE_SAFETY.md`](CONCURRENT_WRITE_SAFETY.md)
- [`MULTI_INSTANCE_SAFETY.md`](MULTI_INSTANCE_SAFETY.md)
- [`TASK_RELIABILITY_DESIGN.md`](TASK_RELIABILITY_DESIGN.md)
