# Workbench URL State Design

> **Phase**: H1.1  
> **Goal**: make key workbench pages recover filters, pagination, tabs, and drawers after refresh/back navigation.

## Shared Utilities

Implementation entry points:

- `admin/src/utils/urlState.ts`
- `admin/src/hooks/useUrlState.ts`

The shared utility uses a query-key allowlist. Unsupported keys are ignored.

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
```

## Security Rules

- Do not write secrets, API keys, tokens, cookies, full prompts, raw responses, or platform credentials into URLs.
- Only stable identifiers and non-sensitive filter values may be stored.
- User-facing labels still come from copywriting/status mapping constants; URL values may be internal stable codes.

## Implemented Pages

### `/dashboard/product-operations`

Persisted:

- date range
- platform
- shop
- product source

Outbound links from the dashboard append `source=dashboard` when missing.

### `/ai/operation-workbench`

Persisted:

- todo type
- priority
- platform
- shop
- keyword
- date range
- page / pageSize
- detail drawer: `drawer=todo&id=...`

Refreshing a URL with drawer parameters reopens the todo detail.

### `/ops/task-center/failures`

Persisted:

- task type
- normalized status
- failure category
- recovery status
- severity
- platform
- shop
- keyword
- date range
- include resolved / marked switches
- page / pageSize
- detail drawer: `drawer=failure&id=...&detailTaskType=...`

The legacy `jumpId` + `taskType` deep link remains supported.

## Deferred Pages

Orders, inventory, customer, and product draft lists already have partial deep-link behavior in places. They are planned for a follow-up H1 batch to avoid mixing wide UI churn into the first URL-state pass.
