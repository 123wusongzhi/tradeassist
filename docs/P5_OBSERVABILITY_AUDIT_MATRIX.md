# P5 Observability Audit Matrix

| 模块 | 入口 | 现有日志 | 现有指标 | 现有 Trace | 关联字段 | 敏感字段风险 | 高基数风险 | 缺口 | 修改结果 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| HTTP API | middleware | slog + structured logger | http_server_* | OTel server span | request_id, trace_id | 低（脱敏） | route_template 低基数 | 无第二套 Request ID | 已统一 |
| Provider | httpclient | 部分 slog | provider_* | client span 预留 | provider, operation | Secret header 禁止 | provider+operation 枚举 | 全量 raw 响应 | 指标+脱敏 |
| Task/Worker | taskcenter | task 日志 | tasks_* | consumer span 传播 | task_type, execution_id | 低 | 禁止 task_id label | trace 传播 | 已补指标 |
| Webhook | webhook | InfoContext | webhook_* | receive/process span | platform, event_group | payload hash only | event_group 枚举 | lag 指标 | 已补 |
| AI Image | imagetask | slog | ai_image_* + provider_timeout | provider span | stage 枚举 | prompt/url 禁止 | stage 枚举 | P5-OBS-001 | 已完成 |
| Security | auth/securitymod | 审计日志 | security_* | 安全事件日志优先 | event_type | Token 禁止 | 无 ID label | 告警规则 | 已完成 |
| Alerting | alerting | 内部记录 | N/A | N/A | fingerprint | safe_details 脱敏 | rule_id 固定 | 外部 channel | code_ready deferred |
