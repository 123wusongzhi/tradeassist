# 抖店商品草稿字段映射

## 创建接口

API：`product.addV2`（commit=false, start_sale_type=1）

## 字段映射表

| 内部字段 | Douyin 字段 | 类型 | 说明 |
|---------|-----------|------|------|
| Title | product_name | string(max 100) | AI 优化后标题 |
| Description | detail_text | string(max 5000) | AI 生成描述，HTML 或文本 |
| CategoryID | category_id | int64 | 必须为叶子类目 |
| Images[].PlatformImageID | image_list[].material_id | string | 先上传获取 |
| SKU.Price | spec_prices[].price | int64 | 分为单位 |
| SKU.Stock | spec_prices[].stock_num | int32 | 库存数量 |
| SKU.ExternalID | spec_prices[].sku_id | string | 内部 SKU ID |
| Attributes | product_properties | []PropertyItem | 类目属性 |
| Weight | delivery_weight | int32 | 克为单位 |

## 不支持字段

| 字段 | 原因 |
|------|------|
| 品牌 | blocked_by_contract_verification |
| 视频 | P3 不实现，P6 规划 |
| 发货时效 | P3 不实现 |

## outer_product_id

每次草稿创建时传入内部 product_draft_id，用于 `tryRecoverDouyinDraftFromPlatform` 查询。

## 注意

- `commit=false`：仅保存草稿，不触发平台审核
- 草稿 product_id 与已上架商品 ID 不同
