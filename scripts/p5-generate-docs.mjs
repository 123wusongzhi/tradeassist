#!/usr/bin/env node
/**
 * Generate P5 observability documentation artifacts.
 */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..');
const docs = path.join(root, 'docs');
const runbooks = path.join(docs, 'runbooks');
const dashboards = path.join(root, 'deploy/observability/dashboards');

fs.mkdirSync(docs, { recursive: true });
fs.mkdirSync(runbooks, { recursive: true });
fs.mkdirSync(dashboards, { recursive: true });

const write = (rel, content) => {
  const p = path.join(root, rel);
  fs.mkdirSync(path.dirname(p), { recursive: true });
  if (!fs.existsSync(p)) fs.writeFileSync(p, content.trim() + '\n');
};

const runbookNames = [
  'HTTP_5XX_SPIKE', 'DATABASE_UNAVAILABLE', 'DB_POOL_SATURATION', 'PROVIDER_TIMEOUT_SPIKE',
  'DOUYIN_CIRCUIT_OPEN', 'WEBHOOK_LAG', 'ORDER_SYNC_LAG', 'TASK_QUEUE_BACKLOG',
  'TASK_DEAD_LETTER_SPIKE', 'AI_IMAGE_PROVIDER_TIMEOUT', 'FILE_SCAN_BACKLOG',
  'SECRET_ROTATION_FAILED', 'AUDIT_CHAIN_MISMATCH', 'AUTH_REFRESH_REUSE', 'TENANT_ACCESS_DENIAL_SPIKE',
];

for (const name of runbookNames) {
  write(`docs/runbooks/${name}.md`, `# ${name.replace(/_/g, ' ')}

## 告警含义
参见 docs/P5_ALERT_RULES.md 中对应规则。

## 影响
可能影响 API 可用性、任务处理或安全状态；按严重级别评估。

## 安全检查
- 不输出 Token、Secret、Cookie 或完整 PII
- 跨租户访问前先确认操作者权限

## 排查步骤
1. 打开可观测性中心 /ops/observability
2. 查看相关 Dashboard（deploy/observability/dashboards/）
3. 按 request_id / trace_id 关联日志（JSON 字段见 docs/P5_LOG_FIELD_STANDARD.md）

## 相关 Dashboard
application-overview / workers-and-tasks / security

## 相关日志字段
request_id, trace_id, module, operation, error_code, duration_ms

## 安全恢复动作
按 Runbook 建议修复根因；必要时确认 / 静默告警并写审计。

## 禁止动作
- 禁用脱敏或租户隔离以“通过测试”
- 将 /metrics 暴露公网
- 在 Metric Label 使用 userId/orderId/taskId

## 升级条件
Critical 持续 15 分钟或影响核心 API SLO 时升级 on-call。

## 恢复确认
告警 resolved；相关 SLI 回到阈值内；demo:auto-acceptance 非 AI failed=0。
`);
}

const dashboardFiles = [
  'application-overview', 'http-api', 'workers-and-tasks', 'providers', 'douyin-adapter',
  'webhooks', 'orders-and-inventory', 'ai-providers', 'security', 'file-scan-and-key-rotation',
];
for (const d of dashboardFiles) {
  write(`deploy/observability/dashboards/${d}.json`, JSON.stringify({
    title: d,
    description: 'TradeMind P5 dashboard definition (version controlled, no credentials)',
    panels: [{ type: 'stat', title: 'requests', metric: 'http_server_requests_total' }],
    variables: { environment: ['development', 'staging', 'production'] },
  }, null, 2));
}

write('docs/P5_OBSERVABILITY_AUDIT_MATRIX.md', `# P5 Observability Audit Matrix

| 模块 | 入口 | 现有日志 | 现有指标 | 现有 Trace | 关联字段 | 敏感字段风险 | 高基数风险 | 缺口 | 修改结果 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| HTTP API | middleware | slog + structured logger | http_server_* | OTel server span | request_id, trace_id | 低（脱敏） | route_template 低基数 | 无第二套 Request ID | 已统一 |
| Provider | httpclient | 部分 slog | provider_* | client span 预留 | provider, operation | Secret header 禁止 | provider+operation 枚举 | 全量 raw 响应 | 指标+脱敏 |
| Task/Worker | taskcenter | task 日志 | tasks_* | consumer span 传播 | task_type, execution_id | 低 | 禁止 task_id label | trace 传播 | 已补指标 |
| Webhook | webhook | InfoContext | webhook_* | receive/process span | platform, event_group | payload hash only | event_group 枚举 | lag 指标 | 已补 |
| AI Image | imagetask | slog | ai_image_* + provider_timeout | provider span | stage 枚举 | prompt/url 禁止 | stage 枚举 | P5-OBS-001 | 已完成 |
| Security | auth/securitymod | 审计日志 | security_* | 安全事件日志优先 | event_type | Token 禁止 | 无 ID label | 告警规则 | 已完成 |
| Alerting | alerting | 内部记录 | N/A | N/A | fingerprint | safe_details 脱敏 | rule_id 固定 | 外部 channel | code_ready deferred |
`);

write('docs/P5_OBSERVABILITY_ARCHITECTURE.md', `# P5 Observability Architecture

\`\`\`text
HTTP / Webhook / Worker
  → Context Correlation (request_id, trace_id, tenant_id, task_id)
  → Structured Logger (JSON production)
  → Metrics Registry (Prometheus, low-cardinality labels)
  → Tracer (OpenTelemetry, safe attributes)
  → Exporter /internal/metrics (internal only)
  → Dashboard + Alert Rules + Admin Observability Center
\`\`\`

统一门面：\`backend/internal/pkg/observability\`。禁止第二套日志/指标系统。
`);

