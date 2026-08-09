---
doc_type: generated
status: generated
owner: maintainers
generator: pnpm docs:generate:env
---

# Environment variables (generated)

> Do not edit by hand. Run `pnpm docs:generate:env`.

Narrative guidance: `docs/reference/configuration/environment.md`.

| Variable | Source | Secret-like | Notes |
| --- | --- | --- | --- |
| `APP_ENV` | `.env.example` | no | ============================================================================= TradeMind 环境变量 — 本地开发主模板 =============================================================================  【本文件用途】   日常本地开发的完 |
| `APP_NAME` | `.env.example` | no |  |
| `APP_VERSION` | `.env.example` | no |  |
| `APP_HTTP_ADDR` | `.env.example` | no |  |
| `ADMIN_DEV_PORT` | `.env.example` | no | Admin 本地开发端口固定来源；pnpm dev 不再扫描 8000-8010 自动换端口。 |
| `ADMIN_DEV_API_PROXY_TARGET` | `.env.example` | no | 仅用于宿主机 Umi dev proxy；只接受显式端口的 localhost / 127.0.0.1 / ::1 HTTP(S) 地址。 |
| `TRUSTED_PROXIES` | `.env.example` | no | 可信反向代理 IP/CIDR，逗号分隔；默认空（忽略 X-Forwarded-For/X-Real-IP），禁止配置 /0。 |
| `ADMIN_PUBLIC_URL` | `.env.example` | no | P7-V2 local performance harness endpoint. Keep these unset for normal development. P7_V2_API_HOST=127.0.0.1 P7_V2_API_PORT=8080 P7_BASE_URL=http://127.0.0.1:8080 P7_DIAGNOSTICS_ENABLED=false P7_DIAGNO |
| `API_PUBLIC_URL` | `.env.example` | no |  |
| `LOG_LEVEL` | `.env.example` | no |  |
| `OBSERVABILITY_ENABLED` | `.env.example` | no | P5 Observability (development defaults) |
| `OBSERVABILITY_MODE` | `.env.example` | no |  |
| `OBSERVABILITY_ENVIRONMENT` | `.env.example` | no |  |
| `LOG_FORMAT` | `.env.example` | no |  |
| `LOG_INCLUDE_SOURCE` | `.env.example` | no |  |
| `LOG_MAX_FIELD_LENGTH` | `.env.example` | no |  |
| `METRICS_ENABLED` | `.env.example` | no |  |
| `METRICS_PATH` | `.env.example` | no |  |
| `METRICS_INTERNAL_ONLY` | `.env.example` | no |  |
| `TRACING_ENABLED` | `.env.example` | no |  |
| `OTEL_SERVICE_NAME` | `.env.example` | no |  |
| `OTEL_SERVICE_VERSION` | `.env.example` | no |  |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `.env.example` | no |  |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `.env.example` | no |  |
| `OTEL_EXPORTER_OTLP_HEADERS` | `.env.example` | no |  |
| `OTEL_EXPORTER_OTLP_INSECURE` | `.env.example` | no |  |
| `OTEL_TRACE_SAMPLE_RATIO` | `.env.example` | no |  |
| `OTEL_EXPORT_TIMEOUT_SECONDS` | `.env.example` | no |  |
| `OTEL_EXPORT_QUEUE_SIZE` | `.env.example` | no |  |
| `OTEL_EXPORT_BATCH_SIZE` | `.env.example` | no |  |
| `OTEL_EXPORT_RETRY_MAX` | `.env.example` | no |  |
| `ALERTING_ENABLED` | `.env.example` | no |  |
| `ALERT_DEFAULT_COOLDOWN_SECONDS` | `.env.example` | no |  |
| `ALERT_RECOVERY_ENABLED` | `.env.example` | no |  |
| `DB_SLOW_QUERY_THRESHOLD_MS` | `.env.example` | no |  |
| `DB_TRACE_ENABLED` | `.env.example` | no |  |
| `APP_MASTER_KEY` | `.env.example` | yes | AES-GCM master key for encrypting settings (64-char hex, base64 of 32 bytes, 32-char raw, or any passphrase hashed with SHA-256) |
| `ENABLE_SWAGGER` | `.env.example` | no | Feature gates (production must disable dangerous flags) |
| `ENABLE_DEV_ROUTES` | `.env.example` | no |  |
| `ENABLE_DEMO_SEED` | `.env.example` | no |  |
| `ENABLE_DEBUG_ENDPOINTS` | `.env.example` | no |  |
| `ADMIN_BOOTSTRAP_EMAIL` | `.env.example` | no | First admin when admin_users is empty: set EMAIL and/or PHONE (+ password). This account is created with role=admin (full permissions) and kept in sync on startup. Login is email or mobile only — not  |
| `ADMIN_BOOTSTRAP_PHONE` | `.env.example` | no |  |
| `ADMIN_BOOTSTRAP_PASSWORD` | `.env.example` | yes |  |
| `DB_DRIVER` | `.env.example` | no | P7-V2 performance harness accounts (APP_ENV=performance only; never use in production) P7V2_PERF_ADMIN_PASSWORD= P7V2_PERF_TENANT_ADMIN_PASSWORD= P7V2_PERF_OPERATOR_PASSWORD= P7V2_PERF_READONLY_PASSWO |
| `DB_HOST` | `.env.example` | no |  |
| `DB_PORT` | `.env.example` | no |  |
| `DB_USER` | `.env.example` | no |  |
| `DB_PASSWORD` | `.env.example` | yes |  |
| `DB_NAME` | `.env.example` | no |  |
| `DB_TIMEZONE` | `.env.example` | no |  |
| `REDIS_ADDR` | `.env.example` | no | ----------------------------------------------------------------------------- Redis (queue / cache) ----------------------------------------------------------------------------- |
| `REDIS_PASSWORD` | `.env.example` | yes |  |
| `REDIS_DB` | `.env.example` | no |  |
| `JWT_SECRET` | `.env.example` | yes | ----------------------------------------------------------------------------- JWT / session (backend) ----------------------------------------------------------------------------- |
| `JWT_EXPIRE_HOURS` | `.env.example` | no |  |
| `AUTH_SESSION_MODE` | `.env.example` | no | Phase P4 — 认证与会话（所有环境默认 secure_session；仅显式本地兼容测试可选 legacy） AUTH_SESSION_MODE=legacy_local_storage|secure_session |
| `AUTH_ACCESS_TOKEN_TTL_MINUTES` | `.env.example` | yes |  |
| `AUTH_REFRESH_TOKEN_TTL_DAYS` | `.env.example` | yes |  |
| `AUTH_SECURE_COOKIE` | `.env.example` | yes |  |
| `AUTH_COOKIE_SAME_SITE` | `.env.example` | yes | AUTH_COOKIE_DOMAIN= |
| `AUTH_LOGIN_MAX_ATTEMPTS` | `.env.example` | no |  |
| `AUTH_LOGIN_WINDOW_MINUTES` | `.env.example` | no |  |
| `AUTH_ACCOUNT_LOCK_MINUTES` | `.env.example` | no |  |
| `AUTH_LOGIN_IP_RATE_LIMIT` | `.env.example` | no |  |
| `AUTH_REFRESH_RATE_LIMIT` | `.env.example` | no |  |
| `AUTH_PASSWORD_MIN_LENGTH` | `.env.example` | yes |  |
| `AUTH_PASSWORD_REQUIRE_CHANGE_ON_ADMIN_RESET` | `.env.example` | yes |  |
| `JWT_ROTATION_GRACE_MINUTES` | `.env.example` | no | JWT 密钥版本（可选；未设置时回退 JWT_SECRET） JWT_ACTIVE_KEY_ID=default JWT_ACTIVE_SECRET= JWT_PREVIOUS_KEY_ID= JWT_PREVIOUS_SECRET= |
| `UPLOAD_MAX_FILES` | `.env.example` | no | 启用 previous key 时填写本次轮换开始时间（RFC3339 UTC，例如 2026-08-02T00:00:00Z）；预发/生产必填。 JWT_ROTATION_STARTED_AT= APP_MASTER_KEY 密钥环（可选版本化） APP_MASTER_ACTIVE_KEY_ID=default APP_MASTER_ACTIVE_KEY= APP_MASTER_PREVIOUS |
| `UPLOAD_MAX_IMAGE_PIXELS` | `.env.example` | no |  |
| `UPLOAD_MAX_IMAGE_WIDTH` | `.env.example` | no |  |
| `UPLOAD_MAX_IMAGE_HEIGHT` | `.env.example` | no |  |
| `UPLOAD_MAX_ANIMATION_FRAMES` | `.env.example` | no |  |
| `PRIVATE_DOWNLOAD_URL_TTL_SECONDS` | `.env.example` | yes | Phase P4.1 — 租户隔离（staging/production 禁止 tenant fallback） --- Phase P4.1 tenant isolation (secure session derives tenant from the authenticated account) --- Optional: simulate a non-zero tenant in deve |
| `EXPORT_DOWNLOAD_URL_TTL_SECONDS` | `.env.example` | no |  |
| `SENSITIVE_DOWNLOAD_URL_TTL_SECONDS` | `.env.example` | no |  |
| `UPLOAD_MAX_MB` | `.env.example` | no | 上传单文件最大体积（MB），默认 10 |
| `STORAGE_PROVIDER` | `.env.example` | no | ----------------------------------------------------------------------------- Storage fail-fast（与 settings.storage.kind 对齐；staging/production 禁止 local） ------------------------------------------------ |
| `MIGRATION_RUN_ON_STARTUP` | `.env.example` | no | ----------------------------------------------------------------------------- CORS（development 未配置时自动放行 localhost；staging/production 必填） --------------------------------------------------------------- |
| `MIGRATION_LOCK_TIMEOUT_SECONDS` | `.env.example` | no |  |
| `COLLECTOR_PLAYWRIGHT_ENABLED` | `.env.example` | no | ----------------------------------------------------------------------------- 对象存储说明（不写环境变量也可用：在管理端「存储设置」写入 settings.storage） -------------------------------------------------------------------------- |
| `COLLECTOR_PLAYWRIGHT_BASE_URL` | `.env.example` | no | 仅显式恢复 Playwright 时使用（须与 COLLECTOR_HTTP_ADDR 端口一致）。 |
| `COLLECTOR_INTERNAL_TOKEN` | `.env.example` | yes | Collector 仅监听回环地址时可留空；任何非回环监听/访问必须设置同一个随机长 Token。 |
| `COLLECTOR_TIMEOUT_SECONDS` | `.env.example` | no | 兼容旧配置；新部署请使用 COLLECTOR_PLAYWRIGHT_BASE_URL。 COLLECTOR_BASE_URL=http://127.0.0.1:3001 后端调用 collector 的 HTTP 超时（秒）；淘宝/天猫会按页面打开超时自动放宽，建议 >= 120 |
| `OPENCLI_BRIDGE_ENABLED` | `.env.example` | no | OpenCLI 使用独立的宿主机轻量 Bridge，不加载 Playwright。 普通安装保持 false；启用后，淘宝/天猫未显式指定引擎时优先 OpenCLI。 |
| `OPENCLI_BRIDGE_BASE_URL` | `.env.example` | no |  |
| `OPENCLI_BRIDGE_DOCKER_BASE_URL` | `.env.example` | no | Compose 内 backend 使用此地址，避免与本地 backend 地址互相覆盖。 |
| `OPENCLI_BRIDGE_HTTP_ADDR` | `.env.example` | no |  |
| `OPENCLI_BRIDGE_TOKEN` | `.env.example` | yes | Bridge 只监听回环地址时可留空；绑定 0.0.0.0 供 Docker 访问时必须设置。 |
| `OPENCLI_BRIDGE_TIMEOUT_SECONDS` | `.env.example` | no |  |
| `COLLECT_DEFAULT_ENGINE_TAOBAO_TMALL` | `.env.example` | no |  |
| `OPENCLI_BIN` | `.env.example` | no |  |
| `OPENCLI_TIMEOUT_MS` | `.env.example` | no |  |
| `OPENCLI_SKU_CLICK_MAX` | `.env.example` | no |  |
| `COLLECT_QUEUE_ENABLED` | `.env.example` | no | 采集异步队列（Redis LIST，Worker 消费；提交任务须 Redis 可用） |
| `COLLECT_WORKER_CONCURRENCY` | `.env.example` | no |  |
| `COLLECT_QUEUE_NAME` | `.env.example` | no |  |
| `COLLECT_BATCH_MAX_URLS` | `.env.example` | no | 单次批量采集最大 URL 数（POST /collect/batches） |
| `COLLECT_BATCH_CONCURRENCY_1688` | `.env.example` | no | 1688 批量采集保守节流（仅 bulk 任务；单链接采集不受影响） |
| `COLLECT_BATCH_DELAY_MIN_MS_1688` | `.env.example` | no |  |
| `COLLECT_BATCH_DELAY_MAX_MS_1688` | `.env.example` | no |  |
| `COLLECT_BATCH_RETRY_ON_BLOCKED` | `.env.example` | no |  |
| `COLLECT_BATCH_RETRY_ON_TIMEOUT` | `.env.example` | no |  |
| `COLLECT_BATCH_MAX_RETRIES_1688` | `.env.example` | no |  |
| `COLLECT_AUTO_RETRY_ENABLED` | `.env.example` | no | Worker 自动退避重试 |
| `COLLECT_MAX_RETRIES` | `.env.example` | no |  |
| `COLLECT_RETRY_BASE_DELAY_SECONDS` | `.env.example` | no |  |
| `COLLECT_RETRY_MAX_DELAY_SECONDS` | `.env.example` | no |  |
| `IMAGE_QUEUE_ENABLED` | `.env.example` | no | 图片任务异步队列（Redis LIST；POST /image/tasks 入队，进程内 Image Worker BRPOP 消费） IMAGE_QUEUE_ENABLED=false 时：创建接口在同请求内同步执行（仅建议本地开发） |
| `IMAGE_QUEUE_NAME` | `.env.example` | no |  |
| `IMAGE_WORKER_CONCURRENCY` | `.env.example` | no |  |
| `IMAGE_TASK_TIMEOUT_SECONDS` | `.env.example` | no | 单次任务 provider 调用 context 上限（秒），与 settings.image.timeout_sec 取更小值 |
| `AI_IMAGE_PROVIDER_TIMEOUT_SECONDS` | `.env.example` | no | P4-R demo regression caps. Provider calls, worker runtime, polling cadence, and trial wall-clock timeout. |
| `AI_IMAGE_TASK_MAX_RUNTIME_SECONDS` | `.env.example` | no |  |
| `AI_IMAGE_POLL_INTERVAL_SECONDS` | `.env.example` | no |  |
| `AI_IMAGE_TRIAL_TIMEOUT_SECONDS` | `.env.example` | no |  |
| `IMAGE_AUTO_RETRY_ENABLED` | `.env.example` | no | 失败进入 retrying + next_retry_at，由进程内调度器到期后 LPUSH（需 IMAGE_QUEUE_ENABLED=true） |
| `IMAGE_MAX_RETRIES` | `.env.example` | no |  |
| `IMAGE_RETRY_BASE_DELAY_SECONDS` | `.env.example` | no |  |
| `IMAGE_RETRY_MAX_DELAY_SECONDS` | `.env.example` | no |  |
| `ORDER_SYNC_QUEUE_ENABLED` | `.env.example` | no | 图片文字翻译：可选自定义字体路径（CJK 推荐 Noto/微软雅黑）；Docker 默认已安装 fonts-noto-cjk TRANSLATE_FONT_PATH=/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc 订单同步异步队列（Redis LIST；Worker BRPOP；ORDER_SYNC_QUEUE_ENABLED=fals |
| `ORDER_SYNC_QUEUE_NAME` | `.env.example` | no |  |
| `ORDER_SYNC_WORKER_CONCURRENCY` | `.env.example` | no |  |
| `ORDER_SYNC_TASK_TIMEOUT_SECONDS` | `.env.example` | no |  |
| `CUSTOMER_MESSAGE_SYNC_QUEUE_ENABLED` | `.env.example` | no | 客服消息异步同步（Redis LIST；CUSTOMER_MESSAGE_SYNC_QUEUE_ENABLED=false 时由 API 进程内同步执行） |
| `CUSTOMER_MESSAGE_SYNC_QUEUE_NAME` | `.env.example` | no |  |
| `CUSTOMER_MESSAGE_SYNC_WORKER_CONCURRENCY` | `.env.example` | no |  |
| `CUSTOMER_MESSAGE_SYNC_TASK_TIMEOUT_SECONDS` | `.env.example` | no |  |
| `PRODUCT_PUBLISH_QUEUE_ENABLED` | `.env.example` | no | 商品刊登异步队列（Redis LIST；PRODUCT_PUBLISH_QUEUE_ENABLED=false 时创建任务后在 API 进程内同步执行） |
| `PRODUCT_PUBLISH_QUEUE_NAME` | `.env.example` | no |  |
| `PRODUCT_PUBLISH_WORKER_CONCURRENCY` | `.env.example` | no |  |
| `PRODUCT_PUBLISH_TASK_TIMEOUT_SECONDS` | `.env.example` | no |  |
| `PUBLISH_BATCH_MAX_PRODUCTS` | `.env.example` | no | 批量刊登矩阵上限（Phase A2.1） |
| `PUBLISH_BATCH_MAX_TARGETS` | `.env.example` | no |  |
| `PUBLISH_BATCH_MAX_TASKS` | `.env.example` | no |  |
| `INVENTORY_SYNC_QUEUE_ENABLED` | `.env.example` | no | 库存同步异步队列（Redis LIST；Worker BRPOP；INVENTORY_SYNC_QUEUE_ENABLED=false 时任务在创建进程内同步执行） |
| `INVENTORY_SYNC_QUEUE_NAME` | `.env.example` | no |  |
| `INVENTORY_SYNC_WORKER_CONCURRENCY` | `.env.example` | no |  |
| `INVENTORY_SYNC_TASK_TIMEOUT_SECONDS` | `.env.example` | no |  |
| `COLLECT_TASK_TIMEOUT_SECONDS` | `.env.example` | no | 采集任务 DB 租约 TTL（多实例 Worker 回收；应 ≥ Collector 超时 + 余量） |
| `WORKER_HEARTBEAT_ENABLED` | `.env.example` | no | 多实例 Worker：实例注册表 + 心跳 + 任务租约 + Reaper（heartbeat 关闭时仍生成 workerId 并写 DB 租约，但不落 worker_instances） |
| `WORKER_HEARTBEAT_INTERVAL_SECONDS` | `.env.example` | no |  |
| `WORKER_STALE_AFTER_SECONDS` | `.env.example` | no |  |
| `WORKER_REAPER_ENABLED` | `.env.example` | no |  |
| `WORKER_REAPER_INTERVAL_SECONDS` | `.env.example` | no |  |
| `WORKER_LEGACY_RUNNING_TIMEOUT_SECONDS` | `.env.example` | no |  |
| `TASK_ALERT_SCAN_ENABLED` | `.env.example` | no | 任务告警：进程内定时扫描（非 Redis）。仅部署级门闸；业务是否启用与扫描间隔见管理端「系统设置」settings.taskcenter。 |
| `TASK_ALERT_SCAN_INTERVAL_SECONDS` | `.env.example` | no |  |
| `TASK_ALERT_SCAN_LOOKBACK_MINUTES` | `.env.example` | no |  |
| `TASK_ALERT_SCAN_LOCK_TTL_SECONDS` | `.env.example` | no |  |
| `BACKUP_ENABLED` | `.env.example` | no | ----------------------------------------------------------------------------- P6 Backup / Restore / Release / Disaster Recovery ------------------------------------------------------------------------ |
| `BACKUP_MODE` | `.env.example` | no |  |
| `BACKUP_SCHEDULE` | `.env.example` | no |  |
| `BACKUP_STORAGE_PROVIDER` | `.env.example` | no |  |
| `BACKUP_STORAGE_BUCKET` | `.env.example` | no |  |
| `BACKUP_STORAGE_PREFIX` | `.env.example` | no |  |
| `BACKUP_ENCRYPTION_ENABLED` | `.env.example` | no |  |
| `BACKUP_ENCRYPTION_KEY_ID` | `.env.example` | yes |  |
| `BACKUP_RETENTION_DAILY` | `.env.example` | no |  |
| `BACKUP_RETENTION_WEEKLY` | `.env.example` | no |  |
| `BACKUP_RETENTION_MONTHLY` | `.env.example` | no |  |
| `BACKUP_MAX_AGE_HOURS` | `.env.example` | no |  |
| `BACKUP_COMMAND_TIMEOUT_SECONDS` | `.env.example` | no |  |
| `BACKUP_VERIFY_ENABLED` | `.env.example` | no |  |
| `BACKUP_RESTORE_DRILL_ENABLED` | `.env.example` | no |  |
| `BACKUP_RESTORE_DRILL_SCHEDULE` | `.env.example` | no |  |
| `POSTGRES_BACKUP_FORMAT` | `.env.example` | no |  |
| `POSTGRES_PG_DUMP_PATH` | `.env.example` | no |  |
| `POSTGRES_PG_RESTORE_PATH` | `.env.example` | no |  |
| `POSTGRES_PSQL_PATH` | `.env.example` | no |  |
| `POSTGRES_WAL_ARCHIVE_ENABLED` | `.env.example` | no |  |
| `POSTGRES_WAL_ARCHIVE_PATH` | `.env.example` | no |  |
| `POSTGRES_PITR_ENABLED` | `.env.example` | no |  |
| `RELEASE_ENABLED` | `.env.example` | no |  |
| `RELEASE_ROOT` | `.env.example` | no |  |
| `RELEASE_ARTIFACT_DIR` | `.env.example` | no |  |
| `RELEASE_CURRENT_LINK` | `.env.example` | no |  |
| `RELEASE_PREVIOUS_LINK` | `.env.example` | no |  |
| `RELEASE_KEEP_COUNT` | `.env.example` | no |  |
| `RELEASE_HEALTH_TIMEOUT_SECONDS` | `.env.example` | no |  |
| `RELEASE_ROLLBACK_ON_FAILURE` | `.env.example` | no |  |
| `RELEASE_REQUIRE_PRE_BACKUP` | `.env.example` | no |  |
| `RELEASE_STRATEGY` | `.env.example` | no |  |
| `RELEASE_TRAFFIC_SWITCH_MODE` | `.env.example` | no |  |
| `DRILL_MODE` | `.env.example` | no | P6-V isolated drill guardrails. Only local/CI drill scripts should enable these. |
| `ALLOW_ISOLATED_RESTORE` | `.env.example` | no |  |
| `TARGET_ENVIRONMENT` | `.env.example` | no |  |
| `P6V_MARKER` | `.env.example` | no |  |
| `PERFORMANCE_TEST_MODE` | `.env.example` | no | ----------------------------------------------------------------------------- P7 Performance / capacity / rate limiting foundation --------------------------------------------------------------------- |
| `ALLOW_PERFORMANCE_DATASET` | `.env.example` | no |  |
| `EXTERNAL_PROVIDER_MODE` | `.env.example` | no |  |
| `DOUYIN_WRITE_ENABLED` | `.env.example` | no |  |
| `AUTO_LISTING_ENABLED` | `.env.example` | no |  |
| `PERFORMANCE_DATASET_MAX_ROWS` | `.env.example` | no |  |
| `PERFORMANCE_TEST_MAX_VUS` | `.env.example` | no |  |
| `PERFORMANCE_TEST_MAX_DURATION_SECONDS` | `.env.example` | no |  |
| `PAGINATION_DEFAULT_LIMIT` | `.env.example` | no |  |
| `PAGINATION_MAX_LIMIT` | `.env.example` | no |  |
| `PAGINATION_MAX_OFFSET` | `.env.example` | no |  |
| `PAGINATION_CURSOR_SIGNING_KEY` | `.env.example` | yes |  |
| `DB_MAX_OPEN_CONNECTIONS` | `.env.example` | no |  |
| `DB_MAX_IDLE_CONNECTIONS` | `.env.example` | no |  |
| `DB_CONN_MAX_LIFETIME_SECONDS` | `.env.example` | no |  |
| `DB_CONN_MAX_IDLE_TIME_SECONDS` | `.env.example` | no |  |
| `DB_QUERY_TIMEOUT_MS` | `.env.example` | no |  |
| `DB_TRANSACTION_TIMEOUT_MS` | `.env.example` | no |  |
| `WORKER_CONCURRENCY_DEFAULT` | `.env.example` | no |  |
| `WORKER_QUEUE_CAPACITY` | `.env.example` | no |  |
| `WORKER_MAX_INFLIGHT` | `.env.example` | no |  |
| `WORKER_PREFETCH` | `.env.example` | no |  |
| `WORKER_SHUTDOWN_TIMEOUT_SECONDS` | `.env.example` | no |  |
| `RATE_LIMIT_ENABLED` | `.env.example` | no |  |
| `RATE_LIMIT_MODE` | `.env.example` | no |  |
| `RATE_LIMIT_REDIS_PREFIX` | `.env.example` | no |  |
| `RATE_LIMIT_FAIL_MODE` | `.env.example` | no |  |
| `RATE_LIMIT_LOCAL_FALLBACK` | `.env.example` | no |  |
| `RATE_LIMIT_POLICY_VERSION` | `.env.example` | no |  |
| `CACHE_ENABLED` | `.env.example` | no |  |
| `CACHE_DEFAULT_TTL_SECONDS` | `.env.example` | no |  |
| `CACHE_MAX_ENTRIES` | `.env.example` | no |  |
| `CACHE_SINGLEFLIGHT_ENABLED` | `.env.example` | no |  |
| `EXPORT_BATCH_SIZE` | `.env.example` | no |  |
| `EXPORT_MAX_ROWS` | `.env.example` | no |  |
| `EXPORT_MAX_BYTES` | `.env.example` | no |  |
| `EXPORT_MAX_CONCURRENT` | `.env.example` | no |  |
| `PPROF_ENABLED` | `.env.example` | no |  |
| `PPROF_INTERNAL_ONLY` | `.env.example` | no |  |
| `WEBHOOK_MAX_BODY_KB` | `.env.example` | no | ----------------------------------------------------------------------------- Inbound platform webhooks (public POST /api/v1/webhooks/:platform/:eventType) -------------------------------------------- |
| `WEBHOOK_MAX_CLOCK_SKEW_SECONDS` | `.env.example` | no |  |
| `WEBHOOK_ENABLE_TEST_VERIFIER` | `.env.example` | no | HMAC-SHA256 verifier for platform=internal-test; only when APP_ENV=development|test (forced off in production). |
| `WEBHOOK_WORKER_INTERVAL_SECONDS` | `.env.example` | no |  |
| `DOUYIN_WEBHOOK_TEST_SHOP_BINDING_ID` | `.env.example` | no |  |
| `ENABLE_DOUYIN_WEBHOOK_DEMO_FALLBACK` | `.env.example` | no |  |
| `COLLECTOR_HTTP_ADDR` | `.env.example` | no | 说明：收件邮箱、Webhook、通知阈值/等级/通道等均仅存 settings（taskcenter / alert_notify），请勿写入本文件。 告警邮件/Webhook 详情链接前缀、Webhook HTTP 超时见管理端「告警通知」「系统设置」。 ----------------------------------------------------------------------- |
| `COLLECTOR_MAIN_SERVICE_URL` | `.env.example` | no |  |
| `COLLECTOR_GOTO_TIMEOUT_MS` | `.env.example` | no | open page timeout (ms); navigation default same family |
| `COLLECTOR_HEADLESS` | `.env.example` | no | 1 or true = headless (default headless); set 0 when opening login browser locally |
| `APP_ENV` | `.env.docker.example` | no | ============================================================================= TradeMind 环境变量 — Docker Compose 全栈部署 =============================================================================  【本文件用途 |
| `GIN_MODE` | `.env.docker.example` | no | Gin ReleaseMode 由代码在 APP_ENV=production 时开启；此处 development 便于排查问题。 |
| `APP_HTTP_ADDR` | `.env.docker.example` | no | 后端监听（docker-compose.full.yml 内已固定为容器内 :8080；勿与宿主机映射混淆） |
| `TRUSTED_PROXIES` | `.env.docker.example` | no | 默认不信任转发头；接入固定反向代理时仅填写其精确容器 IP/CIDR，禁止 /0。 |
| `APP_MASTER_KEY` | `.env.docker.example` | yes | AES-GCM：须非空；示例为 64 位十六进制（32 字节） |
| `JWT_SECRET` | `.env.docker.example` | yes |  |
| `JWT_EXPIRE_HOURS` | `.env.docker.example` | no |  |
| `ADMIN_BOOTSTRAP_EMAIL` | `.env.docker.example` | no |  |
| `ADMIN_BOOTSTRAP_PHONE` | `.env.docker.example` | no |  |
| `ADMIN_BOOTSTRAP_PASSWORD` | `.env.docker.example` | yes |  |
| `OBSERVABILITY_ENABLED` | `.env.docker.example` | no | P5 Observability / OTLP Mock Collector compatible settings. |
| `OBSERVABILITY_MODE` | `.env.docker.example` | no |  |
| `OBSERVABILITY_ENVIRONMENT` | `.env.docker.example` | no |  |
| `LOG_FORMAT` | `.env.docker.example` | no |  |
| `METRICS_ENABLED` | `.env.docker.example` | no |  |
| `METRICS_INTERNAL_ONLY` | `.env.docker.example` | no |  |
| `TRACING_ENABLED` | `.env.docker.example` | no |  |
| `OTEL_SERVICE_NAME` | `.env.docker.example` | no |  |
| `OTEL_SERVICE_VERSION` | `.env.docker.example` | no |  |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `.env.docker.example` | no |  |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `.env.docker.example` | no |  |
| `OTEL_EXPORTER_OTLP_HEADERS` | `.env.docker.example` | no |  |
| `OTEL_EXPORTER_OTLP_INSECURE` | `.env.docker.example` | no |  |
| `OTEL_TRACE_SAMPLE_RATIO` | `.env.docker.example` | no |  |
| `OTEL_EXPORT_TIMEOUT_SECONDS` | `.env.docker.example` | no |  |
| `OTEL_EXPORT_QUEUE_SIZE` | `.env.docker.example` | no |  |
| `OTEL_EXPORT_BATCH_SIZE` | `.env.docker.example` | no |  |
| `OTEL_EXPORT_RETRY_MAX` | `.env.docker.example` | no |  |
| `POSTGRES_DB` | `.env.docker.example` | no | ----------------------------------------------------------------------------- PostgreSQL（Compose postgres 服务 + GORM） ----------------------------------------------------------------------------- |
| `POSTGRES_USER` | `.env.docker.example` | no |  |
| `POSTGRES_PASSWORD` | `.env.docker.example` | yes |  |
| `DB_DRIVER` | `.env.docker.example` | no |  |
| `DB_HOST` | `.env.docker.example` | no |  |
| `DB_PORT` | `.env.docker.example` | no |  |
| `DB_USER` | `.env.docker.example` | no |  |
| `DB_PASSWORD` | `.env.docker.example` | yes |  |
| `DB_NAME` | `.env.docker.example` | no |  |
| `DB_TIMEZONE` | `.env.docker.example` | no |  |
| `REDIS_ADDR` | `.env.docker.example` | no | ----------------------------------------------------------------------------- Redis ----------------------------------------------------------------------------- |
| `REDIS_PASSWORD` | `.env.docker.example` | yes |  |
| `REDIS_DB` | `.env.docker.example` | no |  |
| `COLLECTOR_PLAYWRIGHT_ENABLED` | `.env.docker.example` | no | ----------------------------------------------------------------------------- Playwright Collector 默认停用；恢复时同时设为 true 并启用 compose 的 playwright profile。 ------------------------------------------------- |
| `COLLECTOR_PLAYWRIGHT_BASE_URL` | `.env.docker.example` | no |  |
| `COLLECTOR_INTERNAL_TOKEN` | `.env.docker.example` | yes | 必填：backend 与 collector 共用的随机长 Bearer Token；禁止提交真实值。 |
| `COLLECTOR_TIMEOUT_SECONDS` | `.env.docker.example` | no | 兼容旧配置；新部署请使用 COLLECTOR_PLAYWRIGHT_BASE_URL。 COLLECTOR_BASE_URL=http://collector:3001 |
| `COLLECTOR_HTTP_ADDR` | `.env.docker.example` | no | 容器内监听端口由 compose 固定为 3001（宿主机映射见 COLLECTOR_PUBLISH_PORT） |
| `COLLECTOR_MAIN_SERVICE_URL` | `.env.docker.example` | no | 以下变量与 .env.example 对齐；当前 Collector 代码未读取 MAIN_SERVICE_URL。 |
| `COLLECTOR_GOTO_TIMEOUT_MS` | `.env.docker.example` | no |  |
| `COLLECTOR_HEADLESS` | `.env.docker.example` | no |  |
| `OPENCLI_BRIDGE_ENABLED` | `.env.docker.example` | no | 1688 持久化浏览器 Profile（compose 已挂载 ./data/browser-profiles） COLLECTOR_BROWSER_PROFILE_DIR=/workspace/data/browser-profiles COLLECTOR_STORAGE_STATE_DIR=/workspace/data/storage-states OpenCLI Bridge 运行在宿主机 |
| `OPENCLI_BRIDGE_BASE_URL` | `.env.docker.example` | no | 本地 backend 使用 OPENCLI_BRIDGE_BASE_URL；Compose 会把下一个变量注入容器。 |
| `OPENCLI_BRIDGE_DOCKER_BASE_URL` | `.env.docker.example` | no |  |
| `OPENCLI_BRIDGE_HTTP_ADDR` | `.env.docker.example` | no |  |
| `OPENCLI_BRIDGE_TOKEN` | `.env.docker.example` | yes |  |
| `OPENCLI_BRIDGE_TIMEOUT_SECONDS` | `.env.docker.example` | no |  |
| `COLLECT_DEFAULT_ENGINE_TAOBAO_TMALL` | `.env.docker.example` | no |  |
| `OPENCLI_BIN` | `.env.docker.example` | no |  |
| `OPENCLI_TIMEOUT_MS` | `.env.docker.example` | no |  |
| `OPENCLI_SKU_CLICK_MAX` | `.env.docker.example` | no |  |
| `COLLECT_QUEUE_ENABLED` | `.env.docker.example` | no | ----------------------------------------------------------------------------- 异步队列（与根目录 .env.example 默认对齐） ----------------------------------------------------------------------------- |
| `COLLECT_WORKER_CONCURRENCY` | `.env.docker.example` | no |  |
| `COLLECT_QUEUE_NAME` | `.env.docker.example` | no |  |
| `COLLECT_BATCH_MAX_URLS` | `.env.docker.example` | no |  |
| `COLLECT_AUTO_RETRY_ENABLED` | `.env.docker.example` | no |  |
| `COLLECT_MAX_RETRIES` | `.env.docker.example` | no |  |
| `COLLECT_RETRY_BASE_DELAY_SECONDS` | `.env.docker.example` | no |  |
| `COLLECT_RETRY_MAX_DELAY_SECONDS` | `.env.docker.example` | no |  |
| `IMAGE_QUEUE_ENABLED` | `.env.docker.example` | no |  |
| `IMAGE_QUEUE_NAME` | `.env.docker.example` | no |  |
| `IMAGE_WORKER_CONCURRENCY` | `.env.docker.example` | no |  |
| `IMAGE_TASK_TIMEOUT_SECONDS` | `.env.docker.example` | no |  |
| `AI_IMAGE_PROVIDER_TIMEOUT_SECONDS` | `.env.docker.example` | no |  |
| `AI_IMAGE_TASK_MAX_RUNTIME_SECONDS` | `.env.docker.example` | no |  |
| `AI_IMAGE_POLL_INTERVAL_SECONDS` | `.env.docker.example` | no |  |
| `AI_IMAGE_TRIAL_TIMEOUT_SECONDS` | `.env.docker.example` | no |  |
| `IMAGE_AUTO_RETRY_ENABLED` | `.env.docker.example` | no |  |
| `IMAGE_MAX_RETRIES` | `.env.docker.example` | no |  |
| `IMAGE_RETRY_BASE_DELAY_SECONDS` | `.env.docker.example` | no |  |
| `IMAGE_RETRY_MAX_DELAY_SECONDS` | `.env.docker.example` | no |  |
| `ORDER_SYNC_QUEUE_ENABLED` | `.env.docker.example` | no |  |
| `ORDER_SYNC_QUEUE_NAME` | `.env.docker.example` | no |  |
| `ORDER_SYNC_WORKER_CONCURRENCY` | `.env.docker.example` | no |  |
| `ORDER_SYNC_TASK_TIMEOUT_SECONDS` | `.env.docker.example` | no |  |
| `CUSTOMER_MESSAGE_SYNC_QUEUE_ENABLED` | `.env.docker.example` | no |  |
| `CUSTOMER_MESSAGE_SYNC_QUEUE_NAME` | `.env.docker.example` | no |  |
| `CUSTOMER_MESSAGE_SYNC_WORKER_CONCURRENCY` | `.env.docker.example` | no |  |
| `CUSTOMER_MESSAGE_SYNC_TASK_TIMEOUT_SECONDS` | `.env.docker.example` | no |  |
| `PRODUCT_PUBLISH_QUEUE_ENABLED` | `.env.docker.example` | no |  |
| `PRODUCT_PUBLISH_QUEUE_NAME` | `.env.docker.example` | no |  |
| `PRODUCT_PUBLISH_WORKER_CONCURRENCY` | `.env.docker.example` | no |  |
| `PRODUCT_PUBLISH_TASK_TIMEOUT_SECONDS` | `.env.docker.example` | no |  |
| `INVENTORY_SYNC_QUEUE_ENABLED` | `.env.docker.example` | no |  |
| `INVENTORY_SYNC_QUEUE_NAME` | `.env.docker.example` | no |  |
| `INVENTORY_SYNC_WORKER_CONCURRENCY` | `.env.docker.example` | no |  |
| `INVENTORY_SYNC_TASK_TIMEOUT_SECONDS` | `.env.docker.example` | no |  |
| `COLLECT_TASK_TIMEOUT_SECONDS` | `.env.docker.example` | no |  |
| `WORKER_HEARTBEAT_ENABLED` | `.env.docker.example` | no |  |
| `WORKER_HEARTBEAT_INTERVAL_SECONDS` | `.env.docker.example` | no |  |
| `WORKER_STALE_AFTER_SECONDS` | `.env.docker.example` | no |  |
| `WORKER_REAPER_ENABLED` | `.env.docker.example` | no |  |
| `WORKER_REAPER_INTERVAL_SECONDS` | `.env.docker.example` | no |  |
| `WORKER_LEGACY_RUNNING_TIMEOUT_SECONDS` | `.env.docker.example` | no |  |
| `TASK_ALERT_SCAN_ENABLED` | `.env.docker.example` | no |  |
| `TASK_ALERT_SCAN_INTERVAL_SECONDS` | `.env.docker.example` | no |  |
| `TASK_ALERT_SCAN_LOOKBACK_MINUTES` | `.env.docker.example` | no |  |
| `TASK_ALERT_SCAN_LOCK_TTL_SECONDS` | `.env.docker.example` | no |  |
| `BACKUP_ENABLED` | `.env.docker.example` | no |  |
| `BACKUP_MODE` | `.env.docker.example` | no |  |
| `BACKUP_SCHEDULE` | `.env.docker.example` | no |  |
| `BACKUP_STORAGE_PROVIDER` | `.env.docker.example` | no |  |
| `BACKUP_STORAGE_BUCKET` | `.env.docker.example` | no |  |
| `BACKUP_STORAGE_PREFIX` | `.env.docker.example` | no |  |
| `BACKUP_ENCRYPTION_ENABLED` | `.env.docker.example` | no |  |
| `BACKUP_ENCRYPTION_KEY_ID` | `.env.docker.example` | yes |  |
| `BACKUP_RETENTION_DAILY` | `.env.docker.example` | no |  |
| `BACKUP_RETENTION_WEEKLY` | `.env.docker.example` | no |  |
| `BACKUP_RETENTION_MONTHLY` | `.env.docker.example` | no |  |
| `BACKUP_MAX_AGE_HOURS` | `.env.docker.example` | no |  |
| `BACKUP_COMMAND_TIMEOUT_SECONDS` | `.env.docker.example` | no |  |
| `BACKUP_VERIFY_ENABLED` | `.env.docker.example` | no |  |
| `BACKUP_RESTORE_DRILL_ENABLED` | `.env.docker.example` | no |  |
| `BACKUP_RESTORE_DRILL_SCHEDULE` | `.env.docker.example` | no |  |
| `POSTGRES_BACKUP_FORMAT` | `.env.docker.example` | no |  |
| `POSTGRES_PG_DUMP_PATH` | `.env.docker.example` | no |  |
| `POSTGRES_PG_RESTORE_PATH` | `.env.docker.example` | no |  |
| `POSTGRES_PSQL_PATH` | `.env.docker.example` | no |  |
| `POSTGRES_WAL_ARCHIVE_ENABLED` | `.env.docker.example` | no |  |
| `POSTGRES_WAL_ARCHIVE_PATH` | `.env.docker.example` | no |  |
| `POSTGRES_PITR_ENABLED` | `.env.docker.example` | no |  |
| `RELEASE_ENABLED` | `.env.docker.example` | no |  |
| `RELEASE_ROOT` | `.env.docker.example` | no |  |
| `RELEASE_ARTIFACT_DIR` | `.env.docker.example` | no |  |
| `RELEASE_CURRENT_LINK` | `.env.docker.example` | no |  |
| `RELEASE_PREVIOUS_LINK` | `.env.docker.example` | no |  |
| `RELEASE_KEEP_COUNT` | `.env.docker.example` | no |  |
| `RELEASE_HEALTH_TIMEOUT_SECONDS` | `.env.docker.example` | no |  |
| `RELEASE_ROLLBACK_ON_FAILURE` | `.env.docker.example` | no |  |
| `RELEASE_REQUIRE_PRE_BACKUP` | `.env.docker.example` | no |  |
| `RELEASE_STRATEGY` | `.env.docker.example` | no |  |
| `RELEASE_TRAFFIC_SWITCH_MODE` | `.env.docker.example` | no |  |
| `PERFORMANCE_TEST_MODE` | `.env.docker.example` | no | P7 performance / capacity guardrails for Docker trial deployments. |
| `ALLOW_PERFORMANCE_DATASET` | `.env.docker.example` | no |  |
| `P7_DIAGNOSTICS_ENABLED` | `.env.docker.example` | no |  |
| `P7_DIAGNOSTIC_RUN_ID` | `.env.docker.example` | no |  |
| `P7_DIAGNOSTIC_ROLE` | `.env.docker.example` | no |  |
| `P7_DIAGNOSTIC_DIR` | `.env.docker.example` | no |  |
| `P7_DIAGNOSTIC_BUFFER` | `.env.docker.example` | no |  |
| `P7_DIAGNOSTIC_RUNTIME_SNAPSHOT_INTERVAL_MS` | `.env.docker.example` | no |  |
| `EXTERNAL_PROVIDER_MODE` | `.env.docker.example` | no |  |
| `DOUYIN_WRITE_ENABLED` | `.env.docker.example` | no |  |
| `AUTO_LISTING_ENABLED` | `.env.docker.example` | no |  |
| `PERFORMANCE_DATASET_MAX_ROWS` | `.env.docker.example` | no |  |
| `PERFORMANCE_TEST_MAX_VUS` | `.env.docker.example` | no |  |
| `PERFORMANCE_TEST_MAX_DURATION_SECONDS` | `.env.docker.example` | no |  |
| `PAGINATION_DEFAULT_LIMIT` | `.env.docker.example` | no |  |
| `PAGINATION_MAX_LIMIT` | `.env.docker.example` | no |  |
| `PAGINATION_MAX_OFFSET` | `.env.docker.example` | no |  |
| `PAGINATION_CURSOR_SIGNING_KEY` | `.env.docker.example` | yes |  |
| `DB_MAX_OPEN_CONNECTIONS` | `.env.docker.example` | no |  |
| `DB_MAX_IDLE_CONNECTIONS` | `.env.docker.example` | no |  |
| `DB_CONN_MAX_LIFETIME_SECONDS` | `.env.docker.example` | no |  |
| `DB_CONN_MAX_IDLE_TIME_SECONDS` | `.env.docker.example` | no |  |
| `DB_QUERY_TIMEOUT_MS` | `.env.docker.example` | no |  |
| `DB_TRANSACTION_TIMEOUT_MS` | `.env.docker.example` | no |  |
| `WORKER_CONCURRENCY_DEFAULT` | `.env.docker.example` | no |  |
| `WORKER_QUEUE_CAPACITY` | `.env.docker.example` | no |  |
| `WORKER_MAX_INFLIGHT` | `.env.docker.example` | no |  |
| `WORKER_PREFETCH` | `.env.docker.example` | no |  |
| `WORKER_SHUTDOWN_TIMEOUT_SECONDS` | `.env.docker.example` | no |  |
| `RATE_LIMIT_ENABLED` | `.env.docker.example` | no |  |
| `RATE_LIMIT_MODE` | `.env.docker.example` | no |  |
| `RATE_LIMIT_REDIS_PREFIX` | `.env.docker.example` | no |  |
| `RATE_LIMIT_FAIL_MODE` | `.env.docker.example` | no |  |
| `RATE_LIMIT_LOCAL_FALLBACK` | `.env.docker.example` | no |  |
| `RATE_LIMIT_POLICY_VERSION` | `.env.docker.example` | no |  |
| `CACHE_ENABLED` | `.env.docker.example` | no |  |
| `CACHE_DEFAULT_TTL_SECONDS` | `.env.docker.example` | no |  |
| `CACHE_MAX_ENTRIES` | `.env.docker.example` | no |  |
| `CACHE_SINGLEFLIGHT_ENABLED` | `.env.docker.example` | no |  |
| `EXPORT_BATCH_SIZE` | `.env.docker.example` | no |  |
| `EXPORT_MAX_ROWS` | `.env.docker.example` | no |  |
| `EXPORT_MAX_BYTES` | `.env.docker.example` | no |  |
| `EXPORT_MAX_CONCURRENT` | `.env.docker.example` | no |  |
| `PPROF_ENABLED` | `.env.docker.example` | no |  |
| `PPROF_INTERNAL_ONLY` | `.env.docker.example` | no |  |
| `UPLOAD_MAX_MB` | `.env.docker.example` | no |  |
| `WEBHOOK_MAX_BODY_KB` | `.env.docker.example` | no | Inbound platform webhooks (public; no JWT) |
| `WEBHOOK_MAX_CLOCK_SKEW_SECONDS` | `.env.docker.example` | no |  |
| `WEBHOOK_ENABLE_TEST_VERIFIER` | `.env.docker.example` | no |  |
| `WEBHOOK_WORKER_INTERVAL_SECONDS` | `.env.docker.example` | no |  |
| `DOUYIN_WEBHOOK_TEST_SHOP_BINDING_ID` | `.env.docker.example` | no |  |
| `ENABLE_DOUYIN_WEBHOOK_DEMO_FALLBACK` | `.env.docker.example` | no |  |
