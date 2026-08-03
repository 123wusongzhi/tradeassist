# Provider 扩展机制

TradeMind 通过 Provider 抽象接入第三方和本地能力，避免业务模块直接依赖具体平台或 SDK。

## Provider 类型

```text
AI Provider
Storage Provider
Image Provider
Platform Provider
Collector Provider
```

## AI Provider

用于接入大模型服务。

当前重点：

- **OpenAI**（`openai`）
- **OpenAI-compatible**（`openai_compatible`）
- **DeepSeek**（`deepseek`，Chat Completions）
- **通义千问 / Qwen**（`qwen`，DashScope OpenAI 兼容模式）
- 共享 **`compatclient`** HTTP 实现，各 Provider 负责默认地址、错误码中文化与后续扩展入口
- Prompt 模板、AI 调用记录、标题优化、描述生成、客服建议回复

当前 AI Prompt 模板为实例级设置，不是租户业务数据；其读取和变更仅限 `tenant_id=0` 的全局设置管理员（`settings.manage`）。

后续可扩展：

- DeepSeek / Qwen 专属错误码、多模态、Embedding、Rerank、用量统计
- 多 Provider 配置表（`settings.ai_providers`）
- Doubao、Gemini、Claude、Ollama（亦可经 `openai_compatible` 接入）

## Storage Provider

用于接入文件与对象存储。

当前支持或预留：

- local
- S3
- Cloudflare R2
- MinIO
- Tencent COS
- Aliyun OSS

敏感字段必须加密存储并脱敏展示。

## Image Provider

用于接入图片处理能力。

当前支持或预留：

- noop
- remove.bg
- OpenAI Image
- ComfyUI

图片任务应通过任务状态与队列执行，避免长请求同步阻塞。

`translate_image_text` 采用 OCR → 翻译 → 样式分组 → 确定性渲染链路。OCR 配置统一放在「设置 → 图片 AI 设置」，由图片文字翻译任务读取用户配置，不允许在代码中写死 Provider、服务地址或 API Key。当前下拉只显示生产可用 Provider：`ai_vision`（当前 AI 设置中的视觉模型）、`paddleocr`（本地 PaddleOCR 服务）、`aliyun`（阿里云 OCR）与 `tencent`（腾讯云 OCR）。图片文字翻译采用严格 OCR 模式：用户选择哪个 OCR Provider，任务就必须真实调用该 Provider；OCR 未配置、测试未通过、调用失败或未识别到文字时任务直接失败，不会自动切换到其他 OCR。腾讯云 OCR 支持 `GeneralBasicOCR` / `GeneralFastOCR`，SecretId / SecretKey 加密保存且前端仅脱敏展示；返回的 `TextDetections` 会转换为统一 OCR blocks，低于 `ocr_min_confidence` 的文字块会被过滤。任务详情输出 configuredOcrProvider、actualOcrProvider、ocrBlocksCount、ocrAverageConfidence 与错误信息。设置页提供 OCR 真实调用测试，阿里云与腾讯云都会真实调用服务并校验 blocks 与 bbox。文字会先聚合为 `main_title`、`badge`、`bottom_badge` 等 group，再按 `auto` / `title_badge` / `preserve_original` 等模板排版；黑底标签会重绘圆角胶囊背景，普通文本优先局部擦除并继承原图字重、颜色和对齐，不再默认用白色矩形覆盖所有区域。结果需输出 `renderQuality` 评分，低于商用阈值时标记 `success_with_warnings`。

## Platform Provider

用于接入跨境电商平台能力。

Douyin Shop (`douyin_shop`) Phase 3 adds a reusable OpenAPI client under `backend/internal/providers/platform/douyinshop`. Signing, common request construction, `param_json` body handling, response parsing, error mapping, safe request logging, token auto-refresh, and shop-info calibration are centralized in the provider package. Business services should call this client instead of hand-writing signatures or raw OpenAPI requests. Store connection testing and manual shop-info sync now use a real platform-side token refresh response to update `shops` / `shop_auth_tokens`; App Secret, access token, refresh token, and full sensitive raw responses must never be returned to the frontend or written to logs.

