# TradeMind 文档中心

TradeMind 是一个聚焦 `AI 商品运营工具` 与 `多平台跨境 ERP MVP` 的开源平台。仓库首页负责展示项目定位与产品预览，`docs/` 负责承载开发、部署、架构、协作与维护细节。

如果你是第一次进入这个项目，建议先按角色找到入口，而不是从头顺着文件名阅读。

## 按角色开始

| 你现在想做什么 | 建议先看 |
| --- | --- |
| 我想快速了解项目能做什么 | [../README.md](../README.md) · [roadmap.md](roadmap.md) · [PROGRESS.md](PROGRESS.md) · [FULL_PROJECT_FUNCTION_MAP.md](FULL_PROJECT_FUNCTION_MAP.md) |
| 我想本地跑起来或用 Docker 试用 | [development.md](development.md) · [docker-deployment.md](docker-deployment.md) · [env.md](env.md) |
| 我想接 API、改功能、扩 Provider | [architecture.md](architecture.md) · [api.md](api.md) · [provider.md](provider.md) |
| 我想参与协作或用 AI 工具开发 | [../AGENTS.md](../AGENTS.md) · [ai-workflow.md](ai-workflow.md) · [module-map.md](module-map.md) |

## 核心入口

| 文档 | 说明 | 适合谁 |
| --- | --- | --- |
| [development.md](development.md) | 本地开发、常用命令、端口、环境变量 | 开发者 |
| [docker-deployment.md](docker-deployment.md) | Docker Compose 完整部署、端口、日志、数据卷 | 试用者 / 部署者 |
| [ai-workflow.md](ai-workflow.md) | 跨 AI 工具通用工作流、提示词优化、上下文预算、token 节约和经验沉淀 | 开发者 / AI Agent |
| [ui-copywriting.md](ui-copywriting.md) | 管理端/API 用户可见文案中文化、术语表与 `pnpm check:ui-copy` | 开发者 / AI Agent |
| [env.md](env.md) | 环境变量清单、敏感配置、安全规则与同步要求 | 开发者 / 部署者 / AI Agent |
| [api.md](api.md) | API 公共约定、主要路由与前后端契约同步要求 | 前后端开发者 |
| [architecture.md](architecture.md) | 总体架构、分层、数据与队列、安全原则 | 开发者 / 架构维护者 |
| [provider.md](provider.md) | AI / Storage / Image / Platform / Collector Provider 扩展机制 | Provider 贡献者 |
| [collector-1688-pitfalls.md](collector-1688-pitfalls.md) | 1688 采集已知 bug、防复发约束与回归命令 | Collector / AI Agent |
| [custom-collect-rules.md](custom-collect-rules.md) | 自定义链接采集规则 JSON、API 与错误码 | collect / admin / Collector |
| [github-repo-presentation.md](github-repo-presentation.md) | GitHub 仓库首页、About、Topics、Social Preview 配置清单 | 维护者 / 开源协作者 |
| [open-source-presentation-checklist.md](open-source-presentation-checklist.md) | 开源展示发布前自检：README、About、Topics、头图与文档入口一致性 | 维护者 / 开源协作者 |
| [AI_PRODUCT_OPERATION_UX_AUDIT.md](AI_PRODUCT_OPERATION_UX_AUDIT.md) / [AI_PRODUCT_OPERATION_UX_ACCEPTANCE.md](AI_PRODUCT_OPERATION_UX_ACCEPTANCE.md) | AI 商品运营体验审计、A1.1 稳定性补强与验收状态 | 产品 / 前后端 / AI Agent |
| [CODEX_MANUAL_SIMULATION_REPORT.md](CODEX_MANUAL_SIMULATION_REPORT.md) | Phase R1.3-Codex 模拟人工测试与体验问题扫描报告 | 产品 / 前后端 / AI Agent |
| [POST_F9_ENHANCEMENT_PLAN.md](POST_F9_ENHANCEMENT_PLAN.md) | Phase H1 Post-F9 增强计划、Tag deferred 策略与边界 | 产品 / 开发者 / AI Agent |
| [P4_R_DEMO_REGRESSION_AUDIT.md](P4_R_DEMO_REGRESSION_AUDIT.md) | Phase P4-R Demo 回归稳定性审计、环境隔离与 seed/AI trial 根因入口 | 开发者 / AI Agent |
| [P4_R_DEMO_REGRESSION_REPORT.md](P4_R_DEMO_REGRESSION_REPORT.md) | Phase P4-R 静态闭环扫描报告；Run 2 / Run 3 以实际执行报告为准 | 开发者 / AI Agent |
| [P5_1_OBSERVABILITY_CLOSURE_REPORT.md](P5_1_OBSERVABILITY_CLOSURE_REPORT.md) | Phase P5.1 可观测性执行闭环扫描报告，区分 code-ready / incomplete / deferred | 开发者 / AI Agent |
| [P5_1_EXECUTION_CLOSURE_AUDIT.md](P5_1_EXECUTION_CLOSURE_AUDIT.md) | P5.1 执行闭环审计矩阵，禁止把 registered 当作 instrumented | 开发者 / AI Agent |
| [P5_1_BUSINESS_INSTRUMENTATION.md](P5_1_BUSINESS_INSTRUMENTATION.md) | P5.1 业务埋点真实接线状态与剩余缺口 | 开发者 / AI Agent |
| [P5_1_DATABASE_OBSERVABILITY.md](P5_1_DATABASE_OBSERVABILITY.md) / [P5_1_OTLP_DEPENDENCY_RESOLUTION.md](P5_1_OTLP_DEPENDENCY_RESOLUTION.md) | P5.1 DB runtime/query 观测与 OTLP 依赖冲突处理 | 后端 / AI Agent |
| [P5_2_FINAL_OBSERVABILITY_REPORT.md](P5_2_FINAL_OBSERVABILITY_REPORT.md) | P5.2 九类业务埋点接线与最终可观测性验证报告，区分 code-level passed 与未关闭门禁 | 后端 / AI Agent |
| [P5_V_FINAL_VERIFICATION_AUDIT.md](P5_V_FINAL_VERIFICATION_AUDIT.md) / [P5_V_OTLP_PROTOCOL_IMPLEMENTATION.md](P5_V_OTLP_PROTOCOL_IMPLEMENTATION.md) / [P5_V_FINAL_OBSERVABILITY_REPORT.md](P5_V_FINAL_OBSERVABILITY_REPORT.md) | P5-V 标准 OTLP/HTTP JSON 实现、Mock Collector 协议测试与最终关闭验证门；真实 telemetry backend 仍 Deferred | 后端 / AI Agent |
| [P6_BACKUP_RELEASE_DR_AUDIT.md](P6_BACKUP_RELEASE_DR_AUDIT.md) / [P6_BACKUP_RELEASE_DR_REPORT.md](P6_BACKUP_RELEASE_DR_REPORT.md) | P6 备份、恢复、发布、回滚与灾备代码级基础；P6-VR 关闭证据已记录，真实生产验证仍 Deferred | 后端 / 运维 / AI Agent |
| [P6_V_ISOLATED_RESTORE_DRILL_REPORT.md](P6_V_ISOLATED_RESTORE_DRILL_REPORT.md) / [P6_V_RELEASE_ROLLBACK_DRILL_REPORT.md](P6_V_RELEASE_ROLLBACK_DRILL_REPORT.md) / [P6_V_FINAL_CLOSURE_REPORT.md](P6_V_FINAL_CLOSURE_REPORT.md) | P6-V 隔离 PostgreSQL 恢复与发布回滚真实演练报告；P6-VR 已补齐 Linux Race 并关闭 Phase P6 | 后端 / 运维 / AI Agent |
| [P6_VR_LINUX_RACE_ENVIRONMENT_AUDIT.md](P6_VR_LINUX_RACE_ENVIRONMENT_AUDIT.md) / [P6_VR_LINUX_RACE_REMEDIATION_REPORT.md](P6_VR_LINUX_RACE_REMEDIATION_REPORT.md) / [P6_VR_FINAL_CLOSURE_REPORT.md](P6_VR_FINAL_CLOSURE_REPORT.md) | P6-VR WSL2 Linux Race 环境审计、Go 1.25 toolchain 修复与最终关闭报告；真实生产验证仍 Deferred | 后端 / 运维 / AI Agent |
| [P6_BACKUP_ARCHITECTURE.md](P6_BACKUP_ARCHITECTURE.md) / [P6_RESTORE_ARCHITECTURE.md](P6_RESTORE_ARCHITECTURE.md) / [P6_RELEASE_ARCHITECTURE.md](P6_RELEASE_ARCHITECTURE.md) / [P6_DISASTER_RECOVERY_PLAN.md](P6_DISASTER_RECOVERY_PLAN.md) | P6 各子域设计、安全边界与 Runbook 入口 | 后端 / 运维 |
| [P7_C4_R_CLEANUP_REPORT.md](P7_C4_R_CLEANUP_REPORT.md) / [P7_C4_FINAL_CLOSURE_REPORT.md](P7_C4_FINAL_CLOSURE_REPORT.md) / [P7_C4_PAGINATION_RUNTIME_REPORT.md](P7_C4_PAGINATION_RUNTIME_REPORT.md) / [P7_C4_QUERY_PLAN_REPORT.md](P7_C4_QUERY_PLAN_REPORT.md) / [P7_C4_NPLUSONE_RUNTIME_REPORT.md](P7_C4_NPLUSONE_RUNTIME_REPORT.md) / [P7_C4_RACE_TEST_REPORT.md](P7_C4_RACE_TEST_REPORT.md) | P7-C4 隔离运行验证与 P7-C4-R 残留库清理；**Phase P7-C4 Completed**，Ready for P7-V2；Load/Soak/Baseline 仍 pending P7-V2 | 后端 / 运维 / AI Agent |
| [P7_CONDITIONAL_DEVELOPMENT_CLOSURE_AND_ENGINEERING_WAIVER.md](P7_CONDITIONAL_DEVELOPMENT_CLOSURE_AND_ENGINEERING_WAIVER.md) / [P7_CONDITIONAL_DEVELOPMENT_CLOSURE_FINAL_GATE.md](P7_CONDITIONAL_DEVELOPMENT_CLOSURE_FINAL_GATE.md) / [P8_ENTRY_GATE_REPORT.md](P8_ENTRY_GATE_REPORT.md) | P7 条件开发关闭与工程豁免：**P7 Conditionally Closed**，容量与重复性验收 Deferred to P10，Ready for P8，非 Production Ready | 后端 / 运维 / AI Agent |
| [P9_POSTGRESQL_INTEGRATION_CLOSURE.md](P9_POSTGRESQL_INTEGRATION_CLOSURE.md) / [p9-postgresql-integration-closure.json](p9-postgresql-integration-closure.json) | P9 Product Batch 1–5 PostgreSQL integration baseline closure; Batch 6 ready to start, P9 remains in progress and is not Production Ready | Backend / Product / AI Agent |
| [P8_OWNER_APPROVED_SCOPE_DECISION.md](P8_OWNER_APPROVED_SCOPE_DECISION.md) / [P8_CANONICAL_SCOPE_DISCOVERY.md](P8_CANONICAL_SCOPE_DISCOVERY.md) / [P8_EXECUTION_PLAN.md](P8_EXECUTION_PLAN.md) / [P8_PLAN_FINAL_GATE.md](P8_PLAN_FINAL_GATE.md) / [P8_TASK_BATCH_1_DOMAIN_PERSISTENCE_AND_REPOSITORY.md](P8_TASK_BATCH_1_DOMAIN_PERSISTENCE_AND_REPOSITORY.md) / [P8_TASK_BATCH_2_APPROVAL_EXECUTION_AUDIT_PERSISTENCE.md](P8_TASK_BATCH_2_APPROVAL_EXECUTION_AUDIT_PERSISTENCE.md) / [P8_TASK_BATCH_3_STATE_DRAFT_APPROVAL_SERVICES.md](P8_TASK_BATCH_3_STATE_DRAFT_APPROVAL_SERVICES.md) / [P8_TASK_BATCH_4_EXECUTION_RETRY_IDEMPOTENCY_SERVICES.md](P8_TASK_BATCH_4_EXECUTION_RETRY_IDEMPOTENCY_SERVICES.md) / [P8_TASK_BATCH_5_PLATFORM_DRAFT_ADAPTERS.md](P8_TASK_BATCH_5_PLATFORM_DRAFT_ADAPTERS.md) / [P8_TASK_BATCH_6_PERMISSION_AUDIT_SECRET_FOUNDATION.md](P8_TASK_BATCH_6_PERMISSION_AUDIT_SECRET_FOUNDATION.md) / [P8_TASK_BATCH_7_OPERATION_TASK_API.md](P8_TASK_BATCH_7_OPERATION_TASK_API.md) / [P8_TASK_BATCH_8_ADMIN_OPERATION_TASK_CENTER.md](P8_TASK_BATCH_8_ADMIN_OPERATION_TASK_CENTER.md) / [P8_TASK_BATCH_9_FINAL_INTEGRATION.md](P8_TASK_BATCH_9_FINAL_INTEGRATION.md) / [P8_DEVELOPMENT_CLOSURE.md](P8_DEVELOPMENT_CLOSURE.md) | P8 owner-approved canonical scope through Batch 9 final integration, real local backend Admin/API E2E, final gate, and development closure evidence. P8 Development Complete; `workingBranch=dev`, `committed=false`; P7/P10 boundary preserved, not Production Ready | Product / Backend / Ops / AI Agent |
| [P7_PERFORMANCE_CAPACITY_AUDIT.md](P7_PERFORMANCE_CAPACITY_AUDIT.md) / [P7_PERFORMANCE_ARCHITECTURE.md](P7_PERFORMANCE_ARCHITECTURE.md) / [P7_PERFORMANCE_TARGETS.md](P7_PERFORMANCE_TARGETS.md) / [P7_PERFORMANCE_CAPACITY_REPORT.md](P7_PERFORMANCE_CAPACITY_REPORT.md) / [P7_V2_R3B_CI_RG_FINAL_REPORT.md](P7_V2_R3B_CI_RG_FINAL_REPORT.md) / [P7_V2_R3B_LPF_FINAL_REPORT.md](P7_V2_R3B_LPF_FINAL_REPORT.md) / [P7_V2_R3B_PRR_A_FINAL_REPORT.md](P7_V2_R3B_PRR_A_FINAL_REPORT.md) | P7 性能、容量、R3B 进程身份/冻结证据、Load Profile 指纹及 PRR-A 只读回归诊断入口；历史失败和 incomplete evidence 保留，不代表 Capacity Passed | 后端 / 运维 / AI Agent |
| [P7_V_CAPABILITY_COMPLETENESS_AUDIT.md](P7_V_CAPABILITY_COMPLETENESS_AUDIT.md) / [P7_V_MEDIUM_DATASET_REPORT.md](P7_V_MEDIUM_DATASET_REPORT.md) / [P7_V_DATASET_INTEGRITY_REPORT.md](P7_V_DATASET_INTEGRITY_REPORT.md) / [P7_V_FINAL_CLOSURE_REPORT.md](P7_V_FINAL_CLOSURE_REPORT.md) | P7-V 能力完整性审计、隔离 Medium 数据集真实写入、幂等验证与最终关闭门闸；当前仍 Incomplete，真实生产性能/容量/峰值验证保持 Deferred | 后端 / 运维 / AI Agent |
| [P7_C_CAPABILITY_CLOSURE_AUDIT.md](P7_C_CAPABILITY_CLOSURE_AUDIT.md) / [P7_C_CAPABILITY_CLOSURE_REPORT.md](P7_C_CAPABILITY_CLOSURE_REPORT.md) / [P7_C_CACHE_DECISION.md](P7_C_CACHE_DECISION.md) / [P7_C_RACE_PACKAGE_MAPPING.md](P7_C_RACE_PACKAGE_MAPPING.md) | P7-C 性能能力补齐与运行验证收口；P7-C4 已关闭，Phase P7 Closure 仍 Incomplete（P7-V2 待执行） | 后端 / 运维 / AI Agent |
| [P7_C2_PARTIAL_CLASSIFICATION.md](P7_C2_PARTIAL_CLASSIFICATION.md) / [P7_C2_CAPABILITY_NORMALIZATION_REPORT.md](P7_C2_CAPABILITY_NORMALIZATION_REPORT.md) / [P7_C2_DATASET_RESUME_REPORT.md](P7_C2_DATASET_RESUME_REPORT.md) / [P7_C2_RACE_TEST_REPORT.md](P7_C2_RACE_TEST_REPORT.md) / [P7_C2_FINAL_CLOSURE_REPORT.md](P7_C2_FINAL_CLOSURE_REPORT.md) | P7-C2 能力状态归一化、隔离 PostgreSQL Medium 续跑演练、WSL2 Linux Race 与最终门闸；证据由 P7-C4 承接，Gate passed | 后端 / 运维 / AI Agent |
| [P7_C3_BLOCKER_DEPENDENCY_GRAPH.md](P7_C3_BLOCKER_DEPENDENCY_GRAPH.md) / [P7_C3_PAGINATION_WIRING_AUDIT.md](P7_C3_PAGINATION_WIRING_AUDIT.md) / [P7_C3_PROVIDER_LIMIT_REPORT.md](P7_C3_PROVIDER_LIMIT_REPORT.md) / [P7_C3_PERMISSION_CACHE_INVALIDATION_REPORT.md](P7_C3_PERMISSION_CACHE_INVALIDATION_REPORT.md) / [P7_C3_FINAL_CLOSURE_REPORT.md](P7_C3_FINAL_CLOSURE_REPORT.md) | P7-C3 业务 cursor 接线、Provider limiter 与权限缓存失效；P7-C4 运行证据承接后 Gate passed | 后端 / 运维 / AI Agent |
| [WORKBENCH_URL_STATE_DESIGN.md](WORKBENCH_URL_STATE_DESIGN.md) | H1.1 工作台 URL 状态保持设计与已接入页面 | 前端 / AI Agent |
| [FULL_PROJECT_FUNCTION_MAP.md](FULL_PROJECT_FUNCTION_MAP.md) | Phase F1 全项目 34 模块功能地图与完成度 | 产品 / 开发者 / AI Agent |
| [FULL_PROJECT_MVP_MAIN_FLOW.md](FULL_PROJECT_MVP_MAIN_FLOW.md) | Phase F1 MVP 主链路 16 步定义 | 产品 / 开发者 |
| [FULL_PROJECT_DEVELOPMENT_PLAN.md](FULL_PROJECT_DEVELOPMENT_PLAN.md) | Phase F2–F9 后续开发计划 | 产品 / 开发者 |
| [FULL_PROJECT_MVP_GAP_AUDIT.md](FULL_PROJECT_MVP_GAP_AUDIT.md) | Phase F1 P0–P3 缺口审计 | 产品 / 开发者 |
| [module-map.md](module-map.md) | 模块关联索引，说明改 A 时要检查哪些 B / C / D | 开发者 / AI Agent |
| [roadmap.md](roadmap.md) | 版本路线图与阶段目标 | 所有人 |

