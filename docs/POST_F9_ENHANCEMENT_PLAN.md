# Post-F9 Enhancement Plan

> **Phase**: H1  
> **Status**: Post-F9 Enhancement · MVP Demo Ready · Tag deferred · 非 Production Ready · 抖店 Release Candidate

## Strategy

- Do not create a git tag in H1.
- Continue on `dev` with low-risk, high-value platform polish.
- Keep the Post-F9 freeze boundary: no heavy ERP scope, no new real platform OpenAPI, no automatic direct listing.
- Do not enter real preprod, real Douyin E2E, production gray release, or Production Ready marking.

## H1.0 Documentation Status

Current project status is unified as:

```text
Post-F9 Enhancement
MVP Demo Ready
Tag deferred
非 Production Ready
抖店 Release Candidate
```

F9 conclusions remain valid:

- Phase F9 Passed
- P0 = 0 / P1 = 0
- `pnpm demo:auto-acceptance` passed
- Real preprod / public Storage / real Douyin E2E / gray release remain incomplete environment items

## H1.1 Workbench URL State

Primary target: P2-06 工作台后退 / 刷新状态保持.

First batch (**completed**):

| Page | Scope |
| --- | --- |
| `/dashboard/product-operations` | filters and dashboard-origin links |
| `/ai/operation-workbench` | filters, pagination, drawer detail |
| `/ops/task-center/failures` | filters, pagination, include switches, drawer detail |

## H1.2 Second-Batch URL State

Second batch (**completed**):

| Page | Scope |
| --- | --- |
| `/orders/list` | keyword, pay/sku/inventory status, platform, shop, pagination, source |
| `/orders/exceptions` | keyword, exception type, platform, shop, status, pagination, source |
| `/product/drafts` | keyword, status, platform, shop, publish/ai status, pagination, source |
| `/inventory` | keyword, stock/sync/bind filters, productSkuId deep link, pagination, source |
| `/inventory/alerts` | keyword, alert type, stock status, platform, shop, pagination, source |
| `/inventory/sync-tasks` | filters, batchId/productSkuId/id drawer deep links, pagination, source |
| `/customer/hub` | lightweight platform / shop / source |
| `/customer/conversations` | reply/AI/send filters, conversationId/suggestionId deep links, pagination, source |

Deferred to later H1 batches: publish batches, collect tasks, order sync tasks, and other secondary lists.

## H1.2.1 URL State Browser Check

Third batch (**completed**):

| Scope | Result |
| --- | --- |
| Browser/API spot-check on all H1.1 + H1.2 pages | passed_with_warning |
| P0/P1 fixes | Dashboard `productSource` split, mount URL hydration, drawer reset on three pages, drafts source sync |
| Reports | [`H1_2_URL_STATE_BROWSER_CHECK.md`](H1_2_URL_STATE_BROWSER_CHECK.md), [`h1-2-url-state-browser-check.json`](h1-2-url-state-browser-check.json) |

## Completion Rules

- URL state must not include secrets, tokens, raw prompts, raw responses, or platform credentials.
- Default values should not be written into URL.
- Browser refresh should restore filters and open drawer state where supported.
- Browser back should return to the previous query state.
- P3 items stay deferred unless a separate roadmap decision is recorded.
