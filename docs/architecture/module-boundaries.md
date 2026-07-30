# 模块边界说明

完整 AI 执行规范以 `.agents/skills/modular-architecture/SKILL.md` 为唯一主规范。本文只说明当前仓库的模块地图、边界意图和常见检查方式，避免形成第二套完整规则。

## 当前应用地图

- `admin/`：React + TypeScript + Ant Design Pro 管理端，运行时代码位于 `admin/src/**`。
- `collector/`：Node.js + TypeScript 采集运行时，包含 Playwright Collector、可选
  OpenCLI Bridge 与受管适配器；运行时代码位于 `collector/src/**`。
- `backend/`：Go + Gin + GORM 后端，核心内部包位于 `backend/internal/**`。
- `tests/contracts/`：API 契约测试和关键 endpoint fixture。
- `scripts/`：本地、CI、质量和测试编排脚本，不进入浏览器生产 bundle。

机器可读边界位于 `tests/architecture/module-boundaries.json`。

## 允许依赖方向

```text
Admin pages/routes -> Admin services/hooks/components/ui/utils/types
Collector tasks -> providers/browser/normalizer/types/config
Go API composition -> modules -> providers/repository/infrastructure/pkg
Tests/contracts -> production code
Production code -x-> tests/e2e/mock/fixture
Shared/common/contracts -x-> concrete app implementations
```

## 禁止依赖方向

- Admin 运行时代码直接依赖 Collector 源码。
- Collector 运行时代码直接依赖 Admin 源码。
- Go 后端依赖前端源码。
- 生产代码 import `__tests__`、`*.test.*`、`e2e`、`mock`、`fixture`。
- Shared UI、utils、types 反向依赖 pages 或 feature 内部实现。
- Worker/job 依赖 HTTP handler 或绕过 service 写业务状态。
- Repository 决定业务状态机或依赖 handler。

## Admin 示例

`admin/src/pages/**` 是路由级编排层，可组合 service、hooks 和 UI。`admin/src/services/**` 是 API 传输边界，不应 import 页面或视觉组件。`admin/src/components/ui/**` 应保持领域无关或低领域耦合，不直接调用具体业务 API。

历史上页面内部 `components`、`hooks` 存在深层复用，这类问题先作为 Advisory，后续在拆分大型页面时逐步收敛。

## Collector 示例

Collector 当前围绕 `providers/**`、`browser/**`、`normalizer/**`、`tasks/**`、
`types/**`、`config/**` 和 `opencli-bridge/**` 组织。Playwright 与 OpenCLI 必须保持
独立启动入口、客户端和故障域，backend 只按任务引擎路由；不得重新引入全局 Collector
地址切换或运行时静默回退。新增来源 adapter 时应在 adapter 边界解析第三方结构，并
尽早归一化，不把原始平台结构扩散到全部模块。

价格归一化、质量评分和 URL/字段解析应尽量保持纯函数并覆盖单元测试。Collector 不直接依赖 Admin 页面代码。

## Go 后端示例

Go 后端当前以 `backend/internal/modules/**` 组织业务模块，以 `backend/internal/providers/**` 隔离 AI、图片、存储、平台、OCR、email、collector 等外部能力。

目标方向是 handler 处理 HTTP 边界，service/usecase 承担业务规则和事务编排，repository 负责持久化，adapter/client 隔离第三方平台，worker/job 调用 service/usecase。

当前同一 module package 内可能同时存在 handler/service/repository/model，本轮不强制迁移目录；新增跨 package 越层依赖由架构门禁阻塞或要求解释。

## Adapter 示例

新增 TikTok、Shopee、Lazada、Amazon、抖店、AI、图片、存储、OCR、email 或 collector adapter 时，需要同时检查：第三方 SDK 响应是否限制在 adapter，错误是否归一化，capability/config 是否表达平台差异，测试是否使用 fake adapter，service 是否只接触内部模型。

## Worker / Queue / Scheduler 示例

Queue producer 不决定 consumer 业务细节；consumer 调用 service/usecase；scheduler 只负责触发；job payload 使用稳定结构；retry、幂等和停止逻辑由任务模块明确。Redis 原始命令不应散落在业务模块中。

## Shared 判断标准

新增 shared/common 前必须确认它被多个模块真实复用、语义稳定、不会携带单页面或单平台专用逻辑、不会形成反向依赖，并有合理测试。只被一个模块调用的复杂 service 默认留在 feature/module 内。

## 新模块创建流程

1. 说明模块职责、归属应用、公共入口和依赖方向。
2. 确认是否新增 DTO/type/enum 或 contract fixture。
3. 更新 `tests/architecture/module-boundaries.json` 的 roots/rules，而不是写宽泛 allowlist。
4. 运行 `pnpm architecture:test`、`pnpm architecture:check` 和受影响测试/质量命令。
5. 如存在历史债务，使用 baseline/ratchet 防止恶化，不自动扩大 baseline。

## 历史架构债务处理

历史大型文件、页面内部深层复用、同一 Go package 内混合 handler/service/repository/model 等不在本轮强制迁移。只有确定、稳定、可机器识别且无法立即安全修复的问题才进入 baseline。

## Baseline/Ratchet

Baseline 文件：`tests/architecture/baselines/module-boundaries.json`。

- 普通检查：`pnpm architecture:check`
- 受影响检查：`pnpm architecture:affected`
- 显式更新 baseline：`pnpm architecture:baseline -- --update`

新违规或既有违规数量增加会阻塞；违规减少允许通过并提示收紧 baseline；CI 不更新 baseline。

## 常见错误示例

- 在 `admin/src/components/ui/**` 中 import `@/pages/**`。
- 在 `admin/src/services/**` 中 import React 组件或页面内部 hook。
- 在 `collector/src/providers/**` 中复用 Admin 页面工具。
- 在生产文件中 import `__tests__` 或 `fixtures`。
- Go worker 直接依赖 handler 或绕过 service 修改数据库状态。
- 新平台逻辑散落在页面、repository 和 worker，而不是 adapter/capability。
- 将单页面临时 workaround 放入 shared/common。
