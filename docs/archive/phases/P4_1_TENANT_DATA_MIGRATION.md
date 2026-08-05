# P4.1 Tenant Data Migration

Non-destructive strategy for reconciling legacy `tenant_id = 0` rows before strict multi-tenant enforcement.

## Status Banner

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**

---

## 目标与约束

| 原则 | 说明 |
| --- | --- |
| **非破坏性** | 迁移只 UPDATE `tenant_id`（及审计字段）；不 DELETE、不 TRUNCATE |
| **Dry-run 优先** | 每次批量写入前必须产出计数与样本报告 |
| **Advisory lock** | PostgreSQL 上使用与 schema 迁移相同的 advisory lock，避免并发双写 |
| **禁止自动指派首租户** | 不得 `UPDATE … SET tenant_id = (SELECT MIN(id) FROM tenants)` 或类似启发式 |
| **Staging/Production 禁止 dev fallback** | `config.ResolveRequestTenantID` 在 staging/production 拒绝 fallback（`backend/internal/config/tenant_config.go` L84–86） |

---

## 迁移状态分类

每条待处理记录（或整表批次）标记为以下状态之一：

| State | 含义 | 典型场景 | 处置 |
| --- | --- | --- | --- |
| **`resolved`** | 已确定目标租户且已写入（或无需迁移） | JWT 用户创建的商品；webhook 从 shop 继承的订单 | 无操作 |
| **`pending_resolution`** | 已知需要租户，但缺少权威映射 | `products.tenant_id=0` 且 `created_by` 对应单一 admin | 人工或脚本确认后 UPDATE |
| **`ambiguous`** | 多条 equally-valid 租户候选 | 同一 `created_by` 跨租户；shop 已迁移但 product 未关联 | 进入人工队列，**不自动写入** |
| **`system_global`** | 设计上属于全局/平台级 | `settings` 中 `tenant_id=0` 的默认 seed（`settings/*_defaults.go`）；平台 runtime 快照 | 保持 `0`；读取时显式 fallback |

---

## 权威映射规则（优先级从高到低）

1. **Shop 链路** — 若行有 `shop_id`，采用 `shops.tenant_id`（与 `order/platform_upsert.go` `resolveTenantIDForShop` L144–156 一致）
2. **创建者链路** — `created_by` → `admin_users.tenant_id`（仅当 admin 唯一租户时 → `resolved`；否则 → `ambiguous`）
3. **产物链路** — `collect_tasks.result_product_id` / `product_id` → `products.tenant_id`（product 仍为 0 则 → `pending_resolution`）
4. **Webhook 事件** — `webhook_events.tenant_id` 已由 `shop_resolver.go` 解析；下游 order 以 event 为准
5. **无法推断** — 标记 `ambiguous`，保留 `tenant_id=0` 直至人工裁定

**Explicitly forbidden:** 按创建时间、自增 ID、或「第一个租户」分配。

---

## 表级迁移计划

| Table / area | Default state | Resolution rule | Notes |
| --- | --- | --- | --- |
| `products` | `pending_resolution` | `created_by` → admin tenant; collect import 见 `product/service.go` L711 | P4.1 仍可能写入 `0` |
| `product_skus` / `product_images` | inherit product | JOIN `products.id` | 随 product 批次更新 |
| `orders` | `resolved` or `pending` | Prefer `shop_id` → shop tenant | Sync path already resolves L179–180 `sync_platform.go` |
| `shops` | usually `resolved` | OAuth / manual create stamps tenant | |
| `files` | `resolved` (new uploads) | Upload uses JWT tenant L100 `files/service.go` | Legacy rows: object_key prefix `t{id}/` 辅助审计 |
| `settings` | `system_global` | Seeds at `tenant_id=0` in `settings/*_defaults.go` | 租户覆盖行单独 `tenant_id>0` |
| `collect_tasks` / `collect_batches` | `pending_resolution` | No column yet — schema migration first | See `collect/model.go` |
| `ai_tasks` / `image_tasks` | `pending_resolution` | Backfill via `product_id` | No `tenant_id` column today |
| `operation_logs` | mixed | ctx fallback L92–95 `operationlog/service.go` | Reconcile historical `0` via linked resource |
| `webhook_events` | `resolved` | Shop resolver at ingest | |

---

## 执行流程

### 1. Dry-run（只读）

