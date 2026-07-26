# ORDER SYNC LAG

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