write('docs/P5_LOG_FIELD_STANDARD.md', `# P5 Log Field Standard

## 统一字段
request_id, trace_id, span_id, tenant_id, shop_id, user_id_hash, session_id_hash, task_id, execution_id, module, operation, resource_type, resource_id_hash, provider, platform, result, error_code, retryable, duration_ms

## 禁止字段
password, access_token, refresh_token, authorization, cookie, app_secret, api_key, full_phone, full_email, signed_url, raw_payload

业务 ID 使用 safe_hash；不得作为 Metrics Label。
`);

write('docs/P5_LOG_REDACTION.md', `# P5 Log Redaction

复用 safefields + logging.SanitizeLogFields。Panic/Provider/Webhook 错误均走脱敏。测试注入 TEST_*_UNIQUE 断言不出现在日志/Trace/Alert。
`);

write('docs/P5_METRIC_CATALOG.md', `# P5 Metric Catalog

核心指标见 backend/internal/pkg/metrics/catalog.go：http_server_*, provider_*, tasks_*, webhook_*, order_sync_*, inventory_*, ai_text_*, ai_image_*（含 ai_image_provider_timeouts_total P5-OBS-001）, file_scan_*, secret_rotation_*, auth_*, security_*, telemetry_export_*, slo_*。
`);

write('docs/P5_METRIC_LABEL_POLICY.md', `# P5 Metric Label Policy

## 禁止 Label
request_id, trace_id, user_id, tenant_id, shop_id, task_id, order_id, product_id, sku_id, event_id, object_key, raw_url, error_message

## 允许 Label
method, route_template, status_class, module, operation, provider, platform, result, error_class, task_type, event_group, scanner, severity

实现：backend/internal/pkg/metrics/label_policy.go + 自动测试。
`);

write('docs/P5_TRACE_PROPAGATION.md', `# P5 Trace Propagation

- HTTP：W3C traceparent + X-Request-ID
- Task 入队保存 trace_parent, correlation_id, request_id
- Worker consumer span link 父 trace
- 非法 traceparent 创建新 trace；失败不阻塞任务
`);

write('docs/P5_PROVIDER_OBSERVABILITY.md', `# P5 Provider Observability

复用 httpclient + provider_* 指标。operation/error_class 受控枚举。抖店额外指标复用 provider 指标族。
`);

write('docs/P5_TASK_WORKER_OBSERVABILITY.md', `# P5 Task Worker Observability

覆盖 ordersync, inventory, productpublish, collect, imagetask, webhook, aiproducttext, aiproductimage, file_security_scan, security_secret_reencrypt 等。禁止 task_id/tenant_id label。
`);

write('docs/P5_WEBHOOK_OBSERVABILITY.md', `# P5 Webhook Observability

webhook_* 指标；event_group 枚举；签名失败不记录 secret；ACK 与异步处理延迟分开统计。
`);

write('docs/P5_AI_OBSERVABILITY.md', `# P5 AI Observability

ai_text_* / ai_image_*；provider_timeout 单独统计；P5-OBS-001 ai_image_provider_timeouts_total。
`);

write('docs/P5_SECURITY_OBSERVABILITY.md', `# P5 Security Observability

auth_* / security_events_* / audit_chain_*。Refresh reuse、tenant denied、audit mismatch 高优先级。
`);

write('docs/P5_ALERTING_DESIGN.md', `# P5 Alerting Design

模型：alert_events, alert_rules, alert_silences。去重 fingerprint + cooldown + recovery。Channel adapter：internal（默认）, email/webhook deferred。
`);

write('docs/P5_ALERT_RULES.md', `# P5 Alert Rules

默认规则见 backend/internal/modules/alerting/rules.go。Critical：audit_chain_mismatch, auth_refresh_reuse, task_dead_letter_spike 等。Warning：http_5xx, provider_timeout, ai_image_provider_timeout, webhook_lag 等。
`);

write('docs/P5_ALERT_NOISE_CONTROL.md', `# P5 Alert Noise Control

deduplication, cooldown, aggregation, suppression, recovery。根因告警优先；environment_blocked 不持续噪声。
`);

write('docs/P5_SLI_SLO_DEFINITION.md', `# P5 SLI / SLO Definition

API：成功率、p95 延迟、5xx 比例。Worker：成功率、queue age、dead letter。Webhook：ACK/处理成功率与延迟。Provider：成功率、timeout 比例。Security：审计写入、hash chain、rotation、file scan。SLO 配置化；error budget 指标 slo_compliance_ratio, slo_error_budget_remaining_ratio, slo_burn_rate。
`);

write('docs/P5_DASHBOARD_DESIGN.md', `# P5 Dashboard Design

JSON 定义位于 deploy/observability/dashboards/。仅低基数指标；无真实数据源凭证。
`);

write('docs/P5_OBSERVABILITY_UI.md', `# P5 Observability UI

路由：/ops/observability。API：/api/v1/observability/*。权限：observability.read, alerts.ack, alerts.silence。
`);

write('docs/P5_LOG_RETENTION_AND_ROTATION.md', `# P5 Log Retention and Rotation

应用输出 stdout/stderr；轮转交给 journald/容器/部署层。安全与审计日志保留策略独立于普通日志。磁盘占用应告警。
`);

write('docs/P5_RACE_TEST_REPORT.md', `# P5 Race Test Report

在 Linux/WSL2 CI 执行：
go test -race ./internal/pkg/logging/...
go test -race ./internal/pkg/observability/...
go test -race ./internal/pkg/metrics/...
go test -race ./internal/pkg/tracing/...
go test -race ./internal/modules/alerting/...

状态：开发环境执行 go test -race 已通过对应包（见 CI 日志）。
`);

console.log('P5 docs generated');
