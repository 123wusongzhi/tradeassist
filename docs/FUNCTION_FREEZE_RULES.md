# TradeMind 功能冻结规则（Phase F8 / Post-F9）

> **生效**：Phase F8 完成后至生产决策前；F9 已于 2026-07-07 通过。
> **状态**：Phase F9 Passed · MVP Demo Ready · Ready for demo tag · 非 Production Ready · 抖店 Release Candidate

## 冻结原则

1. **禁止新增业务模块**（售后、财务、多仓 WMS、采购预测、自动补货、复杂 BI 等）。
2. **禁止新增真实平台 OpenAPI**（非抖店已规划平台保持 `local_draft_only`）。
3. **禁止修改抖店 OpenAPI 字段**（Release Candidate 契约冻结）。
4. **禁止重构主链路**（采集 → 草稿 → AI → 发布检查 → 刊登 → 订单/库存/客服）。
5. **只允许修 P0 / P1**（安全、越权、主链路断点、误导发布、无法恢复）。
6. **P2 仅允许**文案 / 样式 / 文档级低风险修复。
7. **P3 全部延后**至冻结解除或生产路线图评审。
8. **禁止**自动直接上架、Production Ready 标记和生产灰度；demo tag 仅按 F9 正式决策执行。

## 冻结范围

| 模块 | 冻结内容 |
| --- | --- |
| 采集中心 | 不新增采集源；不改权限模型 |
| 商品草稿 | 不改核心表结构；不新增 ERP 字段 |
| AI 文案 / 图片 | 不新增 Provider；Prompt 小修可以 |
| 发布检查 | 不改三态契约 |
| 多平台刊登草稿 | 不新增真实平台 API |
| 订单中心 | 不扩展售后/财务 |
| 库存中心 | 不扩展多仓 WMS |
| 客服中心 | 不自动外发 |
| 失败任务中心 | 仅修聚合/重试 P0/P1 |
| 配置状态中心 | 仅修展示/探测 P1 |
| RBAC | 仅修越权 P0/P1 |
| Dashboard | 仅修 KPI/深链 P1 |
| Demo 数据 | dev seed 小补可以 |
| 自动化基线 | 可补 smoke，不扩 E2E 到生产 |

## 允许的工作类型

- P0 / P1 bugfix
- Demo / 文档 / 文案 / 权限 / sensitiveConfirm 漏接
- go test / build / 静态扫描维护
- F9 / Post-F9 发布状态文档更新

## 明确不做（生产决策前）

- 新增业务模块或重型 ERP 能力
- 真实生产灰度
- Production Ready 标记
- 自动直接上架
- 无真实凭证时伪造抖店 E2E 通过

## 解冻条件

项目组正式批准进入预发 / 灰度 / Production Ready 流程时，由 [`PROGRESS.md`](PROGRESS.md) 与路线图更新解冻范围。
