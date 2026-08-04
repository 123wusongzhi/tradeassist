# API 契约

本文件记录 TradeMind 后端 API 的公共约定。新增、删除或修改接口时，必须同步检查后端 handler / service / DTO、前端 services / types / 页面，以及本文档。

## 基础约定

- 基础路径：`/api/v1`
- 健康检查：`GET /health`、`GET /api/v1/health`（综合）；`GET /health/live`（存活）；`GET /health/ready`（就绪，DB/Redis/迁移/生产门闸）
- 可观测性（P5 / P5.1 / P5-V，需权限）：`GET /api/v1/observability/overview|http|tasks|providers|security`；`overview` 会返回运行态 `runtimeStatus` 与 telemetry 导出摘要，用于区分 `standard_protocol_ready` / `mock_verified` / `real_backend_deferred` / `export_degraded` / `disabled` / `incomplete`；`GET /api/v1/observability/alerts`；`POST /api/v1/observability/alerts/:id/ack|silence`；内部指标：`GET /internal/metrics`（默认仅内网/本机）
- 鉴权：管理端受保护接口使用 `Authorization: Bearer <token>`
- 租户边界：业务资源的租户只从已验证 JWT / 服务端会话、OAuth state 或受信任 Worker 上下文恢复；路径、body、队列消息中的资源 ID 不得自行决定租户。`tenant_id=0` 仅用于显式的系统租户语义，非零租户中的 `admin` 角色不会获得全局管理员能力。
- Fail-closed：依赖的 service、DB 或必要配置不可用时，受保护接口返回 5xx 失败，不得以空列表、空对象或成功响应替代。
- 返回格式：统一 JSON 响应，核心字段为 `code`、`message`、`data`、`traceId`
- 敏感信息：接口不得返回完整 API Key、Token、Secret、Cookie 或密码
- P7-C3 cursor 列表：Product、Order、Inventory Center、Task Center、Webhook Event、Operation Log 支持 `cursor` + `limit`，响应额外返回 `items`、`nextCursor`、`hasMore`、`limit`；旧 `page` / `pageSize` / `list` / `pagination` 兼容保留。超过深 offset 返回 `pagination_offset_too_deep`；cursor 篡改、跨租户/店铺或筛选变化分别返回 `pagination_cursor_signature_invalid`、`pagination_cursor_scope_mismatch`、`pagination_cursor_filter_mismatch`。P7-C4 隔离 Medium PostgreSQL 六类分页 runtime、Query Plan、N+1、Provider 限流、Permission Cache 失效与 Linux Race 证据已关闭；Load/Soak/Regression 仍 pending P7-V2。

## Webhook 入站（公开，无 JWT）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/webhooks/:platform/:eventType` | 平台 Webhook 接收：限体、`Content-Type: application/json`、签名/时间戳校验、幂等持久化、快速 ACK；异步由 DB 轮询 Worker 处理。开发可用 `platform=internal-test`（需 `WEBHOOK_ENABLE_TEST_VERIFIER=true`）。成功 `message=accepted`，`data.eventId` / `duplicate`。 |

## Webhook 事件（管理端）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/webhook-events` | 受保护的 Webhook 事件列表；支持 `platform`、`status`、`eventType`、`shopId`、`start`、`end`、`cursor`、`limit`。只返回元数据、摘要和状态，不返回 `payloadBody` 或签名原文。 |

签名头：`X-Webhook-Signature` 或 `X-TradeMind-Signature`；时间戳：`X-Webhook-Timestamp` / `X-TradeMind-Timestamp`（unix 秒或 RFC3339）。`internal-test` 签名为 HMAC-SHA256 hex（payload = `"{unix}.{rawBody}"`）。失败码含 `WEBHOOK_SIGNATURE_*`、`WEBHOOK_TIMESTAMP_EXPIRED`、`WEBHOOK_PAYLOAD_TOO_LARGE` 等，**不**成功 ACK。

示例：

```json
{
  "code": 0,
  "message": "ok",
  "data": {},
  "traceId": "request-id"
}
```

## 认证

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/auth/login` | 管理员登录，支持邮箱或手机号。 |
| `POST` | `/api/v1/auth/register` | 注册新租户及其 `tenant_admin`；租户与管理员在同一事务中创建，不接受客户端指定租户，也不会创建系统级 `admin`。成功后设置 HttpOnly refresh cookie。邮箱、手机号按规范化值执行全局唯一约束。 |
| `POST` | `/api/v1/auth/refresh` | 使用 HttpOnly refresh cookie 轮换会话并签发短期 access token；refresh token 重用会原子撤销对应会话族。 |
| `POST` | `/api/v1/auth/logout` | 退出登录并撤销当前服务端会话；客户端同时清除内存中的 access token。 |
| `POST` | `/api/v1/auth/logout-all` | 撤销当前管理员的全部服务端会话。 |
| `GET` | `/api/v1/auth/profile` | 当前管理员信息。 |

安全会话模式下，access JWT 必须携带非零 UUID `session_id`，并在每次受保护请求中与服务端会话的管理员、租户和 token version 一并校验；缺失、格式无效、已撤销或不匹配的会话均不被接受。`session_id` 是服务端绑定字段，客户端不得指定或覆盖。

## 设置

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/settings` | 读取系统设置。 |
| `PUT` | `/api/v1/settings` | 保存系统设置，敏感字段必须加密。 |
| `POST` | `/api/v1/settings/test-ai` | 经 **AI Gateway** 测试 `settings.ai`（支持 `openai` / `openai_compatible` / `deepseek` / `qwen`）。各服务商 **`{provider}_api_key` / `{provider}_base_url` / `{provider}_model`** 独立存储；可选 JSON：`provider`、`base_url`、`model`、`api_key`（写入当前 provider 对应项；`****` 占位则沿用已保存密钥）、`timeout_sec`，用于**未保存前**用当前表单试连；空 body 仅用库内配置。成功 `data`：`ok`、`message`、`provider`、`model`、`latencyMs`。 |
| `POST` | `/api/v1/settings/test-storage` | 测试 Storage Provider 配置。 |
| `POST` | `/api/v1/storage/test-public-access` | 上传探针图片并通过匿名 HTTP 验证公网可访问性（HTTPS、`image/*`、无登录跳转）；仅 `tenant_id=0` 全局管理员；失败返回 `STORAGE_PUBLIC_*` 错误码。 |
| `POST` | `/api/v1/settings/storage/public-check` | 同上（P1 别名；仅全局管理员） |
| `GET` | `/api/v1/settings/storage/public-check/latest` | 最近一次公网测试结果（未执行时 `not_run`；仅全局管理员） |
| `POST` | `/api/v1/settings/test-image` | 测试 `settings.image` 图片 Provider 配置。可选 JSON：`provider`、`testMode`（`config_only` \| `live`，默认 `config_only`）、`settings`（表单覆盖项，支持未保存先测；脱敏 `****` 占位符会忽略并沿用已保存密钥）。成功 `data`：`ok`、`message`、`provider`、`latencyMs`、`supportedTasks`、`configStatus`。不返回 API Key。 |
| `POST` | `/api/v1/settings/test-ocr` | 测试 `settings.image` 中的 OCR 配置。可选 JSON：`provider`（`ai_vision` / `paddleocr` / `baidu` / `aliyun` / `tencent`）、`settings`（表单覆盖项，支持未保存先测；脱敏密钥占位符会忽略）。`paddleocr` 会用后端生成的测试图调用 OCR 服务，检查连通性、文字 `blocks` 与 `bbox`；成功 `data`：`ok`、`message`、`provider`、`latencyMs`、`blocks`、`bboxOk`。 |

