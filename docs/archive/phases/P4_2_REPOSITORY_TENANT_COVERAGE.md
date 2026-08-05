# P4.2 Repository Tenant Coverage

Tenant scoping helpers and P4.2 table coverage at the data-access layer.

## Status Banner

**Repository Tenant Helpers Extended** · **P4.2 Columns Migrated** · **NOT Production Ready**

---

## Helper packages

| Package | File | Functions |
| --- | --- | --- |
| `repository` | `backend/internal/pkg/repository/tenant_scope.go` | `ScopeTenant`, `FindByID`, `DeleteByID`, `SystemFindByID` |
| `tenantquery` | `backend/internal/pkg/tenantquery/scope.go` | `ScopeTenant`, `ScopeShopTenant`, `ScopeProductTenant`, `FindByIDTenant`, `FindByIDShopTenant` |
| `taskcenter` | `backend/internal/modules/taskcenter/tenant_scope.go` | `applyListTenantScope`, `tenantIDFromGin` |

---

## P4.2 tables with `tenant_id`

| Table / model | Backfill source | Index (P4.2) |
| --- | --- | --- |
| `inventory_sync_tasks` | `shops.tenant_id` via `shop_id` | `idx_inv_sync_tenant_shop` |
| `inventory_sync_batches` | `shops.tenant_id` | — |
| `inventory_change_logs` | `products.tenant_id` via `product_id` | — |
| `order_sync_tasks` | `shops.tenant_id` | `idx_order_sync_tenant_shop` |
| `customer_message_sync_tasks` | `shops.tenant_id` | — |
| `product_publish_tasks` | `shops.tenant_id` | `idx_publish_tenant_shop` |
| `ai_product_text_batches` | `products.tenant_id` via batch items | `idx_ai_text_tenant` |
| `ai_product_image_batches` | `products.tenant_id` via batch items | `idx_ai_image_tenant` |
| `customer_conversations` | `shops.tenant_id` | — |
| `collect_tasks` | `products.tenant_id` via `result_product_id` | — |
| `douyin_image_assets` | `shops.tenant_id` | `idx_douyin_img_tenant_shop` |
| `export_jobs` | stamped at create | `idx_export_jobs_tenant` |
| `files` | stamped at upload | `idx_files_tenant_security` |
| `task_failure_marks` | stamped at write | `idx_task_failure_tenant` |

---

## Worker query pattern

```text
task row (tenant_id, shop_id)
  → tasktenant.BeginWorker
  → security.WorkerSystemContext + ShopScope
  → repository.FindByID / tenantquery.ScopeTenant
```

File scan worker additionally filters `WHERE id = ? AND tenant_id = ?` on status updates.

---

## Tables still shop-only (no direct tenant_id)

- `product_publications`, `product_publication_skus` — tenant inferred via product/shop joins
- `imagetask` / legacy `ai_tasks` — P4.1 backlog

---

## Migration entry

`database.Migrate` calls `migrateP42Security` after P4.1 migration chain.