```sql
-- 示例：products 待解析且 created_by 可映射
SELECT p.id, p.created_by, u.tenant_id AS proposed_tenant, COUNT(*) OVER (PARTITION BY p.id) AS dup
FROM products p
LEFT JOIN admin_users u ON p.created_by = u.id
WHERE p.tenant_id = 0;
```

产出报告字段：`table`, `row_id`, `proposed_tenant_id`, `state`, `rule`, `conflict_reason`.

CLI / job 应支持 `--dry-run`（计数 + CSV/JSON），**零行 UPDATE**。

### 2. Advisory lock（写入阶段）

复用 `backend/internal/database/migration_lock.go`：

```text
RunMigrateWithLock(ctx, db, timeout, func(tx *gorm.DB) error {
    // batch UPDATE … WHERE id IN (…) AND tenant_id = 0
})
```

- PostgreSQL: `pg_try_advisory_lock(8837291, 20260710)`
- MySQL / SQLite: 单实例约定，无 lock（文档化运维窗口）

锁范围：**单次 migration job**，与 GORM AutoMigrate 互斥，防止并行 backfill 与 schema 变更冲突。

### 3. 批量 apply（仅 `resolved` 行）

```sql
BEGIN;
-- 示例：单租户 admin 映射，且 dry-run 已标记 resolved
UPDATE products p
SET tenant_id = u.tenant_id, updated_at = NOW()
FROM admin_users u
WHERE p.tenant_id = 0
  AND p.created_by = u.id
  AND p.id = ANY(:approved_ids);  -- 来自 dry-run 批准列表
COMMIT;
```

- 每批 ≤ 500–1000 行，可中断续跑
- `WHERE tenant_id = 0` 保证幂等
- `ambiguous` / `pending_resolution` 不在自动批次中

### 4. 验证

```sql
-- 迁移后：JWT 租户与 product 不一致（strict 模式应为空）
SELECT p.id FROM products p
JOIN admin_users u ON p.created_by = u.id
WHERE p.tenant_id != u.tenant_id AND p.tenant_id > 0 AND u.tenant_id > 0;

-- 仍为 0 的业务行（应只剩 system_global 或 ambiguous 队列）
SELECT 'products' AS tbl, COUNT(*) FROM products WHERE tenant_id = 0
UNION ALL
SELECT 'orders', COUNT(*) FROM orders WHERE tenant_id = 0;
```

配合 `go test ./internal/securitytests/idor/...` 回归。

---

## 环境变量（开发/demo 仅）

| Variable | Purpose | Production |
| --- | --- | --- |
| `ENABLE_DEV_DEFAULT_TENANT` | 允许 JWT `tenant_id=0` 时使用 fallback | **Forbidden** |
| `DEV_DEFAULT_TENANT_ID` | 开发默认租户 | **Forbidden** |
| `ENABLE_DEMO_DEFAULT_TENANT` / `DEMO_DEFAULT_TENANT_ID` | Demo 环境 | **Forbidden** |

Loader: `backend/internal/config/tenant_config.go`. 启动校验：`validateTenantIsolation` L35–46.

Dev fallback **不是** 数据迁移工具；不得用于 backfill 历史 `tenant_id=0` 行。

---

## 人工裁定队列（ambiguous）

建议运维台账字段：

| Field | Description |
| --- | --- |
| `table` / `row_id` | 目标行 |
| `candidates` | JSON 数组 `[{tenant_id, reason}]` |
| `detected_at` | 发现时间 |
| `resolved_at` / `chosen_tenant_id` | 人工确认后 |

确认后才进入 approved_ids 批次；拒绝的行保持 `tenant_id=0` 且 API 层继续拒绝跨租户访问（P4.1 enforcement）。

---

## 与 schema 迁移的关系

| Migration | File | Scope |
| --- | --- | --- |
| P4.1 security schema | `backend/internal/database/migrate_p4_1.go` | Key rotation tables, `files` security columns/indexes |
| Data backfill | **Not auto-run** | 独立 job；遵循本文 dry-run → lock → apply |

`migrate_p4_1.go` 不修改业务 `tenant_id` 值；数据迁移与 DDL 分离。

---

## 相关文档

- `docs/P4_1_TENANT_ENFORCEMENT_AUDIT.md` — 模块 enforcement 缺口
- `docs/P4_1_REPOSITORY_TENANT_ENFORCEMENT.md` — 查询层 helper
- `docs/P4_TENANT_TABLE_MATRIX.md` — 表级 `tenant_id` 矩阵
