# P7 Pagination Design

Current foundation:

- `pkg/pagination.NormalizePage` clamps list limits and rejects deep offset over `MaxOffset`.
- `pkg/pagination.EncodeCursor` and `DecodeCursor` define a scoped cursor payload with tenant/shop mismatch rejection.
- Product and order lists now reject excessive offset instead of allowing unbounded deep pagination.

Remaining closure:

- Migrate all high-volume lists to keyset/cursor APIs.
- Sign cursors before public use.
- Add cross-tenant and cross-shop HTTP tests for cursor endpoints.
