# 抖店库存适配器设计

## 写路径（库存推送）

API：`sku.syncStock`

```go
params = {
  product_id:  platformProductID,
  sku_id:      platformSKUID,
  stock_num:   stock,  // >= 0
  incremental: false,  // 全量覆盖
}
```

幂等 key：`inventory-push:{platform}:{shopId}:{skuId}:{stockVersion}`

## 读路径（库存查询）

**无独立 stock 查询 API**（设计决策）：Douyin OpenAPI 标准合同中无专用库存查询接口。

读取方式：通过 `product.detail` 的 `spec_prices` 字段解析 SKU stock 字段。

函数：`GetSKUStockFromDetail(ctx, client, shopID, platformProductID, platformSKUID)`

如果 platformSKUID 为空则返回所有 SKU 库存之和。

## 错误处理

| 错误 | 处理 |
|------|------|
| DOUYIN_INVENTORY_RATE_LIMITED | 等待 Retry-After |
| DOUYIN_SKU_NOT_BOUND | 校验失败，不重试 |
| DOUYIN_PRODUCT_NOT_BOUND | 校验失败，不重试 |
| DOUYIN_UNKNOWN_RESULT（push 超时）| ManualReviewRequired=true |
