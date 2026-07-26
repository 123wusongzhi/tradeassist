# Workbench URL State Design

> **Phase**: H1.1 + H1.2 + H1.5
> **Goal**: make key workbench pages recover filters, pagination, tabs, and drawers after refresh/back navigation.

## Shared Utilities

Implementation entry points:

- `admin/src/utils/urlState.ts`
- `admin/src/hooks/useUrlState.ts`

The shared utility uses a query-key allowlist. Unsupported keys are ignored on write; unknown keys already in the URL are preserved.

Allowed state keys include:

```text
page
pageSize
keyword
status
type
taskType
priority
platform
shopId
tab
id
drawer
source
start
end
detailTaskType
failureCategory
severity
recoveryStatus
normalizedStatus
includeResolved
includeMarked
timeRange
payStatus
skuStatus
inventoryStatus
fulfillmentStatus
dateFrom
dateTo
createdFrom
createdTo
updatedFrom
updatedTo
exceptionType
publishStatus
aiStatus
stockStatus
syncStatus
skuBindStatus
productSkuId
batchId
alertType
replyStatus
aiSuggestionStatus
sendStatus
conversationId
suggestionId
productSource
warningCode
resultStatus
retryable
failedPagesOnly
publishMode
taskId
sourcePlatform
targetShopId
```

Legacy deep-link keys (read + write when explicitly set):

```text
jumpOrder
orderId
itemId
jumpId
skuId
missingAiTitle
missingAiDescription
readiness
publishable
pendingReply
hasAiSuggestion
sendFailed
hasOrder
```

## Security Rules

- Do not write secrets, API keys, tokens, cookies, full prompts, raw responses, buyer PII (name / phone / email / address), or platform credentials into URLs.
- Only stable identifiers and non-sensitive filter values may be stored.
- User-facing labels still come from copywriting/status mapping constants; URL values may be internal stable codes.

## Source Parameter

Supported `source` values (navigation context only; **not** used for RBAC):

```text
dashboard
taskcenter
order_detail
inventory
customer
collect
manual
ai_workbench
config_status
publish_batch
order_sync
customer_sync
```

- Dashboard outbound links use `appendSourceToUrl(..., 'dashboard')`.
- Failure task center detail links use `appendSourceToUrl(..., 'taskcenter')`.
- Invalid `source` values are ignored for display logic; pages work without `source`.

## Implemented Pages (H1.1)

### `/dashboard/product-operations`

Persisted: date range, platform, shop, **product source (`productSource`)**. Navigation provenance uses separate `source` (e.g. inbound `source=dashboard` from linked pages). Product origin filter reads legacy `source` only when the value is not a known nav source. Outbound links append `source=dashboard` when missing.

### `/ai/operation-workbench`

Persisted: todo type, priority, platform, shop, keyword, date range, page / pageSize, detail drawer `drawer=todo&id=...`.

### `/ops/task-center/failures`

Persisted: task type, normalized status, failure category, recovery status, severity, platform, shop, keyword, date range, include resolved / marked switches, page / pageSize, detail drawer `drawer=failure&id=...&detailTaskType=...`. Legacy `jumpId` + `taskType` deep link remains supported.

## Implemented Pages (H1.2)

### `/orders/list` (`/orders` redirect)

Persisted: `keyword`, `payStatus`, `skuStatus`, `inventoryStatus`, **`status`**, **`fulfillmentStatus`**, **`start` / `end`** (created date range; semantic alias `dateFrom` / `dateTo`), `platform`, `shopId`, `page`, `pageSize`, `source`. Legacy `jumpOrder` redirects to order detail. Form fields map: `paymentStatus` ↔ `payStatus`, `skuMatchStatus` ↔ `skuStatus`, `inventoryDeductStatus` ↔ `inventoryStatus`, `createdAt` range ↔ `start` / `end`.

### `/orders/exceptions`

Persisted: `keyword`, `exceptionType`, **`severity`**, `platform`, `shopId`, `status`, **`start` / `end`** (created date range), `page`, `pageSize`, `source`. Legacy `orderId` deep link from order detail (with `source=order_detail`) remains supported.

## Keyword UX (H1.4)

Shared utilities:

