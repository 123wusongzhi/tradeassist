# P3.1 Race Test Report

## Environment

| Field | Value |
|-------|-------|
| OS | Windows 10 (native) — **Linux/WSL2 race run deferred to CI** |
| Go | see `go version` at run time |
| CGO_ENABLED | 0 (default Windows) |

## Note

`-race` requires CGO on Windows and is **not supported** on native Win32 builds. Full P3 race matrix must run on WSL2 Ubuntu / Linux CI / Docker Linux per phase checklist.

## Modules (compile + unit test on Windows)

```bash
go test ./internal/modules/douyinshop/...
go test ./internal/modules/webhook/...
go test ./internal/modules/ordersync/...
go test ./internal/modules/order/...
```

## Status

`deferred_linux_race` — code paths compiled; race detector pending Linux runner.

## Focus areas when running on Linux

- Token singleflight / TokenVersion
- Order Webhook concurrent upsert (`order/platform_upsert_test.go`)
- AI apply reconciliation