## 图片 AI

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/image/providers` | 图片 Provider 能力矩阵（`status` / `supportedTasks` / 难度等，不含密钥）。 |
| `POST` | `/api/v1/image/tasks` | 创建当前租户的图片任务；创建时校验关联商品/源文件归属、Provider 与 `taskType` 组合。 |
| `GET` | `/api/v1/image/tasks` | 当前租户的图片任务列表。 |
| `GET` | `/api/v1/image/tasks/:id` | 当前租户的图片任务详情。 |
| `POST` | `/api/v1/image/tasks/:id/retry` | 重试失败任务。 |
| `GET` | `/api/v1/image/tasks/:id/translate-edit-state` | 图片文字翻译人工编辑态：返回原图、已擦除底图、结果图、图片尺寸与可编辑文字块（译文、排版框、擦除框、样式）。 |
| `POST` | `/api/v1/image/tasks/:id/manual-render` | 图片文字翻译人工兜底渲染：按人工编辑后的文字块重新擦除原文并规则重绘译文，结果上传 Storage Provider 并回写任务为 `success_with_review`。 |
| `GET` | `/api/v1/image/tasks/:id/items` | 任务子项列表（源图→结果图、评分 JSON）。 |
| `POST` | `/api/v1/image/tasks/:id/apply` | 将成功任务结果写入 `product_images`（不覆盖原图）。 |
| `GET` | `/api/v1/image/tasks/monitor` | 队列与任务监控快照。 |
| `POST` | `/api/v1/ai/image/tasks` | 创建 AI 图片任务（与 `/image/tasks` 等价）。 |
| `GET` | `/api/v1/ai/image/tasks` | AI 图片任务列表。 |
| `GET` | `/api/v1/ai/image/tasks/:id` | AI 图片任务详情。 |
| `GET` | `/api/v1/ai/image/tasks/:id/translate-edit-state` | 与 `/image/tasks/:id/translate-edit-state` 等价，用于管理端 AI 图片任务页。 |
| `POST` | `/api/v1/ai/image/tasks/:id/manual-render` | 与 `/image/tasks/:id/manual-render` 等价，用于管理端 AI 图片任务页。 |
| `POST` | `/api/v1/ai/image/task-items/:id/save-to-product` | 将任务子项结果保存为新商品图（`applyMode`: main/detail/marketing/ai_generated）。 |
| `POST` | `/api/v1/ai/image/task-items/:id/set-as-main` | 将任务子项结果设为主图（`is_best_main`）。 |
| `POST` | `/api/v1/ai/image/score` | 同步当前租户商品图评分（返回 overall/clarity/cleanliness 等维度）；需要商品写权限。 |

`translate_image_text`（图片文字翻译）读取「设置 → 图片 AI 设置」里的 OCR 配置：`ai_vision` 使用当前 AI 设置中的视觉模型；`paddleocr` 使用本地 PaddleOCR 服务；`aliyun` 会真实调用阿里云 OCR；`tencent` 会真实调用腾讯云 OCR，支持 `GeneralBasicOCR` 与 `GeneralFastOCR`。该任务采用严格 OCR 模式：配置哪个 OCR Provider 就必须实际调用哪个 Provider；OCR 未配置、配置不完整、调用失败或未识别到文字时任务直接失败，不会自动改用其他 OCR。详情输出会包含 `ocr.provider`、`ocr.apiName`、`ocr.configuredOcrProvider`、`ocr.actualOcrProvider`、`ocr.textBlocksCount`、`ocr.averageConfidence`、`ocr.filteredBlocksCount`、`ocr.errorMessage`、`ocr.blocks`、`ocr.groups`、`layout.layoutTemplate` 与 `renderQuality`。每个 OCR block 会补充 `blockClass`、`standardTranslation` 与 `compactTranslation`；顶层会补充 `blockClassifications`、`eraseBBoxCount`、`layoutBBoxCount`、`badgeCount`、`abnormalBadgeCount`、`backgroundPatchScore`、`overlapScore` 与 `finalQualityStatus` 分级：`success`（商用分≥85）、`success_with_review`（75–84，可下载，保存到商品前建议人工检查）、`failed_render_validation`（<65 或中文残留/溢出/遮挡商品主体等硬失败）。调试输出：`debugOriginalUrl`、`debugMaskUrl`、`debugErasedUrl`、`debugFinalUrl`（对应 original/mask/erased/final.png）。65–74 分同任务内自动质量重试一次（`qualityAutoRetried`）。人工兜底使用 `translate-edit-state` 读取可编辑块，再用 `manual-render` 基于原图/已擦除图重新擦除原文并规则重绘译文；输出会记录 `manualEdit`（baseImage、blocks、editedAt、editedBy、eraseMode 等），任务回写为 `success_with_review`。`layout` 还包含 `eraseMode`、`eraseAreaRatio`、`patchAreaRatio`、`flatFillRatio`、`largePatchDetected`、`retryStrategies`、`simulation` 等渲染诊断；顶层同步输出 `configuredOcrProvider`、`actualOcrProvider`、`ocrBlocksCount`、`ocrAverageConfidence`、`detected_source_blocks`、`translated_blocks`、`rendered_blocks`、`target_language_present`、`source_language_residue`、`overflow_blocks`、`style_mismatch_count`、`patch_area_ratio`、`render_quality_score`、`overall_confidence` 便于任务详情和批量排查。`renderQuality` 包含 `textAppliedScore`、`sourceTextRemovedScore`、`layoutScore`、`styleConsistencyScore`、`readabilityScore`、`productPreservationScore`、`commercialUsabilityScore`、`passed` 与 `warnings`；当出现异常 badge、文字重叠、背景补丁、原文残留、版面失衡或商用评分不达标时，任务会以 `low_quality` 返回，不应推荐保存到商品图片或设为主图/详情图。

图片任务、任务子项、关联商品、商品图片与源文件的读取和写入均按当前管理员租户校验；路径 ID 或 body 中的资源 ID 不能跨租户引用。历史任务会在迁移时从可信关联资源回填租户。

## 文件

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/files/upload` | 上传文件到私有隔离区，扫描 clean 后才发布；当前异步隔离仅支持 `public_base` 为空或 `/static`、并由后端状态门禁保护的 local storage，直出目录/CDN 与远端公开对象存储会 fail-closed。 |
| `GET` | `/api/v1/files` | 文件列表。 |
| `DELETE` | `/api/v1/files/:id` | 删除文件。 |

## 商品

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/products` | 商品草稿列表；支持 `operationStep`（`collect_review` / `title` / `description` / `images` / `pricing` / `publish_check` / `ready`）筛选，并在列表行返回轻量 `operationProgress` 摘要。 |
| `POST` | `/api/v1/products` | 创建商品草稿。 |
| `GET` | `/api/v1/products/:id` | 商品详情。 |
| `GET` | `/api/v1/products/:id/operation-progress` | 商品运营进度摘要；只读聚合商品、图片、SKU 与既有发布前检查，不调用平台 API、不创建任务、不修改商品。 |
| `PUT` | `/api/v1/products/:id` | 更新商品草稿。 |
| `DELETE` | `/api/v1/products/:id` | 删除或归档商品。 |
| `POST` | `/api/v1/products/:id/apply-ai-title` | 应用 AI 标题；body 支持 `aiTitle`、`taskId`、`expectedUpdatedAt`、`sourceSnapshotHash`，冲突时返回 `AI_CONTENT_APPLY_CONFLICT`，不会静默覆盖人工修改。 |
| `POST` | `/api/v1/products/:id/undo-ai-title` | 安全撤销最近一次 AI 标题应用；若应用后字段又被人工修改，返回 `AI_CONTENT_UNDO_CONFLICT`。 |
| `POST` | `/api/v1/products/:id/apply-ai-description` | 应用 AI 描述；body 支持 `aiDescription`、`taskId`、`expectedUpdatedAt`、`sourceSnapshotHash`，冲突时返回 `AI_CONTENT_APPLY_CONFLICT`。 |
| `POST` | `/api/v1/products/:id/undo-ai-description` | 安全撤销最近一次 AI 描述应用；若应用后字段又被人工修改，返回 `AI_CONTENT_UNDO_CONFLICT`。 |

**批量 AI 文案（Phase A3.1）**

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/products/ai-text/batches/check` | 创建前检查；返回 `summary` + 每商品×类型 `items`（`ready` / `warning` / `blocked`）。 |
| `POST` | `/api/v1/products/ai-text/batches` | 创建批次；支持 `operationTypes`: `title` / `description`；幂等键 `idempotencyKey`；**不自动应用**。 |
| `GET` | `/api/v1/products/ai-text/batches` | 批次列表。 |
| `GET` | `/api/v1/products/ai-text/batches/:id` | 批次详情 + 复核子项；query `status` 筛选。 |
| `POST` | `/api/v1/products/ai-text/batches/:id/retry-failed` | 重试失败、pending、running 子项（含服务重启后的孤儿项）。 |
| `POST` | `/api/v1/products/ai-text/batches/:id/cancel-pending` | 取消 pending 子项。 |
| `POST` | `/api/v1/products/ai-text/batches/:id/apply-selected` | 批量应用；body `itemIds[]`；逐条冲突保护，`partial_success`。 |
| `POST` | `/api/v1/products/ai-text/batches/:id/undo-applied` | 撤销本批次已应用项。 |
| `POST` | `/api/v1/products/ai-text/items/:id/regenerate` | 单条重新生成。 |
| `POST` | `/api/v1/products/ai-text/items/:id/update-edited-text` | 保存编辑文案。 |
| `POST` | `/api/v1/products/ai-text/items/:id/apply` | 单条应用；冲突 409 + `AI_CONTENT_APPLY_CONFLICT`。 |
| `POST` | `/api/v1/products/ai-text/items/:id/reject` | 放弃建议。 |

设计见 [`BATCH_AI_TEXT_OPERATION_DESIGN.md`](BATCH_AI_TEXT_OPERATION_DESIGN.md)。

### 批量 AI 图片（Phase A3.2）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/products/ai-images/batches/check` | 创建前检查；body 含 `productIds`、`imageIds`、`operationTypes`；返回每图×处理方式 `items`。 |
| `POST` | `/api/v1/products/ai-images/batches` | 创建批次；**不自动应用**；幂等键 `idempotencyKey`。 |
| `GET` | `/api/v1/products/ai-images/batches` | 批次列表。 |
| `GET` | `/api/v1/products/ai-images/batches/:id` | 批次详情 + 复核子项。 |
| `POST` | `/api/v1/products/ai-images/batches/:id/retry-failed` | 重试失败 / pending / running 子项。 |
| `POST` | `/api/v1/products/ai-images/batches/:id/cancel-pending` | 取消 pending 子项。 |
| `POST` | `/api/v1/products/ai-images/batches/:id/apply-selected` | 批量应用；body `itemIds[]`、`applyMode`。 |
| `POST` | `/api/v1/products/ai-images/batches/:id/undo-applied` | 撤销本批次已应用项。 |
| `POST` | `/api/v1/products/ai-images/items/:id/regenerate` | 单条重新处理。 |
| `POST` | `/api/v1/products/ai-images/items/:id/apply` | 单条应用；body `applyMode`；冲突 409。 |
| `POST` | `/api/v1/products/ai-images/items/:id/reject` | 放弃结果。 |

`operationTypes`：`quality_check` / `remove_watermark` / `remove_logo` / `white_background` / `optimize_background` / `translate_text` / `select_best_main`。设计见 [`BATCH_AI_IMAGE_OPERATION_DESIGN.md`](BATCH_AI_IMAGE_OPERATION_DESIGN.md)。

| `POST` | `/api/v1/products/:id/images/select-best-main` | 自动评分并选择最佳主图；JSON `mode`: `score_only` / `recommend` / `auto_set`。 |
| `POST` | `/api/v1/products/:id/sync-images` | 将商品外链图片（如淘宝 alicdn）下载并保存到当前 Storage Provider；JSON `scope`: `all` / `main` / `detail`（默认 `all`）。 |
| `POST` | `/api/v1/pricing/calculate` | 单 SKU 发布价试算（不写入数据库）。 |
| `POST` | `/api/v1/products/:id/pricing/apply` | 对商品 SKU 应用定价规则；`confirm=false` 仅预览，`confirm=true` 更新 `product_skus.price`。 |
| `POST` | `/api/v1/products/pricing/batch-apply` | 批量应用定价规则；需 `productIds` 或 `filters`，空条件须 `confirmAll=true`。 |

