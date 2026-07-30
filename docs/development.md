# 本地开发说明

本文说明贸灵 TradeMind 的本地开发启动方式。完整项目由 Go backend、React admin、Node collector、PostgreSQL 与 Redis 组成。

## 环境要求

- Node.js
- pnpm `9.15+`
- Go `1.25+`
- **二选一**（基础设施）：
  - Docker / Docker Compose（默认，`pnpm dev` 会自动 `docker compose up` 拉起 PostgreSQL / Redis）
  - 或本机已安装并运行 **PostgreSQL**（默认 `127.0.0.1:5432`）与 **Redis**（默认 `127.0.0.1:6379`），账号密码与 `.env` 一致

## 安装依赖

```bash
pnpm install
pnpm install:collector:browsers
```

## 一键开发启动

```bash
pnpm dev
```

`pnpm dev` 会启动本地基础设施与三个必需开发服务：

- PostgreSQL / Redis：优先使用 Docker Compose（`docker-compose.yml`）；若未检测到可用 Docker，则检测本机 `.env` 配置的 PostgreSQL / Redis 端口是否可连接，两者都可用则跳过 Compose
- backend Go 服务
- admin 管理端
- Playwright Collector 采集服务
- 可选 OpenCLI Bridge：仅在 `.env` 设置 `OPENCLI_BRIDGE_ENABLED=true` 时启动，
  启动失败不终止三个必需服务

## 常用命令

```bash
pnpm check:dev
pnpm dev:infra
pnpm dev:backend
pnpm dev:admin
pnpm dev:collector
pnpm p7:dataset -- --profile small
pnpm check:p7
pnpm check:p7:regression
pnpm dev:stop
pnpm dev:reset
```

说明：

- `pnpm check:dev`：检查 Node、pnpm、Go、Docker 或本机 PostgreSQL / Redis、环境变量等。
- `pnpm dev:infra`：仅启动 PostgreSQL 与 Redis。
- `pnpm p7:dataset -- --profile small`：运行 P7 数据集生成器 dry-run；写入隔离数据库需额外传 `--write` 并满足 performance 环境守卫。
- `pnpm check:p7` / `pnpm check:p7:regression`：生成 P7 性能容量与回归门闸报告；真实负载 / Soak / Race 证据未齐时会失败。
- `pnpm p7-v2:r3b:lpf-audit`：仅从冻结 Recovery3 evidence 导出并校验 Load Profile V2；不会启动 k6 或修改 Raw Artifact。
- `pnpm p7-v2:r3b:lpf-comparability`：使用版本化 V2 sidecar 执行 Recovery3 comparability；V1 报告保持不变。
- `pnpm p7-v2:r3b:regression`：仅在 Comparability V2 通过后评估冻结 Raw Artifact；不重新执行性能负载。
- `pnpm p7-v2:r3b:lpf-gate`：执行 LPF-V2 scoped gate；Soak、Demo、最终 Gate 不属于该命令范围。
- `pnpm dev`：启动前会自动释放本机 backend / admin（8000–8010）/ collector 端口上残留的上一进程，避免端口占用导致 backend 启动失败。
- `pnpm dev:stop`：停止默认 `docker-compose.yml` 服务，不删除 volume。
- `pnpm dev:reset`：重置默认 Compose 数据卷，可能清空本地数据库。

## 默认端口

| 服务 | 默认地址 |
| --- | --- |
| backend | `http://127.0.0.1:8080` |
| backend health | `http://127.0.0.1:8080/health` |
| admin | 通常为 `http://127.0.0.1:8000`，以终端输出为准 |
| Playwright Collector | `http://127.0.0.1:3001` |
| Playwright Collector health | `http://127.0.0.1:3001/health` |
| OpenCLI Bridge（可选） | `http://127.0.0.1:3100` |
| OpenCLI Bridge health（启用后） | `http://127.0.0.1:3100/health` |
| PostgreSQL | `127.0.0.1:5432` |
| Redis | `127.0.0.1:6379` |

## 环境变量

本地开发使用 `.env.example` 作为模板：

```bash
cp .env.example .env
```

Windows PowerShell：

```powershell
Copy-Item .env.example .env
```

关键配置：

