# P4 PII Masking & Export

Personally identifiable information masking utilities, permission gates, and export controls.

## Status Banner

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**

---

## Masking Library

Location: `backend/internal/pkg/security/pii.go`

| Function | Input | Output Pattern |
| --- | --- | --- |
| `MaskPhone` | `13812345678` | `138****5678` |
| `MaskEmail` | `user@example.com` | `u***@example.com` |
| `MaskName` | `张三` | `张*` |
| `MaskAddress` | Full address | First 3–6 chars + `****` |
| `MaskIP` | IPv4/IPv6 | Network prefix + `***` |

Unit tests: `backend/internal/pkg/security/security_test.go`.

---

## Runtime Application

### Orders

`GET /api/v1/orders/:id` applies masking before response:

```go
func maskDetailPII(out *DetailDTO) {
    out.CustomerPhone = mask.Phone(out.CustomerPhone)
    out.CustomerEmail = mask.Email(out.CustomerEmail)
}
```

Source: `backend/internal/modules/order/handler.go`.

Uses separate `mask` package alias — aligns with `security.Mask*` semantics.

### Auth / Sessions

- Session list API does **not** return raw IP — only summaries stored at creation
- `IPHash` never serialized to JSON (`json:"-"`)

### Operation Logs

- `ip_hash` stored hashed
- `user_agent_summary` truncated at write time via `authutil.SummarizeUserAgent`

---

## Permission Model

| Key | Roles | Intended Use |
| --- | --- | --- |
| `pii.read_masked` | admin, operator, readonly | Default API responses |
| `pii.read_full` | admin | Support/debug full phone/email |
| `pii.export` | admin | CSV/Excel export with PII |

Matrix: `backend/internal/pkg/adminperm/matrix.go`.

### Current Enforcement Gap

Order detail always masks phone/email regardless of `pii.read_full`. Admin full-read path is **not yet wired** — safer default for P4.

Future pattern:

```go
if !principal.Can(adminperm.PermPIIReadFull) {
    maskDetailPII(out)
}
```

---

## Export Controls (Planned)

No dedicated PII export API in P4. Requirements when implemented:

1. Require `pii.export` permission
2. Audit log entry with action `pii_export`, resource scope, row count
3. Rate limit exports
4. Watermark / purpose field in request body
5. Async generation for large datasets

Operation log hash chain should record export events.

---

## AI & Logging

- Prompts must not embed full customer PII in persistent logs
- Idempotency keys explicitly avoid PII (`idempotency/keys.go`)
- Douyin webhook mapper comment: no PII in normalized summary

---

## Frontend Expectations

Admin UI should:

- Display masked phone/email in order/customer lists
- Never show full API keys — use `sk-****` pattern from settings API
- Gate "show full" buttons on `pii.read_full` when backend supports it

---

## Data Subject Requests (Out of Scope MVP)

Erasure, portability, and consent management are **not implemented**. Classification doc references retention policy only.

---

## Test Cases

| ID | Case | Expected |
| --- | --- | --- |
| PII-01 | Order detail as operator | Masked phone/email |
| PII-02 | MaskPhone short input | `****` |
| PII-03 | MaskEmail no @ | `****` |
| PII-04 | Session JSON | No ip_hash field |
| PII-05 | Admin with pii.read_full (future) | Full phone when enabled |

---

## Deferred Verification

- [ ] Wire `pii.read_full` to order/admin handlers
- [ ] Export API with audit + permission
- [ ] Frontend permission sync with backend keys

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**
