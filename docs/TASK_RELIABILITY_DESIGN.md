# 任务可靠性设计（P2 / P2.2）

> 异步任务（采集、图片、订单同步、客服同步、刊登、库存同步）统一采用 **DB 租约 + 心跳续期 + 重试策略 + 死信** 模式。
> **Phase P2.2**：上述六个 Worker 均已接入共享 `tasklease`（`TryClaim` / `TryClaimPendingOrRetrying`、`execution_id`、心跳续租、`ValidateLease` + `finish*Task` 守卫写回）。矩阵见 [`P2_2_WORKER_LEASE_ADOPTION_MATRIX.md`](P2_2_WORKER_LEASE_ADOPTION_MATRIX.md)。

## 任务租约字段

各任务表共有：

| 字段 | 说明 |
| --- | --- |
| `locked_by` | Worker 实例 ID（`worker.GenerateWorkerID` / Registry） |
| `locked_until` | 租约到期 UTC 时间 |
| `lock_version` | 乐观锁版本，claim 时 `+1` |

Claim 条件：`status` 为 pending/retrying（且 `next_retry_at` 到期），且 `locked_by IS NULL OR locked_until < now`。

## 心跳与续期

- 租约 TTL 来自各队列 env（如 `COLLECT_TASK_TIMEOUT_SECONDS`、`ORDER_SYNC_TASK_TIMEOUT_SECONDS`）。
- 采集租约 ≥ `COLLECTOR_TIMEOUT_SECONDS + 60s` 余量。
- 后台 goroutine 每 **TTL/3**（最小 5s）刷新 `locked_until`。
- Worker 进程通过 `worker.Registry` 写 `worker_instances.last_heartbeat_at`（`WORKER_HEARTBEAT_ENABLED`）。

## Reaper（过期回收）

`WORKER_REAPER_ENABLED` 定时扫描：

- `locked_until < now` 且仍 `running` → 标失败/重试或 `lease_expired` 事件。
- `WORKER_LEGACY_RUNNING_TIMEOUT_SECONDS` 清理无租约元数据的历史卡住行。

配置状态中心 **任务租约** 项标记为 ready。

## 重试策略（`taskretry`）

默认 `Policy`：

| 次数 | 退避 |
| --- | --- |
| 1 | 立即 |
| 2 | 30s |
| 3 | 2m |
| 4 | 10m |
| 5 | 30m |

- `MaxAttempts = 5`，`JitterRatio = 0.15`，`MaxDelay = 30m`。
- `ShouldRetry(attempt, retryable)` 控制是否继续。
- HTTP 429 / 5xx / 超时 / 网络错误 → `retryable=true`。
- 权限、校验、幂等冲突 → `retryable=false`。

## 错误分类（节选）

| Code | Retryable |
| --- | --- |
| `timeout`, `network_error`, `provider_5xx`, `rate_limited` | 是 |
| `lease_expired`, `redis_temporary_failure` | 是 |
| `permission_denied`, `validation_failed`, `idempotency_conflict` | 否 |
| `credential_refresh_required` | 否 |

## Dead Letter（`dead_letter`）

- 采集任务定义 `StatusDeadLetter = "dead_letter"`。
- `taskretry.Policy.IsDeadLetter(attempt)`：attempt ≥ MaxAttempts 时进入死信语义。
- 失败任务中心：`DeadLetter=true` 时 `SafeRetry=false`，需人工处理或改参数后新建任务。
- 死信任务保留错误原因与 `retry_count`，不自动入队。

## 与幂等服务的协作

长耗时编排可并行使用 `idempotency.Service`：

1. `Acquire` 获得业务级执行权；
2. 任务租约保证单 worker 消费；
3. 成功 `Complete` 幂等记录，失败 `Fail(retryable)`。

## 运维

- Env：`WORKER_HEARTBEAT_*`、`WORKER_REAPER_*`、`COLLECT_TASK_TIMEOUT_SECONDS` 等见 `.env.example`。
- `/health` 查队列深度；失败任务中心筛 `lease_expired` / `rate_limited`。
- 多实例须 Redis 队列 + DB 租约。
