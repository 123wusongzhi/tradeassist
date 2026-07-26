# 环境变量说明

本文件是 `.env.example` 与 `.env.docker.example` 的说明索引。新增、删除或重命名环境变量时，必须同步更新本文件，并检查 `docs/module-map.md` 中的关联项。

## 使用方式

本地开发：

```bash
cp .env.example .env
```

Windows PowerShell：

```powershell
Copy-Item .env.example .env
```

Docker 完整部署：

```bash
cp .env.docker.example .env
docker compose -f docker-compose.full.yml up -d --build
```

## 安全规则

- 不提交 `.env`。
- 生产环境必须替换 `JWT_SECRET`、`APP_MASTER_KEY`、`ADMIN_BOOTSTRAP_PASSWORD`、数据库密码。
- AI API Key、平台 Secret、Access Token、Refresh Token、Webhook Secret 不应写入环境模板，优先通过后台 settings 加密保存。
- 日志不得输出完整密钥、Token、Cookie 或密码。

## 后端基础配置

| 变量 | 示例 / 默认 | 服务 | 敏感 | 说明 |
| --- | --- | --- | --- | --- |
| `APP_ENV` | `development` | backend | 否 | 应用环境，生产建议设为 `production`。 |
| `APP_HTTP_ADDR` | `:8080` | backend | 否 | Go API 监听地址。 |
| `P7_V2_API_HOST` | `127.0.0.1` | P7-V2 harness | 否 | 仅 P7-V2 性能环境的 loopback API host；禁止公网或 WSL 非 loopback 地址。 |
| `P7_V2_API_PORT` | `8080` | P7-V2 harness | 否 | 仅 P7-V2 性能环境 API 端口；可迁移至 `18080`、`28080` 或 `38080`。 |
| `P7_BASE_URL` | 派生自前两项 | P7-V2 harness | 否 | 必须等于 `http://${P7_V2_API_HOST}:${P7_V2_API_PORT}`，供 k6 与探针统一使用。 |
| `P7_DIAGNOSTICS_*` | `P7_DIAGNOSTICS_ENABLED=false` | P7-V2 diagnostics | 否 | R3B local diagnostics switch and metadata (`P7_DIAGNOSTIC_RUN_ID` / `ROLE` / `DIR` / buffer / runtime + PG sample intervals). When explicitly enabled, writes bounded JSONL diagnostics to ignored `artifacts/p7-v2-diagnostics/` (or an external `/tmp/...` dir); diagnostic runs are non-formal and invalid for closure. |
| `APP_MASTER_KEY` | 空 / 64 位 hex | backend | 是 | AES-GCM 主密钥，用于 settings 敏感配置加密。 |
| `ADMIN_BOOTSTRAP_EMAIL` | 空 / `admin@example.com` | backend | 否 | 初始管理员邮箱。 |
| `ADMIN_BOOTSTRAP_PHONE` | 空 | backend | 否 | 初始管理员手机号。 |
| `ADMIN_BOOTSTRAP_PASSWORD` | 空 / 示例密码 | backend | 是 | 初始管理员密码，生产必须强密码。 |
| `JWT_SECRET` | `change-me-in-production` | backend | 是 | JWT 签名密钥。 |
| `JWT_EXPIRE_HOURS` | `168` | backend | 否 | JWT 有效期小时数。 |
| `UPLOAD_MAX_MB` | `10` | backend | 否 | 单文件上传大小上限。 |

## 可观测性与 OTLP

| 变量 | 示例 / 默认 | 服务 | 敏感 | 说明 |
| --- | --- | --- | --- | --- |
| `OBSERVABILITY_ENABLED` | `true` | backend | 否 | 是否启用日志、指标、追踪等可观测性基础能力。 |
| `OBSERVABILITY_MODE` | `local` / `hybrid` | backend | 否 | 本地、Prometheus、OTel 或混合模式。 |
| `OBSERVABILITY_ENVIRONMENT` | `development` | backend | 否 | 低基数环境标签。 |
| `TRACING_ENABLED` | `false` | backend | 否 | 是否启用 tracing。真实 OTLP backend 未配置时保持 `false` 或本地 Mock 验证。 |
| `OTEL_SERVICE_NAME` | `trademind-api` | backend | 否 | OTLP resource `service.name`。 |
| `OTEL_SERVICE_VERSION` | 空 | backend | 否 | OTLP resource `service.version`。 |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | 空 | backend | 否 | OTLP/HTTP endpoint；代码会规范化为 `/v1/traces`。为空表示真实 telemetry backend 验证 Deferred。 |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `http/json` | backend | 否 | P5-V 使用标准 OTLP/HTTP JSON。 |
| `OTEL_EXPORTER_OTLP_HEADERS` | 空 | backend | 是 | 可选 header 列表，格式 `k=v,k2=v2`；不得提交真实 Token，日志不得输出。 |
| `OTEL_EXPORTER_OTLP_INSECURE` | `true` / `false` | backend | 否 | 本地 Mock Collector 可为 `true`；生产建议 `false`。 |
| `OTEL_TRACE_SAMPLE_RATIO` | `0` / `0.1` | backend | 否 | 采样比例，生产上限由配置校验限制。 |
| `OTEL_EXPORT_TIMEOUT_SECONDS` | `10` | backend | 否 | 单次导出超时，代码限制最大 30 秒。 |
| `OTEL_EXPORT_QUEUE_SIZE` | `1024` | backend | 否 | OTel batcher 有界队列大小。 |
| `OTEL_EXPORT_BATCH_SIZE` | `128` | backend | 否 | 单批导出数量，不得超过队列大小。 |
| `OTEL_EXPORT_RETRY_MAX` | `2` | backend | 否 | 429/5xx 受控重试次数，上限 5。 |