## 文档分层

| 层级 | 作用 |
| --- | --- |
| `README.md` / `README.en.md` | 对外首页：项目定位、能力概览、界面预览、快速开始。 |
| `docs/README.md` | 文档导航首页：帮助不同角色找到正确入口。 |
| `docs/*.md` | 详细规则与实现说明：开发、部署、架构、契约、协作。 |
| `.cursor/rules/` 与 `AGENTS.md` | AI 协作规则入口，约束工程实践与文档同步。 |

## 开发与部署

| 文档 | 内容 |
| --- | --- |
| [development.md](development.md) | 本地开发环境、`pnpm dev`、分服务启动、调试与故障排查 |
| [docker-deployment.md](docker-deployment.md) | `docker-compose.full.yml`、生产前安全配置、日志与数据管理 |
| [env.md](env.md) | `.env.example`、`.env.docker.example`、Docker 端口、队列变量和敏感配置说明 |

## 架构与扩展

| 文档 | 内容 |
| --- | --- |
| [architecture.md](architecture.md) | Go backend、React admin、Node collector、PostgreSQL、Redis 的整体关系 |
| [api.md](api.md) | `/api/v1` API 契约、统一返回、鉴权与前后端同步要求 |
| [provider.md](provider.md) | Provider 抽象、扩展建议、安全要求 |
| [provider-template.md](provider-template.md) | 新增 Provider 时的接口、配置、错误处理、安全与文档模板 |
| [collector-1688-pitfalls.md](collector-1688-pitfalls.md) | 1688 采集已知问题、禁止做法与回归检查 |
| [roadmap.md](roadmap.md) | AI 商品运营工具、多平台 ERP MVP、完整 ERP 增强的推进顺序 |