- `admin/src/utils/keywordSafety.ts` — max length 80, sensitive-pattern hint (no logging)
- `admin/src/components/common/KeywordSafetyHint.tsx`
- `admin/src/hooks/useKeywordSearchField.ts` — `allowClear` + URL `keyword` / `page` cleanup

Applied on: orders, order exceptions, product drafts, inventory, inventory alerts, customer conversations, task-center failures, AI operation workbench.

Rules:

- Truncate or warn when keyword exceeds 80 characters.
- Show lightweight Alert when keyword resembles phone / email / ID / token / secret; do not block search.
- Clearing keyword removes `keyword` from URL and resets `page` to 1; other filters remain.

### `/product/drafts`

Persisted: `keyword`, `status`, `platform`, `shopId`, `publishStatus`, `aiStatus`, `page`, `pageSize`, `source`. Legacy params `missingAiTitle`, `missingAiDescription`, `readiness`, `publishable` remain compatible and map to `aiStatus` / `publishStatus` aliases.

### `/inventory`

Persisted: `keyword`, `stockStatus`, `syncStatus`, `skuBindStatus`, `platform`, `shopId`, `productSkuId`, `page`, `pageSize`, `source`. Legacy `skuId` alias maps to `productSkuId`.

### `/inventory/alerts`

Persisted: `keyword`, `alertType`, `stockStatus`, `platform`, `shopId`, `page`, `pageSize`, `source`.

### `/inventory/sync-tasks`

Persisted: `keyword`, `status`, `syncStatus`, `platform`, `shopId`, `productSkuId`, `batchId`, `drawer`, `id`, `page`, `pageSize`, `source`. `?id=` reopens task detail drawer on refresh; `syncStatus` aliases `status`.

### `/customer/hub`

Lightweight: `platform`, `shopId`, `source`. Hub cards pass filters into conversation list links.

### `/customer/conversations`

Persisted: `keyword`, `replyStatus`, `aiSuggestionStatus`, `sendStatus`, `platform`, `shopId`, `page`, `pageSize`, `conversationId`, `suggestionId`, `drawer`, `source`. Legacy `pendingReply`, `hasAiSuggestion`, `sendFailed`, `hasOrder`, `status=pending_reply` remain compatible. `conversationId` redirects to `/customer/conversations/:id`; `suggestionId` is honored on conversation detail.

## Implemented Pages (H1.5)

### `/product/publish-tasks`

Persisted: `tab` (`tasks`|`batches`), `status`, `platform`, `shopId`, `productId`, `batchId`, `page`, `pageSize`, `drawer`, `id`, `source`, `start`/`end`. Batch tab links to `/product/publish-batches/:id?source=…`; back preserves `source` + `tab=batches`.

### `/collect/tasks`

Persisted: `keyword`, `status`, `sourcePlatform`, `batchId`, `page`, `pageSize`, `drawer=events`, `id`, `source`. Collect provider filter uses `sourcePlatform` (legacy non-nav `source=1688` still read). Product draft links append `source=collect`.

### `/orders/sync-tasks`

Persisted: `status`, `resultStatus` (alias for `partial_success`), `platform`, `shopId`, `page`, `pageSize`, `drawer=task`, `id`, `source`, date range.

### `/customer/message-sync-tasks`

Persisted: `status`, `resultStatus`, `platform`, `shopId`, `page`, `pageSize`, `drawer=task`, `id`, `source`, date range.

### `/ai/text-batches` + `/product/ai-text-batches/:id`

List: `status`, `page`, `pageSize`, `batchId`, `source`. Detail: `itemId`, `tab`, `source`. AI workbench outbound uses `source=ai_workbench`.

### `/ai/image-batches` + `/product/ai-image-batches/:id`

List: `status`, `warningCode`, `page`, `pageSize`, `batchId`, `source`. Detail: `itemId`, `tab`, `warningCode`, `source`.

## Compatibility Strategy

1. On init, read legacy deep-link params first (`jumpOrder`, `orderId`, `itemId`, `productSkuId`, `skuId`, `batchId`, `suggestionId`, `jumpId`, `tab`).
2. New URL state writes use canonical H1.2 keys; legacy aliases are translated on read, not removed from inbound links.
3. Clearing filters via ProTable reset clears page-specific query keys via `clearUrlState`.
4. Default pagination (page 1, pageSize 20) and empty filters are omitted from the URL.
