# Phase H1.2.1 — URL 状态浏览器点检报告

> **Phase**: H1.2.1  
> **Status**: passed_with_warning  
> **Checked at**: 2026-07-09  
> **Machine report**: [`h1-2-url-state-browser-check.json`](h1-2-url-state-browser-check.json)

## 策略与结论

```text
Post-F9 Enhancement In Progress
MVP Demo Ready
Tag deferred
非 Production Ready
抖店 Release Candidate
不进入灰度 / 真实预发 / 抖店真实 E2E
```

**最终结论**：H1.1 + H1.2 已接入页面的 URL 状态保持经浏览器/API 点检验证通过；发现 4 项 P0/P1 低风险问题并已修复；2 项 P2 遗留不影响 F9 基线。

## 点检范围

| 类别 | 内容 |
| --- | --- |
| 页面 | Dashboard、AI 工作台、失败任务中心、订单、订单异常、商品草稿、库存中心、库存预警、库存同步任务、客服 Hub、客服会话 |
| 账号 | `demo_admin@trademind.local`、`demo_operator@trademind.local`、`demo_readonly@trademind.local` |
| 关注点 | 刷新/后退恢复、分页/筛选/Drawer/Tab、source 参数、旧深链、RBAC 刷新、URL 安全 |

## 点检方法

1. **浏览器点检**（Cursor IDE Browser）：`demo_admin` 登录后逐页构造带 query 的 URL，验证刷新后参数保留、Dashboard 出站 `source=dashboard`、筛选 UI 与 URL 一致。
2. **API/路由烟测**：`scripts/h1-2-url-state-browser-check.mjs` 验证各页 URL 可被 SPA 路由接受且不含禁止 token 键名。
3. **权限烟测**：复用 `demo-rbac-smoke`（operator / readonly 登录与写阻断）。

## 页面点检结果

### 3.1 Dashboard `/dashboard/product-operations`

| 用例 | 结果 | 说明 |
| --- | --- | --- |
| 筛选刷新恢复 | **passed** | `platform` + `productSource=1688` 刷新后 URL 保留 |
| source=dashboard 出站 | **passed** | 点击「订单异常工作台」→ `/orders/exceptions?source=dashboard` |
| 浏览器后退 | **warning** | IDE 浏览器 `Alt+←` 未触发 SPA history；人工浏览器后退待复核（非阻塞） |

### 3.2 AI 运营工作台 `/ai/operation-workbench`

| 用例 | 结果 |
| --- | --- |
| 筛选 / 分页 / Drawer URL | **passed** |
| 重置关闭 Drawer | **passed**（H121-002 修复后） |

### 3.3 失败任务中心 `/ops/task-center/failures`

| 用例 | 结果 |
| --- | --- |
| taskType / 分页 / Drawer | **passed** |
| source=taskcenter 深链 | **passed**（构造 URL + 既有 appendSourceToUrl） |
| 重置关闭 Drawer | **passed**（H121-002） |

### 3.4 订单中心 `/orders/list`

| 用例 | 结果 |
| --- | --- |
| skuStatus / inventoryStatus / keyword / page | **passed** |
| URL 无买家 PII 字段名 | **passed** |

### 3.5 订单异常 `/orders/exceptions`

| 用例 | 结果 |
| --- | --- |
| exceptionType / status / source / orderId 旧深链 | **passed** |

### 3.6 商品草稿 `/product/drafts`

| 用例 | 结果 |
| --- | --- |
| platform / publishStatus / aiStatus / keyword / page | **passed** |
| source 筛选写入 URL | **passed**（H121-003） |

### 3.7–3.11 库存 / 客服

| 页面 | 结果 |
| --- | --- |
| `/inventory` | **passed** |
| `/inventory/alerts` | **passed** |
| `/inventory/sync-tasks` | **passed**（Drawer + reset 修复） |
| `/customer/hub` | **passed** |
| `/customer/conversations` | **passed** |

## 权限刷新点检

| 账号 | 结果 | 说明 |
| --- | --- | --- |
| demo_admin | **passed** | 刷新后全量可见 |
| demo_operator | **passed** | RBAC smoke：店铺隔离 |
| demo_readonly | **passed** | 写 API 403；刷新后写操作仍禁用 |

`source` 参数不参与权限判断；非法 source 由 `normalizeSource` 忽略。

## URL 安全检查

**禁止项**：未在测试 URL 中发现 `buyerName`、`phone`、`email`、`address`、`token`、`secret`、`prompt`、`raw` 等键名。

**允许项**：`keyword`、`status`、`page`、`source`、`drawer`、`productSkuId`、`conversationId` 等稳定标识/筛选项正常使用。

**P2 提示**：`keyword` 可能携带检索词进入浏览器历史（MVP 可接受 trade-off）。

## 修复项

| ID | 级别 | 描述 | 状态 |
| --- | --- | --- | --- |
| H121-001 | P0 | Dashboard `source` 导航来源与商品来源筛选键冲突 | **fixed** — 商品来源改用 `productSource` |
| H121-002 | P1 | 三页 reset 后 Drawer 未关闭 | **fixed** |
| H121-003 | P1 | 商品草稿 source 筛选未写入 URL | **fixed** |
| H121-007 | P1 | Dashboard 挂载时空 filters 写回 URL 导致刷新丢参 | **fixed** — lazy init + compare-before-write |

## 遗留项（P2，不阻塞 F9）

| ID | 描述 |
| --- | --- |
| H121-004 | 订单列表 status / fulfillmentStatus / 日期范围未入 URL |
| H121-005 | 订单异常 severity / 日期范围未入 URL |
| H121-006 | keyword 潜在 PII 暴露面 |
| — | 浏览器后退 SPA history 建议人工复核 |

## 自动化与构建

| 命令 | 结果 |
| --- | --- |
| `pnpm check:ui-copy --strict` | passed |
| `pnpm check:dev` | passed |
| `pnpm build:admin` | passed |
| `git diff --check` | passed |
| `node scripts/h1-2-url-state-browser-check.mjs` | passed (14/14) |
| `pnpm demo:auto-acceptance` | 见当次终端输出 |

## 变更文件（H1.2.1）

- `admin/src/utils/urlState.ts` — `productSource`、`isNavSourceValue`、`resolveProductSourceFromQuery`
- `admin/src/pages/Dashboard/ProductOperations/index.tsx` — 商品来源键拆分 + 挂载写回修复
- `admin/src/pages/AI/OperationWorkbench/index.tsx` — reset 关闭 Drawer
- `admin/src/pages/TaskCenter/Failures/index.tsx` — reset 关闭 Drawer
- `admin/src/pages/Inventory/SyncTasks/index.tsx` — reset 关闭 Drawer
- `admin/src/pages/Product/Drafts/index.tsx` — source 筛选 URL 同步
- `scripts/h1-2-url-state-browser-check.mjs` — 点检脚本
- 本文档 + JSON 报告 + 进度/计划文档
