---
name: project-testing
description: TradeMind 全项目自动化测试分层、测试选择、CI 门禁和回归测试要求的唯一总控规范
---

# TradeMind 全项目自动化测试总控规范

本 Skill 是全项目测试策略和自动测试选择的唯一总控来源。Admin E2E、前端单元、后端、API 契约等专项 Skill 保持独立，本文件只负责引用和编排。

## 1. 自动适用规则

任何新功能、Bug 修复、前端、后端、API、DTO、数据库、Redis、队列、路由、业务状态机、依赖升级、构建配置、CI 或测试配置修改，均自动适用本 Skill。用户不需要主动要求补测试或运行测试。

根据范围继续读取：

- UI / Admin 页面：`.agents/skills/frontend-design/SKILL.md`
- Admin 浏览器回归：`.agents/skills/admin-e2e-testing/SKILL.md`
- 前端单元/组件：`.agents/skills/frontend-unit-testing/SKILL.md`
- Go 后端/Redis/数据库：`.agents/skills/backend-testing/SKILL.md`
- API 契约：`.agents/skills/api-contract-testing/SKILL.md`

## 2. 仓库测试分层

1. 静态检查：`pnpm check:dev`、`pnpm check:ui-copy --strict`、`pnpm build:admin`、`pnpm build:collector`、Go `gofmt`/`go vet`/`go test`。
2. 单元测试：Admin TypeScript/Vitest、collector Node/Vitest、Go `*_test.go`。
3. 组件/模块集成：React Testing Library、Gin `httptest`、Go service/provider 测试。
4. 基础设施集成：PostgreSQL、Redis、GORM AutoMigrate、Redis LIST 队列。
5. API 契约：关键 endpoint 的 method、URL、query、payload、envelope、data shape、error envelope。
6. 浏览器 E2E：现有 Admin Playwright Test P0/P1/P2，不用 E2E 替代纯逻辑单测。

## 3. 变更风险评估

- 纯函数/类型/常量：相关单测 + type/build。
- UI/组件/页面：前端单测/组件测试 + Admin E2E smoke/受影响 spec。
- service/request/DTO/envelope：前端单测 + 后端单测 + API 契约 + 受影响 E2E。
- handler/middleware/权限：Go unit + `httptest` HTTP 集成 + contract。
- repository/model/migration：Go unit + DB integration + migration test。
- Redis/queue/worker：Go unit + Redis integration + idempotency/retry tests。
- 依赖/构建/CI：受影响测试选择必须扩大，不只跑单文件。

## 4. 测试范围选择

优先运行最小但覆盖风险的测试集合。无法确定影响范围时运行 P0 安全集合：静态检查、前端 unit、Node unit、后端 unit、contract、Admin E2E smoke。

## 5. Bug 回归测试要求

Bug 修复应先添加一个修复前失败、修复后通过的回归测试。测试应表达用户可见问题或真实业务行为，不依赖内部实现。若无法稳定自动化、纯文案且已有 smoke、环境限制或一次性外部故障，可不新增，但最终报告必须说明。

## 6. 前端单元测试

遵循 `.agents/skills/frontend-unit-testing/SKILL.md`。Admin 使用 Vitest + jsdom + React Testing Library；不引入 Jest，不测试 Ant Design 内部实现。

## 7. React 组件测试

覆盖共享 UI、表单交互、hooks、URL/deep-link helper、状态映射、payload 构造和请求转换。组件测试只断言用户可见结构与可触发行为。

## 8. Node.js 测试

collector 和脚本使用 Vitest/Node 环境，覆盖配置解析、URL/price/quality 转换、页面规则纯逻辑、错误场景。不得访问真实平台。

## 9. 后端测试

遵循 `.agents/skills/backend-testing/SKILL.md`。Go 后端使用标准 `go test`、`httptest`、`testify`。优先覆盖 auth、权限、商品草稿、SKU、库存、readiness、publish、Douyin、队列、幂等、文件和告警。

## 10. 数据库测试

PostgreSQL 集成测试必须使用 `TEST_DATABASE_URL` 或 CI service container，数据库名必须包含 `test`、`_test` 或 `e2e`。不得 fallback 到开发库。

## 11. Redis / 队列测试

Redis 集成测试必须使用 `TEST_REDIS_URL` 或 CI service container，DB 编号或 key 前缀必须明确隔离。不得连接生产/开发业务 Redis。

## 12. API 契约测试

遵循 `.agents/skills/api-contract-testing/SKILL.md`。本仓库当前没有完整 OpenAPI/Swagger，先以关键 endpoint 契约清单和共享 fixtures 为准。

## 13. Admin E2E 引用

Admin 浏览器回归继续使用 `.agents/skills/admin-e2e-testing/SKILL.md`、`admin/e2e/**` 和 `.github/workflows/admin-e2e.yml`。不得删除、重建或破坏已通过的 Admin E2E。

## 14. 安全测试环境

测试不得读取生产凭据、真实 Token、真实店铺、真实平台写接口、生产数据库或生产 Redis。危险配置必须立即失败。

## 15. 测试数据规则

测试数据使用 `test` / `e2e` / `mock` 前缀，确定性、最小合法、独立、可重复，不依赖开发数据或执行顺序。

## 16. 覆盖率规则

先记录 baseline，不盲设 90% 全局门槛。新增/修改核心逻辑必须有测试，高风险模块逐步 ratchet，不为覆盖率测试 getter、常量或框架样板。

## 17. CI 门禁

PR、dev push、workflow_dispatch、nightly 按影响范围运行静态、unit、integration、contract、E2E。数据库/Redis CI 使用 service containers，不连接外部数据库。

## 18. PR 测试

PR 至 `dev`/`main` 必须运行受影响测试和 P0 安全集合。测试失败不得 skip、only、宽泛 allowlist 或降低断言掩盖。

## 19. dev push 测试

push 到 `dev` 运行受影响测试、核心 unit/contract 和必要 Admin E2E P0。

## 20. Nightly 测试

每日运行后端完整测试、数据库迁移、Redis/队列、API contract 和 Admin E2E full；避免与 `admin-e2e.yml` 对同一 full suite 重复调度。

## 21. 测试失败处理

定位首个真实根因，区分生产缺陷、测试缺陷、Mock 错误、环境错误和 flaky。不得直接重跑多次掩盖，不得简单加长 timeout。

## 22. 禁止项

禁止 Cypress；禁止同一 package 同时 Jest/Vitest；禁止真实平台写接口；禁止生产 DB/Redis；禁止 `.only`/skip；禁止只测 HTTP 200；禁止只写 snapshot；禁止无测试声明完成；未经用户要求不得 commit/push。

## 23. 完成报告格式

最终报告列出：技术栈审计、测试框架、命令、CI、契约、DB/Redis 守卫、运行结果、未运行原因、coverage baseline、修改文件、风险、是否适合签收/commit/push。
