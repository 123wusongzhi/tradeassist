# P4.2 File Scan Worker

Asynchronous file security scanning after upload.

## Status Banner

**File Scan Worker Implemented** · **Tenant-Scoped Queue** · **NOT Production Ready**

---

## Overview

| Item | Value |
| --- | --- |
| Worker type | `file_security_scan` |
| Queue | Redis `file:security:scan` (LPUSH / BRPOP) |
| Entry | `files.StartScanWorker` in `cmd/server/main.go` |
| Timeout | 60s per scan (`context.WithTimeout`) |

---

## Flow

```text
POST /api/v1/files/upload
  → FileRecord security_status = pending_scan
  → EnqueueSecurityScan(tenantId, assetId)
Redis BRPOP
  → RequireTaskTenant(payload.tenantId)
  → BeginWorker → repository.FindByID (tenant scoped)
  → pending_scan → scanning → clean | rejected | quarantined | scan_failed
```

---

## Scanners (composite)

`buildFileScanner` registers:

1. `BasicFilePolicyScanner` — MIME/size/policy checks
2. `ImageDecodeScanner` — decode validation for images

Package: `backend/internal/pkg/filescanner/`

---

## State machine

`files/state_machine.go`:

| From | To | Guard |
| --- | --- | --- |
| `pending_scan` | `scanning` | `CanTransition` |
| `scanning` | `clean` / `rejected` / `quarantined` / `scan_failed` | scanner result |

Download access (`files/access.go`) blocks `pending_scan`, `quarantined`, `rejected`.

---

## Tenant isolation

- Queue message carries `tenantId` + `assetId`
- Enqueue rejects `tenant_id <= 0`
- DB updates include `tenant_id` in WHERE clause
- Object keys use prefix `t{tenantId}/` at upload

---

## Index

`idx_files_tenant_security` on `(tenant_id, security_status)` — created by `migrate_p4_2.go`.

---

## Failure handling

- Scan error → `security_status = scan_failed`
- Worker logs `file_scan_worker_error` (no object content in logs)
- Manual re-upload or admin retry path deferred
