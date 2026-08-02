# Docker 部署说明

本文说明如何使用 Docker Compose 启动完整 TradeMind 项目。

## 组成服务

`docker-compose.full.yml` 包含：

- PostgreSQL 16
- Redis 7
- backend：Go Gin API
- admin：React 管理端，使用 nginx 托管并代理 `/api`
- collector：Node.js + Playwright 采集服务

OpenCLI Bridge 不在 Compose 服务列表中；只有启用 OpenCLI 时才在宿主机单独启动。

## 快速启动

```bash
cp .env.docker.example .env
docker compose -f docker-compose.full.yml up -d --build
```

Windows PowerShell：

```powershell
Copy-Item .env.docker.example .env
docker compose -f docker-compose.full.yml up -d --build
```

## 默认访问地址

| 服务 | 地址 |
| --- | --- |
| Admin | `http://127.0.0.1:8000` |
| Backend Health | `http://127.0.0.1:8080/health` |
| Playwright Collector Health | `http://127.0.0.1:3001/health` |

## 端口配置

可在 `.env` 中覆盖以下端口：

```env
ADMIN_PUBLISH_PORT=8000
BACKEND_PUBLISH_PORT=8080
COLLECTOR_PUBLISH_PORT=3001
POSTGRES_PUBLISH_PORT=5432
REDIS_PUBLISH_PORT=6379
```

完整环境变量说明见 [env.md](env.md)，采集双引擎的完整部署矩阵、验证与旧配置迁移见
[collector-engines.md](collector-engines.md)。修改 Docker 变量时必须同步
`.env.docker.example`、`docker-compose.full.yml`、本文档和 `docs/env.md`。

## Playwright 与 OpenCLI 的部署边界

完整 Compose 始终启动 Playwright Collector，并由 backend 通过
`http://collector:3001` 访问。OpenCLI 不放进容器：它作为宿主机轻量 Bridge
监听 `3100`，backend 仅在任务选择 `engine=opencli` 时访问
`http://host.docker.internal:3100`。因此宿主机 Bridge 停止只会让 OpenCLI
任务失败，不会影响 Playwright、backend 健康检查或其他采集来源。

Compose 中的 Playwright Collector 监听容器网络，因此 `.env` 必须设置同一个随机长
`COLLECTOR_INTERNAL_TOKEN` 供 backend 和 collector 使用；缺失时两端都会 fail-fast。
宿主机 `3001` 映射默认只绑定 `127.0.0.1`。Backend 默认不信任任何转发头；如需保留
反向代理后的真实客户端 IP，仅把实际 nginx/网关的精确 IP 或最小 CIDR 写入
`TRUSTED_PROXIES`，不得使用 `/0`。

默认 `OPENCLI_BRIDGE_ENABLED=false`，普通 Docker 用户不需要安装 OpenCLI。
需要启用时：

```env
OPENCLI_BRIDGE_ENABLED=true
OPENCLI_BRIDGE_BASE_URL=http://127.0.0.1:3100
OPENCLI_BRIDGE_DOCKER_BASE_URL=http://host.docker.internal:3100
OPENCLI_BRIDGE_HTTP_ADDR=0.0.0.0:3100
OPENCLI_BRIDGE_TOKEN=请替换为随机长字符串
COLLECT_DEFAULT_ENGINE_TAOBAO_TMALL=opencli
```

然后先在宿主机执行：

```bash
pnpm opencli:install-adapter
pnpm opencli:doctor
pnpm dev:opencli-bridge
```

Bridge 也会在启动时幂等同步仓库内的 TradeMind 淘宝/天猫适配器；显式安装命令用于
提前检查 OpenCLI 与用户适配器是否冲突。非 TradeMind 的同名本地适配器不会被覆盖。

再启动或重启 Compose。`0.0.0.0` 用于让 Docker Desktop 访问宿主机 Bridge，
所以 Bridge 会强制要求 Token；同时建议用本机防火墙限制 `3100`，不要直接暴露公网。
Compose 仅把 `OPENCLI_BRIDGE_DOCKER_BASE_URL` 注入 backend，因此同一份 `.env`
仍可直接用于本地启动，避免在 `127.0.0.1` 与 `host.docker.internal` 之间反复改值。
Linux 原生 Docker 若不支持 `host.docker.internal`，Compose 已配置
`host-gateway` 映射。状态可在管理端采集引擎选择器或
`GET /api/v1/collect/engines/status` 查看。

