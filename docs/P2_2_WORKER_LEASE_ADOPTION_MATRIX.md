# P2.2 Worker Lease Adoption Matrix

> Phase P2.2 unifies **tasklease** (`execution_id` + heartbeat + `lock_version` / lease_version + finish guards) across six async workers.  
> **Core Reliability Foundation Ready** · **非 Production Ready**.

## Matrix

| Worker | Claim API | `execution_id` | `lease_version` (`lock_version`) | Heartbeat / renewal | ValidateLease before finish | Finish helper | Stale writeback test |
| --- | --- | --- | --- | --- | --- | --- | --- |
| ordersync | `tasklease.TryClaim` | ✓ | ✓ | `StartRenewal` | ✓ (via finish WHERE) | `finishOrderSyncTask` | package `tasklease` + finish guard; dedicated stale file N/A (P2.1) |
| inventory | `tasklease.TryClaim` | ✓ | ✓ | `StartRenewal` | ✓ | `finishInventorySyncTask` | same |
| productpublish | `tasklease.TryClaim` | ✓ | ✓ | `StartRenewal` | ✓ | `finishProductPublishTask` | finish WHERE includes execution_id + lock_version |
| collect | `TryClaimPendingOrRetrying` | ✓ | ✓ | `StartRenewal` | ✓ | `finishCollectTask` | `collect/lease_stale_worker_test.go` |
| imagetask | `TryClaimPendingOrRetrying` | ✓ | ✓ | `StartRenewal` | ✓ | `finishImageTask` | `imagetask/lease_stale_worker_test.go` |
| customersync | `tasklease.TryClaim` | ✓ | ✓ | `StartRenewal` | ✓ | `finishCustomerSyncTask` | `customersync/lease_stale_worker_test.go` |

## Claim semantics

### `TryClaim`

Atomic `pending → running` with new `execution_id`, `lock_version++`, `heartbeat_at`, `locked_by` / `locked_until`.

### `TryClaimPendingOrRetrying` (collect / imagetask)

Allows claim when status is `pending` **or** `retrying` with `next_retry_at IS NULL` (or due), still requiring free/expired lease.

## Heartbeat & renewal

- `StartRenewal` goroutine refreshes `heartbeat_at` / `locked_until` while work runs.
- TTL sourced from module task timeout env (collect / image / order sync / etc.).

## Finish / stale protection

Finish helpers update terminal status only when:

```text
id = ? AND locked_by = ? AND execution_id = ? AND lock_version = ?
```

Stale worker after takeover → `RowsAffected == 0` / `TASK_LEASE_LOST` semantics; no silent overwrite.

`ValidateLease` is available on each module lease wrapper and is used before committing side-effecting finish paths.

## Model fields (all six tables)

| Field | Role |
| --- | --- |
| `locked_by` / `locked_until` | Holder + expiry |
| `lock_version` | Optimistic lease version |
| `heartbeat_at` | Liveness for reaper / monitors |
| `execution_id` | Unique claim identity |

Migrations: P2.1 base columns; P2.2 `migrate_p2_2` indexes / closure for collect / image / customer sync.

## Verification

```bash
node scripts/p2-2-reliability-closure-check.mjs
```

Related: [`TASK_LEASE_AND_HEARTBEAT_DESIGN.md`](TASK_LEASE_AND_HEARTBEAT_DESIGN.md), [`STALE_WORKER_PROTECTION.md`](STALE_WORKER_PROTECTION.md), [`TASK_RELIABILITY_DESIGN.md`](TASK_RELIABILITY_DESIGN.md).