Douyin Shop Phase 4 adds category and category-attribute sync using official-doc-checked OpenAPI methods `shop.getShopCategory` (`/shop/getShopCategory`, recursive from `cid=0`) and `product.getCatePropertyV2` (`/product/getCatePropertyV2`, `category_leaf_id`). Category data is cached in `platform_categories` and attributes in `platform_category_attributes`; raw responses are stored for backend diagnostics but omitted from normal frontend views. Product Detail → Listing saves Douyin listing preparation to `product_platform_publish_configs` (`platform=douyin_shop`, `shopId`, `categoryId`, `categoryPath`, `platformAttributes`) instead of mutating collected raw data. Readiness checks validate store authorization, selected leaf category, required attributes, and stale cache warnings. Phase 4 deliberately does not implement Douyin product publishing, image upload, order sync, or inventory sync.

Douyin Shop Phase 5 adds internal product draft → Douyin listing draft mapping. Mapping is implemented in the product service layer and stored on `product_platform_publish_configs` as preview fields (`mappedTitle`, `mappedDescription`, `mappedImages`, `mappedSkus`, `mappedPrice`, `mappedStock`, `mappingWarnings`, `mappingErrors`, `lastMappedAt`). It supports AI title / AI description priority, main/detail image preview with `need_sync` status for external images, category attributes, SKU specs, price/profit checks, stock confirmation, manual adjustment, save, and readiness validation. Phase 5 still does not call Douyin product creation or image upload APIs; Phase 6 should handle Douyin image upload / image service sync through Provider abstractions.

Douyin Shop Phase 6 adds image upload to the Douyin material center before product draft creation. Product listing drafts now keep extended `mapped_images` entries for `mainImages` / `detailImages`: local image id, source URL, Storage URL/key, Douyin `platformImageId` / `platformImageUrl`, upload status, failed error code/message, upload time, processed flag, and sanitized raw response. External images are downloaded with timeout, size cap, format/dimension validation, and SSRF private-network blocking, then written to the current Storage Provider before calling Douyin. Storage-backed images are read server-side from the configured Storage Provider; frontend URLs, tokens, and secrets are not used for platform calls. The provider method is `UploadImage(ctx, shopID, req)` and uses the Phase 3 `douyinshop.Client` with official-doc-checked method `supplyCenter.material.batchUploadImageSync` (`/supplyCenter/material/batchUploadImageSync`), preserving token auto-refresh and safe logs. Phase 6 does not create Douyin products, sync orders, or sync inventory.

Douyin Shop Phase 7 adds platform product draft creation from saved mapping + uploaded images. The provider method is `CreateProductDraft(ctx, shopID, req)` in `douyinshop/product.go`, calling official-doc-checked `product.addV2` with `commit=false` and `start_sale_type=1` so items stay in the Douyin draft box and are not directly listed online. Payload assembly lives in `productpublish/douyin_payload.go` and reads `product_platform_publish_configs` mapped fields only (never collect raw). Publish tasks reuse `product_publish_tasks` with `publishMode=save_as_platform_draft`; success writes `product_publications` / `product_publication_skus`. Failures classify into the failure task center with codes such as `DOUYIN_CREATE_PRODUCT_FAILED`. Phase 7 does not sync orders or inventory.

Douyin Shop Phase 9.1 adds SKU binding calibration after platform draft creation. Provider method `GetProductDetail(ctx, shopID, platformProductID)` in `douyinshop/product.go` calls official-doc-checked **`product.detail`** with `show_draft=true` to read draft-box SKU lines (`spec_prices` / `sell_properties`). Service layer `productpublish/douyin_sku_binding.go` matches local `product_publication_skus` by attrs → spec name+price → similar (ambiguous); never guesses low-confidence binds. APIs: `GET/POST /api/v1/product-publications/:id/douyin/sku-bindings*`.

Douyin Shop Phase 9.2 adds manual SKU binding fallback for `ambiguous` / `unmatched` rows. APIs: `POST /api/v1/product-publication-skus/:id/douyin/bind-sku`, `POST .../unbind-sku`. Manual bind validates platform ownership, product ID, non-empty platform SKU ID, and conflict with other local specs; sets `bindStatus=bound`, `bindConfidence=100`, `bindMessage=手动绑定`. Unbind clears `external_sku_id` and marks `unmatched`. `GET .../sku-bindings` returns cached `platformSkus` candidates and `inventorySyncReady`. Inventory sync blocks until all SKUs are bind-ready (`DOUYIN_SKU_BINDING_REQUIRED`, `DOUYIN_SKU_BINDING_CONFLICT`, etc.). Operation logs: `douyin.sku.binding.manual_bind/unbind/recheck/conflict`. Next: full Douyin end-to-end acceptance.