OpenCLI 的 `EMPTY_RESULT` 表示没有返回数据，可能由登录态、验证、限流、软拦截或
页面结构变化导致。它不会被直接解释为商品下架；只有明确的 `ITEM_NOT_FOUND`
错误才使用不存在分类。

Playwright 是淘宝/天猫可手动选择的备用引擎，不是 OpenCLI 失败后的自动回退。
任务实际引擎已持久化并显示在任务列表；Bridge 停止后可明确改用 Playwright 新建或
重试任务。详细行为见 [采集引擎与部署指南](collector-engines.md#任务如何选择引擎)。

P5-V 可观测性默认使用 `OTEL_EXPORTER_OTLP_PROTOCOL=http/json`。Docker 本地试用不配置真实 telemetry backend 时，`OTEL_EXPORTER_OTLP_ENDPOINT` 保持为空并视为 Deferred；不要把 Mock Collector 验证写成生产 collector 已上线。

P7 性能数据集与负载测试只能在隔离 `APP_ENV=performance` 环境执行；普通 Docker 试用与生产部署必须保持 `PERFORMANCE_TEST_MODE=false`、`ALLOW_PERFORMANCE_DATASET=false`，不得把隔离压测描述为真实生产容量验证。

## 安全配置

生产环境或公网部署前必须修改：

- `JWT_SECRET`
- `APP_MASTER_KEY`
- `ADMIN_BOOTSTRAP_PASSWORD`
- `POSTGRES_PASSWORD`
- `DB_PASSWORD`
- 所有第三方平台、AI、存储、Webhook、邮箱等密钥

不要把真实密钥提交到仓库，也不要写入镜像。

## 常用命令

启动：

```bash
docker compose -f docker-compose.full.yml up -d --build
```

查看状态：

```bash
docker compose -f docker-compose.full.yml ps
```

查看日志：

```bash
docker compose -f docker-compose.full.yml logs -f backend
docker compose -f docker-compose.full.yml logs -f admin
docker compose -f docker-compose.full.yml logs -f collector
docker compose -f docker-compose.full.yml logs -f postgres
docker compose -f docker-compose.full.yml logs -f redis
```

停止并保留数据卷：

```bash
docker compose -f docker-compose.full.yml down
```

清空数据卷：

```bash
docker compose -f docker-compose.full.yml down -v
```

> `down -v` 会删除 PostgreSQL、Redis、上传目录等 Compose 管理的数据卷，请谨慎执行。

## 默认管理员

默认管理员由 `.env` 中的以下变量决定：

```env
ADMIN_BOOTSTRAP_EMAIL=admin@example.com
ADMIN_BOOTSTRAP_PASSWORD=admin123456
```

首次登录后请尽快修改密码。生产环境不要使用示例密码。

## 与本地开发 Compose 的区别

- `docker-compose.yml`：仅用于本地开发基础设施，包含 PostgreSQL + Redis。
- `docker-compose.full.yml`：用于完整 Docker 部署，包含 PostgreSQL + Redis + backend + admin + collector。

**1688 采集浏览器 Profile**：`docker-compose.full.yml` 为 collector 挂载 `./data/browser-profiles` 与 `./data/storage-states`，用于持久化 1688 登录 Cookie（含 Login Data、Cookies、History、Local Storage、Session Storage 等 Chromium 用户数据）。这些目录**必须持久化挂载、禁止提交 Git**（已在 `.gitignore` 忽略；本地 `collector/data/browser-profiles/` 同理）。容器内默认无图形界面，**首次登录建议在宿主机本地运行 collector（`COLLECTOR_HEADLESS=0`）完成 1688 登录**，Profile 目录可被 Docker 复用；或在已配置远程桌面的 Linux 服务器上打开登录浏览器。

两套 Compose 的服务、端口和数据卷应分开理解。

## 配置校验

CI 会执行轻量 Docker 配置检查：

```bash
docker compose -f docker-compose.full.yml config
```

本地修改 Dockerfile、Compose 或 `.env.docker.example` 后，建议先执行同样命令确认语法和变量引用正确。