- `DB_DRIVER=postgres`
- `DB_PORT=5432`
- `REDIS_ADDR=127.0.0.1:6379`
- `APP_HTTP_ADDR=:8080`
- `COLLECTOR_HTTP_ADDR=:3001`
- `COLLECTOR_PLAYWRIGHT_BASE_URL=http://127.0.0.1:3001`
- `OPENCLI_BRIDGE_ENABLED=false`
- `OPENCLI_BRIDGE_BASE_URL=http://127.0.0.1:3100`
- `OPENCLI_BRIDGE_DOCKER_BASE_URL=http://host.docker.internal:3100`
- `OTEL_EXPORTER_OTLP_PROTOCOL=http/json`（P5-V 标准 OTLP/HTTP JSON；真实 backend 未配置时 `TRACING_ENABLED=false`）

完整变量说明见 [env.md](env.md)，双引擎的部署选择和旧配置迁移见
[collector-engines.md](collector-engines.md)。新增或修改变量时，还要按
[module-map.md](module-map.md) 检查 Docker、README、部署文档和代码默认值。

不要提交 `.env` 或任何真实密钥。

## 分服务调试

基础设施：

```bash
pnpm dev:infra
```

后端：

```bash
pnpm dev:backend
```

管理端：

```bash
pnpm dev:admin
```

采集服务：

```bash
pnpm dev:collector
```

## 后端格式化

修改或新增 `backend/**/*.go` 后，在 `backend` 目录执行：

```bash
go fmt ./...
```

## 采集引擎调试

Playwright Collector 与 OpenCLI Bridge 是两个独立进程：

```bash
# Playwright（所有既有采集能力）
pnpm dev:collector

# OpenCLI（当前仅淘宝/天猫）
pnpm opencli:install-adapter
pnpm opencli:doctor
pnpm dev:opencli-bridge
```

本地需要 OpenCLI 时，在 `.env` 设置 `OPENCLI_BRIDGE_ENABLED=true`。`pnpm dev`
会把 Bridge 作为可选子进程启动；Bridge 异常不会结束 backend、admin 或
Playwright Collector。未启用 Bridge 时，后端会把未显式指定引擎的任务安全地
解析为 Playwright；显式选择 OpenCLI 则返回清晰的不可用错误，不会静默回退。
同一份 `.env` 切换 Docker 时无需改写本地地址：Compose 只读取
`OPENCLI_BRIDGE_DOCKER_BASE_URL` 注入容器。
Bridge 启动时会把仓库内 `collector/opencli-adapters/tmall/` 的受管适配器同步到
`~/.opencli/clis/tmall/`。同步是幂等的，只更新带 TradeMind 标记的适配器；
若检测到用户自己的同名适配器会停止并提示备份，不会直接覆盖。

OpenCLI 当前只支持淘宝/天猫。Playwright 是可手动选择的备用引擎，但任务运行失败后
不会自动跨引擎回退。支持范围、API 选择、Docker 混合部署和验收步骤统一见
[采集引擎与部署指南](collector-engines.md)。

```bash
pnpm collect:test -- --url "https://detail.1688.com/offer/..."
pnpm collect:test -- --source aliexpress --url "https://www.aliexpress.com/item/..."
```

## 故障排查

- Docker 未安装或未启动：可安装 Docker Desktop，或在本机启动 PostgreSQL / Redis（端口与 `.env` 中 `DB_HOST`/`DB_PORT`、`REDIS_ADDR` 一致）。
- 端口冲突：修改 `.env` 或停止占用端口的进程。
- 后端连不上数据库：使用 Docker 时确认 `docker compose ps` 中 PostgreSQL 为 healthy；使用本机服务时确认对应端口可连接。
- Collector 无法打开浏览器：重新执行 `pnpm install:collector:browsers`。
- `host.docker.internal:3100 connection refused`：确认这是 OpenCLI 任务，并在宿主机执行
  `pnpm opencli:doctor`、`pnpm dev:opencli-bridge`；普通 Playwright 任务不应访问 3100。
- OpenCLI 返回 `EMPTY_RESULT` 但浏览器能打开商品：这不代表商品已下架。先确认 OpenCLI
  扩展连接的是同一个 Chrome、登录/验证已完成，再检查限流或适配器结构变化；任务会按
  可恢复的解析失败展示，不再直接标为商品不存在。
- 管理端设置页若仍显示采集服务监听地址 `:3100`，这是历史 UI 空值回退，不会修改
  运行进程；以 `.env` 的 `COLLECTOR_HTTP_ADDR=:3001` 和 `3001/health` 为准。
- 更完整的端口、Token、旧镜像与旧 `COLLECTOR_BASE_URL` 排查见
  [collector-engines.md](collector-engines.md#常见问题)。
