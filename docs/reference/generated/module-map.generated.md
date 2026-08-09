---
doc_type: generated
status: generated
owner: maintainers
generator: pnpm docs:generate:module-map
---

# Module / change impact map (generated)

> Do not edit by hand. Run `pnpm docs:generate:module-map`.

## Change → documentation impact

| Rule | Paths | Required docs | Generator / check |
| --- | --- | --- | --- |
| `env-change` | `.env.example`<br>`.env.docker.example`<br>`backend/internal/config/**`<br>`docs/reference/configuration/**` | `docs/reference/configuration/environment.md` | docs:generate:env |
| `package-script-change` | `package.json`<br>`admin/package.json`<br>`collector/package.json` | `docs/reference/generated/commands.generated.md` | docs:generate:commands |
| `api-route-change` | `backend/internal/api/**`<br>`backend/internal/modules/**/handler*.go`<br>`backend/internal/router/**` | `docs/reference/api/conventions.md`<br>`docs/reference/api/routes.generated.md` | docs:generate:api |
| `provider-change` | `backend/internal/providers/**` | `docs/reference/providers/overview.md` | docs:check |
| `workflow-change` | `.github/workflows/**`<br>`scripts/workflow/**`<br>`config/agent/**` | `docs/guides/contributing/verification.md`<br>`AGENTS.md` | docs:check |
| `agent-skill-change` | `.agents/skills/**`<br>`AGENTS.md`<br>`.cursor/rules/**` | `AGENTS.md`<br>`config/agent/context-map.json` | agent:check |
| `architecture-boundary-change` | `tests/architecture/**`<br>`scripts/architecture/**` | `docs/architecture/overview.md` | architecture:check |
| `status-capability-change` | `docs/status/current.md`<br>`README.md`<br>`README.en.md` | `docs/status/current.md` | docs:check |

## Architecture boundaries

Source: `tests/architecture/module-boundaries.json`

