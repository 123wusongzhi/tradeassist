# P3 抖店适配器审计矩阵

> 状态：代码已实现 / 真实凭证 E2E 验证已推迟  
> 最后更新：2026-07-11

## 矩阵说明

| 列名 | 含义 |
|------|------|
| 能力 | P3 要求的业务能力 |
| 现有代码入口 | 主要实现文件 |
| 现有 API DTO | 请求/响应 Go 类型 |
| 现有字段映射 | 关键字段映射注释 |
| 当前是否真实调用 | 是/否（需真实凭证） |
| 当前幂等方式 | 幂等保护机制 |
| 当前错误分类 | ErrorClass / Error.Code |
| 当前任务类型 | failureclassifier task type |
| 当前权限检查 | 权限校验位置 |
| 缺失能力 | blocked_by_contract_verification 项目 |
| 计划修改文件 | P3 新增/修改文件 |

---

## 完整矩阵

| 能力 | 现有代码入口 | 现有 API DTO | 现有字段映射 | 当前是否真实调用 | 当前幂等方式 | 当前错误分类 | 当前任务类型 | 当前权限检查 | 缺失能力 | 计划修改文件 |
|------|-------------|------------|------------|----------------|------------|------------|------------|------------|---------|------------|
| OAuth 授权流程 | `shop/douyin_oauth.go` | `DouyinAuthorizeURLResult` | state → Redis + DB | 否（需真实凭证） | Redis state + DB DouyinOAuthState | `auth_error` | `douyin_oauth_state_invalid` | admin JWT | redirect_uri 白名单校验 | `douyin_oauth_state.go`, `migrate_p3_douyin.go` |
| Token 刷新 | `douyinshop/client.go` EnsureFreshAccessSingleflight | `TokenBundle` | AccessToken/RefreshToken → shop_auth_tokens | 否 | singleflight + TokenVersion | `auth_error` | `douyin_token_refresh_failed` | shop auth status | TokenVersion 版本冲突检测 | `token_lock.go`, `errors.go` |
| 店铺信息 | `douyinshop/shop.go` GetShopInfo | `ShopInfo` | platform_shop_id → shops.external_shop_id | 否 | — | `auth_error` / `permission` | — | shop auth check | — | `facade.go` |
| 类目同步 | `douyinshop/category.go` GetCategories | `Category`, `CategoryAttribute` | category_id → platform_categories | 否 | — | `platform_api_error` | — | — | — | `facade.go` |
| 图片上传 | `douyinshop/image.go` UploadImage | `UploadImageRequest`, `PlatformImage` | SourceURL → image_id | 否 | DouyinImageAsset content hash | `unknown_result` / `platform_api_error` | `douyin_image_upload_failed` | shop auth | 内容哈希去重 | `douyin_image_asset.go`, `idempotency/keys.go` |
| 商品草稿创建 | `douyinshop/product.go` CreateProductDraft | `CreateProductDraftRequest`, `PlatformProductResult` | commit=false, start_sale_type=1 | 否 | DouyinProductDraftCreate key | `unknown_result` | `douyin_draft_create_failed` | gray release check | unknown_result 恢复路径 | `productpublish/douyin_create.go`, `idempotency/keys.go` |
| 商品详情 | `douyinshop/product.go` GetProductDetail | `PlatformProductDetail`, `PlatformProductSKU` | spec_prices → SKU stock | 否 | — | `not_found` | — | — | — | `facade.go` |
| 订单列表同步 | `douyinshop/order.go` SyncOrdersPage | `orderSearchListData` | shop_order_list → PlatformOrder | 否 | ordersync idempotency | `rate_limited` / `auth_error` | `douyin_order_sync_failed` | shop auth | DouyinSyncCursor | `douyin_sync_cursor.go` |
| 订单详情 | `douyinshop/order_detail.go` GetOrderDetail | `OrderDetail` | order.orderDetail → shop_order_id | 否 | — | `contract_mismatch` / `not_found` | `douyin_order_detail_failed` | — | — | `order_detail.go` |
| 库存推送 | `douyinshop/inventory.go` SyncInventory | `SyncInventoryRequest` | sku.syncStock: product_id, sku_id, stock_num | 否 | inventory push idempotency | `unknown_result` | `douyin_inventory_push_failed` | shop auth | — | `facade.go` |
| 库存查询 | `douyinshop/inventory_query.go` GetSKUStockFromDetail | via product.detail | spec_prices.stock → int | 否 | — | `not_found` | — | — | 无独立 stock API | `inventory_query.go` |
| 客服消息 | `douyinshop/customer.go` | `CustomerCapability` | — | **否（blocked_by_contract_verification）** | — | `contract_mismatch` | `douyin_customer_contract_mismatch` | — | IM 接口合同需人工核查 | `customer.go` |
| Webhook 签名 | `douyinshop/webhook_sign.go` | `DouyinSignatureVerifier` | SHA1(appSecret+body) | 是（本地验证，无网络） | — | `validation` | `douyin_webhook_signature_failed` | app_secret 配置检查 | — | `webhook_sign.go`, `douyin_verifier.go` |
| Webhook 事件路由 | `webhook/douyin_handler.go` | `NormalizedWebhookEvent` | tag → event_type | 是（本地，无网络） | webhook idempotency | — | `douyin_webhook_dispatch_failed` | webhook registry | — | `douyin_handler.go`, `webhook_events.go` |
| 品牌列表 | `douyinshop/brand.go` GetBrandList | — | blocked_by_contract_verification | **否** | — | `contract_mismatch` | — | — | 需平台合同 | `brand.go` |
| Provider 健康检查 | `douyinshop/health.go` | `ConfigHealthChecker`, `TokenMetaHealthChecker` | 只读元数据 | 是（本地） | — | `not_configured` / `auth_error` | — | — | — | `health.go` |

---

## 关键阻断项（blocked_by_contract_verification）

1. **客服消息 API** — IM 接口需通过抖店开放平台合同申请
2. **品牌列表 API** — 需合同授权；当前使用 standard_brand_id 代替
3. **真实凭证 E2E** — 所有涉及 OpenAPI 网络调用的能力均待真实 App Key/Secret + 店铺授权后验证

## 注意事项

- `commit=false` — 所有草稿创建均为草稿箱保存，不自动上架
- `unknown_result` — 超时后写入结果未知时，设置 `UnknownResult=true, SafeRetry=false, ManualReviewRequired=true`
- `token_version` — 多实例 token 刷新竞争保护
- 所有敏感字段（app_secret, access_token）不出现在日志
