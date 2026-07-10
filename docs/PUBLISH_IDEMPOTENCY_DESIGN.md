# 刊登幂等设计（P2）

> 多商品批量刊登、单任务 create-draft 均需防止重复创建子任务与重复调用平台 API。

## 两层幂等

| 层级 | 存储 | 粒度 |
| --- | --- | --- |
| 批次 | `product_publish_batches.idempotency_key` | 一次向导提交 |
| 子任务 | `product_publish_tasks.input.idempotencyKey` | 商品 × 平台 × 店铺 × 生效配置 |

统一幂等键（可选接入 `idempotency_records`）：

```text
idempotency.PublishDraft(shopId, productDraftId, publishVersion)
→ publish-draft:{shopId}:{productDraftId}:{publishVersion}
```

## 批次 Key 生成

`batchIdempotencyKey(adminId, productIds, targets, commonConfig, overrides)`：

```text
publish-batch:{adminId}:{configHash}
```

- `productIds`、targets（`platform:shopId`）**排序后**哈希，顺序无关。
- `configHash` 纳入 `commonConfig` 与四层 `overrides`。
- 集成测试验证乱序 productIds 产生相同 key。

### 活跃批次唯一索引

```sql
CREATE UNIQUE INDEX ux_publish_batches_idempotency_active
 ON product_publish_batches (idempotency_key)
 WHERE idempotency_key <> '' AND status NOT IN ('failed','cancelled');
```

迁移前检查重复 `idempotency_key`；有则阻塞并需人工清理。

### 重复提交行为

1. 创建前查询 `idempotency_key` 且 status ∉ `{failed, cancelled}`。
2. 命中 → 返回已有批次响应（`batchCreateResponseFromExisting`）。
3. `Create` 遇唯一冲突 → 再次查询返回已有批次（竞态兜底）。

## 子任务 Key 生成

`taskIdempotencyKey(productId, platform, shopId, effectiveConfig)`：

```text
publish-task:{productId}:{platform}:{shopId}:{configHash(effectiveConfig)}
```

`effectiveConfig` 为统一配置与覆盖深度合并结果；配置变化产生新 key，允许合法二次刊登。

## 成功任务去重

`findExistingSuccessfulTask`：同 product + platform + shop + effectiveConfig 且已成功 → 批量创建时 **引用已有任务**，不新建、不调平台 API。

`retry-failed` 仅针对失败/取消项；并发 claim 防止双 Worker 重试同一子任务。

## 单商品 create-draft

商品详情「创建抖店草稿」走 `product_publish_tasks` 队列：

- Worker DB 租约 + `PRODUCT_PUBLISH_TASK_TIMEOUT_SECONDS`。
- 抖店 Phase 10.2：`product.detail` 按 `out_product_id` 回查，避免超时后重复 `product.addV2`。

## 验收与上限

- 批次上限 env：`PUBLISH_BATCH_MAX_PRODUCTS=100`、`MAX_TARGETS=20`、`MAX_TASKS=300`。
- 连点同参数 → 同一 `batchId`；改 overrides → 新 key；`failed` 批次不阻塞新提交。
- 详见 `MULTI_PLATFORM_PUBLISHING_DESIGN.md`、`PUBLISH_BATCH_MIGRATION.md`。