`GET /api/v1/products/:id` 商品详情会返回统一商品草稿视图：基础字段 `source`、`sourceUrl`、`title`、`originalTitle`、`aiTitle`、`description`、`aiDescription`、`currency`、`status`；图片字段 `mainImages`、`descriptionImages`；结构字段 `attributes`、`skuGroups`、`skus`；价格 / 库存聚合字段 `costPrice`、`salePrice`、`stock`；采集与发布字段 `collectWarnings`、`publishStatus`；高级调试字段 `raw` / `rawData`。前端普通视图只展示标准字段与 warning，`raw` 仅用于高级详情。

`operationProgress` 统一使用实际数据实时计算：采集结果、标题、描述、图片、价格、通用参数、发布检查、刊登草稿准备。返回字段包括 `completionPercent`、`currentStep`、`currentStepLabel`、`nextActionLabel`、`nextActionKey`、`nextActionUrl`、`completedSteps`、`pendingSteps`、`blockers`、`warnings`、`publishReady`、`updatedAt`。列表摘要只返回完成度、当前步骤、下一步入口、阻断/建议数量和可刊登状态；列表聚合批量读取图片、SKU 与图片任务状态，禁止逐行调用平台或自动创建任务。

`pricing.rule` 支持：`costSource`（`collected` / `manual`）、`manualCostPrice`、`markupType`（`fixed` / `percent` / `multiplier` / `none`）、`markupAmount`、`markupPercent`、`markupMultiplier`、`shippingCost`、`weight`、`shippingCostPerWeight`、`platformCommissionPercent`、`exchangeRate`、`minProfit`、`minMarginPercent`、`minPublishPrice`、`roundingMode`（`none` / `integer` / `.9` / `.95` / `.99` / `9.99` / `19.90`）。试算返回 `landedCost`、`commissionFee`、`estimatedProfit`、`profitMarginPercent`；应用后写入 `product_skus.price` 并写操作日志。

`settings` 分组 **`pricing`**：默认加价方式/比例/倍率、固定运费、按重量运费单价（预留）、平台佣金、最低利润、最低利润率、汇率、尾数、平台覆盖、`batch_max_size`（默认 500）。**不**创建刊登任务、**不**调用平台 API。

发布前检查 `GET /api/v1/products/:id/readiness` 返回兼容字段 `status=ready|warning|blocked`，并新增 `result=passed|warning|failed`，以及用户可见 `statusLabel` / `resultLabel`。每个 `checks[]` 项含 `title`、`message`、`severity`（同 `level`）与 `technicalDetails.rawCode`（内部码，前端默认折叠）。`failed` 阻止创建刊登任务；`warning` 可继续但前端必须人工确认。采集 warning 码（如 `DETAIL_IMAGES_INCOMPLETE`）在后端统一中文化。

**统一刊登中心（Ozon 首期）与旧多平台草稿（Phase A1.2）**

Admin 日常入口为 `/product/publishing-center`，先选择商品、平台、店铺和平台叶子类目；商品草稿列表/详情的“去刊登”只进入该通用入口，旧 `/product/ozon-publish` 保留查询参数并重定向。首期只有 Ozon 开放完整字段，其他平台显示为尚未接入，不用本地占位字段伪装成可提交能力。旧多平台本地草稿能力如需使用，仅放在商品详情高级操作并显示为“保存本地快照”。

刊登目标、商品和店铺均按当前可信租户查询；`/product-publish/targets` 中的“全局”仅指不绑定单一商品，不表示跨租户店铺。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/products/:id/publish-targets` | 可刊登平台、店铺与能力分级（`real_draft_create` / `local_draft_only` / …） |
| `POST` | `/api/v1/products/:id/publish-targets/check` | 多目标独立预检查；body 含 `targets[]`、`commonConfig`、`targetConfigs` |
| `POST` | `/api/v1/products/:id/publish-targets/create-drafts` | 批量创建刊登草稿；形成 `product_publish_batches` + 子任务；支持 `onlyReady`、`retryFailedOnly` + `batchId` |
| `GET` | `/api/v1/product-publish/targets` | 当前租户可刊登平台与店铺（批量向导） |
| `POST` | `/api/v1/product-publish/batch-targets/check` | 多商品 × 多目标矩阵预检查；body 含 `productIds[]`、`targets[]`、`commonConfig`、`overrides` |
| `POST` | `/api/v1/product-publish/batch-targets/create-drafts` | 多商品批量创建刊登草稿；`onlyReady`、`includeWarnings` |
| `GET` | `/api/v1/product-publish/batches` | 多商品刊登批次列表 |
| `GET` | `/api/v1/product-publish/batches/:id` | 批次详情与子任务（仅创建者可访问，历史无 `createdBy` 批次兼容） |
| `POST` | `/api/v1/product-publish/batches/:id/retry-failed` | 只重试失败子任务 |
| `POST` | `/api/v1/product-publish/batches/:id/cancel-pending` | 只取消 pending 子任务 |

**批量规模限制（Phase A2.1）**：环境变量 `PUBLISH_BATCH_MAX_PRODUCTS`（默认 100）、`PUBLISH_BATCH_MAX_TARGETS`（默认 20）、`PUBLISH_BATCH_MAX_TASKS`（默认 300，即商品数 × 目标数）。超限时 HTTP 400，message：`本次选择的商品和刊登目标较多，请分批创建刊登草稿。`

**幂等**：`create-drafts` 对相同 admin + 商品 + 目标 + 配置 hash 返回已有活跃批次；任务级 dedup 按 `product + platform + shop + config hash` 跳过已成功项。Ozon 真实提交必须带 `Idempotency-Key`（最长 200 字符）；同一当前租户、商品、店铺和相同请求内容重放原任务，不同内容复用同一 key 返回冲突，不能重复调用平台。其他平台暂保持既有兼容行为。

**配置校验（Phase A2.2）**：`batch-targets/check` 与 `create-drafts` 校验 `commonConfig` / `overrides`（数值非负、策略枚举、商品 / 平台 / 店铺越权与匹配）。失败时 HTTP 400，`code=40004`（`PUBLISH_CONFIG_INVALID`），`data` 含 `title`、`message`、`technicalDetails.field`。

**`commonConfig` 结构**：嵌套 `price` / `image` / `inventory` / `package` + `remark`（详见 [`MULTI_PLATFORM_PUBLISHING_DESIGN.md`](MULTI_PLATFORM_PUBLISHING_DESIGN.md) §A2.2）。

**`overrides` 结构**：`products`、`platforms`、`shops`、`productTargets` 四层局部覆盖；合并优先级见设计文档。

**数据库**：显式 migration 见 [`docs/PUBLISH_BATCH_MIGRATION.md`](PUBLISH_BATCH_MIGRATION.md)。

详见 [`docs/MULTI_PLATFORM_PUBLISHING_DESIGN.md`](MULTI_PLATFORM_PUBLISHING_DESIGN.md)。

刊登提交 `POST /api/v1/products/:id/publish` 会保存 `product_publish_tasks`，任务字段包括 `productId`、`targetPlatform`、`targetStoreId`、`status`（队列态，兼容旧值）、`publishStatus`（业务态：`draft` / `checking` / `ready` / `publishing` / `success` / `failed` / `cancelled`）、`publishMode`、`title`、`description`、`images`、`skus`、`price`、`currency`、`checkResult`、`platformPayload`、`platformResult`、`platformProductId`、`retryable`、`errorCode`、`errorMessage`、`createdAt`、`updatedAt`。平台字段映射快照包含 `platformTitle`、`platformDescription`、`platformImages`、`platformSkus`、`platformPrice`、`platformStock`、`platformCategory`、`platformAttributes`。对于 Ozon，创建记录只表示“已创建提交，等待处理”；仅 Worker 得到 Ozon 的真实 `platformProductId` 后才可显示“商品已创建，等待平台审核”。Ozon `failed` 或结果未知时只有服务端明确保存 `retryable=true` 才开放手动重试；否则必须先按 `offer_id` / 平台商品 ID 在 Ozon 与刊登进度中人工核对，且不能通过新 `Idempotency-Key` 创建绕过提交。

## AI

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/ai/title-optimize` | AI 标题优化（同步/任务，见实现）。 |
| `POST` | `/api/v1/ai/description-generate` | AI 描述生成。 |
| `GET` | `/api/v1/ai/tasks` | 当前租户的 AI 任务列表（不返回其他租户记录）。 |
| `GET` | `/api/v1/ai/tasks/:id` | 当前租户的 AI 任务详情；跨租户或不存在时均返回 404。 |

客服 AI 回复建议见 **`POST /api/v1/customer/conversations/:id/ai/generate-reply`**（非 legacy `/ai/chat`）。

## Dev / Demo 种子（非 production）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/dev/demo-seed/full-project-edge-cases` | **仅 dev/demo 环境**；需 **admin** 权限。写入订单 partial_success、库存同步失败、客服发送失败等样本；不调用真实外部平台。production 禁用。 |

## 采集