## 协作与工程规则

| 文档 | 内容 |
| --- | --- |
| [branching.md](branching.md) | `main` / `dev` / `feat/*` / `fix/*` / `release/*` 分支策略与 PR 规则 |
| [ai-workflow.md](ai-workflow.md) | Codex、Cursor 等 AI 工具的通用执行流程、提示词优化、上下文控制和自我成长机制 |
| [ai-coding-rules.md](ai-coding-rules.md) | AI 编程规则、配置文件与文档同步要求 |
| [module-map.md](module-map.md) | 模块关联索引，避免代码、配置、文档、CI 漏同步 |
| [task-checklist.md](task-checklist.md) | 按任务类型收尾自查：Go、Admin、Collector、环境变量、API、Provider、Docker、CI |
| [cursor-rules-usage.md](cursor-rules-usage.md) | Cursor rules 使用说明 |
| [../AGENTS.md](../AGENTS.md) | 通用 AI Agent 协作入口 |
| [../.cursor/rules/README.md](../.cursor/rules/README.md) | Cursor rules 索引 |
| [../CONTRIBUTING.md](../CONTRIBUTING.md) | 贡献指南、提交建议、PR 要求 |

## 社区与治理

| 文档 | 内容 |
| --- | --- |
| [sponsor.md](sponsor.md) | 微信 / 支付宝赞助入口与赞助榜 |
| [../SECURITY.md](../SECURITY.md) | 安全漏洞披露与部署安全建议 |
| [../CODE_OF_CONDUCT.md](../CODE_OF_CONDUCT.md) | 社区行为准则 |
| [../NOTICE](../NOTICE) | 第三方声明、商标和致谢 |
| [../LICENSE](../LICENSE) | Apache-2.0 开源协议 |
| [../CHANGELOG.md](../CHANGELOG.md) | 版本与重要变更记录 |