## Webhook 入站（公开 HTTP）

| 变量 | 示例 / 默认 | 服务 | 敏感 | 说明 |
| --- | --- | --- | --- | --- |
| `WEBHOOK_MAX_BODY_KB` | `512` | backend | 否 | `POST /api/v1/webhooks/:platform/:eventType` 请求体上限（KiB）。 |
| `WEBHOOK_MAX_CLOCK_SKEW_SECONDS` | `300` | backend | 否 | 允许的时间戳时钟偏差；超时或远未来时间戳返回 `WEBHOOK_TIMESTAMP_EXPIRED`。 |
| `WEBHOOK_ENABLE_TEST_VERIFIER` | `false` | backend | 否 | 启用 `internal-test` HMAC-SHA256 测试验签；**仅** `APP_ENV=development` / `test` 生效，production 强制关闭。 |
| `WEBHOOK_WORKER_INTERVAL_SECONDS` | `3` | backend | 否 | DB 轮询 `webhook_events.status=queued` 的间隔秒数。 |
| `DOUYIN_WEBHOOK_TEST_SHOP_BINDING_ID` | 空 | backend | 否 | P3.2 多店铺 Webhook 显式测试兜底绑定 ID；仅 `development` / `test` 生效，`staging` / `production` 配置后 fail-fast。 |
| `ENABLE_DOUYIN_WEBHOOK_DEMO_FALLBACK` | `false` | backend | 否 | Demo 环境是否允许使用显式 Webhook 兜底绑定；`staging` / `production` 必须为 `false`。 |

## 数据库

| 变量 | 示例 / 默认 | 服务 | 敏感 | 说明 |
| --- | --- | --- | --- | --- |
| `DB_DRIVER` | `postgres` | backend | 否 | 默认 PostgreSQL；仅遗留库或明确要求时用 MySQL。 |
| `DB_HOST` | `127.0.0.1` / `postgres` | backend | 否 | 数据库地址。 |
| `DB_PORT` | `5432` | backend | 否 | PostgreSQL 默认 5432。 |
| `DB_USER` | `trademind` | backend | 否 | 数据库用户。 |
| `DB_PASSWORD` | `trademind` | backend | 是 | 数据库密码。 |
| `DB_NAME` | `trademind` | backend | 否 | 数据库名。 |
| `DB_TIMEZONE` | `UTC` | backend | 否 | 数据库时区。 |
| `DB_MAX_OPEN_CONNECTIONS` | `100` | backend | 否 | P7 数据库连接池最大打开连接数；生产非法配置 fail-fast。 |
| `DB_MAX_IDLE_CONNECTIONS` | `10` | backend | 否 | P7 数据库连接池最大空闲连接数。 |
| `DB_CONN_MAX_LIFETIME_SECONDS` | `3600` | backend | 否 | P7 单连接最大生命周期。 |
| `DB_CONN_MAX_IDLE_TIME_SECONDS` | `900` | backend | 否 | P7 空闲连接最长保留时间。 |
| `DB_QUERY_TIMEOUT_MS` | `5000` | backend | 否 | P7 查询超时预算；逐步接入仓储查询。 |
| `DB_TRANSACTION_TIMEOUT_MS` | `10000` | backend | 否 | P7 事务超时预算；必须大于等于查询超时。 |
| `POSTGRES_DB` | `trademind` | docker postgres | 否 | Docker Postgres 初始化库名。 |
| `POSTGRES_USER` | `trademind` | docker postgres | 否 | Docker Postgres 用户。 |
| `POSTGRES_PASSWORD` | 示例密码 | docker postgres | 是 | Docker Postgres 密码。 |

## Redis