采集规则（含 AI Prompt 模板）目前是实例级资源，不是租户业务数据；其 CRUD、读取、测试及启停仅允许 `tenant_id=0` 的全局设置管理员（`settings.manage`）。`GET /api/v1/collect/monitor` 同为实例级监控，仅该全局管理员可读取。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/collect/engines/status` | 被动返回 Playwright / OpenCLI 的启用、配置、可达与就绪状态，以及淘宝/天猫有效默认引擎；不返回地址或 Token，不执行 `opencli doctor`，不租用或聚焦浏览器窗口。 |
| `POST` | `/api/v1/collect/tasks` | 创建采集任务。淘宝/天猫可传 `engine=playwright\|opencli`；OpenCLI 未启用时显式选择返回 `503` 与 `data.errorCode=OPENCLI_BRIDGE_DISABLED`。任务会持久化引擎，执行失败不会自动换引擎。`source=custom` 时若 URL 属于已有 **available/beta** 专用采集器域名，返回业务码 **40002**，`data.errorCode=CUSTOM_COLLECT_PROVIDER_CONFLICT`，含 `recommendedProvider` 与 `message`。 |
| `POST` | `/api/v1/collect/batches` | 创建批量采集任务；淘宝/天猫同样支持 `engine`，OpenCLI 批次不执行 Playwright 登录预检。 |
| `GET` | `/api/v1/collect/tasks` | 采集任务列表。 |
| `GET` | `/api/v1/collect/tasks/:id` | 采集任务详情。 |
| `POST` | `/api/v1/collect/tasks/:id/retry` | 重试采集任务。 |
| `POST` | `/api/v1/collect/rules/ai-generate` | AI 根据商品 URL 生成自定义采集规则（分析页面摘要 → AI → 校验 → 自动规则测试）。1688 / AliExpress 等 **available/beta** 专用平台返回 **40002**。规则非法返回 **40003** `AI_RULE_INVALID`。 |
| `POST` | `/api/v1/collect/rules/ai-generate-and-save` | 同上并直接保存为 `collect_rule`。 |
| `GET` / `POST` / `DELETE` 等 | `/api/v1/collect/browser-profiles` | 自定义采集浏览器 Profile 管理；所有路由需要 `collect_profile.manage` 写权限，且 Profile 只在当前可信租户中可见或可操作。`tenant_id=0` 为显式系统域。 |
| `GET` | `/api/collector/providers/1688/auth-status` | 1688 采集浏览器登录态检测（同 `/api/v1/collector/...`）。 |
| `POST` | `/api/collector/providers/1688/open-login-browser` | 打开持久化 Playwright 浏览器供 1688 手动登录。 |
| `GET` | `/api/collector/providers/pinduoduo/auth-status` | 拼多多登录态检测（兼容 GET；内部走 check-login 逻辑）。 |
| `POST` | `/api/v1/collect/providers/pinduoduo/check-login` | 拼多多登录态检测（推荐）。body 可选 `{ "url": "商品详情链接", "testUrl": "设置页检测链接" }`；检测优先级：body.url → 最近失败任务 URL → 设置 `collect_pinduoduo_auth_check_url` → 仅 pifa 首页（`homepage_only`）。 |
| `POST` | `/api/collector/providers/pinduoduo/check-login` | 同上（`/api/collector` 别名）。 |
| `POST` | `/api/collector/providers/pinduoduo/open-login-browser` | 打开拼多多采集浏览器手动登录；body 可选 `{ "url": "商品或 pifa 链接" }`（勿传无参 `mobile.yangkeduo.com` 首页）。 |
| `POST` | `/api/v1/collect/providers/taobao_tmall/check-login` | 淘宝/天猫登录态检测（批量采集开始前也会调用）。body 可选 `{ "url": "商品详情链接", "testUrl": "设置页检测链接" }`；未登录返回业务错误文案；需安全验证时阻止批量开始。 |
| `POST` | `/api/collector/providers/taobao_tmall/check-login` | 同上（`/api/collector` 别名）。 |
| `POST` | `/api/collector/providers/taobao_tmall/open-login-browser` | 打开淘宝/天猫采集浏览器手动登录；body 可选 `{ "url": "商品链接" }`。 |

### 浏览器扩展采集（侧边栏）

浏览器扩展使用一次性配对码换取 90 天设备令牌（仅存哈希），之后用 `Bearer <deviceToken>`
访问任务接口；设备令牌限定租户与 Admin 用户，可随时撤销。任务由扩展直接创建/完成，
不经过 Redis 队列、Playwright Collector 或 OpenCLI Bridge。

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| `POST` | `/api/v1/collect/browser-extension/pairings` | Admin JWT + 商品写权限 | 生成一次性配对码（10 分钟有效）。 |
| `POST` | `/api/v1/collect/browser-extension/pairings/exchange` | 公开 | 用配对码换取 `{ device, deviceToken }`，一次性。 |
| `GET` | `/api/v1/collect/browser-extension/devices` | Admin JWT + 商品读权限 | 已连接设备列表。 |
| `DELETE` | `/api/v1/collect/browser-extension/devices/:id` | Admin JWT + 商品写权限 | 撤销设备令牌。 |
| `GET` | `/api/v1/collect/browser-extension/session` | 设备令牌 | 校验并返回设备信息。 |
| `POST` | `/api/v1/collect/browser-extension/tasks` | 设备令牌 | 创建交互采集任务，body `{ "source": "taobao_tmall", "url": "..." }`，任务状态为 `running`，绑定设备。 |
| `POST` | `/api/v1/collect/browser-extension/tasks/:id/result` | 设备令牌 | 提交归一化商品 JSON，创建商品草稿并置任务为 `success`；非本设备返回 `403`，任务已结束返回 `403`，并发提交返回 `409`。 |
| `POST` | `/api/v1/collect/browser-extension/tasks/:id/failure` | 设备令牌 | 上报失败码/信息，任务置为 `failed`。 |

安全会话启用时，`chrome-extension://<extension-id>` 只可在**不携带 Cookie**的以下
四个精确 `POST` 路由通过 Cookie-CSRF 来源检查：`pairings/exchange`、`tasks`、
`tasks/:id/result`、`tasks/:id/failure`。前者仍必须提交有效、未使用、未过期的一次性
配对码，后三者仍必须通过专用设备 Bearer Token、当前 Admin 状态/商品写权限和任务归属
校验。该边界不使用路径前缀或 CORS 通配；Admin 创建配对码、设备列表和撤销路由仍走
原有 Admin JWT/权限与 CSRF 规则。

任务响应字段与通用采集任务一致的关键子集：`id`、`source`、`sourceUrl`、`status`、
`resultProductId`。只支持 `source=taobao_tmall`，URL 按淘宝/天猫规则校验。

### 采集引擎契约

OpenCLI 当前仅支持 `source=taobao_tmall`。创建单任务时使用：

```json
{
  "source": "taobao_tmall",
  "url": "https://detail.tmall.com/item.htm?id=123456",
  "engine": "opencli"
}
```

创建批次时使用：

```json
{
  "source": "taobao_tmall",
  "urls": [
    "https://detail.tmall.com/item.htm?id=123456",
    "https://item.taobao.com/item.htm?id=789012"
  ],
  "engine": "playwright"
}
```

`engine` 可省略。淘宝/天猫仅在 OpenCLI Bridge 已启用且配置首选为 `opencli` 时
解析为 OpenCLI；否则解析为 Playwright。其他来源省略时均为 Playwright。
创建成功后，引擎写入任务请求快照；任务列表与详情通过
`requestOptions.engine` 返回它，管理端据此显示“实际引擎”。重试、Worker 重启或
引擎故障都不会自动改变它。

| 场景 | HTTP / 错误码 | 行为 |
| --- | --- | --- |
| `engine` 不是 `playwright` 或 `opencli` | `400` / `COLLECT_ENGINE_INVALID` | 拒绝创建 |
| 非淘宝/天猫请求 `engine=opencli` | `400` / `COLLECT_ENGINE_SOURCE_UNSUPPORTED` | 拒绝创建，不回退 |
| 显式 OpenCLI 但 Bridge 未启用 | `503` / `OPENCLI_BRIDGE_DISABLED` | 拒绝创建，不回退 |
| Bridge 在任务创建后离线 | 任务按 OpenCLI 失败 | 保留实际引擎与原始错误 |

`GET /api/v1/collect/engines/status` 的每个引擎包含 `enabled`、`configured`、
`reachable`、`ready`、`status`、`message` 与 `supportedSources`，顶层
`defaultEngine` 表示淘宝/天猫的**有效**默认引擎。响应不得包含服务地址、Token、
OpenCLI 原始输出或本机路径。部署和排错见
[collector-engines.md](collector-engines.md)。

`GET /api/collector/providers/1688/auth-status` 返回示例：

```json
{
  "provider": "1688",
  "status": "ok",
  "loggedIn": true,
  "needVerification": false,
  "message": "1688 登录态正常",
  "lastCheckedAt": "2026-05-20T12:00:00.000Z",
  "profilePath": "/path/to/collector/data/browser-profiles/1688"
}
```

`status` 取值：`ok`（已登录）、`not_logged_in`（需要登录）、`wechat_auth_required`（微信扫码）、`app_redirect`（App 引导页）、`verification_required`（需验证）、`homepage_only`（仅首页可访问，无法确认登录）、`unknown`（暂时无法确认）。

拼多多 `check-login` 返回扩展字段（无 Cookie/HTML）：`profileKey`（`pinduoduo`）、`checkedUrl`、`finalUrl`、`accessStatus`、`urlType`（`wholesale_detail` | `goods_detail` | `homepage` | `app_redirect` | `unknown`）、`checkMode`、`evidence`（`hasProductTitle` / `hasPrice` / `hasMainImage` 等）。**仅当打开商品详情页且识别到标题/价格/主图之一，且无登录/微信/App 引导时** 才返回 `ok`；**pifa 首页可访问不判已登录**。

`POST open-login-browser` 与 `check-login` 使用同一 **`pinduoduo` Profile**（与 1688、custom 隔离）。采集浏览器登录窗口 **1280×900**。

自定义 Profile 的 `profileId` / `profileKey` 仅是请求快照定位字段：后端会按当前租户、Profile 状态与 URL 域名校验，不允许借此访问其他租户的登录态。固定 Provider Profile 对非零租户采用 `tenant_{id}_{provider}` key；`tenant_id=0` 保留历史 provider key。