## 仓库关键文件说明

| 路径 | 作用 |
| --- | --- |
| `.github/CODEOWNERS` | 定义关键目录负责人，PR 改动匹配路径时请求维护者 review。 |
| `.github/dependabot.yml` | 自动检查 GitHub Actions、pnpm、Go modules、Docker 依赖更新。 |
| `.github/labeler.yml` | 按改动路径为 PR 自动打 `area:*`、`needs:*` 标签。 |
| `.github/workflows/` | Go / Node / Docker 配置检查 / PR Labeler 等 GitHub Actions。 |
| `.github/ISSUE_TEMPLATE/` | Bug、Feature、Documentation Issue 模板与入口配置。 |
| `.github/PULL_REQUEST_TEMPLATE.md` | PR 描述、测试方式、目标分支与关联内容同步清单。 |
| `.cursor/rules/` | Cursor / AI Agent 项目级持久规则。 |
| `AGENTS.md` | 通用 AI 编程工具协作入口，适用于 Cursor 以外的 Agent。 |
| `CHANGELOG.md` | 版本与重要变更记录。 |
| `.env.example` | 本地开发环境变量模板。 |
| `.env.docker.example` | Docker 完整部署环境变量模板。 |
| `docker-compose.yml` | 本地开发基础设施：PostgreSQL + Redis。 |
| `docker-compose.full.yml` | 完整 Docker 部署编排：PostgreSQL + Redis + backend + admin + collector。 |

