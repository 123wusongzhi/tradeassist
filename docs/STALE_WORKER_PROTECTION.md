# 陈旧 Worker 防护（P2.1）

> 当 Worker 进程挂起、网络分区或 GC 停顿时，必须防止 **过期 execution** 继续写入任务结果或触发重复副作用。P2.1 通过 **租约 TTL + 心跳 + execution_id** 三层机制实现。

## 威胁模型

| 场景 | 无防护后果 | P2.1 防护 |
| --- | --- | --- |
| Worker A claim 后长时间 STW | A 恢复后仍写成功，与 B 重复执行 | `locked_until` 过期 + `ValidateLease` 拒绝 |
| Worker A 与 B 同时认为持有任务 | 双写平台/库存 | claim 原子条件 + `execution_id` 唯一有效 |
| 心跳停止但进程未退出 | 假 running 占坑 | `heartbeat_at` + Reaper / `TakeoverExpired` |
| 队列重复投递 | 重复业务副作用 | `idempotency.Service` 业务键 |

## 机制详解

### 1. 租约 TTL（`locked_until`）

- Claim 时设置 `locked_until = now + leaseTTL`（默认约 90s–180s，模块可配置）。
- 续租失败或不再续租 → TTL 到期后该 execution **失效**。

### 2. 心跳（`heartbeat_at`）

- `tasklease.StartRenewal` 每 `TTL/3` 调用 `RenewHeartbeat`。
- 更新 `heartbeat_at` 与 `locked_until`。
- `TakeoverExpired` 要求 `heartbeat_at < staleCutoff`，避免误抢仍健康 Worker。

### 3. 执行身份（`execution_id` + `lock_version`）

每次 claim / takeover 生成新 UUID 并递增 `lock_version`。

结果写入 SQL 必须包含：

```sql
WHERE id = ? AND locked_by = ? AND execution_id = ? AND lock_version = ?
```

实现见各模块 `finish*Task`（如 `ordersync/lease.go`）。

### 4. Reaper 与 legacy 回收

`WORKER_REAPER_ENABLED` 扫描：

- `status = running AND locked_until < now` → 标 `failed` / `retrying` / `lease_expired`。
- 无租约元数据且 `updated_at` 早于 `WORKER_LEGACY_RUNNING_TIMEOUT_SECONDS` → legacy 回收（刊登模块 `RecoverLegacyRunning`）。

### 5. 业务幂等兜底

即使任务租约失效后发生重复执行，关键写路径仍受 `idempotency_records` 保护（见 [`DOMAIN_IDEMPOTENCY_INTEGRATION.md`](DOMAIN_IDEMPOTENCY_INTEGRATION.md)）。

## Worker 侧约定

1. Claim 成功后 **必须** `StartRenewal`，`defer stop()`。
2. 调用第三方 API 前可 `ValidateLease`（长 HTTP 前可选）。
3. 写 DB 终态前 **必须** 条件更新；`ErrLeaseLost` 时：
   - 不调用 Complete/Fail 幂等（若尚未持有 record）或按 record 状态幂等处理；
   - 记录日志，依赖队列重投或人工重试。
4. 禁止在 lease lost 后 **补写** `succeeded` 到任务表。

## 运维信号

| 信号 | 含义 |
| --- | --- |
| 失败任务 `lease_expired` | Worker 未及时续租或进程死亡 |
| 幂等 `IDEMPOTENCY_LEASE_LOST` | 业务编排持锁超时 |
| `/health` workers 块 | 进程级心跳是否正常 |
| `ix_*_heartbeat_at` 索引 | 支持 Reaper 扫描 |

配置状态中心 **P2.1 任务行级心跳与租约** 项（`configstatus/p21_status.go`）标记为 configured。

## 与「Production Ready」边界

Stale worker 防护降低重复写风险，**不**等同于生产验收通过。AI Provider Key、Storage 公网、抖店真实 E2E 等仍可能阻塞最终验收（见 `PROGRESS.md`）。

## 相关文档

- [`TASK_LEASE_AND_HEARTBEAT_DESIGN.md`](TASK_LEASE_AND_HEARTBEAT_DESIGN.md)
- [`CONCURRENT_WRITE_SAFETY.md`](CONCURRENT_WRITE_SAFETY.md)
- [`TASK_RELIABILITY_DESIGN.md`](TASK_RELIABILITY_DESIGN.md)