Douyin Shop Phase 9 adds inventory sync MVP via existing inventory orchestration (`inventory` module). The provider implements `InventorySyncProvider.SyncInventory` in `douyinshop/inventory.go`, calling official-doc-checked `sku.syncStock` with `product_id`, `sku_id`, `stock_num`, and `incremental=false` (full stock snapshot). Sync is gated by `inventory_sync_enabled` in platform open config (default off). Reuses `POST /api/v1/product-publication-skus/:id/sync-inventory`, `POST /api/v1/products/:id/sync-inventory`, `GET /api/v1/inventory-sync/tasks*`, `POST /api/v1/inventory-sync/tasks/:id/retry`, and inventory sync batch APIs. Missing `product_publication_skus.external_sku_id` (platform SKU ID) is not guessed — returns `DOUYIN_SKU_NOT_BOUND`. Failures classify into failure task center (`DOUYIN_INVENTORY_SYNC_FAILED`, `DOUYIN_INVENTORY_PERMISSION_DENIED`, `DOUYIN_INVENTORY_RATE_LIMITED`, etc.). Operation logs: `douyin.inventory.sync.start/success/failed/retry`, `douyin.inventory.sku.failed`. Phase 9 does not implement multi-warehouse stock, auto-replenish, or scheduled auto sync by default.

Douyin Shop Phase 8 adds order sync MVP via existing order sync orchestration (`ordersync` module). The provider implements `OrderSyncProvider.SyncOrders` in `douyinshop/order.go`, calling official-doc-checked `order.searchList` with `page`, `size`, `create_time_start`, and `create_time_end` (unix seconds). **Phase 8.1** auto-paginates per task (default max **5 pages** or **500 orders**); configure `order_sync_max_pages` in platform open settings or pass `maxPages` on `POST /api/v1/shops/:id/sync-orders`. Per-page failures are recorded in task `output.pageErrors`; mixed success yields `partial_success`. Task output includes `totalFetched`, `totalPages`, `successPages`, `failedPages`, `nextCursor`/`nextPage`, `createdOrders`, `updatedOrders`, `matchedItems`, `unmatchedItems`, and `deductedStockItems`. List response `shop_order_list` / nested `sku_order_list` are mapped to neutral `PlatformOrder` snapshots (amounts converted from fen to yuan; buyer nickname masked; encrypted address fields omitted from raw). Sync is gated by `order_sync_enabled` in platform open config (default off). Reuses `order.UpsertSyncedOrders`, `MatchOrderItemsForOrder`, optional `DeductInventoryForOrder`, order exception workbench for unmatched SKU, and failure task center for sync failures. Phase 8 does not call Douyin inventory APIs, after-sale/refund APIs, or scheduled polling by default.

**Phase 10.4 (Release Candidate observability)** does **not** add Prometheus. Production monitoring reuses `GET /health` queue blocks, task center failures/alerts (`sub:douyin_*`), operation logs, product operations dashboard, and Douyin runtime APIs: `GET /api/v1/platform/douyin/health`, `GET .../metrics-summary` (in-process 24h counters), `GET .../release-gate`, `POST .../run-health-check`, plus `production-preflight` / `runtime-status`. E2E scripts: `scripts/douyin-e2e-*` (exit `3` + `blocked_by_real_credentials` without credentials; write requires `ALLOW_DOUYIN_WRITE_TEST=true`). CI job `backend-race` in `.github/workflows/go.yml`. See [`DOUYIN_RELEASE_GATE.md`](DOUYIN_RELEASE_GATE.md).

Ozon (`ozon`) beta — 店铺级凭证接入（`Client-ID` + `Api-Key`，无需 OAuth），能力：商品刊登 + 店铺信息/连接测试。Provider 位于 `backend/internal/providers/platform/ozon`：

