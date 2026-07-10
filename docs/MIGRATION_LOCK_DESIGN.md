# 数据库迁移锁设计（P2）

> 多 API 实例并行启动时，通过 **PostgreSQL Advisory Lock** 保证 `AutoMigrate` 串行执行，避免 DDL 竞态。

## 实现位置

- `backend/internal/database/migration_lock.go`
- 启动入口：`cmd/server/main.go`

```go
database.RunMigrateWithLock(ctx, db, timeout, database.AutoMigrate)
```

## Advisory Lock 键

使用 **两个 int32** 作为 `pg_advisory_lock` 键（会话级锁）：

| 键 | 值 | 说明 |
| --- | --- | --- |
| `migrationLockKey1` | `8837291` | 项目固定盐值 |
| `migrationLockKey2` | `20260710` | P2 可靠性阶段标识（2026-07-10） |

```sql
SELECT pg_try_advisory_lock(8837291, 20260710);  -- 非阻塞尝试
-- 迁移完成后
SELECT pg_advisory_unlock(8837291, 20260710);
```

`pg_try_advisory_lock` 失败时不阻塞连接，应用层循环重试。

## 执行流程

```text
启动 → MIGRATION_RUN_ON_STARTUP?
  ├─ true  → RunMigrateWithLock
  │           ├─ postgres: try lock → 成功则 AutoMigrate → unlock (defer)
  │           └─ 失败: 每 500ms 重试至 timeout
  └─ false → 直接 AutoMigrate（无锁，适合外部已迁移）
```

### 超时

- `MIGRATION_LOCK_TIMEOUT_SECONDS`（默认 **120**）。
- Context 与 lock wait 共用该超时。
- 超时错误：`migration lock: timeout waiting for advisory lock`。

### 非 PostgreSQL

`db.Dialector.Name() != "postgres"` 时 **直接** `run(db)`，不加锁。

MySQL 部署约定：单窗口迁移或 CI 专用迁移 Job。

## 环境变量

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `MIGRATION_RUN_ON_STARTUP` | `true` | 启动时 AutoMigrate |
| `MIGRATION_LOCK_TIMEOUT_SECONDS` | `120` | 等待锁最长时间 |

关闭启动迁移（大型生产变更）：

```env
MIGRATION_RUN_ON_STARTUP=false
```

须改为发布流程中 **单次** 执行迁移（仍可使用同 advisory 脚本）。

## P2 迁移内容

`migrateP2Reliability` 在 `AutoMigrate` 链中：

- 表：`idempotency_records`、`webhook_events`
- 索引：客服 `client_message_id`、webhook 唯一、幂等状态/租约索引

与 Phase 10.2 订单唯一索引、刊登批次索引 **独立**，均在 AutoMigrate 序列内顺序执行。

## 运维

滚动发布可并行启动；holder 崩溃会话结束自动释锁。监控 `database_migrate_failed` 与 lock timeout。