## 维护规则

新增或修改功能时，请同步检查：

- 启动命令变化：更新 `README.md`、`README.en.md`、`development.md`。
- Docker 行为变化：更新 `docker-deployment.md`、`.env.docker.example`。
- 环境变量变化：更新 `.env.example`、必要时更新 `.env.docker.example`，并同步 [env.md](env.md)。
- API / Provider / 队列 / 数据库变化：更新 [api.md](api.md)、[provider.md](provider.md)、[module-map.md](module-map.md) 或对应架构文档。
- 分支、CI、PR 流程变化：更新 `branching.md`、`CONTRIBUTING.md`、PR 模板。
- 较大模块或阶段性变更：更新 [PROGRESS.md](PROGRESS.md)。

详细规则见 [ai-coding-rules.md](ai-coding-rules.md)。
## P7-V2 R3B Audit Docs

| Document | Purpose |
| --- | --- |
| [P7_V2_R3B_DUAL_P99_REGRESSION_COMMON_CAUSE_AUDIT.md](P7_V2_R3B_DUAL_P99_REGRESSION_COMMON_CAUSE_AUDIT.md) | Dual p99 regression common-cause audit, cleanup evidence, root-cause classification, and final gate entry. |
| [P7_V2_R3B_DUAL_P99_LOW_CARDINALITY_DIAGNOSTICS.md](P7_V2_R3B_DUAL_P99_LOW_CARDINALITY_DIAGNOSTICS.md) | Non-formal dual p99 low-cardinality diagnostics instrumentation checkpoint and diagnostic pair plan. |
| [P7_V2_R3B_SQL_FINGERPRINT_PG_WAIT_DIAGNOSTICS.md](P7_V2_R3B_SQL_FINGERPRINT_PG_WAIT_DIAGNOSTICS.md) | Non-formal SQL fingerprint / PG wait diagnostics pair, root-cause close, and repair-path selection (not valid for P7 closure). |
| [P7_V2_R3B_WEBHOOK_TAIL_REGRESSION_REPAIR.md](P7_V2_R3B_WEBHOOK_TAIL_REGRESSION_REPAIR.md) | Webhook Ingestion p95/p99 failed-metric audit, branch-mix evidence, minimal repair path, and local verification summary. |
| [P7_V2_R3B_WEBHOOK_TAIL_REPAIR_FINAL_GATE.md](P7_V2_R3B_WEBHOOK_TAIL_REPAIR_FINAL_GATE.md) | Generated final gate for the local webhook tail repair evidence; this does not close P7-V2 without a new formal pair. |
| [P7_V2_R3B_FORMAL_PAIR_REPEATABILITY_AND_ORDER_BIAS_AUDIT.md](P7_V2_R3B_FORMAL_PAIR_REPEATABILITY_AND_ORDER_BIAS_AUDIT.md) | Non-formal repeatability/order-bias audit status, Process Identity Probe V2 evidence, and required B-C-C-B diagnostic gate entry. |
| [P7_V2_R3B_FORMAL_PAIR_REPEATABILITY_AND_ORDER_BIAS_AUDIT_FINAL_GATE.md](P7_V2_R3B_FORMAL_PAIR_REPEATABILITY_AND_ORDER_BIAS_AUDIT_FINAL_GATE.md) | Generated final gate for the repeatability/order-bias audit; expected to fail until B-C-C-B diagnostic evidence is present. |
| [P7_V2_R3B_BINARY_BOUND_FAILED_FORMAL_PAIR_AUDIT_INDEX.md](P7_V2_R3B_BINARY_BOUND_FAILED_FORMAL_PAIR_AUDIT_INDEX.md) | Audit index for the failed binary-bound Formal Pair used as input to the B-C-C-B repeatability matrix. |
| [P7_V2_R3B_BINARY_BOUND_REPEATABILITY_MATRIX.md](P7_V2_R3B_BINARY_BOUND_REPEATABILITY_MATRIX.md) | Diagnostic-only B-C-C-B repeatability matrix for frozen B/C binaries, variance analysis, root-cause classification, and repair-path selection. |
| [P7_V2_R3B_BINARY_BOUND_REPEATABILITY_MATRIX_FINAL_GATE.md](P7_V2_R3B_BINARY_BOUND_REPEATABILITY_MATRIX_FINAL_GATE.md) | Generated final gate for the binary-bound repeatability matrix; passing does not close P7-V2 or production readiness. |
| [P7_V2_R3B_BINARY_BOUND_DIAGNOSTIC_CLEANUP.md](P7_V2_R3B_BINARY_BOUND_DIAGNOSTIC_CLEANUP.md) | Exact diagnostic database/process/listener cleanup report after the B-C-C-B matrix. |
| [P7_V2_R3B_FORMAL_HOST_ISOLATION_REPAIR.md](P7_V2_R3B_FORMAL_HOST_ISOLATION_REPAIR.md) | Formal Host Isolation Contract V3 repair evidence, harness sub-root-cause classification, lifecycle/dataset/warmup/cooldown/quiet-window/predictive-barrier/PostgreSQL isolation binding, and guardrail non-changes. |
| [P7_V2_R3B_FORMAL_HOST_ISOLATION_FINAL_GATE.md](P7_V2_R3B_FORMAL_HOST_ISOLATION_FINAL_GATE.md) | Generated final gate for the Host Isolation Contract V3 repair; passing does not start the post-repair validation matrix or close P7-V2. |
| [P7_V2_R3B_HOST_ISOLATION_V3_CURRENT_SELF_VARIANCE_AUDIT.md](P7_V2_R3B_HOST_ISOLATION_V3_CURRENT_SELF_VARIANCE_AUDIT.md) | Audit of the failed V2 host-isolation validation matrix, C1/C2 failed metrics, root-cause classification, and the bounded V3 repair path. |
| [P7_V2_R3B_FORMAL_HOST_ISOLATION_V3_FINAL_GATE.md](P7_V2_R3B_FORMAL_HOST_ISOLATION_V3_FINAL_GATE.md) | Generated final gate for the Host Isolation V3 bounded repair; passing still does not create a formal plan or production-readiness claim. |
| [P7_V2_R3B_HOST_ISOLATION_V3_INCOMPLETE_MATRIX_CLOSEOUT.md](P7_V2_R3B_HOST_ISOLATION_V3_INCOMPLETE_MATRIX_CLOSEOUT.md) | Audit-only closeout for the consumed incomplete V3 validation matrix; C2/B2 must not be backfilled. |
| [P7_V2_R3B_DEDICATED_BENCHMARK_HOST_PREFLIGHT.md](P7_V2_R3B_DEDICATED_BENCHMARK_HOST_PREFLIGHT.md) / [P7_V2_R3B_DEDICATED_BENCHMARK_HOST_FINAL_GATE.md](P7_V2_R3B_DEDICATED_BENCHMARK_HOST_FINAL_GATE.md) | Generated dedicated Linux benchmark-host contract attestation and host gate; required before a fresh B-C-C-B diagnostic matrix. |
| [P7_V2_R3B_DEDICATED_BENCHMARK_HOST_VALIDATION_FINAL_GATE.md](P7_V2_R3B_DEDICATED_BENCHMARK_HOST_VALIDATION_FINAL_GATE.md) | Generated final gate for a future dedicated-host B-C-C-B validation matrix; expected to fail until all four fresh runs are present. |
| [P7_CONDITIONAL_DEVELOPMENT_CLOSURE_AND_ENGINEERING_WAIVER.md](P7_CONDITIONAL_DEVELOPMENT_CLOSURE_AND_ENGINEERING_WAIVER.md) | Engineering waiver and conditional development closure: Ready for P8, capacity acceptance deferred to P10, not Production Ready. |
| [P7_CONDITIONAL_DEVELOPMENT_CLOSURE_FINAL_GATE.md](P7_CONDITIONAL_DEVELOPMENT_CLOSURE_FINAL_GATE.md) / [P8_ENTRY_GATE_REPORT.md](P8_ENTRY_GATE_REPORT.md) | Generated conditional closure and P8 entry gates; they do not change historical performance evidence. |
| [P8_OWNER_APPROVED_SCOPE_DECISION.md](P8_OWNER_APPROVED_SCOPE_DECISION.md) / [p8-owner-approved-scope-decision.json](p8-owner-approved-scope-decision.json) / [P8_CANONICAL_SCOPE_DISCOVERY.md](P8_CANONICAL_SCOPE_DISCOVERY.md) / [p8-canonical-scope-discovery.json](p8-canonical-scope-discovery.json) / [P8_EXECUTION_PLAN.md](P8_EXECUTION_PLAN.md) / [p8-execution-plan.json](p8-execution-plan.json) / [P8_PLAN_FINAL_GATE.md](P8_PLAN_FINAL_GATE.md) / [P8_TASK_BATCH_1_DOMAIN_PERSISTENCE_AND_REPOSITORY.md](P8_TASK_BATCH_1_DOMAIN_PERSISTENCE_AND_REPOSITORY.md) / [p8-task-batch-1-domain-persistence-and-repository.json](p8-task-batch-1-domain-persistence-and-repository.json) / [P8_TASK_BATCH_2_APPROVAL_EXECUTION_AUDIT_PERSISTENCE.md](P8_TASK_BATCH_2_APPROVAL_EXECUTION_AUDIT_PERSISTENCE.md) / [p8-task-batch-2-approval-execution-audit-persistence.json](p8-task-batch-2-approval-execution-audit-persistence.json) / [P8_TASK_BATCH_3_STATE_DRAFT_APPROVAL_SERVICES.md](P8_TASK_BATCH_3_STATE_DRAFT_APPROVAL_SERVICES.md) / [p8-task-batch-3-state-draft-approval-services.json](p8-task-batch-3-state-draft-approval-services.json) / [P8_TASK_BATCH_4_EXECUTION_RETRY_IDEMPOTENCY_SERVICES.md](P8_TASK_BATCH_4_EXECUTION_RETRY_IDEMPOTENCY_SERVICES.md) / [p8-task-batch-4-execution-retry-idempotency-services.json](p8-task-batch-4-execution-retry-idempotency-services.json) | Owner-approved P8 canonical scope, execution plan, plan gate, Batch 1 domain persistence evidence, Batch 2 approval/execution/audit persistence evidence, Batch 3 state-machine/draft-version/approval service evidence, and Batch 4 execution/retry/idempotency service evidence. P8 is in progress, not completed, and not Production Ready. |

