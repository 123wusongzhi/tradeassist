# F9.5 回滚与灰度准备报告

> **日期**：2026-07-07  
> **结论**：**preparation_passed_execution_blocked_by_environment**

## 回滚

- Runbook：[`DOUYIN_ROLLBACK_RUNBOOK.md`](DOUYIN_ROLLBACK_RUNBOOK.md)
- 演练：[`DOUYIN_ROLLBACK_DRILL_REPORT.md`](DOUYIN_ROLLBACK_DRILL_REPORT.md)（environment_simulation_only）
- 本地产物：`backend/tmp/server.exe`、`admin/dist/`

## 灰度

- 建议 48–72h · 单店铺 · 仅草稿 · 订单 readonly 优先
- **当前不允许进入灰度**（预发 + 抖店 E2E 未通过）

- 机器可读：[`f9-rollback-gray.json`](f9-rollback-gray.json)