## 店铺与平台

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/shops` | 店铺列表（现行路径；legacy `/stores` 已废弃）。 |
| `GET` | `/api/v1/shops/:id` | 店铺详情。 |
| `POST` | `/api/v1/shops/:id/sync-orders` | 手动触发订单同步。 |

订单号在租户内唯一（`tenant_id + order_no`）。订单及其商品、SKU、店铺等引用必须属于同一租户；跨租户引用被拒绝，不能借 body 或路径中的 ID 扩大访问范围。
| `POST` | `/api/v1/shops/:id/oauth/douyin/refresh` | 刷新抖店授权 Token（示例；各平台 OAuth 见下表）。 |

现行平台 Provider 与开放平台应用配置接口：

平台开放应用配置和刊登配置均为 tenant 0 的实例级配置。`/platform/settings/:platform` 与 `/platform/publish-settings/:platform` 的读取（包括脱敏值）、写入及连接测试仅限全局配置管理员；敏感字段仍只返回 `****`。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/platform/providers` | 返回已注册平台 Provider、能力、状态、`appConfigSchema` 与设置分组。`douyin_shop` 已注册为抖店 / Douyin Shop Provider；`ozon` 为 Ozon beta（店铺级 `Client-ID` + `Api-Key`，能力为商品刊登 + 店铺信息）。 |
| `GET` | `/api/v1/platform/settings/:platform` | 读取平台开放应用配置 schema 与脱敏后的当前值。敏感字段只返回 `****`。 |
| `PUT` | `/api/v1/platform/settings/:platform` | 保存平台开放应用配置。敏感字段加密存储，传入 `****` 表示保留原值。`douyin_shop` 会校验 App Key、App Secret、回调地址、环境与超时时间；发起 OAuth 还需要 `service_id`。 |
| `POST` | `/api/v1/platform/settings/:platform/test-connection` | 测试已保存的平台开放应用配置。`douyin_shop` 应用配置测试校验配置完整性与授权可用性，不做商品 / 订单 / 库存调用。 |
| `GET` | `/api/v1/shops/oauth/douyin/start` | 发起抖店 OAuth；生成 Redis state（10 分钟，绑定管理员、服务端租户、`platform=douyin_shop`、可选 `shopId`），返回 `redirectUrl`。 |
| `GET` | `/api/v1/shops/oauth/douyin/callback` | 抖店授权公开回调；校验并一次性消费 state，从 state 恢复租户，处理 `code` / `error`，换取 token，按租户创建或更新 `shops` / `shop_auth_tokens`，成功跳转 `/settings/platforms?platform=douyin_shop&auth=success`。 |
| `GET` | `/api/v1/shops/:id/oauth/douyin/authorize-url` | 已有抖店店铺重新授权，返回 `redirectUrl`。 |
| `POST` | `/api/v1/shops/:id/oauth/douyin/refresh` | 使用加密保存的 refresh token 刷新抖店 access token，并用刷新响应校准店铺基础信息；失败时按场景标记 `expired` / `invalid`。 |
| `POST` | `/api/v1/shops/:id/oauth/douyin/revoke` | 本地解除抖店授权，清理 / 失效 token，保留历史数据。 |
| `POST` | `/api/v1/shops/:id/oauth/douyin/test` | 真实测试抖店店铺连接：检查授权、必要时刷新 token、读取并校准店铺基础信息；不返回 token 明文。 |
| `POST` | `/api/v1/shops/:id/oauth/douyin/sync-shop-info` | 手动同步 / 校准抖店店铺基础信息，复用 Phase 3 OpenAPI Client 与 token 自动刷新能力。 |
| `GET` | `/api/v1/platform/douyin/categories` | 读取本地缓存的抖店类目树；支持 `keyword`、`parentId`、`onlyLeaf`、`refresh=false`、`shopId`（仅 `refresh=true` 时用于手动刷新）。 |
| `POST` | `/api/v1/platform/douyin/categories/sync` | 使用已授权抖店店铺 token 同步类目缓存，body/query 传 `shopId`；写入 `platform_categories`，幂等 upsert。 |
| `GET` | `/api/v1/platform/douyin/categories/stats` | 返回抖店类目缓存数量、叶子类目数量和最近同步时间，供平台开放配置页展示。 |
| `GET` | `/api/v1/platform/douyin/categories/:categoryId/attributes` | 读取某个抖店类目的本地属性缓存；返回必填、可选项、属性值选项和同步时间，不返回 raw。 |
| `POST` | `/api/v1/platform/douyin/categories/:categoryId/attributes/sync` | 使用已授权抖店店铺 token 刷新某个叶子类目的属性缓存，body/query 传 `shopId`；写入 `platform_category_attributes`，幂等 upsert。 |
| `POST` | `/api/v1/platform/douyin/production-preflight` | 抖店上线前生产预检（配置、授权、开关、Storage 公网、数据状态）；body 可选 `{ "liveTest": true }` 对首家已授权店铺做 Token 刷新联调。 |
| `GET` | `/api/v1/platform/douyin/production-preflight/latest` | 读取最近一次预检结果（存于 settings `douyin_preflight.latest_result`）。 |
| `GET` | `/api/v1/platform/douyin/runtime-status` | 读取抖店运行状态（`normal` / `paused` / `emergency_disabled`）、原因与变更时间。 |
| `POST` | `/api/v1/platform/douyin/runtime-status/pause` | 暂停抖店任务；body `{ "reason": "..." }` 必填；记录 `douyin.platform.pause` 操作日志。 |
| `POST` | `/api/v1/platform/douyin/runtime-status/resume` | 恢复抖店运行；body `{ "reason": "..." }` 必填。 |
| `POST` | `/api/v1/platform/douyin/runtime-status/emergency-disable` | 紧急停用；阻止 Worker 调用抖店写接口；body `{ "reason": "..." }` 必填。 |
| `GET` | `/api/v1/platform/ozon/categories` | 读取本地缓存的 Ozon 类目树；同步时按 Ozon 官方类目树的递归 `children` 结构解析。支持 `keyword`、`onlyLeaf`、`activeOnly`、`limit`。叶子类目 ID 为 `<description_category_id>:<type_id>` 复合键，响应含 `descriptionCategoryId` / `typeId`；失效类目在缓存中的 `status` 为 `inactive`，对应同步差异事件为 `deactivated`，保存商品配置时由服务端拒绝选用。 |
| `POST` | `/api/v1/platform/ozon/categories/sync` | 使用已授权 Ozon 店铺凭证创建异步类目同步任务（body 可选 `shopId`，缺省用最近授权店铺）；响应为 `{ stats, run, runId }`，Redis 可用时 `run.status` 初始为 `pending`，不得文案宣称“同步已完成”。仅刷新共享的 `platform_categories` 缓存，**不修改任何商品**。同步运行和差异记录以发起租户隔离。 |
| `GET` | `/api/v1/platform/ozon/categories/sync-runs` | 读取当前租户发起的同步运行；支持 `limit`，状态为 `pending` / `running` / `succeeded` / `partial` / `failed`。 |
| `GET` | `/api/v1/platform/ozon/categories/sync-runs/:id` | 读取当前租户的一次同步运行详情及其 `summary`。 |
| `GET` | `/api/v1/platform/ozon/categories/changes` | 读取当前租户的同步差异；`changeType` 为 `added` / `changed` / `deactivated` / `reactivated`，支持 `limit`、`changeType` 筛选。每项稳定返回 `categoryName`、`occurredAt`、`detail`，供 UI 直接呈现。 |
| `GET` | `/api/v1/platform/ozon/categories/stats` | 返回 Ozon 类目缓存数量、叶子类目数量与最近同步时间（24h TTL 提示）。 |
| `GET` | `/api/v1/platform/ozon/categories/:id/attributes` | 读取某个 Ozon 叶子类目的属性模板缓存（`platform_category_attributes`）；除 `required`、`dictionaryId`、`options` 外，稳定返回 `isCollection`、`maxValueCount`、`attributeComplexId`、`complexIsCollection`、`categoryDependent`。`cacheStale` 提示超过 24h；调用方必须按这些元数据表达多值和可重复复杂字段组，不能降级成静默单值。 |
| `GET` | `/api/v1/platform/ozon/categories/:id/attributes/:attrId/values` | 按 `shopId`、至少两个字符的 `keyword` 远程搜索单个字典属性值；要求店铺操作权限。返回 `{ list: [{ id, value }] }`，仅用于补全有界缓存，不写入类目/商品，也不调用 Ozon 商品导入或库存接口。 |
| `POST` | `/api/v1/platform/ozon/categories/:id/attributes/sync` | 刷新叶子类目属性模板缓存（body 可选 `shopId`）；字典属性预取字典值（`/v1/description-category/attribute/values` 分页）。授权失效或 API Key 停用时返回统一错误 envelope，保留 `data.errorCode=OZON_CATEGORY_ATTR_SYNC_FAILED`，`message` 给出更新店铺凭证的安全提示，不返回上游响应或凭证明文。 |
| `GET` | `/api/v1/platform/ozon/categories/:id/attribute-mappings` | 读取该类目的「Ozon 属性 ↔ 本地字段」映射配置（`platform_category_attribute_mappings`）。 |
| `PUT` | `/api/v1/platform/ozon/categories/:id/attribute-mappings` | 整体替换该类目的属性映射，body `{ "items": [{ "attributeId", "attributeName", "localField", "enabled" }] }`；上品时按映射自动填充 attributes（字典属性匹配 `dictionary_value_id`）。 |
| `GET` | `/api/v1/platform/ozon/category-mappings` | 读取当前租户的本地来源类目 → Ozon 叶子类目映射；可按 `shopId` 筛选。映射是租户拥有的数据，店铺级映射优先于租户默认映射。 |
| `POST` | `/api/v1/platform/ozon/category-mappings/recommend` | 根据可选 `shopId`、`sourceCategoryKey` 和可选 `sourceCategoryName` 返回推荐候选；候选未确认、不会写入映射或商品。传入 `shopId` 时必须是当前租户已授权且启用的 Ozon 店铺。 |
| `PUT` | `/api/v1/platform/ozon/category-mappings` | 保存人工确认的映射；body 含 `shopId?`、`sourceCategoryKey`、`sourceCategoryName?`、`categoryId`、`categoryPath?`、`status?`。只能指向活动 Ozon 叶子类目；`schemaHash` 由服务端根据当前属性模板生成，客户端不可指定。 |
| `GET` | `/api/v1/products/:id/platform-configs/:platform` | 读取商品的平台刊登准备配置。Ozon 调用应传查询参数 `shopId`，按商品 + 平台 + 店铺精确读取；响应含 `legacyFallback` 说明是否临时读取旧商品级配置。除稳定 `id`、类目/schema 外，返回版本化 `platformAttributes` v2（普通多值 `attributes` + 可重复 `complexGroups`）、`ozonImages`、可编辑 `ozonListing`，以及后端统一解析的 `ozonPreview`。预览为每个 SKU 返回最终价格及来源、本地库存及 `stockSource=local_inventory`、最终图片顺序和错误；商品级标题/描述/币种/重量尺寸/仓库/VAT 同样带值来源。旧商品没有新版图片选择时只使用各 SKU 原图，不默认追加公共图。 |
| `PUT` | `/api/v1/products/:id/platform-configs/:platform` | 保存商品的平台刊登准备配置。Ozon body 必须含 `shopId` 与活动叶子类目，可保存 `platformAttributes: { version: 2, attributes: { attrId: [{ value, dictionaryValueId? }] }, complexGroups: [{ complexId, attributes }] }`、版本化 `ozonImages`，以及 `ozonListing: { version: 1, titleOverride?, descriptionOverride?, currencyCode?, skuPriceOverrides, package: { weightG?, widthMm?, heightMm?, depthMm?, warehouseId?, vat? } }`。SKU 售价覆盖只作用于当前 Ozon 店铺；库存字段不在配置中，仍以 `product_skus.stock` 和既有调整审计链路为唯一来源。SKU/图片必须属于当前商品；原始主图优先，只有缺原图时才允许显式替代，公共图按保存顺序追加并做 URL 去重。`product_platform_publish_configs` 的唯一键迁移为 `(product_id, platform, config_scope_key)`，Ozon 的 scope 为店铺 UUID，旧行保留安全回退并在下次保存时物化为店铺级行，不会让多个 Ozon 店铺互相覆盖。保存只写 TradeMind，不创建刊登提交、不调用 Ozon 商品或库存写接口。 |
| `POST` | `/api/v1/products/:id/readiness/validate` | 对指定 `platform=ozon`、`shopId` 做实时只读发布前检查；响应除 `checks[]`、`schemaHash`、`schemaChanged` 外返回 `resolvedOzon`，逐 SKU 给出最终有效价格、库存、图片、来源与错误，并给出标题/描述/币种/包裹配置的最终值和来源。重量尺寸/仓库先取商品 + Ozon 店铺配置，再取全局 `platform_publish_ozon` 预设；仍缺失即阻断。多值、复杂组合、字典值、必填属性、类目/schema 与图片计划均按实时模板校验；无法正确表达的具体属性会返回错误而非单值降级。预览、此预检和任务不可变快照共用 `ResolveOzonListing`；Adapter 消费该快照并再次校验实时 schema。预检只调用 seller info、类目/属性/字典等 Ozon 只读接口，不调用商品导入或库存接口；权限继续执行既有商品操作保护，并校验所选店铺操作权限。 |
| `POST` | `/api/v1/product-publish/ozon/category-groups/check` | 对 `productIds[]` 按本地来源类目分组，返回每组建议/已确认 Ozon 类目与 `ready` / `needs_work` / `skipped` 异常项；`shopId` 必填，且必须是当前租户已授权、启用并允许当前管理员操作的 Ozon 店铺。 |
| `POST` | `/api/v1/product-publish/ozon/category-groups/confirm` | 确认一个或多个分组；body 为必填 `shopId`、`groups[]`（每项含 `sourceCategoryKey`、`sourceCategoryName?`、`productIds[]`、`categoryId`、`categoryPath?`）和 `saveMappings`。同一请求内商品不可跨组重复。仅保存商品级配置；`saveMappings=true` 还要求 `config.manage` 权限，不提交到 Ozon。 |