| [P9_SCOPE_DISCOVERY.md](P9_SCOPE_DISCOVERY.md) / [p9-scope-discovery.json](p9-scope-discovery.json) / [P9_OWNER_SCOPE_DECISION.md](P9_OWNER_SCOPE_DECISION.md) / [p9-owner-scope-decision.json](p9-owner-scope-decision.json) / [P9_EXECUTION_PLAN.md](P9_EXECUTION_PLAN.md) / [p9-execution-plan.json](p9-execution-plan.json) / [P9_TASK_BATCH_1_SCOPE.md](P9_TASK_BATCH_1_SCOPE.md) / [p9-task-batch-1-scope.json](p9-task-batch-1-scope.json) / [P9_ENTRY_GATE_REPORT.md](P9_ENTRY_GATE_REPORT.md) / [p9-entry-gate-report.json](p9-entry-gate-report.json) / [P9_PLAN_FINAL_GATE.md](P9_PLAN_FINAL_GATE.md) / [p9-plan-final-gate.json](p9-plan-final-gate.json) / [P9_TASK_BATCH_1_SCOPE_GATE.md](P9_TASK_BATCH_1_SCOPE_GATE.md) / [p9-task-batch-1-scope-gate.json](p9-task-batch-1-scope-gate.json) / [P9_TASK_BATCH_5_BACKEND_APIS.md](P9_TASK_BATCH_5_BACKEND_APIS.md) / [p9-task-batch-5-backend-apis.json](p9-task-batch-5-backend-apis.json) / [P9_TASK_BATCH_5_BACKEND_APIS_GATE.md](P9_TASK_BATCH_5_BACKEND_APIS_GATE.md) / [p9-task-batch-5-backend-apis-gate.json](p9-task-batch-5-backend-apis-gate.json) | P9 scope discovery, owner decision, execution plan, historical gates, and completed fixture-only Batch 5 backend API evidence. P9 remains in progress, `workingBranch=dev`, `committed=false`, PostgreSQL integration baseline passed; final P9 closure remains pending Batch 6–7, P10 boundary preserved, not Production Ready | Product / Backend / Ops / AI Agent |
## Phase P3.2 Douyin Webhook Docs

| Document | Purpose |
| --- | --- |
| [P3_2_MULTI_SHOP_WEBHOOK_AUDIT.md](P3_2_MULTI_SHOP_WEBHOOK_AUDIT.md) | Multi-shop webhook routing audit matrix. |
| [P3_2_MULTI_SHOP_WEBHOOK_REPORT.md](P3_2_MULTI_SHOP_WEBHOOK_REPORT.md) | Generated P3.2 static scan report. |
| [DOUYIN_WEBHOOK_SHOP_RESOLUTION.md](DOUYIN_WEBHOOK_SHOP_RESOLUTION.md) | Resolver inputs, trust boundary, fallback policy, and error codes. |
| [DOUYIN_WEBHOOK_TENANT_ISOLATION.md](DOUYIN_WEBHOOK_TENANT_ISOLATION.md) | Tenant/shop scoped event persistence and order upsert rules. |
| [DOUYIN_WEBHOOK_APP_SECRET_BINDING.md](DOUYIN_WEBHOOK_APP_SECRET_BINDING.md) | App key / binding validation notes. |
| [P3_2_RACE_TEST_REPORT.md](P3_2_RACE_TEST_REPORT.md) | Race verification status. Do not mark passed without a real Linux/WSL2/Docker Linux run. |