| 变量 | 示例 / 默认 | 服务 | 敏感 | 说明 |
| --- | --- | --- | --- | --- |
| `REDIS_ADDR` | `127.0.0.1:6379` / `redis:6379` | backend | 否 | Redis 地址。 |
| `REDIS_PASSWORD` | 空 | backend | 是 | Redis 密码。 |
| `REDIS_DB` | `0` | backend | 否 | Redis DB 编号。 |

## Collector

| 变量 | 示例 / 默认 | 服务 | 敏感 | 说明 |
| --- | --- | --- | --- | --- |
| `COLLECTOR_BASE_URL` | `http://127.0.0.1:3100` | backend | 否 | Go API 调用 Collector 的基础地址。 |
| `COLLECTOR_TIMEOUT_SECONDS` | `120` | backend | 否 | 后端调用 Collector 超时；淘宝/天猫任务会按页面打开超时自动放宽（约 `gotoTimeoutMs + 90s`）。 |
| `COLLECTOR_HTTP_ADDR` | `:3100` / `:3001` | collector | 否 | Collector 监听地址。 |
| `COLLECTOR_MAIN_SERVICE_URL` | `http://127.0.0.1:8080` | collector | 否 | Collector 回调或访问后端的基础地址预留。 |
| `COLLECTOR_GOTO_TIMEOUT_MS` | `45000` | collector | 否 | Playwright 页面打开超时。 |
| `COLLECTOR_HEADLESS` | `1` | collector | 否 | 是否无头浏览器运行；本地打开登录浏览器时可设为 `0`。 |
| `COLLECTOR_BROWSER_PROFILE_DIR` / `BROWSER_PROFILE_ROOT` | `collector/data/browser-profiles`（相对 collector 包根目录） | collector | 否 | 1688 持久化 Profile 根目录（1688 使用子目录 `1688`）。Docker 通常设为 `/workspace/data/browser-profiles`。 |
| `COLLECTOR_STORAGE_STATE_DIR` | `data/storage-states` | collector | 否 | Playwright storageState 导出目录（预留）。 |
| `COLLECTOR_1688_AUTH_PROBE_URL` | 注释示例 | collector | 否 | 登录态检测时用于探测的商品详情 URL。 |
| `COLLECTOR_USER_AGENT` | 注释示例 | collector | 否 | 可选 UA 覆盖。 |

## 队列与任务

