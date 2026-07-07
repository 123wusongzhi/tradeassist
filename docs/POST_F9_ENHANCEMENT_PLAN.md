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

First batch:

| Page | Scope |
| --- | --- |
| `/dashboard/product-operations` | filters and dashboard-origin links |
| `/ai/operation-workbench` | filters, pagination, drawer detail |
| `/ops/task-center/failures` | filters, pagination, include switches, drawer detail |

Second batch candidates:

| Page | Scope |
| --- | --- |
| `/orders` / `/orders/exceptions` | list filters and pagination |
| `/inventory` / `/inventory/sync-tasks` | list filters, deep links, pagination |
| `/customer/hub` / `/customer/conversations` | conversation filters and source retention |
| `/products/drafts` | draft filters and pagination |

## Completion Rules

- URL state must not include secrets, tokens, raw prompts, raw responses, or platform credentials.
- Default values should not be written into URL.
- Browser refresh should restore filters and open drawer state where supported.
- Browser back should return to the previous query state.
- P3 items stay deferred unless a separate roadmap decision is recorded.
