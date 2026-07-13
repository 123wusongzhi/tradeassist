# P5.2 Race Test Report

Linux Race Verification: not executed in this Windows environment.

Required command set for WSL2, Linux CI or Docker Linux:

```bash
go test -race ./internal/pkg/httpclient/... ./internal/pkg/logging/... ./internal/pkg/metrics/... ./internal/pkg/tracing/... ./internal/pkg/observability/...
go test -race ./internal/modules/webhook/... ./internal/modules/ordersync/... ./internal/modules/inventory/... ./internal/modules/aiproducttext/... ./internal/modules/aiproductimage/... ./internal/modules/files/... ./internal/modules/securitymod/... ./internal/modules/auth/... ./internal/modules/alerting/... ./internal/modules/observabilitymod/...
```

Do not mark Linux Race Verification Passed until a real Linux run reports 0 data races.