Ozon 商品配置与实时发布检查的错误语义：

- `400`：请求或已保存业务配置不可操作，例如目标店铺与商品配置不一致；`data.errorCode` 提供稳定领域码。
- `403`：资源可见，但账号无写权限；全局管理员跨租户查看时返回 `CROSS_TENANT_OPERATION_FORBIDDEN`，店铺仅查看授权返回对应操作权限错误。
- `404`：商品或店铺不存在，或不在当前账号可见范围内；不泄露跨租户资源存在性。
- `502`：Ozon 拒绝只读预检请求或店铺凭证失效，例如 `OZON_CREDENTIAL_INVALID`。
- `503`：Ozon 限流、5xx 或临时网络故障，例如 `OZON_UPSTREAM_UNAVAILABLE`。

所有上述响应继续使用统一 envelope 并携带 `traceId`；不得把上游响应正文、Client-ID、Api-Key 或其他凭证明文返回给 Admin。
| `POST` | `/api/v1/products/:id/platform-configs/douyin_shop/build-mapping` | 根据当前商品草稿、抖店店铺/类目/属性配置生成并保存抖店刊登草稿预览；不调用抖店创建商品或图片上传接口。 |
| `GET` | `/api/v1/products/:id/platform-configs/douyin_shop/mapping` | 读取已保存的抖店刊登草稿映射。 |
| `PUT` | `/api/v1/products/:id/platform-configs/douyin_shop/mapping` | 保存人工调整后的抖店刊登草稿映射。 |
| `POST` | `/api/v1/products/:id/platform-configs/douyin_shop/validate` | 校验抖店刊登草稿映射；可传入临时映射 body，也可不传 body 校验已保存映射。 |
| `POST` | `/api/v1/products/:id/platform-configs/douyin_shop/images/upload` | 上传当前抖店刊登草稿中的待上传图片到抖店素材中心。body：`imageTypes`（`main` / `detail`）、`retryFailed`、`force`。外链会先下载并写入当前 Storage Provider，再通过后端 Douyin Client 上传；不创建抖店商品。 |
| `POST` | `/api/v1/products/:id/platform-configs/douyin_shop/images/:imageKey/retry` | 重试单张抖店图片上传。`imageKey` 可用 `localImageId`、`main:0` / `detail:0`、`storageKey` 或已有 `platformImageId`。 |
| `GET` | `/api/v1/products/:id/platform-configs/douyin_shop/images/status` | 读取当前抖店图片上传状态、Storage 状态、平台图片 ID / URL、失败原因和统计。 |
| `POST` | `/api/v1/products/:id/platform-configs/douyin_shop/create-draft` | 根据已保存抖店映射与已上传素材图创建抖店平台商品草稿。body：`shopId`（必填）、`publishMode`（默认 `save_as_platform_draft`）、`force`（已有 platformProductId 时二次确认）。会先执行发布前检查；`failed` 阻止创建。 |
| `GET` | `/api/v1/products/:id/platform-configs/douyin_shop/publish-tasks` | 列出当前商品的抖店刊登任务（分页）。 |
| `POST` | `/api/v1/product-publish/tasks/:id/cancel` | 取消 pending/running 刊登任务。 |

抖店 SKU 绑定校准与手动兜底（Phase 9.1 / 9.2，`product_publications.id` 或 `product_publication_skus.id` 为路径参数）：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/product-publications/:id/douyin/sku-bindings` | 读取当前 `product_publication_skus` 绑定状态汇总（`bound` / `skipped` / `unmatched` / `ambiguous` / `failed` 计数与行明细）；含 `platformSkus` 平台候选、`inventorySyncReady` / `inventorySyncBlockReason`。 |
| `POST` | `/api/v1/product-publications/:id/douyin/sync-sku-bindings` | 调用官方 `product.detail`（`show_draft=true`）拉取抖店 SKU 列表并校准本地映射，回写 `external_sku_id`、`bindStatus`、`bindConfidence`、`bindMessage`、`lastSyncedAt`；更新 `product_publications.skuBindingSyncedAt` 与 `raw_data.platformSkus` 缓存。已绑定 SKU 跳过；多候选标记 `ambiguous` 不强行绑定。 |
| `POST` | `/api/v1/product-publication-skus/:id/douyin/bind-sku` | 人工绑定抖店 SKU。body：`platformSkuId`（必填）、`platformSkuName`、`bindReason`（如 `manual`）。校验 publication 归属 `douyin_shop`、平台商品 ID 存在、SKU ID 非空、不与其他本地规格冲突；覆盖旧绑定时记录操作日志。成功后 `bindStatus=bound`、`bindConfidence=100`、`bindMessage=手动绑定`。 |
| `POST` | `/api/v1/product-publication-skus/:id/douyin/unbind-sku` | 解除绑定。body：`reason`（如 `manual_unbind`）。清空 `external_sku_id`，`bindStatus=unmatched`、`bindMessage=已手动解除绑定`。 |

错误码：`DOUYIN_PRODUCT_DETAIL_FAILED`、`DOUYIN_PRODUCT_NOT_FOUND`、`DOUYIN_PRODUCT_DETAIL_PERMISSION_DENIED`、`DOUYIN_SKU_BINDING_SYNC_FAILED`、`DOUYIN_SKU_BINDING_UNMATCHED`、`DOUYIN_SKU_BINDING_AMBIGUOUS`、`DOUYIN_SKU_MANUAL_BIND_FAILED`、`DOUYIN_SKU_MANUAL_UNBIND_FAILED`、`DOUYIN_PLATFORM_SKU_ID_MISSING`、`DOUYIN_SKU_BINDING_CONFLICT`、`DOUYIN_SKU_BINDING_REQUIRED`。

操作日志：`douyin.sku.binding.manual_bind`、`douyin.sku.binding.manual_unbind`、`douyin.sku.binding.recheck`、`douyin.sku.binding.conflict`（不记录 token / secret）。

抖店库存同步（Phase 9，复用既有 inventory 模块，无新增割裂路径）：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/products/:id/publication-skus` | 商品详情库存 Tab 读取刊登 SKU 映射与 `inventorySyncCapability`（`douyin_shop` 为 `beta`）。 |
| `POST` | `/api/v1/product-publication-skus/:id/sync-inventory` | 单 SKU 库存同步；body：`stock`、`options`、`fromInventoryAlert`。要求 `product_publications.external_product_id` 与 `product_publication_skus.external_sku_id` 已绑定。 |
| `POST` | `/api/v1/products/:id/sync-inventory` | 单商品多 SKU 库存同步；body：`shopId`、`skuIds[]`、`options`。 |
| `GET` | `/api/v1/inventory` | 库存中心 SKU 列表（F3）；筛选 stockStatus / skuBindStatus / syncStatus / hasException 等。 |
| `GET` | `/api/v1/inventory/alerts` | 库存预警列表。 |
| `GET` | `/api/v1/inventory/effects` | 订单库存扣减/回滚影响（扣减记录页数据源）。 |
| `GET` | `/api/v1/inventory/logs` | 本地库存变更流水。 |
| `GET` | `/api/v1/inventory-sync/tasks` | 库存同步任务列表。 |
| `GET` | `/api/v1/inventory-sync/tasks/:id` | 任务详情。 |
| `POST` | `/api/v1/inventory-sync/tasks/:id/retry` | 重试 failed 任务。 |
| `POST` | `/api/v1/inventory-sync/batches` | 批量库存同步（默认低并发）。 |

