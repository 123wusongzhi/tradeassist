# 抖店商品草稿创建幂等性设计

## 关键约束

- `commit=false`：只创建草稿，不自动上架
- `start_sale_type=1`：保持下架状态

## 幂等 Key

```
douyin-product-draft-create:{shopId}:{productDraftId}:{publishVersion}
```

在 `idempotency/keys.go` `DouyinProductDraftCreate` 函数生成。

## 流程

```
1. Acquire idempotency key
   DecisionAlreadySucceeded → 复用已有 platform_product_id
   DecisionInProgress → 等待/轮询

2. 调用 product.addV2 (commit=false)

3. 成功
   → Complete idempotency record
   → 持久化 platform_product_id 到 product_publications

4. 超时（unknown_result）
   → tryRecoverDouyinDraftFromPlatform(outer_product_id)
     → 通过 product.detail?outer_product_id 查询平台
     → 找到 → 视为成功，Complete
     → 未找到 → ManualReviewRequired=true
   → Fail idempotency record (retryable=false)

5. 平台校验失败（validation / contract_mismatch）
   → Fail idempotency record (retryable=false)
```

## 测试约定

草稿创建测试使用 `testdata/douyin/product_draft_create.json` synthetic fixture。
真实 E2E 测试需真实凭证，当前已推迟。