```json
{
  "version": 1,
  "applications": {
    "admin": {
      "root": "admin/src",
      "type": "frontend-app",
      "notes": "React + TypeScript + Ant Design Pro 管理端运行时代码"
    },
    "collector": {
      "root": "collector/src",
      "type": "node-app",
      "notes": "Node.js + TypeScript + Playwright 采集服务"
    },
    "backend": {
      "root": "backend/internal",
      "type": "go-backend",
      "notes": "Go Gin/GORM 后端内部包"
    },
    "contracts": {
      "root": "tests/contracts",
      "type": "contract-tests",
      "notes": "API 契约测试和稳定 fixture"
    },
    "scripts": {
      "root": "scripts",
      "type": "tooling",
      "notes": "本地、CI、质量和测试编排脚本"
    }
  },
  "modules": [
    {
      "id": "admin-pages",
      "root": "admin/src/pages",
      "layer": "page",
      "type": "frontend-runtime",
      "allowedDependencies": [
        "admin-components",
        "admin-ui",
        "admin-services",
        "admin-hooks",
        "admin-utils",
        "admin-constants",
        "admin-types"
      ],
      "forbiddenDependencies": [
        "collector-runtime",
        "backend-internal",
        "scripts-runtime",
        "tests",
        "mocks"
      ],
      "publicEntrypoints": [],
      "notes": "路由级编排；当前存在页面内部 components/hooks 的历史深层复用，先 Advisory 管理。"
    },
    {
      "id": "admin-components",
      "root": "admin/src/components",
      "layer": "component",
      "type": "frontend-runtime",
      "allowedDependencies": [
        "admin-ui",
        "admin-utils",
        "admin-constants",
        "admin-types",
        "admin-services"
      ],
      "forbiddenDependencies": [
        "admin-pages",
        "collector-runtime",
        "backend-internal",
        "scripts-runtime",
        "tests",
        "mocks"
      ],
      "publicEntrypoints": [],
      "notes": "共享或低领域耦合 UI 组件；业务组件调用 service 需保持边界清晰。"
    },
    {
      "id": "admin-ui",
      "root": "admin/src/components/ui",
      "layer": "shared-ui",
      "type": "frontend-runtime",
      "allowedDependencies": [
        "admin-utils",
        "admin-constants",
        "admin-types"
      ],
      "forbiddenDependencies": [
        "admin-pages",
        "admin-services",
        "collector-runtime",
        "backend-internal",
        "scripts-runtime",
        "tests",
        "mocks"
      ],
      "publicEntrypoints": [],
      "notes": "领域无关共享 UI，不直接绑定页面状态机或 API。"
    },
    {
      "id": "admin-services",
      "root": "admin/src/services",
      "layer": "transport",
      "type": "frontend-runtime",
      "allowedDependencies": [
        "admin-utils",
        "admin-constants",
        "admin-types"
      ],
      "forbiddenDependencies": [
        "admin-pages",
        "admin-components",
        "admin-ui",
        "collector-runtime",
        "backend-internal",
        "scripts-runtime",
        "tests",
        "mocks"
      ],
      "publicEntrypoints": [],
      "notes": "API 调用和 request/response 边界。"
    },
    {
      "id": "admin-hooks",
      "root": "admin/src/hooks",
      "layer": "hook",
      "type": "frontend-runtime",
      "allowedDependencies": [
        "admin-services",
        "admin-utils",
        "admin-constants",
        "admin-types"
      ],
      "forbiddenDependencies": [
        "admin-pages",
        "collector-runtime",
        "backend-internal",
        "scripts-runtime",
        "tests",
        "mocks"
      ],
      "publicEntrypoints": [],
      "notes": "跨页面 hook 应避免依赖页面内部实现。"
    },
    {
      "id": "admin-utils",
      "root": "admin/src/utils",
      "layer": "utility",
      "type": "frontend-runtime",
      "allowedDependencies": [
        "admin-constants",
        "admin-types"
      ],
      "forbiddenDependencies": [
        "admin-pages",
        "admin-components",
        "admin-services",
        "collector-runtime",
        "backend-internal",
        "scripts-runtime",
        "tests",
        "mocks"
      ],
      "publicEntrypoints": [],
      "notes": "确定性工具函数，不承载隐藏全局状态。"
    },
    {
      "id": "admin-constants",
      "root": "admin/src/constants",
      "layer": "constant",
      "type": "frontend-runtime",
      "allowedDependencies": [
        "admin-types"
      ],
      "forbiddenDependencies": [
        "admin-pages",
        "admin-components",
        "admin-services",
        "collector-runtime",
        "backend-internal",
        "scripts-runtime",
        "tests",
        "mocks"
      ],
      "publicEntrypoints": [],
      "notes": "稳定常量、枚举和映射。"
    },
    {
      "id": "admin-types",
      "root": "admin/src/types",
      "layer": "type",
      "type": "frontend-runtime",
      "allowedDependencies": [],
      "forbiddenDependencies": [
        "admin-pages",
        "admin-components",
        "admin-services",
        "admin-hooks",
        "admin-utils",
        "collector-runtime",
        "backend-internal",
        "scripts-runtime",
        "tests",
        "mocks"
      ],
      "publicEntrypoints": [],
      "notes": "前端类型边界；公共契约优先与 API contract 保持一致。"
    },
    {
      "id": "collector-runtime",
      "root": "collector/src",
      "layer": "application",
      "type": "node-runtime",
      "allowedDependencies": [
        "collector-providers",
        "collector-browser",
        "collector-normalizer",
        "collector-tasks",
        "collector-types",
        "collector-config"
      ],
      "forbiddenDependencies": [
        "admin-pages",
        "admin-components",
        "admin-services",
        "backend-internal",
        "scripts-runtime",
        "tests",
        "mocks"
      ],
      "publicEntrypoints": [
        "collector/src/index.ts"
      ],
      "notes": "Collector 运行时不依赖 Admin 页面代码。"
    },
    {
      "id": "collector-providers",
      "root": "collector/src/providers",
      "layer": "adapter",
      "type": "node-runtime",
      "allowedDependencies": [
        "collector-browser",
        "collector-normalizer",
        "collector-types",
        "collector-config"
      ],
      "forbiddenDependencies": [
        "admin-pages",
        "admin-components",
        "admin-services",
        "backend-internal",
        "tests",
        "mocks"
      ],
      "publicEntrypoints": [
        "collector/src/providers/registry.ts"
      ],
      "notes": "来源 adapter、parser、normalize 和质量评分边界。"
    },
    {
      "id": "collector-browser",
      "root": "collector/src/browser",
      "layer": "infrastructure",
      "type": "node-runtime",
      "allowedDependencies": [
        "collector-types",
        "collector-config"
      ],
      "forbiddenDependencies": [
        "admin-pages",
        "admin-components",
        "backend-internal",
        "tests",
        "mocks"
      ],
      "publicEntrypoints": [],
      "notes": "Playwright/browser/session 基础设施。"
    },
    {
      "id": "collector-normalizer",
      "root": "collector/src/normalizer",
      "layer": "domain-normalize",
      "type": "node-runtime",
      "allowedDependencies": [
        "collector-types"
      ],
      "forbiddenDependencies": [
        "collector-providers",
        "admin-pages",
        "admin-components",
        "backend-internal",
        "tests",
        "mocks"
      ],
      "publicEntrypoints": [
        "collector/src/normalizer/index.ts"
      ],
      "notes": "标准化逻辑不依赖具体 UI 或 provider 内部实现。"
    },
    {
      "id": "collector-tasks",
      "root": "collector/src/tasks",
      "layer": "orchestration",
      "type": "node-runtime",
      "allowedDependencies": [
        "collector-providers",
        "collector-normalizer",
        "collector-types",
        "collector-config"
      ],
      "forbiddenDependencies": [
        "admin-pages",
        "admin-components",
        "backend-internal",
        "tests",
        "mocks"
      ],
      "publicEntrypoints": [],
      "notes": "采集任务编排。"
    },
    {
      "id": "collector-types",
      "root": "collector/src/types",
      "layer": "type",
      "type": "node-runtime",
      "allowedDependencies": [],
      "forbiddenDependencies": [
        "collector-providers",
        "collector-browser",
        "collector-tasks",
        "admin-pages",
        "backend-internal",
        "tests",
        "mocks"
      ],
      "publicEntrypoints": [],
      "notes": "Collector 稳定类型。"
    },
    {
      "id": "collector-config",
      "root": "collector/src/config",
      "layer": "config",
      "type": "node-runtime",
      "allowedDependencies": [
        "collector-types"
      ],
      "forbiddenDependencies": [
        "admin-pages",
        "backend-internal",
        "tests",
        "mocks"
      ],
      "publicEntrypoints": [],
      "notes": "环境变量和配置解析。"
    },
    {
      "id": "backend-api",
      "root": "backend/internal/api",
      "layer": "api-composition",
      "type": "go-backend",
      "allowedDependencies": [
        "backend-modules",
        "backend-providers",
        "backend-rdb",
        "backend-queue",
        "backend-pkg"
      ],
      "forbiddenDependencies": [
        "admin-pages",
        "collector-runtime",
        "tests"
      ],
      "publicEntrypoints": [],
      "notes": "路由组装和依赖注入，可依赖模块和 provider 注册。"
    },
    {
      "id": "backend-modules",
      "root": "backend/internal/modules",
      "layer": "domain-module",
      "type": "go-backend",
      "allowedDependencies": [
        "backend-providers",
        "backend-rdb",
        "backend-queue",
        "backend-pkg"
      ],
      "forbiddenDependencies": [
        "admin-pages",
        "collector-runtime",
        "tests"
      ],
      "publicEntrypoints": [],
      "notes": "handler/service/repository/model 目前多在同一 package 内，新跨模块越层依赖默认阻塞。"
    },
    {
      "id": "backend-providers",
      "root": "backend/internal/providers",
      "layer": "adapter",
      "type": "go-backend",
      "allowedDependencies": [
        "backend-pkg"
      ],
      "forbiddenDependencies": [
        "backend-api",
        "admin-pages",
        "collector-runtime",
        "tests"
      ],
      "publicEntrypoints": [],
      "notes": "第三方能力 adapter/client。"
    },
    {
      "id": "backend-rdb",
      "root": "backend/internal/rdb",
      "layer": "infrastructure",
      "type": "go-backend",
      "allowedDependencies": [
        "backend-pkg"
      ],
      "forbiddenDependencies": [
        "backend-api",
        "backend-modules",
        "admin-pages",
        "collector-runtime",
        "tests"
      ],
      "publicEntrypoints": [],
      "notes": "Redis 基础设施封装。"
    },
    {
      "id": "backend-queue",
      "root": "backend/internal/queue",
      "layer": "infrastructure",
      "type": "go-backend",
      "allowedDependencies": [
        "backend-pkg"
      ],
      "forbiddenDependencies": [
        "backend-api",
        "admin-pages",
        "collector-runtime",
        "tests"
      ],
      "publicEntrypoints": [],
      "notes": "通用队列抽象，不反向依赖 handler。"
    },
    {
      "id": "backend-pkg",
      "root": "backend/internal/pkg",
      "layer": "shared-primitive",
      "type": "go-backend",
      "allowedDependencies": [],
      "forbiddenDependencies": [
        "backend-api",
        "backend-modules",
        "backend-providers",
        "admin-pages",
        "collector-runtime",
        "tests"
      ],
      "publicEntrypoints": [],
      "notes": "Go 内部共享基础类型和工具。"
    },
    {
      "id": "contracts",
      "root": "tests/contracts",
      "layer": "contract-test",
      "type": "test",
      "allowedDependencies": [
        "admin-services",
        "backend-modules"
      ],
      "forbiddenDependencies": [],
      "publicEntrypoints": [],
      "notes": "测试代码可以依赖生产代码，生产代码不得反向依赖 contracts。"
    },
    {
      "id": "scripts-runtime",
      "root": "scripts",
      "layer": "tooling",
      "type": "tooling",
      "allowedDependencies": [],
      "forbiddenDependencies": [
        "admin-pages",
        "admin-components",
        "collector-runtime",
        "backend-internal"
      ],
      "publicEntrypo
```