Provider 调用官方 `sku.syncStock`（`incremental=false` 全量更新）；受 `inventory_sync_enabled` 开关控制（默认关闭）。缺失平台 SKU ID 或 `bindStatus=unmatched/failed` 返回 `DOUYIN_SKU_BINDING_REQUIRED`；`bindStatus=ambiguous` 返回 `DOUYIN_SKU_BINDING_AMBIGUOUS`；绑定冲突返回 `DOUYIN_SKU_BINDING_CONFLICT`；不猜测同步。库存同步前须全部 SKU 处于可同步绑定状态（bound / skipped 且已有 `external_sku_id`）。

### P9 Inventory Sync Backend API（Batch 5）

Batch 5 的 fixture/mock-only 后端 API 使用 `/api/v1/inventory-sync`，复用现有认证、租户上下文、RBAC、审计和签名 keyset cursor。所有写请求必须带 `Idempotency-Key`；JSON body 必须为受限 `application/json`，拒绝未知字段和多余 JSON 值。该 API 不接收凭证、不调用真实 Douyin、不读写真实平台库存，也不启动 worker/cron/queue。

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `POST` | `/api/v1/inventory-sync/runs` | `inventory_sync.run` | Create a fixture-backed sync run |
| `GET` | `/api/v1/inventory-sync/runs` | `inventory_sync.read` | Signed keyset run history |
| `GET` | `/api/v1/inventory-sync/runs/:runId` | `inventory_sync.read` | Safe run detail/statistics/error summary |
| `POST` | `/api/v1/inventory-sync/runs/:runId/rerun` | `inventory_sync.rerun` | Guarded retry of a failed/cancelled retryable run |
| `GET` | `/api/v1/inventory-sync/runs/:runId/snapshots` | `inventory_snapshot.read` | Immutable snapshot list and result filter |
| `GET` | `/api/v1/inventory-sync/snapshots/:snapshotId` | `inventory_snapshot.read` | Immutable snapshot detail |
| `GET` | `/api/v1/inventory-sync/bindings` | `sku_binding.read` | Tenant-scoped binding list |
| `GET` | `/api/v1/inventory-sync/bindings/:bindingId` | `sku_binding.read` | Safe binding detail |
| `GET` | `/api/v1/inventory-sync/bindings/:bindingId/history` | `sku_binding.read` | Calibration/manual decision history |
| `GET` | `/api/v1/inventory-sync/snapshots/:snapshotId/calibrations` | `sku_binding.read` | Versioned calibration candidates |
| `POST` | `/api/v1/inventory-sync/snapshots/:snapshotId/recalibrate` | `sku_binding.manage` | Idempotent controlled new calibration version |
| `GET` | `/api/v1/inventory-sync/manual-binding-requests` | `sku_binding.read` | Pending/status manual request list |
| `GET` | `/api/v1/inventory-sync/manual-binding-requests/:requestId` | `sku_binding.read` | Request and immutable decisions |
| `POST` | `/api/v1/inventory-sync/manual-binding-requests/:requestId/confirm` | `sku_binding.resolve_manual` | Revision-checked manual confirmation |
| `POST` | `/api/v1/inventory-sync/manual-binding-requests/:requestId/reject` | `sku_binding.resolve_manual` | Revision-checked manual rejection |
| `GET` | `/api/v1/inventory-sync/runs/:runId/audit-events` | `inventory_sync.audit.read` | Allowlisted tenant-scoped audit timeline |

List endpoints return `{items, nextCursor, hasMore, limit}` and never expose offset/page totals. DTOs intentionally omit raw provider cursors, checkpoints, payloads, credential fields, and idempotency hashes.

通用刊登任务接口（含抖店）：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/product-publish/tasks` | 刊登任务列表 |
| `GET` | `/api/v1/product-publish/tasks/:id` | 任务详情（含 `platformPayload` 平台提交内容、`platformProductId` 抖店商品 ID、`retryable` 是否可重试） |
| `POST` | `/api/v1/product-publish/tasks/:id/retry` | 重试 failed 任务 |

`product_platform_publish_configs.mapped_images` 在抖店 Phase 6 保存扩展结构：

```json
{
  "mainImages": [
    {
      "localImageId": "",
      "sourceUrl": "",
      "storageUrl": "",
      "storageKey": "",
      "platformImageId": "",
      "platformImageUrl": "",
      "imageType": "main",
      "uploadStatus": "pending|processing|uploaded|failed|skipped",
      "errorCode": "",
      "errorMessage": "",
      "uploadedAt": "",
      "processed": false
    }
  ],
  "detailImages": []
}
```

抖店 OAuth / Client / 类目 / 映射 / 图片错误码：`DOUYIN_APP_CONFIG_INCOMPLETE`、`DOUYIN_OAUTH_STATE_INVALID`、`DOUYIN_OAUTH_DENIED`、`DOUYIN_OAUTH_CODE_MISSING`、`DOUYIN_TOKEN_EXCHANGE_FAILED`、`DOUYIN_TOKEN_REFRESH_FAILED`、`DOUYIN_SHOP_INFO_FAILED`、`DOUYIN_AUTH_EXPIRED`、`DOUYIN_PERMISSION_DENIED`、`UNKNOWN_DOUYIN_AUTH_ERROR`、`DOUYIN_API_ERROR`、`DOUYIN_RATE_LIMITED`、`DOUYIN_REQUEST_TIMEOUT`、`DOUYIN_RESPONSE_PARSE_FAILED`、`UNKNOWN_DOUYIN_ERROR`、`DOUYIN_CATEGORY_SYNC_FAILED`、`DOUYIN_CATEGORY_EMPTY`、`DOUYIN_CATEGORY_NOT_SELECTED`、`DOUYIN_CATEGORY_NOT_LEAF`、`DOUYIN_CATEGORY_ATTR_SYNC_FAILED`、`DOUYIN_REQUIRED_ATTR_MISSING`、`DOUYIN_CATEGORY_CACHE_STALE`、`DOUYIN_CATEGORY_PERMISSION_DENIED`、`DOUYIN_TITLE_MISSING`、`DOUYIN_TITLE_TOO_LONG`、`DOUYIN_DESCRIPTION_MISSING`、`DOUYIN_DESCRIPTION_NEEDS_REVIEW`、`DOUYIN_MAIN_IMAGE_MISSING`、`DOUYIN_MAIN_IMAGE_NOT_UPLOADED`、`DOUYIN_MAIN_IMAGE_UPLOAD_FAILED`、`DOUYIN_DETAIL_IMAGE_UPLOAD_PARTIAL_FAILED`、`DOUYIN_IMAGE_NEED_UPLOAD`、`DOUYIN_IMAGE_UPLOAD_EXPIRED`、`DOUYIN_IMAGE_NEED_SYNC`、`DOUYIN_DETAIL_IMAGE_EMPTY`、`DOUYIN_DETAIL_IMAGE_NEED_SYNC`、`DOUYIN_ATTR_VALUE_INVALID`、`DOUYIN_SKU_MISSING`、`DOUYIN_SKU_PRICE_INVALID`、`DOUYIN_SKU_STOCK_UNCONFIRMED`、`DOUYIN_SKU_ATTR_INCOMPLETE`、`DOUYIN_PRICE_MISSING`、`DOUYIN_PRICE_INVALID`、`DOUYIN_PROFIT_TOO_LOW`、`DOUYIN_STOCK_UNCONFIRMED`、`DOUYIN_STOCK_INVALID`、`DOUYIN_COLLECT_NEEDS_REVIEW`、`IMAGE_URL_NOT_ACCESSIBLE`、`IMAGE_DOWNLOAD_FAILED`、`IMAGE_READ_FAILED`、`IMAGE_FORMAT_UNSUPPORTED`、`IMAGE_SIZE_TOO_LARGE`、`IMAGE_DIMENSION_INVALID`、`IMAGE_PROCESS_FAILED`、`STORAGE_UPLOAD_FAILED`、`DOUYIN_IMAGE_UPLOAD_FAILED`、`DOUYIN_STORE_NOT_AUTHORIZED`、`DOUYIN_CREATE_PRODUCT_FAILED`、`DOUYIN_PRODUCT_PAYLOAD_INVALID`。API 错误响应 `data.errorCode` 返回业务码；callback 失败通过 `reason` query 返回。所有响应均不得返回 App Secret、access token 或 refresh token 明文。

所有平台 OAuth state 均绑定 `platform + tenant + shop`（适用时）并一次性消费；Token 刷新、授权状态写入与撤销都必须同时匹配店铺 ID 和租户。订单同步、客户消息同步、商品刊登、库存同步的队列消费者会用持久化店铺重新确认任务租户，任务 ID 本身不构成执行授权。普通业务 worker 只接受正数租户上下文；实例级 worker 监控、备份、恢复等全局操作必须由 `tenant_id=0` 的全局管理员门禁，非零租户的 `admin` 标签不构成该权限。

## 抖店可观测性 / Health & Metrics（Phase 10.4）

抖店 runtime、preflight（含 latest）及 Storage public-check 是实例级运维接口，仅全局管理员可访问；其中 preflight、runtime 状态变更与 health-check 等写操作仍要求配置权限。这些门禁和预检结果不构成生产就绪声明。

> **不** 提供 Prometheus `/metrics`。抖店生产监控复用进程健康、任务中心、操作日志与运营看板。E2E 脚本见 `scripts/douyin-e2e-*`；门禁见 [`DOUYIN_RELEASE_GATE.md`](DOUYIN_RELEASE_GATE.md)。

### 进程健康（含抖店相关队列）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/health` | 匿名；`data.status` 为 `up` / `degraded`；含 `checks.database`、`checks.redis` |
| `GET` | `/api/v1/health` | 同上 |

