# P7-C3 Pagination Wiring Audit

Status: Partial.

Implemented:

- Product, Order, Inventory Center, Webhook Event, and Operation Log lists accept `cursor` + `limit`, return `items`, `nextCursor`, `hasMore`, and keep legacy `list` + `pagination`.
- Cursor payload binds tenant, optional shop scope, stable filter fingerprint, sort value, and UUID tie-breaker.
- Legacy offset remains bounded by `pagination_offset_too_deep`.
- Webhook Event now has authenticated `GET /api/v1/webhook-events`; payload body is not returned.

Partial:

- Task Center supports signed cursor over the merged projection, but still depends on cross-table in-memory merge. It is not yet a pure repository SQL keyset implementation.

