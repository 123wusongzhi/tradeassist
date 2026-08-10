---
doc_type: status
audience: maintainer
status: current
owner: maintainers
source_of_truth:
  - README.md
  - backend/
  - admin/src/
  - collector/src/
review_cycle_days: 30
last_verified_commit: d6a3a5ac4c6de06f184f3d7d43afc88395d97b89
---

# 当前能力与限制

贸灵 TradeMind 是开源 AI 跨境电商运营平台，优先服务 **AI 商品运营** 与 **多平台跨境 ERP MVP**。
详细产品叙事见根目录 `README.md`。本页只记录**当前能力与必须知情的限制**。

## 能力矩阵

| 领域 | 当前能力 | 主要限制 | 事实来源 |
| --- | --- | --- | --- |
| 商品采集 | 浏览器扩展当前页采集；淘宝/天猫可选 OpenCLI；Playwright Collector 代码保留 | Playwright 默认停用，其他后台来源不可新建任务；平台页面结构变化仍需隔离验证 | `collector/`、`browser-extension/`、`docs/collector-engines.md` |
| 商品草稿 | 草稿、SKU、图片、发布前检查；Ozon 支持逐级叶子类目、动态属性、变体资格与完整确认快照 | Ozon 历史映射缺少人工证据时必须重新确认；其他平台字段完整性因适配进度而异 | `backend/internal/modules/`、`admin/src/pages/Product/` |
| AI 内容 | 标题/描述、Prompt、异步任务、应用与撤销 | 依赖外部 AI Provider 可用性与配额 | AI 设置页、backend AI task 模块 |
| AI 图片 | 多 Provider 异步图片任务 | 第三方服务稳定性与费用不在仓库保证范围 | Image provider / 任务页 |
| 店铺授权 | 抖店 OAuth、Ozon API Key 等接入路径；Ozon 仅在只读连接测试明确成功后标记授权 | 真实店铺授权与回调需自有应用配置；保存凭证本身不代表平台连接成功 | platform provider、店铺设置 |
| 商品刊登 | 统一刊登中心；Ozon 支持只读预检、事实状态、任务警告与人工核对 | **真实提交前必须在隔离店铺小批量验证**；`imported` 不等于可售，结果未知禁止自动重试 | PublishingCenter、platform adapters |
| 订单 / 库存 | 同步、SKU 匹配、预警与同步任务等 MVP 能力 | 多仓/采购/财务等重型 ERP 不在当前范围 | orders / inventory 模块 |
| 客服 AI | 建议回复；人工确认外发 | 不默认自动外发 | customer 模块 |
| 工程地基 | Provider 抽象、租户、幂等、契约测试、架构 ratchet；本地开发与完整 Docker 栈互斥，开发进程按身份验证后清理 | 生产就绪取决于部署方受控验收；未知端口占用者需人工处理 | `tests/`、`scripts/` |

## 当前高风险限制

- 连接真实店铺、执行刊登、库存同步或其他外部写操作前，必须在隔离环境验证。
- Ozon 只有取得明确的 `sellableVerified=true` 证据才能显示已发布；`imported`、`needs_action` 与 `result_unknown` 均要求继续核对。
- Ozon mutation 后结果未知时不得取消或自动重试；必须按 `offer_id` / 平台商品 ID 人工核对并记录证据。
- 测试与 Agent 不得连接生产 DB/Redis，不得执行未拦截的第三方写请求。
- 对外展示不要用内部阶段编号代替成熟度说明；细节以本页与代码为准。
- `frontend-design` 已正式 vendor 为项目 fork（见 `skills-lock.json`、`NOTICE`、ADR `docs/architecture/adr/0001-frontend-design-project-vendor.md`）。深度设计按需参考；默认 Admin 使用 `trademind-admin-ui`。

## 下一步决策（文档/工程）

- 是否引入完整 OpenAPI 作为 API 真源（当前为 Go 路由静态扫描快照）。
- 阶段报告批量归档策略的持续清理（`docs/archive/phases`）。

## 历史进度

旧逐日/阶段流水账已冻结：`docs/archive/progress/PROGRESS-legacy-2026-08-05.md`。
后续能力变化更新本页；过程记录优先用 PR / `CHANGELOG.md`。
