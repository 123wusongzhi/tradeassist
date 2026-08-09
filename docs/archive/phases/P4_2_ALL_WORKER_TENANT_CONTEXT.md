# P4.2 All Worker Tenant Context

How each registered worker obtains and enforces tenant (and shop) context.

## Status Banner

**Production Workers Tenant-Gated** · **System Workers Explicit** · **NOT Production Ready**

---

## tasktenant API

| Function | Purpose |
| --- | --- |
| `RequireTaskTenant` | Rejects `tenant_id <= 0` |
| `BuildWorkerContext` | Sets `AuthSourceWorker`, optional `ShopScope` |
| `BeginWorker` | Resolves tenant from shop when needed, returns scoped `context.Context` |
| `BeginWorkerFromShop` | Shop-only entry for shop-bound jobs |
| `ResolveShopTenant` / `ResolveProductTenant` | DB lookups for missing tenant on task rows |
| `EnsureResourceTenantMatch` / `EnsureShopInScope` | Post-load validation helpers |

Package: `backend/internal/pkg/tasktenant/`

---

## Per-worker behavior

### collect (`collect/worker.go`)

1. Parse Redis message → task UUID
2. Load `collect_tasks.tenant_id`
3. `BeginWorker(..., probe.TenantID, uuid.Nil, "collect")`
4. `RunCollectJob` runs under worker context

### order_sync (`ordersync/worker.go`)

1. BRPOP task id
2. Probe `order_sync_tasks` for `tenant_id`, `shop_id`
3. `BeginWorker(..., "order_sync")` with shop in scope

### customer_message_sync (`customersync/worker.go`)

Same pattern as order_sync with operation `"customer_message_sync"`.

### product_publish (`productpublish/worker_consumer.go`)

Same pattern with `"product_publish"`.

### inventory_sync (`inventory/worker_consumer.go`)

Resolves `shop_id` from task; `BeginWorker` with tenant + shop.

### file_security_scan (`files/scan_worker.go`)

1. Queue payload: `{ tenantId, assetId }`
2. `RequireTaskTenant` on enqueue and dequeue
3. `BeginWorker` → `repository.FindByID` → state machine transitions

### webhook (`webhook/processor.go`)

1. `ProcessEvent` / `ProcessEventByRowID` require `tenant_id`
2. `ProcessQueuedEvents` builds worker context per queued row
3. Shop scope from `internal_shop_id`

### security_secret_reencrypt (`securitymod/reencrypt_worker.go`)

- Uses `security.WorkerSystemContext(0, ...)` — **system-global** rotation
- Polls `key_rotation_jobs` where `status=running` and `dry_run=false`
- Not tenant-scoped by design (master key ring is global)

### Deferred / no tasktenant

| Worker | Reason |
| --- | --- |
| `imagetask` | Image tasks lack `tenant_id`; uses lease only |
| `task_alert_scan` | Cross-tenant failure aggregation |
| `export` | Model ready; consumer not yet tenant-gated |

---

## Registration (`cmd/server/main.go`)

Workers started with `worker.Registry` heartbeat:

- `files.StartScanWorker` → `file_security_scan`
- `securitymod.StartReencryptWorker` → `security_secret_reencrypt`
