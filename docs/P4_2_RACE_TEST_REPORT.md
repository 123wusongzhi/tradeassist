# P4.2 Race Test Report

Go race detector verification for tenant/security worker changes.

## Status Banner

**Not Run on Windows Native** · **Deferred to Linux / WSL2 / CI** · **NOT Production Ready**

---

## Policy

The Go race detector (`go test -race`) is **not supported** on Windows native toolchains. P4.2 closure therefore **does not mark race tests as passed** when executed from Windows.

Run on:

- WSL2 (Ubuntu) with `CGO_ENABLED=1`
- GitHub Actions `backend-race` job (see Phase P3.2 precedent)
- Linux staging CI agent

---

## Recommended packages

```bash
cd backend
CGO_ENABLED=1 go test -race -count=1 \
  ./internal/securitytests/... \
  ./internal/modules/webhook/... \
  ./internal/modules/files/... \
  ./internal/modules/ordersync/... \
  ./internal/modules/inventory/... \
  ./internal/pkg/tasktenant/...
```

---

## P4.2-sensitive concurrency surfaces

| Surface | Risk |
| --- | --- |
| `tasktenant.BeginWorker` + worker BRPOP loops | Shared context wiring |
| `files/scan_worker` status transitions | Concurrent scan + upload |
| `webhook/ProcessQueuedEvents` | Parallel row processing |
| `securitymod/reencrypt_worker` | Batch updates vs API pause |
| `migrate_p4_2` backfill | One-time migration (not race-tested) |

---

## Historical baseline

- **P3.2**: WSL2 Linux `go test -race` on webhook/order packages — **passed**
- **P2**: WSL2 race verification — **passed** (no data race)

P4.2 does not regress that baseline; full re-run pending Linux CI execution.

---

## Result

| Environment | Status | Notes |
| --- | --- | --- |
| Windows native (closure script host) | **deferred_on_windows** | Expected |
| Linux / WSL2 | **not_run_in_p42_closure** | Execute before production tag |
| CI `backend-race` | **not_run_in_p42_closure** | Trigger on PR to `dev` |

**Passed only when race tests actually execute on Linux with exit code 0.**
