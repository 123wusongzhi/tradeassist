# P4 Upload & Download Security

File upload validation, object key sanitization, static serving, and safe remote download.

## Status Banner

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**

---

## Upload Paths

| Entry | Handler | Validation Layer |
| --- | --- | --- |
| `POST /api/v1/files/upload` | `files.Handler.Upload` | `files/service.go` (inline) |
| Product/platform image uploads | Product handlers | May use safedownload for remote URLs |
| AI image pipelines | `aiproductimage` | `safedownload` for URL fetch |

---

## pkg/security/upload.go

Central validation function `ValidateUpload(cfg, filename, contentType, data)`:

| Check | Limit (default) |
| --- | --- |
| File size | `cfg.MaxUploadBytes()` / `UPLOAD_MAX_MB` |
| Extension | jpg, jpeg, png, gif, webp |
| Content-Type | Must be `image/*` |
| Decode | `image.DecodeConfig` (+ webp fallback) |
| MIME vs format | Must match decoded format |
| Dimensions | Max 8192×8192 (`UPLOAD_MAX_IMAGE_WIDTH/HEIGHT`) |
| Pixel bomb | Max 50M pixels (`UPLOAD_MAX_IMAGE_PIXELS`) |
| GIF frames | Max 300 (`UPLOAD_MAX_ANIMATION_FRAMES`) |

Additional helpers:

- `SanitizeObjectKey(key)` — rejects `..`, leading `/`, backslashes
- `DrainAndLimit(r, max)` — bounded read

---

## files/service.go (Current Upload)

Direct upload path performs:

1. Size limit via `io.LimitReader`
2. `http.DetectContentType`
3. Extension whitelist
4. Image decode verification
5. Path traversal check on filename
6. Storage provider upload
7. Persist `FileRecord` with `tenant_id`, `security_status=pending_scan`

**Gap:** `ValidateUpload` from security package is available but not yet unified in all upload code paths — files service uses parallel inline checks.

---

## FileRecord Security Fields

```go
TenantID       int64
SecurityStatus string  // default: pending_scan
ScanStatus     string  // default: pending_scan
ObjectKey      string  // unique, sanitized
```

Future: async scanner updates `security_status` to `clean` or `blocked`.

Model: `backend/internal/modules/files/model.go`.

---

## Static Download

| Route | Auth | Risk |
| --- | --- | --- |
| `GET /static/*filepath` | Public | Path traversal, unauthorized file access |

Handler: `files.StaticHandler` — must resolve object keys through storage provider, not raw filesystem paths.

**Verify:** object keys are UUID-based paths, not user-controlled full paths.

---

## Safe Remote Download

Package: `backend/internal/pkg/safedownload/safedownload.go`

Used when server fetches user-supplied URLs (e.g. product image import):

| Control | Detail |
| --- | --- |
| Scheme | http/https only |
| Credentials in URL | Blocked |
| Private IP | Blocked at DNS and dial |
| Metadata endpoints | Blocked (`169.254.169.254`, `*.internal`) |
| Redirects | Max 5; each hop re-validated |
| Body size | Max 10MB default |
| Content | Image decode required when `RequireImage=true` |

Error codes: `SAFE_DOWNLOAD_*` constants.

---

## Configuration

| Env Var | Default | Purpose |
| --- | --- | --- |
| `UPLOAD_MAX_MB` | 10 | Max upload bytes |
| `UPLOAD_MAX_IMAGE_PIXELS` | 50000000 | Decompression bomb limit |
| `UPLOAD_MAX_IMAGE_WIDTH` | 8192 | Width cap |
| `UPLOAD_MAX_IMAGE_HEIGHT` | 8192 | Height cap |
| `UPLOAD_MAX_ANIMATION_FRAMES` | 300 | GIF frame cap |
| `UPLOAD_MAX_FILES` | 10 | Batch upload cap (auth config) |

Source: `backend/internal/config/auth_config.go`.

---

## Threat Matrix

| Threat | Mitigation | Status |
| --- | --- | --- |
| Polyglot file upload | Magic + decode verify | ✓ |
| Zip/image bomb | Pixel/dimension limits | ✓ |
| Path traversal | SanitizeObjectKey, filename check | ✓ |
| SSRF on URL import | safedownload | ✓ |
| Malware | security_status hook | ⏳ Scanner not wired |
| Cross-tenant file access | tenant_id on record | ⚠ List/delete scoping partial |

---

## Recommendations

1. Unify all uploads on `security.ValidateUpload`
2. Wire virus scan worker to update `security_status`
3. Authenticate static downloads or sign URLs for non-public buckets
4. Add tenant filter to `GET /files` and `DELETE /files/:id`

---

## Deferred Verification

- [ ] Fuzz upload with malicious GIF/WebP
- [ ] Attempt `../` in object keys
- [ ] SSRF canary against safedownload (internal IP URLs)

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**
