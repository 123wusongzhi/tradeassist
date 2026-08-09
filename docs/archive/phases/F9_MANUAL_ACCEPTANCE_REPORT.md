# F9.1 最终人工总走查报告

> **日期**：2026-07-07  
> **环境**：http://localhost:8000 + http://127.0.0.1:8080  
> **结论**：**passed_with_warning** · P0=0 · P1=0

## 16 步主链路

| Step | Path | Result | Note |
| --- | --- | --- | --- |
| 1 | `/user/login` | ✅ passed | demo 账号 seed 可用 |
| 2 | `/dashboard/product-operations` | ✅ passed | KPI/待办/最近任务可见 |
| 3 | `/settings/config-status` | ✅ passed | Storage 公网/抖店待凭证已标注 |
| 4 | `/collect/hub` | ✅ passed | 未点击真实 1688 采集 |
| 5 | `/product/drafts` | ✅ passed | 1250+ 草稿 |
| 6 | `/product/drafts/:id` | ✅ passed | 沿用 A1.1/R1.2 |
| 7 | `/product/ai-text-batches/:id` | ✅ passed | 试跑 16/16 |
| 8 | `/product/ai-image-batches/:id` | ⚠️ passed | 试跑 14/16 warning |
| 9 | 发布检查 Tab | ✅ passed | readiness 三态 |
| 10 | `/product/publish-batches/new` | ✅ passed | local_draft_only |
| 11 | `/order/list` | ✅ passed | API smoke |
| 12 | `/order/exceptions` | ✅ passed | F7 样本 |
| 13 | `/inventory/list` | ✅ passed | API smoke |
| 14 | `/customer/hub` | ✅ passed | 人工确认发送 |
| 15 | `/ops/task-center/failures` | ✅ passed | 1024px 无溢出 |
| 16 | `/dashboard/product-operations` | ✅ passed | 自动刷新 KPI |

## 截图

- `docs/f9-screenshots/f9-step02-dashboard-1920.png`
- `docs/f9-screenshots/f9-product-drafts-1366.png`

## 机器可读

- [`f9-manual-acceptance.json`](f9-manual-acceptance.json)
