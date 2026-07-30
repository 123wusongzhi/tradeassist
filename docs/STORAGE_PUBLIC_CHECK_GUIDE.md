# Storage Public Check Guide

## Manual test (admin)

Settings → 存储 → **测试公网访问**  
Requires `settings.manage` permission.

## API

```http
POST /api/v1/settings/storage/public-check
GET  /api/v1/settings/storage/public-check/latest
POST /api/v1/storage/test-public-access   # legacy alias
```

## Flow

1. Upload 1×1 PNG under `system-tests/storage-public-check/`.
2. Resolve public URL from configured provider.
3. Anonymous HTTPS GET with SSRF checks (`pkg/storagepublic`, `pkg/safedownload`).
4. Verify status, Content-Type, image decode.
5. Delete test object.

## Not auto-run

Page load must not trigger upload; operator must click test.
