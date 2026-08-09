# P4 SSRF Security

Server-Side Request Forgery protections for outbound HTTP fetches (image import, AI pipelines).

## Status Banner

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**

---

## Overview

User-controlled URLs must not reach internal services, cloud metadata endpoints, or private networks. TradeMind implements **`pkg/safedownload`** for guarded HTTP GET requests.

Primary consumers:

- `backend/internal/modules/aiproductimage/service.go` — remote image fetch
- `backend/internal/modules/product/douyin_images.go` — platform image upload pipeline

---

## safedownload.Download Flow

```text
Download(ctx, rawURL, opts)
  loop:
    validateURL(ctx, current)
    fetchOnce → data OR redirect location
    if redirect → validateURL again (max 5 hops)
    if RequireImage → validateImageBytes
  return Result{Data, ContentType, FinalURL}
```

Source: `backend/internal/pkg/safedownload/safedownload.go`.

---

## URL Validation

| Rule | Error Code |
| --- | --- |
| Scheme not http/https | `SAFE_DOWNLOAD_SCHEME_NOT_ALLOWED` |
| Userinfo in URL (`http://user:pass@host`) | `SAFE_DOWNLOAD_CREDENTIALS_IN_URL` |
| Host localhost, 0.0.0.0, *.localhost | `SAFE_DOWNLOAD_PRIVATE_HOST` |
| Metadata hosts (169.254.169.254, *.internal) | `SAFE_DOWNLOAD_METADATA_ENDPOINT` |
| DNS resolves to private/loopback/link-local IP | `SAFE_DOWNLOAD_PRIVATE_IP` |
| Dial to private IP (TOCTOU mitigation) | `SAFE_DOWNLOAD_PRIVATE_IP` |

### Blocked IP Ranges

- Loopback, private RFC1918, link-local
- IPv4 `0.0.0.0/8`
- AWS/GCP metadata `169.254.169.254`

Exported helpers for tests: `ValidateURL`, `IsPrivateIP`, `ErrCode`.

---

## HTTP Client Hardening

```go
CheckRedirect: ErrUseLastResponse  // manual redirect validation
DialContext: assertIPNotPrivate before connect
Timeout: ResponseTimeout (default 30s)
Body: io.LimitReader MaxBodyBytes+1 (default 10MB)
```

User-Agent: `TradeMind-SafeDownload/1.0` (configurable in Options).

---

## Redirect Handling

- Max redirects: 5 (default)
- Each `Location` resolved relative to previous URL
- Full `validateURL` on every hop (DNS re-check)

Prevents open redirect → internal URL bypass.

---

## Response Validation

When `RequireImage=true` (default):

1. Content-Type should be `image/*` (if present)
2. Body must decode as image (jpeg/png/gif/webp)
3. Dimensions must be > 0

Errors: `SAFE_DOWNLOAD_INVALID_CONTENT_TYPE`, `SAFE_DOWNLOAD_IMAGE_DECODE_FAILED`.

---

## Default Options

```go
MaxBodyBytes:    10 << 20
MaxRedirects:    5
ConnectTimeout:  10s
ResponseTimeout: 30s
RequireImage:    true
```

Override per call site for non-image downloads (if added in future).

---

## Out of Scope / Gaps

| Vector | Status |
| --- | --- |
| Collector Playwright navigations | Separate trust boundary (Node service) |
| Webhook inbound HTTP | Signature verification, not SSRF |
| Storage provider SDK callbacks | Provider-specific |
| DNS rebinding race | Partial — dial-time IP check helps |
| IPv6 unique local | Covered by `IsPrivate()` |

All new server-side URL fetch features **must** use `safedownload` or equivalent review.

---

## Integration Checklist

When adding URL fetch:

- [ ] Call `safedownload.Download` or `ValidateURL` first
- [ ] Set context timeout
- [ ] Log error code via `safedownload.ErrCode`, not full URL with credentials
- [ ] Do not follow redirects outside safedownload client

---

## Test Matrix (Manual)

| URL | Expected |
| --- | --- |
| `http://127.0.0.1/` | Blocked PRIVATE_HOST/IP |
| `http://169.254.169.254/` | METADATA_ENDPOINT |
| `http://example.com/redir→internal` | Blocked on redirect hop |
| `http://169.254.169.254.nip.io` | Blocked at DNS (if resolves private) |
| Valid public image URL | Success |

Automated tests: `backend/internal/pkg/safedownload/safedownload_test.go`.

---

## Deferred Verification

- [ ] Periodic SSRF canary in staging
- [ ] Review all `http.Get` / `http.Client` usages outside safedownload
- [ ] DNS rebinding timed attack test

**Security Foundation Implemented** · **Real Environment Security Verification Deferred** · **NOT Production Ready / NOT Penetration Test Passed**
