# P4.2 Full Tenant and Security Worker Audit

Phase P4.2 extends P4.1 tenant isolation to background workers, async queues, and security automation.

## Status Banner

**Tenant Worker Context Implemented** · **Security Workers Registered** · **Real Environment Security Verification Deferred** · **NOT Production Ready**

---

## Scope

| Area | P4.2 deliverable |
| --- | --- |
| Worker tenant gate | `tasktenant.BeginWorker` / `RequireTaskTenant` on production Redis/DB workers |
| Model columns | `tenant_id` on sync/batch/export/collect/AI batch tables |
| Migration | `migrate_p4_2.go` — AutoMigrate + backfill + indexes |
| Security workers | `security_secret_reencrypt`, `file_security_scan` |
| Secret rotation targets | `settings_encrypted`, `shop_auth_tokens` |
| Regression tests | IDOR (22 automated), shop scope (5 automated) |

---

## Worker audit matrix

| Worker type | Registry constant | Tenant context | Shop scope | Notes |
| --- | --- | --- | --- | --- |
| collect | `collect` | ✓ `BeginWorker` from task `tenant_id` | — | Skips job when tenant missing |
| order_sync | `order_sync` | ✓ task + shop | ✓ `ShopID` in scope | |
| customer_message_sync | `customer_message_sync` | ✓ task + shop | ✓ | |
| product_publish | `product_publish` | ✓ task + shop | ✓ | |
| inventory_sync | `inventory_sync` | ✓ task + shop | ✓ | |
| webhook | `webhook` | ✓ row `tenant_id` + shop | ✓ | `ProcessQueuedEvents` |
| file_security_scan | `file_security_scan` | ✓ queue payload `tenantId` | — | `repository.FindByID` scoped load |
| security_secret_reencrypt | `security_secret_reencrypt` | System (`tenant_id=0`) | — | Global key rotation job |
| image | `image` | △ deferred | — | No `tasktenant` yet |
| task_alert_scan | `task_alert_scan` | △ system scan | — | Cross-tenant alert aggregation |
| export | `export` | △ model has column | — | Worker wiring deferred |

---

## Module tenant column additions (P4.2)

`migrate_p4_2.go` AutoMigrate + backfill from `shops` / `products`:

- `inventory_sync_tasks`, `inventory_sync_batches`, `inventory_change_logs`
- `order_sync_tasks`, `customer_message_sync_tasks`, `product_publish_tasks`
- `ai_product_text_batches`, `ai_product_image_batches`
- `customer_conversations`, `collect_tasks`, `douyin_image_assets`
- `export_jobs`, `files` (security index), `task_failure_marks`, `task_alerts`

---

## Residual gaps

1. **imagetask** worker — no `tasktenant` gate; image tasks lack `tenant_id` column (P4.1 backlog).
2. **IDOR automation** — 22 cases implemented; closure target 40+ (see `P4_2_IDOR_TEST_REPORT.md`).
3. **Shop scope automation** — 5 cases; closure target 20+ (see `P4_2_SHOP_SCOPE_TEST_REPORT.md`).
4. **Race detector** — deferred on Windows native builds (see `P4_2_RACE_TEST_REPORT.md`).

---

## Verification

```bash
node scripts/p4-2-security-final-closure-check.mjs
go test ./internal/securitytests/... -count=1
```

**Security Foundation Implemented** · **Tenant Worker Context Implemented** · **Real Environment Security Verification Deferred**
