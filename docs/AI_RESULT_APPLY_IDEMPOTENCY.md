# AI Result Apply Idempotency (P2.2)

> Phase P2.2 closes AI text/image **apply** under the shared `idempotency.Service`.  
> Status: **Core Reliability Foundation Ready** · **非 Production Ready** · does not imply real-platform E2E.

## Scope

| Flow | Module | Scope | Key builder |
| --- | --- | --- | --- |
| Text apply | `aiproducttext` | `ScopeAIText` | `AITextApply(batch, item, product, version)` |
| Image apply | `aiproductimage` | `ScopeAIImage` | `AIImageApply(batch, item, product, version, slot)` |

Entry points:

- Text: `POST .../ai-text/items/:id/apply` → `applyOneItem` → `acquireTextApply`
- Image: `POST .../ai-images/items/:id/apply` → `service_apply.applyOneItem` → `acquireImageApply`

Helpers live in `idempotency_apply.go` (per module); keys in `idempotency/keys.go`.

## Key patterns

```text
ai-text-apply:{batchId}:{itemId}:{targetProductId}:{targetVersion}
ai-image-apply:{batchId}:{itemId}:{targetProductId}:{targetVersion}:{slot}
```

- `targetVersion` is the frozen product / image `updated_at` (or equivalent) captured at review time.
- Image `slot` is stable per apply mode (`main`, `gallery:0`, `detail:{imageId}`, `replace:{id}`, `white_background`, …) via `applySlot`.
- Legacy 3-part keys (`LegacyAITextApply` / `LegacyAIImageApply`) exist for migration/tests only.

## Request hash rules

Hashes use `idempotency.HashRequest` (SHA-256 of JSON-normalized payload). **No secrets or PII.**

**Text apply** (`textApplyRequestHash`):

```json
{
  "batchId", "itemId", "targetProductId", "targetVersion",
  "operationType", "normalizedResultHash"
}
```

**Image apply** (`imageApplyRequestHash`):

```json
{
  "batchId", "itemId", "targetProductId", "targetVersion",
  "slot", "applyMode", "normalizedResultHash"
}
```

Same key + different hash → `IDEMPOTENCY_KEY_CONFLICT`.

## Flow

```text
Acquire(scope, key, requestHash, owner, DefaultLease)
  ├─ AlreadySucceeded → replay Complete summary / applicationId (no second write)
  ├─ InProgress       → IDEMPOTENCY_IN_PROGRESS
  ├─ KeyConflict      → IDEMPOTENCY_KEY_CONFLICT
  └─ Acquired         → apply product/image under version guard
                         ├─ version conflict → Fail(non-retryable) + conflict code
                         └─ success          → Complete(resource = application)
```

Owners default to `ai-text-apply` / `ai-image-apply`.

## Version conflict codes

| Code | When |
| --- | --- |
| `AI_TEXT_TARGET_VERSION_CONFLICT` | Product content changed since frozen snapshot |
| `AI_IMAGE_TARGET_VERSION_CONFLICT` | Target image / product image state changed |
| `IDEMPOTENCY_IN_PROGRESS` | Concurrent apply holds the lease |
| `IDEMPOTENCY_KEY_CONFLICT` | Same key, different request hash (or permanent fail) |

Handlers map conflict statuses to HTTP conflict responses; item status may become `conflict`.

## Concurrent behavior

- Concurrent identical applies: one `Acquired`, others `InProgress` or replay after success.
- Successful apply creates at most one application record per key (enforced by `ux_idempotency_scope_key` + domain guards).
- Unit coverage: `aiproducttext/apply_idempotency_test.go`, `aiproductimage/apply_idempotency_test.go`.

## Related

- Undo: [`AI_RESULT_UNDO_DESIGN.md`](AI_RESULT_UNDO_DESIGN.md)
- Keys / scopes: [`IDEMPOTENCY_DESIGN.md`](IDEMPOTENCY_DESIGN.md)
- Closure matrix: [`P2_2_RELIABILITY_CLOSURE_MATRIX.md`](P2_2_RELIABILITY_CLOSURE_MATRIX.md)
