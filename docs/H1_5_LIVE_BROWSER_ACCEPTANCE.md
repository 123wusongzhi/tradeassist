# Phase H1.5.1 — Live Browser Acceptance

> **Phase**: H1.5.1  
> **Status**: passed  
> **Checked at**: 2026-07-10  
> **Machine report**: [`h1-5-live-browser-acceptance.json`](h1-5-live-browser-acceptance.json)

## Strategy

```text
Phase H1.5.1 Completed
Post-F9 Enhancement In Progress
MVP Demo Ready
Tag deferred
非 Production Ready
抖店 Release Candidate
不进入真实预发 / 抖店真实 E2E / 生产灰度
```

## Environment

| Item | Value |
| --- | --- |
| Admin URL | `http://localhost:8000` |
| API URL | `http://127.0.0.1:8080` |
| Chrome | 149.0.7827.200 (system Chrome via Playwright) |
| Edge | Core pages spot-check via Chromium |
| Backend `/health` | database=ok, redis=ok, workers running |

## Accounts

| Account | Role | Result |
| --- | --- | --- |
| `demo_admin@trademind.local` | admin | passed |
| `demo_operator@trademind.local` | operator | passed |
| `demo_readonly@trademind.local` | readonly | passed (write API 403) |

## Chrome core cases (13/13 passed)

| Page | Refresh | Back | Forward | Notes |
| --- | --- | --- | --- | --- |
| Dashboard | passed | passed | passed | `source=dashboard` outbound OK |
| AI 运营工作台 | passed | passed | passed | filter + pagination |
| 失败任务中心 | passed | passed | passed | taskType + drawer |
| 订单列表 | passed | passed | passed | status / fulfillment / keyword |
| 订单异常 | passed | passed | passed | exceptionType / severity |
| 商品草稿 | passed | passed | passed | platform / keyword |
| 刊登批次 | passed | passed | passed | tab=batches |
| 采集任务 | passed | passed | passed | sourcePlatform |
| 订单同步任务 | passed | passed | passed | resultStatus |
| 库存同步任务 | passed | passed | passed | status |
| 客服会话 | passed | passed | passed | replyStatus |
| AI 文案批次 | passed | passed | passed | status + page |
| AI 图片批次 | passed | passed | passed | warningCode |

## Edge spot-check (7/7 passed)

Dashboard back/forward、AI 工作台 Drawer 刷新、失败任务中心、刊登批次、订单同步 Drawer、客服详情刷新、AI 图片 itemId 深链 — 全部 passed。

## Responsive screenshots

| Resolution | Count | Result | Directory |
| --- | --- | --- | --- |
| 1366×768 | 11/11 | passed | 本地生成，不纳入仓库；见 `scripts/h1-5-live-browser-acceptance.mjs` |
| 1024×768 | 8/8 | passed_with_warning | 同上 |

1024 下允许筛选折行与表格横滚；无按钮遮挡、Drawer 可关闭。

## Screenshot security

passed — 无 PII / Token / Prompt / raw response 文件名；截图为 Demo Admin 脱敏数据。

## RBAC

| Check | Result |
| --- | --- |
| admin 全页面 | passed |
| operator 店铺 scope | passed（`demo-rbac-smoke`） |
| readonly 写阻断 403 | passed |
| source 不绕过权限 | passed |

## Fixes applied (H1.5.1)

| ID | Issue | Fix |
| --- | --- | --- |
| H151-001 | ProTable 首请求清空 URL 筛选（订单/异常/失败任务/草稿/客服） | 为 ProTable 增加 `params` 从 `urlState` 种子化 |
| H151-002 | AI 工作台挂载写回空 filters 清空 URL | `workbenchUrlPatch` + `sameWorkbenchUrlPatch` compare-before-write |
| H151-003 | 客服 `replyStatus=pending_reply` 未识别 | legacy filter + 写回 `pending_reply` |

## AI image baseline

见 [`H1_5_AI_IMAGE_BASELINE_CONFIRMATION.md`](H1_5_AI_IMAGE_BASELINE_CONFIRMATION.md) — **stable_range_14_to_15_of_16** / **14/16** 本轮。

## Verification

```bash
node scripts/h1-5-live-browser-acceptance.mjs
node scripts/h1-5-live-browser-acceptance-check.mjs
node scripts/h1-5-secondary-url-browser-check.mjs
pnpm demo:auto-acceptance
```

## Final conclusion

**Phase H1.5.1 Completed** · **Live Browser Acceptance Passed** · Chrome 核心签收、Edge 抽查、后退/前进/刷新、Drawer/深链、1366/1024 真实截图、RBAC 全部通过；F9 基线无退化。
