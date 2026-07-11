# 冻结后 P2 / P3 后续池

> **不阻塞 F9**；P3 **禁止**在 Post-F9 冻结边界内实现。生产级前须重新评估 P2 是否升为 P1。
> **H1 状态**：Tag deferred；P2-06 工作台后退 / 刷新状态保持 **H1.2.1 浏览器点检完成**（H1.2 页面 + 低风险补漏）。

## P2 — 体验优化

| ID | 描述 | 模块 | 备注 |
| --- | --- | --- | --- |
| P2-01 | 独立 OCR 设置菜单 | 设置 | 当前合并在图片 AI |
| P2-02 | 商品列表 1000+ 筛选优化 | 商品 | |
| P2-03 | 大批次刊登耗时提示 | 刊登 | |
| P2-04 | 采集首次登录引导加强 | 采集 | |
| P2-05 | AI 图片试跑 warning 收敛 | AI 图片 | H1.3 结构化码 + 批次/失败任务/配置状态联动 |
| P2-06 | 工作台后退/刷新状态保持 | 工作台 | H1.2.1 + H1.4 + **H1.5.1 live browser passed** |
| P2-07 | 多分辨率截图验收 | 全局 | **H1.5.1** 1366/1024 真实 PNG 归档 |
| P2-08 | Admin 内嵌文档中心 | 文档 | deferred |
| P2-09 | 空状态进一步统一 | 全局 | F7 已大部分完成 |
| P2-10 | Integrations 总览可读性 | 设置 | |
| P2-11 | Webhook 告警缺配置提示 | 告警 | |
| P2-12 | 旧版批量 AI 入口进一步收敛 | AI | |
| P2-F8-01 | 库存预警推荐操作列增强 | 库存 | 冻结后 backlog |
| P2-F8-02 | 客服复杂 Demo 样本扩展 | 客服 | edge-case seed 已补基础 |

## Reliability foundation（Phase P2.x，已完成 · 非 Production Ready）

| 项 | 状态 | 备注 |
| --- | --- | --- |
| AI 文案/图片 apply 幂等 | **done (P2.2)** | 不再 backlog；见 `AI_RESULT_APPLY_IDEMPOTENCY.md` |
| AI apply undo | **done (P2.2)** | `AI_RESULT_UNDO_DESIGN.md` |
| Webhook HTTP 公开路由 | **done (P2.2)** | `POST /api/v1/webhooks/:platform/:eventType`；平台业务适配仍后置 |
| collect / imagetask / customersync tasklease | **done (P2.2)** | 与 ordersync/inventory/publish 统一 |

## P3 — 后续增强（禁止混入 F8）

| ID | 描述 | 处置 |
| --- | --- | --- |
| P3-01 | TikTok/Shopee 等真实刊登 OpenAPI | deferred |
| P3-02 | SHEIN/Temu 生产级采集 | deferred |
| P3-03 | 自动直接上架 | **禁止** |
| P3-04 | 售后/退款/财务 | **禁止** |
| P3-05 | 多仓 WMS / 自动补货 | **禁止** |
| P3-06 | 全自动客服 | **禁止** |
| P3-07 | 复杂 BI / 自定义 Dashboard | deferred |
| P3-08 | Prometheus `/metrics` | deferred by design |
| P3-F8-01 | 更多 dedicated 单测 | 按需补充 |
| P3-F8-02 | 非抖店真实 OpenAPI | deferred |

## 评审规则

- F9 通过后、tag deferred 期间：逐条评审 P2 是否升为 P1。
- P3 进入路线图需单独 ADR / PROGRESS 记录。
