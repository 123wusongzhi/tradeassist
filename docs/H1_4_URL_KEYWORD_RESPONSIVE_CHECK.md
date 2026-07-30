# Phase H1.4 — URL 状态补漏 + Keyword UX + 响应式点检报告

> **Phase**: H1.4  
> **Status**: passed_with_warning  
> **Checked at**: 2026-07-09  
> **Machine report**: [`h1-4-url-keyword-responsive-check.json`](h1-4-url-keyword-responsive-check.json)

## 策略与结论

```text
Post-F9 Enhancement In Progress
MVP Demo Ready
Tag deferred
非 Production Ready
抖店 Release Candidate
不进入灰度 / 真实预发 / 抖店真实 E2E
```

**最终结论**：H1.4 订单/异常 URL 状态补漏与 keyword UX 安全增强已完成；浏览器后退/前进与 1024 宽度点检以 `passed_with_warning` 记录（IDE 浏览器 history 与窄屏折行需真实浏览器人工复核，不阻塞 F9 基线）。

## 点检范围

| 类别 | 内容 |
| --- | --- |
| URL 补漏 | `/orders/list` status / fulfillmentStatus / 创建时间范围；`/orders/exceptions` severity / 创建时间范围 |
| Keyword UX | 长度限制 80、敏感信息轻提示、清空同步 URL + page 重置 |
| 页面 | Dashboard、AI 工作台、失败任务、订单、订单异常、商品草稿、库存、库存预警、客服会话 |
| 分辨率 | 1366×768、1024×768 |
| 浏览器 | Cursor IDE Browser / Chromium（后退前进建议真实浏览器复核） |

## URL 状态补漏结果

### 订单列表 `/orders/list`

| 参数 | 结果 | 说明 |
| --- | --- | --- |
| `status` | **passed** | 订单状态筛选刷新/后退保持 |
| `fulfillmentStatus` | **passed** | 履约状态筛选保持 |
| `start` / `end` | **passed** | 创建时间范围（语义等同 `dateFrom` / `dateTo`） |
| 清空筛选 | **passed** | reset 清除全部 query；keyword 单独清空保留其他筛选项 |

### 订单异常 `/orders/exceptions`

| 参数 | 结果 | 说明 |
| --- | --- | --- |
| `severity` | **passed** | 严重程度筛选保持 |
| `start` / `end` | **passed** | 创建时间范围保持 |
| `source` + `orderId` | **passed** | 订单详情 / Dashboard 深链仍可用 |

## Keyword UX 结果

| 项 | 结果 | 说明 |
| --- | --- | --- |
| 最大长度 80 | **passed** | 超长截断并提示「搜索关键词过长，请缩短后再搜索。」 |
| 敏感信息提示 | **passed** | 手机号/邮箱/身份证/密钥样式词轻量 Alert，不阻断搜索 |
| 清空按钮 | **passed** | allowClear + onClear 移除 `keyword`、重置 `page=1`、保留其他筛选 |
| 接入页面 | **passed** | orders、exceptions、drafts、inventory、alerts、conversations、failures、operation-workbench |

## 浏览器后退 / 前进结果

| 页面 | 结果 | 说明 |
| --- | --- | --- |
| Dashboard | **passed_with_warning** | 筛选刷新 OK；IDE `Alt+←` 未必触发 SPA history |
| AI 工作台 | **passed_with_warning** | 筛选/分页 URL OK；真实浏览器后退建议人工复核 |
| 失败任务中心 | **passed_with_warning** | 同上 |
| 订单 / 异常 | **passed_with_warning** | 新增 status/severity/date URL 刷新 OK |
| 商品草稿 / 库存 / 客服 | **passed_with_warning** | keyword 清空与筛选保持 OK |

## 1366 / 1024 分辨率结果

| 分辨率 | 结果 | 说明 |
| --- | --- | --- |
| 1366×768 | **passed** | 无白屏；ProTable 横向滚动可用；筛选区 vertical layout 可用 |
| 1024×768 | **passed_with_warning** | 筛选区可能多行折行；表格需横向滚动；主要按钮未遮挡 |

点检页面：`/dashboard/product-operations`、`/orders`、`/orders/exceptions`、`/product/drafts`、`/inventory`、`/inventory/sync-tasks`、`/customer/conversations`、`/ops/task-center/failures`、`/settings/config-status`。

## 修复项

1. `ORDER_QUERY_KEYS` 增加 `status`、`fulfillmentStatus`、`start`、`end` 并双向同步表单。
2. `EXCEPTION_QUERY_KEYS` 增加 `severity`、`start`、`end` 并写入 URL。
3. 新增 `keywordSafety.ts`、`KeywordSafetyHint.tsx`、`useKeywordSearchField` 钩子。
4. `urlState.ts` 增加 `fulfillmentStatus`、`dateFrom`/`dateTo` 等允许键与 `queryTimeRange()`。

## 遗留项

| ID | 级别 | 说明 |
| --- | --- | --- |
| H14-001 | P2 | keyword 值仍可能进入浏览器历史（已提示，未禁止） |
| H14-002 | P2 | 真实浏览器后退/前进建议人工复核 |

## URL 安全检查

- 禁止键名（buyerName、phone、email、accessToken 等）未写入 allowlist。
- keyword 敏感检测仅前端展示，不写 URL、不写日志。

## 最终结论

**Phase H1.4 Completed** — 订单/异常次要 URL 状态与 keyword UX 安全增强已交付；F9 自动化基线保持；Tag deferred / 非 Production Ready 策略不变。
