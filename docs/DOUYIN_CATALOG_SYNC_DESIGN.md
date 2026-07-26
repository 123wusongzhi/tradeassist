# 抖店类目同步设计

## API

| 功能 | 方法 | 参数 |
|------|------|------|
| 获取类目树 | `shop.getShopCategory` | shop_id |
| 获取类目属性 | `product.getCatePropertyV2` | category_id |

## 数据流

```
GetCategories()
  → shop.getShopCategory
  → 返回 []Category{ID, ParentID, Name, IsLeaf}

GetCategoryAttributes(categoryID)
  → product.getCatePropertyV2
  → 返回 []CategoryAttribute{PropertyID, Name, Required, Options}
```

## 缓存策略

- 类目树变化少，建议 Redis 缓存 1 小时
- 属性列表按 category_id 缓存 30 分钟

## 字段映射

| Douyin 字段 | 内部字段 | 说明 |
|------------|---------|------|
| category_id | PlatformCategoryID | |
| leaf | IsLeaf | true 表示可挂商品的叶子类目 |
| property_id | AttributeID | |
| option_value_id | OptionID | |
| required | Required | 是否必填属性 |

## 注意事项

- 品牌列表 `blocked_by_contract_verification`：`brand.go` 返回显式不支持错误
- `standard_brand_id` 字段通过商品映射写入，不从品牌列表匹配
