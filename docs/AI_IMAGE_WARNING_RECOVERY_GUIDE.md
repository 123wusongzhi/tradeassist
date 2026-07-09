# AI 图片 Warning 收敛与恢复指南（H1.3）

> **状态**：Post-F9 Enhancement · MVP Demo Ready · Tag deferred · 非 Production Ready

## 目标

AI 图片试跑允许 `passed` 或 `passed_with_warning`，但每个 warning 必须：

- **可解释**：中文原因 + 可折叠内部码
- **可定位**：批次详情 / 失败任务中心可跳转
- **可恢复**：标明需配置 vs 可重试
- **不误导**：warning 不显示为 failed；Provider 缺失不伪装成功

## 结构化 Warning 码

| 内部码 | 中文说明 | 可恢复 | 需配置 |
| --- | --- | --- | --- |
| `provider_config_missing` | 图片 AI Provider 未选择 | 是 | 是 |
| `dashscope_key_missing` | 通义万相 API Key 未配置 | 是 | 是 |
| `white_background_provider_missing` | 白底图能力不可用 | 是 | 是 |
| `logo_remove_unsupported` | 去 Logo 暂不支持 | 否 | 否 |
| `image_download_failed` | 源图下载失败 | 是 | 否 |
| `storage_public_url_missing` | Storage 公网地址未配置 | 是 | 是 |
| `provider_timeout` / `provider_rate_limited` | 超时 / 限流 | 是 | 否 |

完整列表见 `backend/internal/modules/aiproductimage/warning_codes.go`。

## 页面入口

| 页面 | 能力 |
| --- | --- |
| `/product/ai-image-batches/:id` | 批次概览、Provider 状态、可恢复项统计 |
| `/ai/image-batches` | 批次列表 → 复核 |
| `/ops/task-center/failures?taskType=ai_image` | 失败分类与跳转配置 |
| `/settings/config-status` | AI 图片 / 通义万相 / Storage 公网 |

## 恢复路径

1. **Provider 未配置** → 设置 → 图片 AI → 选择 Provider 并保存
2. **通义万相 Key 缺失** → 设置 → 图片 AI → `dashscope_image_api_key`
3. **Storage 公网** → 设置 → 存储 → `public_base` → 测试公网访问
4. **处理失败可重试** → 批次详情「重试失败项」或单条「重新处理」

## 试跑基线（H1.3）

- 允许：`passed` / `passed_with_warning`（典型 14/16）
- 不强行要求 16/16
- F9 主链路结论不受影响

## 相关文档

- [`STORAGE_PUBLIC_URL_GUIDE.md`](STORAGE_PUBLIC_URL_GUIDE.md)
- [`POST_F9_ENHANCEMENT_PLAN.md`](POST_F9_ENHANCEMENT_PLAN.md)
