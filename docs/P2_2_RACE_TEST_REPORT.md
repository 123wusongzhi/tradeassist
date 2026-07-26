# P2.2 Race Test Report

## Environment

| 项 | 值 |
| --- | --- |
| 环境 | WSL2 Ubuntu-22.04 |
| Go 版本 | go1.26.2 linux/amd64 |
| CGO | `CGO_ENABLED=1` |
| 宿主机 | Windows 10（本机无 gcc，无法直接 `-race`） |
| 执行脚本 | `scripts/p2-2-race-wsl.sh` |
| 执行日期 | 2026-07-11 |

## Commands

```bash
CGO_ENABLED=1 go test -race \
  ./internal/modules/idempotency/... \
  ./internal/pkg/tasklease/... \
  ./internal/modules/aiproducttext/... \
  ./internal/modules/aiproductimage/... \
  ./internal/modules/webhook/... \
  ./internal/modules/collect/... \
  ./internal/modules/imagetask/... \
  ./internal/modules/customersync/...
```

## Results

| 模块 | 结果 | Data race |
| --- | --- | --- |
| idempotency | passed | 无 |
| tasklease | passed | 无 |
| aiproducttext | passed | 无 |
| aiproductimage | passed | 无 |
| webhook | passed | 无 |
| collect | passed | 无 |
| imagetask | passed | 无 |
| customersync | passed | 无 |

**结论：通过。未发现 data race。**

## Notes

- Windows 宿主 `CGO_ENABLED=0` 且无 gcc，故 race 在 WSL Linux 执行。
- 未伪造 Windows `-race` 通过。
