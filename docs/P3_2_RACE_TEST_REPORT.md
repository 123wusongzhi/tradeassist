# P3.2 Race Test Report

Final result: passed

Race verification was completed in WSL2 Ubuntu 22.04 during this pass.

Environment:

- OS: Ubuntu 22.04 on WSL2.
- Go: go1.26.2 linux/amd64, installed temporarily under `/tmp/codex-go`.
- `CGO_ENABLED`: 1.
- gcc: Ubuntu 11.4.0-1ubuntu1~22.04.3.

Command:

```bash
go test -race \
  ./internal/modules/order/... \
  ./internal/modules/ordersync/... \
  ./internal/modules/webhook/... \
  ./internal/providers/platform/douyinshop/... \
  ./internal/modules/shop/... \
  ./internal/modules/productpublish/... \
  ./internal/modules/aiproducttext/... \
  ./internal/modules/aiproductimage/...
```

Result summary:

- `internal/modules/order`: passed.
- `internal/modules/ordersync`: passed.
- `internal/modules/webhook`: passed.
- `internal/providers/platform/douyinshop`: passed.
- `internal/modules/shop`: passed.
- `internal/modules/productpublish`: passed.
- `internal/modules/aiproducttext`: passed.
- `internal/modules/aiproductimage`: passed.

No data race was reported by this command. This does not imply real Douyin credential E2E, production gray release, tag readiness, or Production Ready status.
