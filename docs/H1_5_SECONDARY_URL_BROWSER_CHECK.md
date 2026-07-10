# H1.5 Secondary URL State + Browser Sign-off

> **Phase**: H1.5  
> **Status**: Post-F9 Enhancement · MVP Demo Ready · Tag deferred · 非 Production Ready · 抖店 Release Candidate  
> **Machine report**: [`h1-5-secondary-url-browser-check.json`](h1-5-secondary-url-browser-check.json)

## Scope

Extend shared URL state to secondary task/batch list pages:

| Page | Route | URL keys |
| --- | --- | --- |
| Publish batches / tasks | `/product/publish-tasks` (`tab=batches`) | `keyword`, `status`, `platform`, `shopId`, `batchId`, `tab`, `page`, `pageSize`, `id`, `drawer`, `source`, `start`/`end` |
| Publish batch detail | `/product/publish-batches/:id` | `source` (back link preserves list context) |
| Collect tasks | `/collect/tasks` | `keyword`, `status`, `sourcePlatform`, `batchId`, `page`, `pageSize`, `drawer`, `id`, `source` |
| Order sync tasks | `/orders/sync-tasks` | `status`, `platform`, `shopId`, `resultStatus`, `page`, `pageSize`, `drawer`, `id`, `source` |
| Customer message sync | `/customer/message-sync-tasks` | `status`, `platform`, `shopId`, `resultStatus`, `page`, `pageSize`, `drawer`, `id`, `source` |
| AI text batches | `/ai/text-batches` | `status`, `page`, `pageSize`, `batchId`, `source` |
| AI text batch detail | `/product/ai-text-batches/:id` | `itemId`, `tab`, `source` |
| AI image batches | `/ai/image-batches` | `status`, `warningCode`, `page`, `pageSize`, `batchId`, `source` |
| AI image batch detail | `/product/ai-image-batches/:id` | `itemId`, `tab`, `warningCode`, `source` |

Legacy route aliases:

- `/product/publish-batches` → `/product/publish-tasks?tab=batches`
- `/product/ai-text-batches` → `/ai/text-batches`
- `/product/ai-image-batches` → `/ai/image-batches`

## New `source` values

```text
ai_workbench
config_status
publish_batch
order_sync
customer_sync
```

Navigation-only; invalid values ignored; does not affect RBAC or shop scope.

## Collect `source` vs `sourcePlatform`

- Navigation context: `source` (dashboard, taskcenter, …)
- Collect provider filter / form prefill: `sourcePlatform` (or legacy non-nav `source=1688|custom|…`)

## Browser sign-off

| Browser | Coverage | Result |
| --- | --- | --- |
| Chrome | Full secondary list back/forward/refresh/deep link cases | passed (H1.5.1 live sign-off) |
| Edge | Core pages sampled | passed (H1.5.1) |

Documented cases: filter → paginate → drawer/detail → refresh → browser back → forward.

## Responsive 1366 / 1024

| Resolution | Result | Notes |
| --- | --- | --- |
| 1366×768 | passed | Table horizontal scroll; filters wrap; drawers closable |
| 1024×768 | passed_with_warning | Filter vertical wrap; no blocking overflow on spot-check pages |

Screenshot dirs: [`screenshots/h1-5/`](screenshots/h1-5/README.md)

## URL security

- No PII / token / prompt / raw response keys in allowlist
- `keyword` max 80 + sensitive hint on collect tasks (shared H1.4 UX)

## Accepted warnings

1. `keyword` may still enter browser history (MVP trade-off).
2. AI batch list `status` / `warningCode` URL restore; list API only supports `page`/`pageSize` — client filter on current page where applicable.

## Verification commands

```bash
pnpm check:ui-copy --strict
pnpm check:dev
pnpm build:admin
git diff --check
node scripts/h1-5-secondary-url-browser-check.mjs
node scripts/h1-5-live-browser-acceptance-check.mjs
pnpm demo:auto-acceptance   # when backend running
```

See also [`H1_5_LIVE_BROWSER_ACCEPTANCE.md`](H1_5_LIVE_BROWSER_ACCEPTANCE.md).

## Not in scope (unchanged)

- No tag / Production Ready / real preprod / Douyin real E2E / production gray release
- No new platform OpenAPI or heavy ERP features