| 变量前缀 | 示例变量 | 服务 | 说明 |
| --- | --- | --- | --- |
| `COLLECT_*` | `COLLECT_QUEUE_ENABLED`、`COLLECT_WORKER_CONCURRENCY`、`COLLECT_QUEUE_NAME`、`COLLECT_BATCH_MAX_URLS`、`COLLECT_BATCH_CONCURRENCY_1688`、`COLLECT_BATCH_DELAY_MIN_MS_1688`、`COLLECT_BATCH_DELAY_MAX_MS_1688`、`COLLECT_BATCH_RETRY_ON_BLOCKED`、`COLLECT_BATCH_RETRY_ON_TIMEOUT`、`COLLECT_BATCH_MAX_RETRIES_1688` | backend | 采集任务队列、批量 URL 限制、1688 批量节流与重试。settings **`collector`** 分组可覆盖 1688 批量项。 |
| `IMAGE_*` / `AI_IMAGE_*` | `IMAGE_QUEUE_ENABLED`、`IMAGE_TASK_TIMEOUT_SECONDS`、`AI_IMAGE_PROVIDER_TIMEOUT_SECONDS`、`AI_IMAGE_TASK_MAX_RUNTIME_SECONDS`、`AI_IMAGE_POLL_INTERVAL_SECONDS`、`AI_IMAGE_TRIAL_TIMEOUT_SECONDS` | backend / scripts | 图片任务队列、Provider/Worker 超时、Demo trial 轮询和总等待上限。 |
| `TRANSLATE_FONT_PATH` | — | backend | 可选。图片文字翻译程序绘制所用字体（TTF/TTC）；未设置时自动查找 Noto CJK / 微软雅黑 / 内置英文字体。Docker 镜像默认安装 `fonts-noto-cjk`。 |
| `ORDER_SYNC_*` | `ORDER_SYNC_QUEUE_ENABLED`、`ORDER_SYNC_QUEUE_NAME` | backend | 平台订单同步任务。 |
| `CUSTOMER_MESSAGE_SYNC_*` | `CUSTOMER_MESSAGE_SYNC_QUEUE_ENABLED` | backend | 客服消息同步任务。 |
| `PRODUCT_PUBLISH_*` | `PRODUCT_PUBLISH_QUEUE_ENABLED`、`PUBLISH_BATCH_MAX_PRODUCTS`（100）、`PUBLISH_BATCH_MAX_TARGETS`（20）、`PUBLISH_BATCH_MAX_TASKS`（300） | backend | 商品刊登任务队列与批量矩阵上限。 |
| `INVENTORY_SYNC_*` | `INVENTORY_SYNC_QUEUE_ENABLED` | backend | 库存同步任务。 |
| `WORKER_*` | `WORKER_HEARTBEAT_ENABLED`、`WORKER_REAPER_ENABLED` | backend | 多实例 Worker 心跳、过期判断和回收。 |
| `TASK_ALERT_*` | `TASK_ALERT_SCAN_ENABLED`、`TASK_ALERT_SCAN_INTERVAL_SECONDS` | backend | 任务告警扫描。 |
| `BACKUP_*` | `BACKUP_ENABLED`、`BACKUP_MODE`、`BACKUP_STORAGE_PROVIDER`、`BACKUP_ENCRYPTION_ENABLED`、`BACKUP_RETENTION_DAILY` | backend | P6 备份、加密、校验、保留与恢复演练门闸。生产环境要求加密开启，且不得使用本地单副本。 |
| `POSTGRES_*` | `POSTGRES_BACKUP_FORMAT`、`POSTGRES_PG_DUMP_PATH`、`POSTGRES_PG_RESTORE_PATH`、`POSTGRES_WAL_ARCHIVE_ENABLED`、`POSTGRES_PITR_ENABLED` | backend | PostgreSQL 逻辑备份与 PITR 基础配置。真实生产 PITR 演练保持 Deferred。 |
| `RELEASE_*` | `RELEASE_ENABLED`、`RELEASE_ROOT`、`RELEASE_REQUIRE_PRE_BACKUP`、`RELEASE_STRATEGY`、`RELEASE_ROLLBACK_ON_FAILURE` | backend | P6 发布制品、发布前备份、受控发布与应用回滚配置。生产发布必须要求发布前备份。 |
| `PERFORMANCE_*` | `PERFORMANCE_TEST_MODE`、`PERFORMANCE_DATASET_MAX_ROWS`、`PERFORMANCE_TEST_MAX_VUS`、`PERFORMANCE_TEST_MAX_DURATION_SECONDS` | backend / scripts | P7 隔离性能测试与数据集保护；production 禁止开启测试模式。 |
| `PAGINATION_*` | `PAGINATION_DEFAULT_LIMIT`、`PAGINATION_MAX_LIMIT`、`PAGINATION_MAX_OFFSET`、`PAGINATION_CURSOR_SIGNING_KEY` | backend | P7 列表分页默认值、最大 limit、深 offset 保护与 Cursor HMAC 签名密钥；production 必须显式配置签名密钥。 |
| `RATE_LIMIT_*` | `RATE_LIMIT_ENABLED`、`RATE_LIMIT_MODE`、`RATE_LIMIT_FAIL_MODE`、`RATE_LIMIT_POLICY_VERSION` | backend | P7 HTTP 限流配置；production 禁用需显式审批变量。 |
| `CACHE_*` | `CACHE_ENABLED`、`CACHE_DEFAULT_TTL_SECONDS`、`CACHE_MAX_ENTRIES`、`CACHE_SINGLEFLIGHT_ENABLED` | backend | P7 缓存与 singleflight 治理基础配置。 |
| `EXPORT_*` | `EXPORT_BATCH_SIZE`、`EXPORT_MAX_ROWS`、`EXPORT_MAX_BYTES`、`EXPORT_MAX_CONCURRENT` | backend | P7 导出批量、行数、字节数和并发上限。 |
| `PPROF_*` | `PPROF_ENABLED`、`PPROF_INTERNAL_ONLY` | backend | P7 Profiling 安全开关；production 禁止 public pprof。 |

新增队列变量时，还要同步健康检查说明、任务中心页面和 `docs/PROGRESS.md`。

## Docker 端口覆盖

`.env.docker.example` 支持以下宿主机端口覆盖：

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `ADMIN_PUBLISH_PORT` | `8000` | 管理端宿主机端口。 |
| `BACKEND_PUBLISH_PORT` | `8080` | 后端 API 宿主机端口。 |
| `COLLECTOR_PUBLISH_PORT` | `3001` | Collector 宿主机端口。 |
| `POSTGRES_PUBLISH_PORT` | `5432` | PostgreSQL 宿主机端口。 |
| `REDIS_PUBLISH_PORT` | `6379` | Redis 宿主机端口。 |

## 前端

| 变量 | 示例 / 默认 | 服务 | 说明 |
| --- | --- | --- | --- |
| `VITE_API_BASE` | `/api` | admin | 管理端 API 基础路径，当前在 `.env.example` 中为预留注释。 |

## 新增变量检查清单

新增或修改环境变量时必须检查：

- `.env.example`
- `.env.docker.example`
- `docker-compose.yml`
- `docker-compose.full.yml`
- `docs/env.md`
- `docs/development.md`
- `docs/docker-deployment.md`
- `README.md` / `README.en.md` 中的启动说明
- 相关代码默认值与安全校验
