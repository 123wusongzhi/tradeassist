# P7 Database Index Audit

Status: initial index foundation added, query-plan validation pending.

The P7 migration adds PostgreSQL-only candidate indexes for:

- `products (tenant_id, created_at DESC, id DESC)`
- `orders (tenant_id, created_at DESC, id DESC)`
- `orders (tenant_id, shop_id, created_at DESC, id DESC)`
- `inventory_sync_tasks (tenant_id, status, updated_at DESC)`
- `collect_tasks (tenant_id, updated_at DESC, id DESC)`
- `webhook_events (tenant_id, status, created_at DESC)`
- `operation_logs (tenant_id, created_at DESC, id DESC)`
- `files (tenant_id, security_status, created_at DESC)`
- `backup_jobs (status, created_at DESC)`
- `release_runs (environment, state)`

These are candidates based on existing list/filter patterns. Production large-table concurrent index rollout remains deferred and must be planned separately.