`data` 中与抖店 Worker 相关的块（队列启用时）：

| 字段 | 说明 |
| --- | --- |
| `orderSyncQueue` | 订单同步 Redis 队列深度、Worker 并发、`redisAvailable` |
| `productPublishQueue` | 商品刊登（含抖店草稿创建）队列 |
| `inventorySyncQueue` | 库存同步（含 `sku.syncStock`）队列 |
| `workers` | 各 Worker 心跳；`degraded=true` 时整体 `status=degraded` |

### 抖店运行态、健康与指标（Phase 10.4，无 Prometheus）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/platform/douyin/runtime-status` | `normal` / `paused` / `emergency_disabled`、原因与时间 |
| `GET` | `/api/v1/platform/douyin/health` | 抖店聚合健康：`overallStatus`（`healthy` / `degraded` / `unhealthy` / `disabled`）、`config` / `auth` / `storage` / `tasks` / `api` 分区、`grayRelease`、`runtime`；快照写入 settings `health_snapshot` |
| `GET` | `/api/v1/platform/douyin/metrics-summary` | 滚动 24h 内存指标（API 成功率/耗时、Token 刷新、任务 stale、刊登/订单/库存/SKU 计数等）；**非** Prometheus `/metrics` |
| `GET` | `/api/v1/platform/douyin/release-gate` | Release Candidate 门禁清单：`overallConclusion`（默认 `Release Candidate`）、`items[]`（`key` / `label` / `status` / `message`）；`credentials` 项在无真实 E2E 时为 `blocked` |
| `POST` | `/api/v1/platform/douyin/run-health-check` | 执行健康聚合 + taskcenter 抖店告警 scan；返回与 `GET .../health` 相同结构并持久化快照 |
| `POST` | `/api/v1/platform/douyin/production-preflight` | 上线预检；`data.blockedByRealCredentials` 为 true 时表示无真实凭证 |
| `GET` | `/api/v1/platform/douyin/production-preflight/latest` | 最近一次预检 JSON |

### 任务中心（失败 / 告警 / 摘要）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/task-center/summary` | 失败任务与告警计数摘要 |
| `GET` | `/api/v1/task-center/failures` | 失败任务列表；`taskType` 含 `ai_text`（批量 AI 文案子项）；深链 `detailUrl` → `/product/ai-text-batches/:id?itemId=` |
| `GET` | `/api/v1/task-center/failures/:taskType/:id` | 失败详情（脱敏 raw） |
| `GET` | `/api/v1/task-center/alerts` | 站内告警列表 |
| `POST` | `/api/v1/task-center/alerts/scan` | 扫描并生成告警（dedupe） |
| `POST` | `/api/v1/task-center/alerts/:id/notify` | Webhook 通知（需配置） |
| `GET` | `/api/v1/task-center/failure-categories` | 含 `sub:douyin_*` 分类 |

任务中心的列表、详情、摘要、告警、通知与命令均限定在当前租户；只有系统租户的全局管理员可执行显式的全局扫描。跨租户 ID 按不存在处理，且不得产生重试、标记、通知或状态更新副作用。

### 操作日志与运营看板

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/operation-logs` | 查询 `action`（如 `douyin.auth.success`）；不返回 Secret/Token |
| `GET` | `/api/v1/dashboard/product-operations` | 运营总览 KPI、漏斗、异常（只读 DB 聚合，不调平台 OpenAPI；含 RBAC 店铺 scope） |
| `GET` | `/api/v1/dashboard/overview` | 模块化 overview + 10 张运营卡片 |
| `GET` | `/api/v1/dashboard/todos` | 统一待办流（P0/P1/P2 优先级） |
| `GET` | `/api/v1/dashboard/health` | 子系统健康 + 配置风险摘要 |

### AI 商品运营工作台（Phase A3.3）

工作台严格从可信请求上下文确定租户；所有查询与刷新只处理该租户数据。`tenant_id=0` 是精确系统域，不是跨租户通配符。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/ai/operation-workbench/summary` | 待办统计卡片（文案/图片/发布检查/刊登异常/今日已处理） |
| `GET` | `/api/v1/ai/operation-workbench/todos` | 分页待办列表；支持 `type` / `priority` / `platform` / `shopId` / `keyword` / 时间 |
| `GET` | `/api/v1/ai/operation-workbench/todos/:id` | 单条待办详情 |
| `POST` | `/api/v1/ai/operation-workbench/todos/refresh` | 重新聚合待办（只读，不写库、不调平台 API） |

## P6 Backup / Restore / Release / DR API

All P6 write operations require Bearer authentication and backend RBAC. The frontend never receives shell commands, full backup paths, storage secrets or database credentials.

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/ops/backups` | `backup.read` | 备份记录列表；不返回完整对象路径。 |
| `POST` | `/api/v1/ops/backups` | `backup.create` | 创建备份任务；未启用备份时生成待复核记录。 |
| `GET` | `/api/v1/ops/backups/:id` | `backup.read` | 备份详情。 |
| `POST` | `/api/v1/ops/backups/:id/verify` | `backup.verify` | 执行备份校验。 |
| `POST` | `/api/v1/ops/backups/:id/hold` | `backup.hold` | 添加手动保留。 |
| `DELETE` | `/api/v1/ops/backups/:id` | `backup.delete` | 删除非运行、非 hold 的备份记录。 |
| `GET` | `/api/v1/ops/restores` | `restore.read` | 恢复验证列表。 |
| `POST` | `/api/v1/ops/restores` | `restore.execute` | 创建隔离恢复验证；production 目标默认拒绝。 |
| `GET` | `/api/v1/ops/restores/:id` | `restore.read` | 恢复验证详情。 |
| `POST` | `/api/v1/ops/restores/:id/verify` | `restore.verify` | 写入恢复完整性验证。 |
| `GET` | `/api/v1/ops/releases` | `release.read` | 发布记录列表。 |
| `POST` | `/api/v1/ops/releases` | `release.create` | 创建发布记录和 manifest 摘要。 |
| `GET` | `/api/v1/ops/releases/:id` | `release.read` | 发布详情。 |
| `POST` | `/api/v1/ops/releases/:id/execute` | `release.execute` | 执行受控发布状态机。 |
| `POST` | `/api/v1/ops/releases/:id/rollback` | `release.rollback` | 应用层回滚；禁止自动数据库恢复。 |
| `GET` | `/api/v1/ops/dr/status` | `dr.read` | 灾备状态与 Deferred 项。 |
| `POST` | `/api/v1/ops/dr/drills` | `dr.execute` | 记录隔离演练；必须确认隔离环境。 |

P6-VR closure evidence is recorded in `docs/P6_VR_FINAL_CLOSURE_REPORT.md`: isolated restore, isolated release rollback, Linux race, and final gates passed. P6 still does not mark Production Ready and does not perform real production restore, PITR drill or traffic switch.

## P7 Performance / Capacity API Status

P7 currently adds backend configuration, database tables, local rate-limit middleware, guarded dataset / load / soak / race scripts and validation gates, but does **not** expose public management APIs yet. P7-V has real isolated Medium dataset evidence (`insertedRows=1,900,150`, `failedRows=0`), while load, soak, regression and final closure remain incomplete and must not be described as production performance verification.

Planned ops routes remain design-only until implemented with RBAC, re-authentication for writes and audit logging:

| 方法 | 路径 | 状态 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/ops/performance/overview` | planned | 聚合 API / DB / Worker / Provider 性能概览。 |
| `GET` | `/api/v1/ops/performance/regressions` | planned | 性能回归记录。 |
| `GET` | `/api/v1/ops/capacity/overview` | planned | 数据规模、连接池、Worker 容量与扩容建议。 |
| `GET` | `/api/v1/ops/rate-limits` | planned | 限流策略只读展示，不暴露 Redis key 或明文 PII。 |
| `PUT` | `/api/v1/ops/rate-limits/:policyId` | planned | 高权限、重认证、审计后修改受控策略。 |
| `GET` | `/api/v1/ops/quotas` | planned | Tenant / Shop / User / System 配额模板。 |
| `POST` | `/api/v1/ops/profiling/cpu` | planned | 内部高权限 profiling，duration 有上限，不返回任意路径。 |

Current code-level P7 endpoints affected: product and order list APIs reject excessive deep offset via P7 pagination guard; HTTP requests can be locally rate-limited when `RATE_LIMIT_ENABLED=true`.

## 修改 API 时的同步要求

- 后端：handler、service、DTO、权限和错误处理一起检查。
- 前端：`admin/src/services`、`admin/src/types`、相关页面字段和状态映射一起检查。
- 文档：同步本文档、`docs/module-map.md` 和必要的 README 能力描述。
- 安全：涉及密钥、Token、密码、Cookie 时同步 `SECURITY.md`。
- 任务：耗时接口必须使用任务状态，不应在 HTTP 请求中长时间阻塞。
## P3.2 Douyin Webhook Routing Addendum

For `platform=douyin_shop` / `douyin`, the public webhook route resolves the verified payload to a concrete shop binding before persistence. Accepted events carry `tenantId`, `internalShopId`, `platformShopId`, `appId`, and `bindingId` into `webhook_events` and downstream order upsert. Duplicate detection is scoped by `platform + tenant_id + platform_shop_id + event_id`, so the same platform `event_id` from two shops does not collide.

Resolution failures are non-success ACKs and may use codes such as `DOUYIN_WEBHOOK_SHOP_NOT_RESOLVED`, `DOUYIN_WEBHOOK_SHOP_AMBIGUOUS`, `DOUYIN_WEBHOOK_BINDING_REVOKED`, `DOUYIN_WEBHOOK_AUTHORIZATION_EXPIRED`, `DOUYIN_WEBHOOK_APP_BINDING_MISMATCH`, and `DOUYIN_WEBHOOK_TENANT_MISMATCH`.
