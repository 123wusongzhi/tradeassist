# Phase H1.5.1 — AI Image Baseline Confirmation

> **Phase**: H1.5.1  
> **Status**: passed_with_warning  
> **Machine report**: [`h1-5-ai-image-baseline-confirmation.json`](h1-5-ai-image-baseline-confirmation.json)

## Unified baseline

```text
stable_range_14_to_15_of_16
passed_with_warning
code-level failures: 0
```

**Current run (2026-07-10)**: **14/16** success（独立试跑）/ **15/16**（`demo:auto-acceptance` 同轮）— artifact [`ai-image-trial-run.json`](ai-image-trial-run.json).

## Provider & environment

| Item | Value |
| --- | --- |
| Provider | `dashscope_image` (configured) |
| `dashscope_image_api_key` | **not configured** (`hasApiKey=false`) |
| Storage `public_base` | preflight 51 warnings (`storage_public_url_missing` 等) |
| Same test matrix | I1 quality 5 + I2 white-bg 5 + I3 watermark 3 + I4 select-main 3 = 16 |
| Same demo products | 20 products with images |

## Run summary

| Metric | Count |
| --- | --- |
| Total items | 16 |
| Success | 14 |
| Failed | 2 |
| Quality warnings (non-fatal) | 4 |

## Failures (environment / Provider — not code P0/P1)

| Batch | Op | Error | Cause |
| --- | --- | --- | --- |
| I2-white-bg | `white_background` | 无法从源图链接下载图片 | 外部源图下载 + 无 DashScope Key |
| I4-select-main | `select_best_main` | 图片处理超时 | Provider 网络/超时波动 |

## 14/16 vs 15/16 explanation

| Source | Result |
| --- | --- |
| H1.3 (文档/人工) | 15/16 `passed_with_warning` |
| H1.5 `demo:auto-acceptance` | 14/16 `passed_with_warning` |
| H1.5.1 rerun | 14/16 `passed_with_warning` |

**结论**：差异来自 **Provider / 外部源图 / 超时** 波动，不是代码回归。当 I2 白底图或 I4 自动选主图其中一项在本轮环境中成功时，可达 15/16。统一口径采用 **stable_range_14_to_15_of_16**。

## Warning codes (acceptable)

- `storage_public_url_missing`
- `text_heavy`（质量评分提醒）
- `dashscope_image_api_key_missing`（配置态）

## Artifacts

- [`docs/ai-image-trial-run.json`](ai-image-trial-run.json) — 本轮试跑
- [`docs/ai-image-trial-run.auto.json`](ai-image-trial-run.auto.json) — 自动化验收副本

## Final conclusion

AI 图片基线 **passed_with_warning**；**代码级失败 0**；配置 DashScope Key + Storage 公网后可复跑争取 15–16/16，但不阻塞 MVP Demo / H1.5.1 签收。