- 授权：在店铺管理中填写 `appKey`（Client ID）与 `accessToken`（Api-Key），加密存储于 `shop_auth_tokens`；连接测试调用 `POST /v1/seller/info`（只读），返回店铺名、币种、国家。
- 凭证与网络边界：Ozon Seller API 主机由 Provider 固定，租户保存的 `authConfig` 不可覆盖请求基址或注入明文凭证；历史 Ozon `authConfig` 不会回传或参与运行。仅进程内受信任测试覆盖可替换基址，禁止接收租户输入。
- 刊登：`PublishProduct` 调用 `POST /v3/product/import` 提交商品 → 轮询 `POST /v1/product/import/info`（`imported` 且 `product_id>0` 才算成功；`failed`/`skipped` 判失败；已导入商品带 error 级提示时记录为警告）→ 可选按仓库 `POST /v2/products/stocks` 写库存。多 SKU 时每个本地 SKU 生成一个 Ozon 商品（`offer_id` 取 SKU 编码），写入 `product_publication_skus.external_sku_id`。商品导入和库存写入禁用 HTTP 传输层自动重试，避免超时或 5xx 后产生隐藏的重复写；只读查询仍保留有界重试。
- 图片要求：Ozon 商品图片必须是 Ozon 服务器可访问的公开 `https://` 图片 URL（导入时直接引用，不经过 TradeMind 中转）。本地/对象存储的图片需先配置公开访问地址（`PublicURL`）后再刊登。
- 类目与属性：商品级 Ozon 类目优先于任何全局预设；`platform_publish_ozon` 中的类目仅是历史 fallback。同步缓存会记录 `added` / `changed` / `deactivated` / `reactivated` 差异；租户可将本地来源类目映射到已确认的 Ozon 叶子类目。商品必须显式保存其动态属性与 schema hash，推荐映射只是候选，绝不静默应用到商品。
- 类目树与尺寸：类目同步按 Ozon 官方树的递归 `children` 结构解析，不依赖固定层级。真实提交前，重量以及深度、宽度、高度均必须为大于 0 的数值；任一缺失或不合法会由发布前校验阻断。
- 复杂属性：平台模板中 `attribute_complex_id > 0` 的属性在导入 payload 中按 complex ID 稳定分组写入 `complex_attributes`；普通属性继续写入顶层 `attributes`，避免把 Ozon 复杂属性错误地当作普通属性提交。
- 同步运行：启用 Redis 时类目同步为异步队列任务，先返回 `pending` / `running` 运行记录；Redis 不可用仅作为开发环境 inline fallback，仍保留同一运行记录生命周期。响应恰好达到 20,000 节点安全上限时运行标记为 `partial`，**不会**因缺失数据停用已有类目；超过上限时 Provider 直接报错且不写入部分树。只有确认完整的响应才能把已消失的缓存类目标记为停用。
- 字典值：类目属性缓存只预取有界字典值，避免大字典无限写入缓存。运营者可按店铺与关键字远程搜索单一字典属性；该调用和发布前精确校验都只访问 Ozon 只读类目/属性接口，绝不调用 `/v3/product/import` 或库存写接口。
- 发布前校验：提交前按店铺重新校验类目活动状态、属性模板、必填动态属性和 schema hash。缺失必填属性、类目停用或 schema 变化会硬阻断真实 Ozon 写入。自动填充可以生成候选属性，但正常商品级流程必须先显式保存配置；Worker 在导入前再次拉取实时模板并比较保存时的 schema hash，变化时不调用商品导入。
- 合同币种：`currency_code` 留空时自动读取卖家合同币种（避免 `currency_differs_from_contract`）；`vat` 默认 `0`（跨境卖家通常 0%，按合同国家规定可改）。
- 刊登预设：`platform_publish_ozon` 保留仓库 `warehouse_id`、币种、VAT、默认品牌/类型/原产国/制造商、重量尺寸与补充属性 JSON 等店铺/实例默认值；不再作为单一固定商品类目来源。
- 状态边界：创建“本地草稿”只在 TradeMind 写入本地记录，明确为“本地草稿已创建，未调用 Ozon”。真实提交必须经用户二次确认，先创建 TradeMind 提交任务；Worker 才可调用 Ozon。仅当 Ozon 返回真实 product ID 后才可声明“Ozon 商品已创建，等待平台审核”。
- 重试与人工核对：Ozon 真实提交强制要求 `Idempotency-Key`；本地草稿服务端按同一商品/店铺/配置复用已有活动记录，避免重复草稿。Ozon 返回 `failed` 或结果未知时，系统不自动重试、不参与批量重试，也不能用新 `Idempotency-Key` 绕过；运营者必须先在 Ozon 与任务中心人工核对真实结果，再决定后续处理。
- 边界：MVP 不做订单同步、库存同步、客服消息与 Webhook；商品导入是创建操作，可归档（`/v1/product/archive`）恢复，不在本版本提供删除/归档操作。

当前重点平台：

- Douyin Shop（抖店，真实平台闭环优先）
- TikTok Shop
- Shopee
- Lazada
- Amazon
- Ozon

当前真实平台接入顺序优先跑通抖店，不要把抖店与 TikTok Shop 混用：抖店统一内部标识为 `douyin_shop`，TikTok Shop 仍代表跨境平台。已完成 Phase 1–10.4（Release Candidate）：平台配置、OAuth、Client/签名、类目属性、字段映射、图片上传、平台商品草稿创建、订单同步 MVP、库存同步 MVP、SKU 绑定校准与手动兜底、生产预检/运行状态、可观测性与 E2E 脚本/CI。**真实 E2E 仍为 `blocked_by_real_credentials`**。下一阶段：有凭证环境全链路验收与灰度观察。

