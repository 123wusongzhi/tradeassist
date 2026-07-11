# AI Result Undo Design (P2.2)

> Undo restores product / image state from **apply application records**, guarded by `idempotency.Service` and target-version checks.  
> **非 Production Ready** · not a claim of platform E2E.

## Entry points

| Flow | API | Service | Key |
| --- | --- | --- | --- |
| Text undo | `POST .../ai-text/batches/:id/undo-applied` | `UndoApplied` → `acquireTextUndo` | `AITextUndo(applicationId, targetVersion)` |
| Image undo | `POST .../ai-images/batches/:id/undo-applied` | `UndoApplied` → `acquireImageUndo` | `AIImageUndo(applicationId, targetVersion)` |

```text
ai-text-undo:{applicationId}:{targetVersion}
ai-image-undo:{applicationId}:{targetVersion}
```

Scopes remain `ScopeAIText` / `ScopeAIImage`. Owners: `ai-text-undo` / `ai-image-undo`.

## Request hash

```json
{ "applicationId", "targetVersion" }
```

→ `idempotency.HashRequest`.

## Status chain

```text
pending_review → applied → (undo) → pending_review
                              ↘ conflict (version mismatch)
```

- Item must be `applied` with a valid `application_id`.
- Successful undo clears `application_id` / `applied_at` / `applied_by` and returns item to `pending_review`.
- Response item status string for success path: `undone`.
- Application row is updated via product / image undo helpers (restore previous content or image slot).

### Image-specific

- `set_main` undo restores `previousBestMainId` when recorded on the application.
- Slot-scoped apply records ensure gallery / detail / replace undos do not collide across slots.

## Conflict codes

| Code | Meaning |
| --- | --- |
| `AI_TEXT_UNDO_VERSION_CONFLICT` | Product changed after apply; unsafe to restore |
| `AI_IMAGE_UNDO_VERSION_CONFLICT` | Image / product image changed after apply |
| `IDEMPOTENCY_IN_PROGRESS` | Concurrent undo on same key |
| `IDEMPOTENCY_KEY_CONFLICT` | Same undo key, different hash / permanent fail |

On version conflict the idempotency record is `Fail(..., retryable=false)` with the domain conflict code.

## Replay

- Already-succeeded undo key → treat as replay (`undone` / no second restore).
- Does not invent a new application; resource type stays `product_ai_content_application` / `product_image_application`.

## Related

- Apply: [`AI_RESULT_APPLY_IDEMPOTENCY.md`](AI_RESULT_APPLY_IDEMPOTENCY.md)
- Domain keys: `backend/internal/modules/idempotency/keys.go`