主要能力：

- 店铺授权
- 店铺信息
- 订单同步
- 商品刊登
- 库存同步
- 客服消息同步与人工发送

平台 App Secret、Access Token、Refresh Token 等必须加密存储。

平台开放应用配置与刊登配置均保存在 tenant 0 的实例级设置域。包括返回脱敏值的读取在内，均只允许全局配置管理员；非零租户管理员不能借配置接口推断或读取实例设置。店铺、店铺授权与刊登目标仍严格按当前可信租户查询。

抖店 runtime、health/metrics、release gate、production preflight 及 Storage 公网检测是实例级运维接口，仅全局管理员可访问；其中 pause/resume/emergency-disable、health-check 和 preflight 等写操作仍额外要求配置权限。这些门禁与预检结果不构成生产就绪声明。

平台授权与后台任务还必须遵守租户信任边界：OAuth `state` 同时绑定平台、租户和店铺，回调写入 Token 或授权状态时必须使用 `tenant_id + shop_id` 条件；Provider 调用只接受由已校验店铺恢复出的标准租户上下文。队列中的 task/shop ID 只是定位信息，不能作为租户授权依据，worker 必须重新读取持久化店铺并校验租户后才能执行平台读写。

### SMTP 传输安全

SMTP 隐式 TLS 使用 TLS 1.2+，并以配置的 SMTP 主机名执行证书与主机名验证（不允许跳过验证）。显式 `UseTLS` 时，服务器若不支持 STARTTLS 会直接失败；支持 STARTTLS 时升级失败同样失败。未启用隐式 TLS 的连接在服务器宣告 STARTTLS 时会升级，凭据不完整会被拒绝。

## Collector Provider

用于接入商品采集来源。

当前重点：

- 1688
- 拼多多
- 淘宝/天猫（Playwright + OpenCLI 双引擎）
- AliExpress beta
- 自定义规则采集 beta

采集服务必须输出统一商品结构，包括标题、图片、属性、SKU、描述图与 raw 原始数据。

### Collector Engine 路由

- Playwright Collector 是完整 Node/Playwright 服务，本地和 Docker 均监听 `3001`。
- OpenCLI Bridge 是宿主机轻量进程，监听 `3100`，不加载 `BrowserManager` 或 Playwright。
- backend 在任务创建时解析并持久化 `engine`，worker 按该值选择客户端；运行失败不跨引擎回退。
- OpenCLI 当前仅支持 `taobao_tmall`。Bridge 停止时，1688、拼多多、自定义规则及显式 Playwright 任务不受影响。
- TradeMind OpenCLI 适配器位于 `collector/opencli-adapters/tmall/`，Bridge 启动时仅同步带受管标记的文件，不覆盖用户自己的同名适配器。
- OpenCLI `EMPTY_RESULT` 是登录态、风控、页面结构变化与真实无数据的共同信号，统一归类为可恢复的解析失败；只有明确的 `ITEM_NOT_FOUND` 才归类为商品不存在。
- `GET /api/v1/collect/engines/status` 是管理端唯一的引擎状态来源；Token、CLI 原始输出和本机路径不得通过 API 暴露。

| 来源 | Playwright | OpenCLI |
| --- | --- | --- |
| `taobao_tmall` | 支持，可作为手动备用 | 支持，可在 Bridge 启用后设为默认 |
| `1688`、`pinduoduo`、`aliexpress`、`custom` | 支持 | 不支持 |
| `shein_temu` | 规划中，当前不可创建任务 | 不支持 |

“备用”不等于运行时自动回退。引擎在创建任务时解析并持久化，重试仍使用同一引擎。
部署拓扑、地址选择、迁移和故障排查统一见
[采集引擎与部署指南](collector-engines.md)。

## 扩展建议

新增 Provider 时建议：

1. 先定义接口和统一数据结构。
2. 再实现具体 Provider。
3. 所有外部请求设置超时。
4. 不在日志中输出密钥。
5. 对错误进行可读归因，便于前端展示和任务重试。
6. 必要时同步更新 README、本文档和相关设置页面。

新增 Provider 前请复制或参考 [provider-template.md](provider-template.md)，并按 [module-map.md](module-map.md) 检查 settings、环境变量、API、前端页面、任务队列和文档联动。
