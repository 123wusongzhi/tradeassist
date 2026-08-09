# TradeAssist / TradeMind 文档与 Agent 上下文重构执行手册

> **审阅基线**：`main@b453069`，2026-08-05
> **仓库**：`123wusongzhi/tradeassist`
> **用途**：交给代码 Agent，按原子步骤重构“给 Agent 看的说明”和“给人看的说明”。
> **审阅方式**：基于 GitHub `main` 分支的静态代码与文档审阅；未运行本地服务、数据库、第三方平台或完整 CI。实施时必须以当前分支代码、测试和可执行配置再次校验事实。

---

## 0. 这份手册如何使用

这份文件是**迁移执行计划**，不是新的长期 Agent 规则。执行 Agent 必须：

1. 不在一个超大改动中一次性完成全部迁移。
2. 严格按本手册的 PR / 提交顺序实施。
3. 先建立机器可验证的上下文路由，再删减旧入口。
4. 先移动文件、后改内容；不要在同一个提交里同时做大规模 `git mv` 和重写。
5. 不根据旧文档“修正代码”；应根据代码、测试、配置和近期实现反向修正文档。
6. 不执行真实 Ozon、抖音或其他第三方平台写请求。
7. 不自动扩大任何测试或架构 baseline。
8. 未经明确要求，不 commit、不 push。
9. 遇到无法确认的事实，标记为 `unverified`，不要猜。
10. 每个 PR 都必须能独立通过对应检查并可独立回滚。

---

# 1. 审阅结论

## 1.1 核心判断

当前问题不是单纯的“文档旧”，而是四类问题叠加：

1. **Agent 入口失去路由作用**
   `AGENTS.md` 把 README、文档中心、工作流、多个 Skill、模块图、检查清单、API、环境、分支、进度、Cursor 规则等同时列为“必读”，然后又重复自动触发规则、命令、禁令和交付流程。结果是 Agent 在尚未判断任务类型前就加载大量上下文。

2. **CI 正在主动维持上下文膨胀**
   `scripts/workflow/check-skill-triggers.mjs` 要求 `AGENTS.md` 直接引用全部核心 Skill，并以矩阵强制小型 Admin UI 任务加载 5 个 Skill；纯文档任务也要求加载 `code-quality`。仅改 Markdown 会被现有门禁反向阻止。

3. **长期规则混入了短期项目快照**
   多个 `alwaysApply: true` 的 Cursor 规则记录的是早期 MVP、旧 API 清单、旧路线图和“未来预留”，但代码与 README 已出现订单、库存、租户、Ozon 刊登、图片 Provider 等现行能力。过时内容会持续误导 Agent。

4. **人类文档、开发日志、验收证据和历史报告没有分层**
   `docs/README.md`、`docs/PROGRESS.md`、`docs/api.md`、`docs/provider.md` 等混合当前说明、阶段编号、测试结果、工作树状态、实现历史与运行手册；`docs/` 根目录又堆积大量 P4–P9 报告、截图、基线和运行产物。

## 1.2 可量化现状

以下数字用于说明规模，不作为实施后的唯一质量指标：

| 对象 | 当前规模/现象 | 主要问题 |
|---|---:|---|
| `AGENTS.md` | 117 行 | 入口同时承担项目介绍、路由、规范、命令、文档同步和工作流 |
| `.agents/skills/frontend-design/SKILL.md` | 622 行 | 自动适用、重复触发、硬编码组件/视口/命令/报告格式 |
| `scripts/workflow/check-skill-triggers.mjs` | 332 行 | 强制 AGENTS 引用所有核心 Skill |
| `tests/workflow/skill-trigger-matrix.json` | 160 行 | 小 UI 任务要求 5 个 Skill；跨模块要求 8 个 |
| `docs/ai-workflow.md` | 268 行 | 声称“少读上下文”，但自身要求再读取多份重叠文档 |
| `docs/PROGRESS.md` | 772 行 | 当前状态、逐日流水账、分支/工作树、测试证据混为一体 |
| `docs/api.md` | 563 行 | 公共约定、端点清单、阶段记录、发布门和平台细节混杂 |
| 根 `package.json` | 279 行 | 稳定命令与大量 P4–P9 临时/阶段命令混在同一入口 |
| Agent 入口、规则、核心工作流文档 | 约 3,800–3,900 行 | 正常任务可能被多入口重复加载 |

## 1.3 最高优先级风险

| ID | 严重度 | 问题 | 直接后果 |
|---|---|---|---|
| A-01 | P0 | `AGENTS.md` 把大量文件列为必读 | 启动上下文过大，Agent 难以区分真源与参考 |
| A-02 | P0 | 检查脚本强制 AGENTS 引用 8 个核心 Skill | 无法仅靠编辑入口完成瘦身 |
| A-03 | P0 | `00-project-overview.mdc`、`08-api-db-security.mdc`、`09-dev-workflow.mdc` 为全局规则且明显过时 | Agent 会按旧 MVP、旧 API、旧阶段路线执行 |
| A-04 | P0 | README 的阶段状态与 `15-external-docs-no-phase-status.mdc` 直接冲突 | 同一仓库同时给出相反规则 |
| A-05 | P1 | 622 行 UI Skill 自动应用到几乎所有 Admin 变更 | 小改动也产生大上下文和固定高成本验收 |
| A-06 | P1 | Skills 之间互相引用并重复声明质量、测试、架构和最终报告 | 形成引用图和重复规范，修改一处要同步多处 |
| H-01 | P1 | `docs/PROGRESS.md` 是持续追加的巨型日志 | “当前状态”难以读取，历史信息不断污染上下文 |
| H-02 | P1 | API/Provider 文档同时承担现行参考和阶段历史 | 容易与代码不一致，难以判断哪段仍有效 |
| H-03 | P1 | `docs/README.md` 将大量阶段报告当作核心导航 | 新用户无法快速找到稳定文档 |
| H-04 | P2 | 手工维护 `module-map`、命令清单、环境变量和端点表 | 易漂移，且要求多文件同步 |
| H-05 | P2 | `.workbuddy/memory` 等工具记忆进入仓库 | 工具特定临时上下文可能被误当长期真源 |
| H-06 | P2 | `pnpm.cmd`、P7/P9 专项命令进入通用文档 | 平台不兼容，且临时流程占据常用入口 |

---

# 2. 重构目标与非目标

## 2.1 必须达到的目标

### Agent 上下文目标

- `AGENTS.md` 只做：**全局不可违反规则 + 上下文选择入口 + 最短工作闭环 + 交付格式**。
- 自动加载内容总量控制在：
  - `AGENTS.md`：不超过 100 行、10 KiB。
  - 所有 `alwaysApply: true` Cursor 文件合计：不超过 120 行、12 KiB。
- 普通任务只加载 1 个主领域 Skill；存在明确风险条件时再加载 1–2 个专项 Skill。
- 纯文档任务不加载代码质量、前端设计、后端测试或 E2E Skill。
- 小型 Admin UI 修改默认不加载 5 个 Skill。
- 历史文档、阶段报告、运行证据永远不进入默认上下文。
- Skill 不再通过正文互相级联引用；由统一路由清单选择。
- 质量检查与测试选择优先由脚本决定，不靠长篇自然语言重复列举。

### 人类文档目标

- README 面向访客：产品价值、现行能力、快速开始、文档入口、成熟度边界。
- `docs/README.md` 面向不同受众导航，不再罗列所有历史报告。
- 当前状态、历史进展、运行手册、架构决策、API 参考、测试证据分目录管理。
- 可由代码生成的内容不再手工复制：
  - 命令索引从 `package.json` 生成。
  - 环境变量参考从 schema / example / config 生成或校验。
  - API 路由索引从路由注册或 OpenAPI 生成。
  - 模块影响图从机器配置生成。
- 稳定文档中不出现 P4/P5/P6/P7/P8/P9 等内部阶段编号、工作树状态、临时分支名和一次性测试通过数量。
- 历史资料保留 Git 历史，但从稳定导航和 Agent 默认上下文移除。

## 2.2 非目标

本次文档重构**不得顺带改变**：

- API URL、HTTP method、payload、响应 envelope。
- 数据库 schema、migration、租户语义、权限模型。
- Ozon / 抖音 / 其他平台真实业务行为。
- Admin 页面业务流程。
- Provider 接口或队列状态机。
- 测试 baseline、架构 baseline、允许列表。
- 产品名称或品牌，除非仅统一文档用词。
- 现有功能是否“生产可用”的事实判断；不确定时应写限制，不应自作结论。

---

# 3. 新的事实来源层级

任何文档段落都必须先判断属于哪一层。越靠前，权威越高。

| 优先级 | 真源 | 适合承载 | 不适合承载 |
|---:|---|---|---|
| 1 | 代码、schema、migration、路由注册、配置解析、测试 | 实际行为与接口 | 长篇解释 |
| 2 | `package.json`、workflow、Compose、env schema、架构边界配置 | 可执行命令、CI、部署和约束 | 产品叙事 |
| 3 | 自动生成的 reference | 路由、命令、环境变量、模块清单 | 手工修改 |
| 4 | 手工维护的 current reference / ADR | 设计意图、稳定约定、为何如此 | 每日进度 |
| 5 | Agent router / Skill | 如何选择上下文、如何执行任务 | 当前功能快照、完整 API 清单 |
| 6 | Guide / Runbook | 教程、排错、操作流程 | 规范的唯一真源 |
| 7 | Status | 当前能力、已知限制、近期风险 | 逐次测试日志 |
| 8 | Archive / CI artifact | 历史报告、原始证据、截图、基线 | 默认导航和 Agent 上下文 |

## 3.1 “一个事实，一个所有者”规则

以下信息只能在指定位置完整定义，其他文件只链接：

| 信息 | 唯一所有者 |
|---|---|
| 全局 Agent 禁令 | `AGENTS.md` |
| 任务到上下文/检查的映射 | `config/agent/context-map.json` |
| 文件变化到文档影响的映射 | `config/agent/change-impact.json` |
| 领域实施步骤 | 对应 `.agents/skills/*/SKILL.md` |
| 稳定命令 | `package.json` / workspace package scripts；文档由生成器产出 |
| 环境变量 | `config/env.schema.json`（建议新增）或现有 config schema |
| API 路由 | 路由注册 / OpenAPI；文档只生成或解释通用约定 |
| 当前产品能力和限制 | `docs/status/current.md` |
| 历史变化 | GitHub PR / release / `docs/status/history/` |
| 架构决策原因 | `docs/architecture/adr/` |
| 测试原始输出、截图、性能报告 | CI artifacts 或根 `artifacts/` |
| 公共产品介绍 | `README.md` / `README.en.md` |
| 项目命名 | `docs/reference/terminology.md` |

---

# 4. 目标目录结构

```text
.
├── AGENTS.md
├── config/
│   └── agent/
│       ├── context-map.json
│       ├── change-impact.json
│       └── schema.json
├── .agents/
│   └── skills/
│       ├── docs-maintenance/
│       │   └── SKILL.md
│       ├── backend-development/
│       │   └── SKILL.md
│       ├── collector-development/
│       │   └── SKILL.md
│       ├── trademind-admin-ui/
│       │   └── SKILL.md
│       ├── code-quality/
│       │   └── SKILL.md
│       ├── modular-architecture/
│       │   └── SKILL.md
│       ├── project-testing/
│       │   └── SKILL.md
│       ├── frontend-unit-testing/
│       │   └── SKILL.md
│       ├── admin-e2e-testing/
│       │   └── SKILL.md
│       ├── backend-testing/
│       │   └── SKILL.md
│       ├── api-contract-testing/
│       │   └── SKILL.md
│       └── frontend-design/
│           └── SKILL.md              # 上游锁定 Skill，仅按需参考
├── .cursor/
│   └── rules/
│       ├── README.md
│       ├── 00-agent-router.mdc       # 唯一 alwaysApply 适配器
│       ├── admin-ui.mdc
│       ├── backend.mdc
│       ├── collector.mdc
│       └── docs.mdc
├── docs/
│   ├── README.md
│   ├── guides/
│   │   ├── getting-started.md
│   │   ├── development.md
│   │   ├── ai-assisted-development.md
│   │   └── contributing/
│   │       ├── branching.md
│   │       └── verification.md
│   ├── reference/
│   │   ├── terminology.md
│   │   ├── configuration/
│   │   │   ├── environment.md
│   │   │   └── environment.generated.md
│   │   ├── api/
│   │   │   ├── conventions.md
│   │   │   ├── authentication.md
│   │   │   └── routes.generated.md
│   │   ├── providers/
│   │   │   ├── overview.md
│   │   │   ├── ai.md
│   │   │   ├── image.md
│   │   │   ├── storage.md
│   │   │   ├── collector.md
│   │   │   └── platforms/
│   │   │       ├── douyin.md
│   │   │       └── ozon.md
│   │   ├── generated/
│   │   │   ├── commands.generated.md
│   │   │   └── module-map.generated.md
│   │   └── admin-ui-system.md
│   ├── architecture/
│   │   ├── overview.md
│   │   └── adr/
│   │       └── README.md
│   ├── runbooks/
│   │   ├── local-development.md
│   │   ├── testing.md
│   │   ├── release.md
│   │   ├── collector/
│   │   └── performance/
│   ├── status/
│   │   ├── current.md
│   │   └── history/
│   └── archive/
│       ├── README.md
│       ├── agent-rules-v1/
│       ├── phases/
│       ├── progress/
│       └── tool-memory/
├── scripts/
│   ├── workflow/
│   │   ├── select-context.mjs
│   │   └── check-agent-context.mjs
│   └── docs/
│       ├── inventory.mjs
│       ├── check-links.mjs
│       ├── check-stale-claims.mjs
│       ├── check-ownership.mjs
│       ├── check-generated.mjs
│       ├── generate-command-reference.mjs
│       ├── generate-module-map.mjs
│       ├── generate-env-reference.mjs
│       └── generate-api-routes.mjs
└── tests/
    └── workflow/
        └── context-routing-matrix.json
```

> 不要求第一个 PR 一次创建全部文件。目录按后续 PR 逐步形成。

---

# 5. 现有文件逐项处置

## 5.1 Agent 入口与 Cursor 规则

| 文件 | 动作 | 原因 | 最终状态 |
|---|---|---|---|
| `AGENTS.md` | **重写** | 现有入口列出大量“必读”、重复触发、命令和工作流 | ≤100 行的路由器 |
| `.cursorrules` | **先改为兼容指针，后删除** | 内容仍是早期 MVP 且重复 | 过渡期 ≤15 行；确认客户端不再依赖后删除 |
| `.cursor/rules/README.md` | **重写** | 当前索引要求新规则同步多文件 | 只解释适配器、globs、context selector |
| `00-project-overview.mdc` | **移出 active rules 并归档** | 早期 MVP 与现行订单、库存、Ozon、租户能力冲突 | `docs/archive/agent-rules-v1/` |
| `01-architecture.mdc` | **拆分** | 稳定边界与旧目录/旧 Provider 快照混合 | 不变量进入短 Skill；现行结构进入 architecture/reference |
| `02-backend-go-gin.mdc` | **提炼为 Skill/薄适配器** | 跨工具规则不应只存在于 Cursor | `backend-development` Skill + ≤20 行 Cursor 适配器 |
| `03-frontend-react-antd-pro.mdc` | **提炼** | 含旧状态与实现快照 | 合并到项目 UI overlay / reference |
| `04-ui-style.mdc` | **重写为薄适配器** | 当前强制加载 622 行 Skill 及多个相关 Skill | 只路由到 `trademind-admin-ui` |
| `05-ai-provider.mdc` | **拆分并缩短** | 将已实现能力写成“后续预留”，还含接口建议快照 | 稳定边界进 Provider Skill/ADR；能力进 provider reference |
| `06-storage-provider.mdc` | **提炼** | 实施规范与当前 Provider 清单应分开 | 稳定边界进 Skill；清单进 generated/reference |
| `07-collector-node-playwright.mdc` | **改为跨工具 Skill** | 含稳定坑点，也含易过期的 available/beta/planned 快照 | `collector-development` + collector runbook/reference |
| `08-api-db-security.mdc` | **立即取消 alwaysApply，拆分后归档** | API 清单严重过时，安全规则与 schema 建议混杂 | 安全不变量进 AGENTS/Skill；路由与 schema 由代码生成 |
| `09-dev-workflow.mdc` | **归档，不更新阶段号** | 仍是 7 个早期阶段和旧路线图 | 历史资料，不再 active |
| `10-progress-sync.mdc` | **删除/替换** | 强制持续膨胀 `PROGRESS.md` | 仅在当前能力发生变化时更新 `status/current.md` |
| `11-local-dev-postgres.mdc` | **保留不变量并合并** | PostgreSQL 默认是稳定约束 | 合并到核心/后端 Skill，避免单独 alwaysApply |
| `12-ai-coding-doc-sync.mdc` | **改为指针** | 手工同步矩阵与 AGENTS/其他文档重复 | 指向 `change-impact.json` 和 `docs:impact` |
| `13-ai-workflow.mdc` | **删除或改薄指针** | 与 AGENTS、ai-workflow 重复 | 由 selector 统一路由 |
| `14-ui-copywriting.mdc` | **保留为 scoped 薄规则** | 领域明确，适合 globs | 指向 terminology / copywriting reference |
| `15-external-docs-no-phase-status.mdc` | **规则代码化后移除 alwaysApply** | 规则合理，但 README 正在违反 | 由 `check-stale-claims.mjs` 执行 |
| `admin-e2e-testing.mdc` 等 wrapper | **统一重写** | 当前 wrapper 级联多个 Skill | 每个 wrapper 最多指向一个 context ID |

## 5.2 Skills

| Skill | 动作 | 具体要求 |
|---|---|---|
| `frontend-design` | **保留为上游锁定、只按需参考** | 先核对 `skills-lock.json` 哈希；不要直接在其上继续堆项目规则 |
| 新增 `trademind-admin-ui` | **创建项目 overlay** | 100–140 行，包含项目视觉/组件/交互不变量；需要深度设计时再读上游 Skill |
| `code-quality` | **缩至 80–120 行** | 仅保留 diff hygiene、错误处理、安全、高风险升级；TS/React/Go/DB 细节移出 |
| `modular-architecture` | **缩至 100–140 行** | 只在新模块、边界、公共类型、adapter、worker、migration、跨模块重构触发 |
| `project-testing` | **改为 50–80 行** | 解释测试原则；具体命令由 `test:affected` 与 context map 给出 |
| `frontend-unit-testing` | **缩至 50–80 行** | 只讲前端单测边界、mock 与断言 |
| `admin-e2e-testing` | **缩至 80–120 行** | 保留写请求拦截、console、深链、响应式风险；移除页面名和固定“7 tabs”快照 |
| `backend-testing` | **缩至 70–110 行** | 按 DB/Redis/queue/worker/adapter 风险分类，不硬编码端点清单 |
| `api-contract-testing` | **缩至 60–100 行** | 关注 envelope、auth、tenant、DTO compatibility；端点来自 generated route index |
| 新增 `backend-development` | **创建** | handler → service → repository/provider/queue 分层、事务和错误边界 |
| 新增 `collector-development` | **创建** | Playwright/OpenCLI 边界、资源释放、引擎隔离、归一化输出 |
| 新增 `docs-maintenance` | **创建** | 文档分类、真源、链接、生成文件、archive 规则 |

### Skills 的统一禁令

所有 Skill 必须满足：

- 不把其他 Skill 列为“必读”。
- 不包含项目当前阶段、当前分支、测试通过数量、临时 gate。
- 不包含完整 API 路由清单。
- 不包含长目录树快照。
- 不硬编码当前页面 tab 数、精确路由数量或组件数量。
- 不重复 AGENTS 的全局禁令。
- 不各自定义最终报告格式。
- 不写 `pnpm.cmd`；统一写 `pnpm`，Windows 说明放开发指南。
- 不要求每个小任务固定跑全部视口和全部 E2E；按风险升级。
- 每个 Skill 只拥有一个领域，最大 160 行 / 16 KiB。
- 超过阈值时必须拆为 Skill + human reference/runbook。

## 5.3 人类文档

| 文件 | 动作 | 目标 |
|---|---|---|
| `README.md` | **重写部分章节** | 删除内部 Phase/closure/tag/deferred 状态表；保留产品能力、快速开始、成熟度边界 |
| `README.en.md` | **同步语义重写** | 与中文 README 信息等价，不要求逐行复制 |
| `docs/README.md` | **重写** | 按用户、贡献者、运维、维护者四类受众导航，≤100 行 |
| `docs/ai-workflow.md` | **拆分** | Agent 执行协议进 AGENTS/context；人类原理改名 `guides/ai-assisted-development.md` |
| `docs/ai-coding-rules.md` | **合并后归档** | 不再作为并列规范真源 |
| `docs/module-map.md` | **生成化** | 由 `change-impact.json` / architecture config 生成 |
| `docs/task-checklist.md` | **合并/生成化** | 人类检查说明进 contribution guide；实际命令由 selector 输出 |
| `docs/PROGRESS.md` | **冻结并归档** | 新建 `status/current.md`；旧文件移到 `archive/progress/` |
| `docs/api.md` | **拆分** | conventions/auth 手工维护；routes 自动生成；平台细节独立 |
| `docs/provider.md` | **拆分** | overview + AI/image/storage/collector/platforms；阶段历史归档 |
| `docs/architecture.md` | **更新并移动** | 纳入 browser-extension、统一刊登、测试/契约边界；不写阶段历史 |
| `docs/development.md` | **移动并精简** | 只保留稳定开发流程；P7/P9 性能/恢复命令移 runbook |
| `docs/env.md` | **拆分/生成** | 稳定应用变量与性能测试变量分开 |
| `docs/branching.md` | **清理** | 去掉 P9 等专项命令，保留分支与 PR 规则 |
| `CONTRIBUTING.md` | **去重** | 只保留贡献流程并链接文档，不复制所有规范 |
| PR 模板 | **重写** | 不再要求手工同步 6 个 Agent 规则文件；记录 selector 输出和已运行检查 |
| `.workbuddy/memory/*` | **移出活跃路径** | 有历史价值则归档，否则删除并加入 ignore |
| P4–P9 报告 | **分类归档** | 叙事报告进 `archive/phases`；原始 JSON/截图进 CI artifacts 或根 `artifacts/` |

## 5.4 需要谨慎处理的特殊项

### `skills-lock.json`

当前 `frontend-design` 记录为来自 `anthropics/skills` 的锁定文件。执行以下步骤：

1. 计算本地 Skill SHA-256。
2. 与 `skills-lock.json` 中 `computedHash` 比较。
3. 若一致：
   - 不直接重写该文件。
   - 将其改为“深度设计可选参考”。
   - 创建项目 overlay `trademind-admin-ui`。
4. 若不一致：
   - 先查 Git 历史，确认是否已被项目修改。
   - 维护者必须在“恢复上游”与“正式 vendor/fork”中二选一。
   - 若正式 fork，更新 source metadata、hash、NOTICE，并记录 ADR。
5. 禁止静默改文件却不更新 lock。

跨平台哈希命令：

```bash
node -e "const fs=require('node:fs');const c=require('node:crypto');const p='.agents/skills/frontend-design/SKILL.md';console.log(c.createHash('sha256').update(fs.readFileSync(p)).digest('hex'))"
```

### `CONTRIBUTING.md` 中的来源归属

该文件存在指向其他仓库的来源说明。Agent 不得自动删除：

1. 对照 `LICENSE`、`NOTICE` 和 Git 历史。
2. 若是 Apache-2.0 或其他许可证要求，保留并移到正确的 `NOTICE`。
3. 若只是过时模板内容，需由维护者确认后删除。
4. 文档重构不得被当作清理法律归属的理由。

### TradeAssist / TradeMind 命名

新增 `docs/reference/terminology.md`，明确：

- 仓库 slug：`tradeassist`
- npm workspace 名：以实际 `package.json` 为准
- 产品展示名：`贸灵 TradeMind`
- 代码包名、镜像名、文档展示名如何使用
- 禁止再引入新的拼写或中英文变体

---

# 6. 原子实施计划

下面按 7 个 PR 拆分。每个编号步骤都应成为可勾选任务；不要跳过顺序。

---

## PR 0：冻结基线与建立迁移清单

### 0.1 创建分支并确认用户改动

从仓库根目录执行：

```bash
git status --short --branch
git rev-parse HEAD
git log -1 --oneline
git switch -c docs/agent-context-v2-baseline
```

验收：

- 工作树中已有改动已记录。
- 不覆盖、回滚或格式化与本任务无关的文件。
- 基线 commit 写入 PR 描述。

### 0.2 生成现状清单

新增 `scripts/docs/inventory.mjs`，输出：

- 所有 `.md`、`.mdc`、`SKILL.md`、Agent JSON 文件。
- 行数、字节数、SHA-256。
- frontmatter 的 `doc_type`、`status`、`owner`。
- 入链数量与出链数量。
- 是否被 `AGENTS.md`、Cursor rule、package script、workflow 引用。
- 是否匹配阶段/历史/证据模式。
- 是否位于默认 Agent 上下文。

建议输出：

```text
artifacts/docs-inventory/2026-08-05.json
artifacts/docs-inventory/2026-08-05.md
```

不要把运行时生成清单塞回 `docs/README.md`。

### 0.3 记录旧门禁结果

先验证脚本名称：

```bash
pnpm run
```

再执行现有工作流检查：

```bash
pnpm workflow:check
pnpm quality:sensitive
```

如命令不存在，记录实际脚本名，不要凭文档猜。失败时只记录：

- 命令。
- 首个根因。
- 是否为本次改动导致。
- 阻塞范围。

### 0.4 建立段落迁移表

在 PR 描述或临时 `artifacts/docs-inventory/migration-map.csv` 中，为每个旧文档块标记：

```text
source_file
source_heading
classification
target_file
truth_source
action
status
```

`classification` 只能是：

- `global-agent-invariant`
- `domain-procedure`
- `human-guide`
- `current-reference`
- `generated-reference`
- `runbook`
- `architecture-decision`
- `current-status`
- `history`
- `evidence`
- `delete-duplicate`

### 0.5 PR 0 验收

- 只新增清单/审阅工具，不改变现有规范语义。
- 锁定 Skill 哈希已记录。
- 当前门禁结果已记录。
- 每个待迁移文件有目标分类。
- 无大规模移动或删除。

---

## PR 1：先建立机器可读上下文路由

这是整个迁移的关键。没有完成本 PR，不允许精简 `AGENTS.md`。

### 1.1 新建 `config/agent/context-map.json`

建议 schema：

```json
{
  "version": 2,
  "budgets": {
    "alwaysLoadedMaxBytes": 12288,
    "normalRequiredFilesMax": 3,
    "normalRequiredBytesMax": 32768,
    "deepRequiredFilesMax": 6,
    "deepRequiredBytesMax": 71680
  },
  "contexts": {
    "docs-maintenance": {
      "path": ".agents/skills/docs-maintenance/SKILL.md",
      "kind": "skill"
    },
    "admin-ui": {
      "path": ".agents/skills/trademind-admin-ui/SKILL.md",
      "kind": "skill"
    },
    "frontend-design-deep": {
      "path": ".agents/skills/frontend-design/SKILL.md",
      "kind": "optional-reference"
    },
    "frontend-unit": {
      "path": ".agents/skills/frontend-unit-testing/SKILL.md",
      "kind": "skill"
    },
    "admin-e2e": {
      "path": ".agents/skills/admin-e2e-testing/SKILL.md",
      "kind": "skill"
    },
    "backend-development": {
      "path": ".agents/skills/backend-development/SKILL.md",
      "kind": "skill"
    },
    "backend-testing": {
      "path": ".agents/skills/backend-testing/SKILL.md",
      "kind": "skill"
    },
    "api-contract": {
      "path": ".agents/skills/api-contract-testing/SKILL.md",
      "kind": "skill"
    },
    "architecture-change": {
      "path": ".agents/skills/modular-architecture/SKILL.md",
      "kind": "skill"
    },
    "collector-development": {
      "path": ".agents/skills/collector-development/SKILL.md",
      "kind": "skill"
    },
    "deep-code-review": {
      "path": ".agents/skills/code-quality/SKILL.md",
      "kind": "optional-reference"
    }
  },
  "tasks": [
    {
      "id": "documentation-only",
      "priority": 100,
      "match": {
        "include": ["**/*.md", "**/*.mdc", "config/agent/**"],
        "exclude": ["package.json", ".github/workflows/**", "scripts/**"]
      },
      "requiredContexts": ["docs-maintenance"],
      "optionalContexts": [],
      "checks": ["docs:check"],
      "depth": "light"
    },
    {
      "id": "small-admin-ui",
      "priority": 80,
      "match": {
        "include": ["admin/src/pages/**", "admin/src/components/**", "admin/src/**/*.less"],
        "riskFlagsAbsent": ["api-contract", "shared-boundary", "write-flow", "route-change"]
      },
      "requiredContexts": ["admin-ui"],
      "optionalContexts": ["frontend-unit"],
      "checks": ["test:frontend", "quality:affected", "test:affected"],
      "depth": "light"
    },
    {
      "id": "admin-interaction-or-write",
      "priority": 90,
      "match": {
        "include": ["admin/src/**"],
        "riskFlagsAny": ["write-flow", "route-change", "deep-link", "modal-drawer", "responsive-layout"]
      },
      "requiredContexts": ["admin-ui", "admin-e2e"],
      "optionalContexts": ["frontend-unit", "frontend-design-deep"],
      "checks": ["test:frontend", "test:e2e:smoke", "quality:affected", "test:affected"],
      "depth": "deep"
    },
    {
      "id": "backend-service",
      "priority": 70,
      "match": {
        "include": ["backend/**/*.go"],
        "riskFlagsAbsent": ["db-schema", "api-contract", "queue-worker", "platform-adapter", "shared-boundary"]
      },
      "requiredContexts": ["backend-development"],
      "optionalContexts": ["backend-testing"],
      "checks": ["test:backend", "quality:affected", "test:affected"],
      "depth": "light"
    },
    {
      "id": "api-contract-change",
      "priority": 95,
      "match": {
        "riskFlagsAny": ["api-contract", "dto", "envelope", "auth", "tenant-boundary"]
      },
      "requiredContexts": ["backend-development", "api-contract"],
      "optionalContexts": ["backend-testing", "admin-e2e"],
      "checks": ["test:contracts", "test:backend", "quality:affected", "test:affected"],
      "depth": "deep"
    },
    {
      "id": "architecture-change",
      "priority": 100,
      "match": {
        "riskFlagsAny": ["shared-boundary", "new-module", "migration", "queue-worker", "platform-adapter"]
      },
      "requiredContexts": ["architecture-change"],
      "optionalContexts": ["backend-development", "backend-testing", "api-contract", "deep-code-review"],
      "checks": ["architecture:affected", "quality:affected", "test:affected"],
      "depth": "deep"
    },
    {
      "id": "collector-change",
      "priority": 80,
      "match": {
        "include": ["collector/**", "browser-extension/**"]
      },
      "requiredContexts": ["collector-development"],
      "optionalContexts": [],
      "checks": ["test:collector", "quality:affected", "test:affected"],
      "depth": "light"
    }
  ]
}
```

实施注意：

- 示例不是要求原样照抄所有路径；Agent 必须对照实际目录调整。
- `requiredContexts` 是最低必读。
- `optionalContexts` 只有命中明确风险条件才读，禁止自动全部加载。
- 测试命令与上下文读取分开；“需要运行测试”不等于“需要加载长测试 Skill”。
- `archive/**`、`artifacts/**`、`docs/status/history/**` 默认禁止作为上下文。
- JSON 使用原生解析，避免仅为配置新增 YAML 依赖。

### 1.2 新建 `config/agent/change-impact.json`

用于替代手工 `docs/module-map.md` 和多处文档同步清单。

示例：

```json
{
  "version": 1,
  "rules": [
    {
      "id": "env-change",
      "paths": [".env.example", ".env.docker.example", "backend/internal/config/**"],
      "requiredDocs": ["docs/reference/configuration/environment.generated.md"],
      "generator": "docs:generate:env"
    },
    {
      "id": "package-script-change",
      "paths": ["package.json", "admin/package.json", "collector/package.json"],
      "requiredDocs": ["docs/reference/generated/commands.generated.md"],
      "generator": "docs:generate:commands"
    },
    {
      "id": "api-route-change",
      "paths": ["backend/internal/api/**", "backend/internal/modules/**/handler*.go"],
      "requiredDocs": ["docs/reference/api/routes.generated.md"],
      "generator": "docs:generate:api"
    },
    {
      "id": "provider-change",
      "paths": ["backend/internal/providers/**"],
      "requiredDocs": ["docs/reference/providers/**"],
      "check": "docs:check:providers"
    },
    {
      "id": "workflow-change",
      "paths": [".github/workflows/**", "scripts/workflow/**"],
      "requiredDocs": ["docs/guides/contributing/verification.md"],
      "check": "docs:check"
    }
  ]
}
```

### 1.3 新建 `scripts/workflow/select-context.mjs`

必须支持：

```bash
pnpm agent:context -- --files "admin/src/pages/X/index.tsx"
pnpm agent:context -- --files-from-git
pnpm agent:context -- --intent documentation-only
pnpm agent:context -- --files-from-git --json
```

算法必须按以下顺序：

1. 读取 `config/agent/context-map.json`。
2. 规范化路径分隔符为 `/`。
3. 从参数、`git diff --name-only` 或 intent 获取目标文件。
4. 根据路径和显式风险标记匹配 task。
5. 按 `priority` 排序。
6. 合并 `requiredContexts`，去重。
7. `optionalContexts` 只显示触发条件，不自动加入 required。
8. 输出：
   - task id。
   - 目标文件。
   - required context 路径。
   - optional context 与触发条件。
   - required checks。
   - depth。
   - 选择原因。
9. 计算 required context 文件数和字节数。
10. 超预算时退出非 0，并指出是哪一规则造成。
11. 若选择到 `archive`、`history`、`artifacts`，直接失败。
12. 若路径不存在，直接失败。
13. 不读取 Skill 正文来决定下一个 Skill，避免级联。

### 1.4 重写 `scripts/workflow/check-skill-triggers.mjs`

不要直接在旧逻辑上继续叠条件。分两步迁移：

#### 兼容提交

- 保留旧 `workflow:check` 命令名。
- 新 checker 同时读取 v1 matrix 和 v2 context map。
- 暂时允许 AGENTS 仍引用旧 Skill。
- 打印 deprecation warning，不先破坏 CI。

#### 切换提交

删除以下约束：

- `AGENTS.md` 必须直接包含每个核心 Skill 路径。
- 每个代码场景必须加载 `code-quality` 和 `project-testing`。
- Admin 文件必须无条件加载 `frontend-design`。
- Backend 文件必须无条件加载 `backend-testing`。
- wrapper 必须互相引用指定 Skill。

新增以下约束：

- 所有 context path 存在。
- context ID 唯一。
- task ID 唯一。
- required 与 optional 不冲突。
- normal task 不超过预算。
- archive/evidence 不可进入 required。
- Skill 不得互相引用其他 Skill 路径。
- Cursor adapter 不得链接超过一个 context/Skill。
- `alwaysApply: true` 总字节数不超过预算。
- docs-only 场景 required 仅为 `docs-maintenance`。
- small-admin-ui 场景 required 最多 1 个 context。
- cross-module 场景 required 最多 4 个 context；其余为 optional。
- 所有命令都存在于当前 package scripts。
- selector 对矩阵场景给出预期输出。
- 不存在 reference cycle。

### 1.5 将矩阵升级为 v2

将：

```text
tests/workflow/skill-trigger-matrix.json
```

重命名为：

```text
tests/workflow/context-routing-matrix.json
```

字段调整：

```text
expectedSkills       -> expectedRequiredContexts
forbiddenSkills      -> forbiddenRequiredContexts
expectedChecks       -> expectedChecks
depth                -> depth
reason               -> reason
```

关键场景目标：

| 场景 | required contexts | optional contexts | 说明 |
|---|---|---|---|
| documentation-only | `docs-maintenance` | 无 | 不加载代码 Skill |
| small-admin-ui | `admin-ui` | `frontend-unit` | 交互/写操作才升级 E2E |
| admin-interaction-bug | `admin-ui`, `admin-e2e` | `frontend-unit` | 不自动加载架构 |
| collector-pure-function | `collector-development` | 无 | 质量/测试由命令处理 |
| backend-service | `backend-development` | `backend-testing` | 默认一个主上下文 |
| backend-repository | `architecture-change`, `backend-development` | `backend-testing` | DB 风险时升级 |
| migration-change | `architecture-change`, `backend-development` | `api-contract`, `backend-testing` | 根据外部 shape 决定 |
| shared-type-change | `architecture-change`, `api-contract` | 前后端领域 Skill | 深度但不强制 7 个 |
| new-platform-adapter | `architecture-change`, `backend-development` | `backend-testing`, `api-contract` | 无真实平台写入 |
| cross-module-feature | 最多 4 个 | 其余 optional | 由风险合并，不是全部 8 个 |
| test-only-change | 对应测试 Skill 1 个 | `deep-code-review` | 不加载 UI 设计 |
| architecture-config-change | `architecture-change` | `deep-code-review` | 验证 checker 自身 |

### 1.6 添加 package scripts

建议：

```json
{
  "scripts": {
    "agent:context": "node scripts/workflow/select-context.mjs",
    "agent:check": "node scripts/workflow/check-agent-context.mjs",
    "workflow:check": "pnpm agent:check",
    "docs:inventory": "node scripts/docs/inventory.mjs",
    "docs:impact": "node scripts/docs/check-impact.mjs",
    "docs:check": "node scripts/docs/check-all.mjs",
    "docs:generate": "node scripts/docs/generate-all.mjs",
    "docs:generate:check": "node scripts/docs/generate-all.mjs --check"
  }
}
```

不要删除旧命令名，先用 alias 保持 CI/开发者兼容。

### 1.7 PR 1 验收

```bash
pnpm agent:context -- --intent documentation-only
pnpm agent:context -- --files "admin/src/pages/Products/DraftDetail/index.tsx"
pnpm agent:check
pnpm workflow:check
```

必须满足：

- docs-only 只返回 1 个 required context。
- small-admin-ui 最多 1 个 required context。
- 所有 v2 场景通过。
- 旧 CI 命令仍可运行。
- 尚未大改 AGENTS，因此可独立合并。

---

## PR 2：重写 AGENTS 与工具适配器

### 2.1 用下面骨架重写 `AGENTS.md`

目标内容可直接按此骨架落地，再用实际命令名校验：

```markdown
# TradeAssist Agent Entry

本文件是跨 AI 工具的唯一全局入口。代码、测试、配置和生成清单优先于说明文档。

## 1. 全局不可违反规则

- [CORE-001] 不提交或输出真实密钥、Token、Cookie、密码、平台凭证和生产数据。
- [CORE-002] 未经用户明确要求，不 commit、不 push；不覆盖用户已有改动。
- [CORE-003] 不以重构、UI 优化或文档同步为名改变 API、权限、租户边界、状态机或业务语义。
- [CORE-004] 第三方平台、AI、存储、图片和采集实现必须留在对应 Provider/adapter 边界。
- [CORE-005] 测试不得执行未拦截的真实第三方写请求，不得连接生产 DB/Redis。
- [CORE-006] 不用 skip、ignore、宽泛 allowlist 或自动扩大 baseline 掩盖失败。
- [CORE-007] 必要检查未运行时不得声明完成；必须说明未运行原因。
- [CORE-008] 当前代码事实与文档冲突时，以代码/测试/配置为准并修正文档。

## 2. 选择最小上下文

1. 确认目标文件或任务类型。
2. 运行：
   `pnpm agent:context -- --files-from-git`
   或
   `pnpm agent:context -- --intent <task-id>`
3. 只读取输出中的 `requiredContexts`。
4. 仅在输出条件命中时读取 `optionalContexts`。
5. 默认禁止读取 `docs/archive/**`、`docs/status/history/**` 和 `artifacts/**`。
6. 不通过 Skill 正文继续级联加载其他 Skill。

上下文路由真源：`config/agent/context-map.json`。
文档影响真源：`config/agent/change-impact.json`。

## 3. 工作闭环

1. 检查工作树和目标文件。
2. 用代码、测试和配置确认现状，不凭文档猜。
3. 保持改动小而聚焦。
4. 按 selector 输出执行检查。
5. 运行 `pnpm docs:impact -- --files-from-git`。
6. 只更新受影响的真源或生成文档。
7. 交付时列出：改动、检查、未验证项、风险。

## 4. 常用入口

- 人类文档导航：`docs/README.md`
- 当前能力与限制：`docs/status/current.md`
- 架构概览：`docs/architecture/overview.md`
- 生成命令索引：`docs/reference/generated/commands.generated.md`
- 贡献流程：`CONTRIBUTING.md`

## 5. 最终说明格式

- 改了什么：
- 为什么：
- 已运行检查：
- 未运行检查及原因：
- 文档影响：
- 剩余风险：
```

必须删除：

- 项目当前功能清单。
- 所有 Skill 的完整列表。
- “开始前必读 10+ 文件”表。
- 固定命令矩阵。
- 重复的文档同步清单。
- 早期 MVP 阶段范围。
- 详细 UI 五视口流程。
- 9 步以上重复工作方式。
- 当前阶段/进度链接作为必读项。

### 2.2 新建唯一 always-apply Cursor 适配器

`.cursor/rules/00-agent-router.mdc`：

```markdown
---
description: TradeAssist cross-tool agent router
alwaysApply: true
---

Read `AGENTS.md` first.
Use `pnpm agent:context -- --files-from-git` or an explicit intent.
Load only required contexts; do not load archive, history, or artifacts by default.
Do not duplicate global rules in Cursor-specific files.
```

### 2.3 创建 scoped Cursor 适配器

`admin-ui.mdc` 示例：

```markdown
---
description: Route Admin UI changes to the project UI context
globs: admin/src/**/*.{ts,tsx,less,css,scss}
alwaysApply: false
---

Use context id `admin-ui` from `config/agent/context-map.json`.
Escalate to `admin-e2e` only for interaction, route, responsive-layout, or write-flow risk.
```

其他适配器同理：

- `backend.mdc` → `backend-development`
- `collector.mdc` → `collector-development`
- `docs.mdc` → `docs-maintenance`

每个文件：

- 最多 20 行。
- 不复制规范。
- 不引用两个以上 Skill。
- 不写项目快照。
- `alwaysApply: false`。

### 2.4 处理旧 Cursor 规则

执行顺序：

1. 新 adapter 合并。
2. selector/checker 已通过。
3. 对每个旧 `.mdc` 执行 `rg`，确认仍有价值的段落已迁移。
4. 用 `git mv` 移到：
   `docs/archive/agent-rules-v1/`
5. 在 archive README 写明：
   - 冻结日期。
   - 原始用途。
   - 不再作为 Agent 规范。
   - 新入口。
6. 下一提交再更新链接和内容。
7. 不在 `.cursor/rules/` 中保留 archived `.mdc`，避免编辑器继续加载。

### 2.5 处理 `.cursorrules`

过渡内容：

```markdown
# Legacy compatibility pointer

The active project instructions are in `AGENTS.md`.
Task routing is defined in `config/agent/context-map.json`.
Do not treat this file as a separate source of truth.
```

在一个发布周期或确认所有开发工具都不依赖后：

- 删除 `.cursorrules`。
- 在迁移 PR 记录删除依据。
- 不复制 AGENTS 内容到其他工具配置。

### 2.6 可选工具适配器

若项目确实使用 GitHub Copilot，可新增 `.github/copilot-instructions.md`，但只能是 5–15 行指针。不要创建另一套规范。

### 2.7 PR 2 验收

- `AGENTS.md` ≤100 行、≤10 KiB。
- 全部 `alwaysApply: true` 文件合计 ≤120 行、≤12 KiB。
- `AGENTS.md` 不包含 `.agents/skills/...` 全列表。
- active Cursor rules 不含 P4–P9、MVP 路线图、API 清单。
- `pnpm agent:check` 通过。
- v2 routing matrix 全通过。
- `rg -n "Stage update|P[4-9]|Not Production Ready|Release Candidate" AGENTS.md .cursor/rules` 无结果。

---

## PR 3：压缩 Skills 并解除级联

### 3.1 为每个 Skill 使用统一结构

```markdown
---
name: <skill-id>
description: <一句话说明>
---

# <Skill 名称>

## 适用条件

- 明确列出触发路径或风险。
- 明确列出不适用场景。
- 不在此处触发其他 Skill。

## 输入

- 目标文件：
- 已确认的行为：
- 风险标记：
- selector 输出：

## 领域不变量

- [DOMAIN-001] 一条规则只写一次。
- [DOMAIN-002] 每条规则可由代码、测试或 ADR 验证。

## 原子执行步骤

1. 定位入口。
2. 确认现有契约。
3. 修改最小范围。
4. 添加/更新对应测试。
5. 运行 selector 指定检查。

## 升级条件

只有满足以下条件才读取 optional reference：

- ...
- ...

## 验证

- 命令来自 selector 或 generated command index。
- 不复制全项目命令矩阵。

## 输出

使用 `AGENTS.md` 的统一最终说明格式。
```

### 3.2 压缩 `code-quality`

保留：

- 检查 diff 是否意外扩大。
- 错误处理、日志脱敏、输入验证。
- 认证、权限、租户、第三方写、并发、事务、文件上传等高风险升级。
- 不扩大 baseline。
- 失败时报告首个根因。

删除/迁出：

- React/Go/SQL 的完整编码教程。
- 所有测试命令的长清单。
- 架构拆分规则。
- 每种语言的风格细节。
- 另一个最终报告模板。
- 对所有其他 Skill 的链接。

### 3.3 压缩 `modular-architecture`

保留：

- handler/service/repository/provider/queue 边界。
- shared/common 引入条件。
- 循环依赖和越层依赖。
- 新模块、adapter、worker、migration、公共 type 的触发条件。
- Architecture Baseline/Ratchet 的“不新增违规”。

迁出：

- 当前目录树。
- 当前组件名。
- DraftDetail 等具体页面历史。
- 当前平台列表。
- 大量检查命令。
- 小改动是否适用的重复解释；由 context map 决定。

### 3.4 建立项目 UI overlay

`trademind-admin-ui` 只应包含：

- 项目级页面宽度、间距、层级和响应式原则。
- 共享组件使用原则。
- loading/empty/error/readonly/submitting 状态必须清晰。
- 写操作二次确认与真实请求拦截要求。
- 根节点横向溢出检查。
- 可访问性最低要求。
- 风险分级视口矩阵。

建议视口分级：

| 风险 | 必测视口 |
|---|---|
| 纯文案/图标/局部状态且不改布局 | 当前桌面 + 一个窄屏 |
| 布局、表格、Modal/Drawer、Tabs、Toolbar | 桌面、平板、手机，共 3 档 |
| shared 组件、全局布局、关键写路径 | 现有 5 档 |
| 无 DOM/样式变化 | 不触发视觉视口验收 |

把以下内容移到 `docs/reference/admin-ui-system.md`：

- 当前共享组件清单。
- 每个组件 API 和示例。
- 当前 layout token。
- 页面模板。
- 设计 rationale。
- 截图或视觉示例。

把精确 E2E 路径/页面清单放测试配置，不放 Skill。

### 3.5 压缩测试 Skills

统一原则：

- Skill 说明“何时、测什么、禁止什么”。
- `test:affected` 决定实际命令。
- 端点/页面/文件清单不在 Skill 硬编码。
- E2E 默认拦截所有非 GET；真实写测试必须由人工受控环境单独执行。
- 不重复 `AGENTS.md` 的 secrets/commit/push 禁令。
- 失败只报告首个根因与未覆盖风险。

### 3.6 删除 Skill 间直接引用

执行：

```bash
rg -n "\.agents/skills/.+/SKILL\.md" .agents/skills
```

目标：

- 除了明确的非必读参考说明外，结果为 0。
- 更严格的做法是在 checker 中禁止任何 Skill 正文出现其他 Skill 路径。
- 组合由 `context-map.json` 完成。

### 3.7 PR 3 验收

- 每个自动加载 Skill ≤160 行、≤16 KiB。
- 上游 `frontend-design` 不进入 small-admin-ui required context。
- small-admin-ui required context 总量 ≤32 KiB。
- docs-only 不引用任何代码 Skill。
- Skill 内无“必须再读全部其他 Skill”。
- Skill 内无 P 阶段号、测试通过数量、当前分支。
- `pnpm agent:check` 通过。

---

## PR 4：建立人类文档信息架构

### 4.1 先创建目录

```bash
mkdir -p docs/guides/contributing
mkdir -p docs/reference/configuration
mkdir -p docs/reference/api
mkdir -p docs/reference/providers/platforms
mkdir -p docs/reference/generated
mkdir -p docs/architecture/adr
mkdir -p docs/runbooks/collector
mkdir -p docs/runbooks/performance
mkdir -p docs/status/history
mkdir -p docs/archive/agent-rules-v1
mkdir -p docs/archive/phases
mkdir -p docs/archive/progress
mkdir -p docs/archive/tool-memory
```

### 4.2 只做 `git mv` 的提交

示例；实际文件存在性必须先检查：

```bash
git mv docs/development.md docs/guides/development.md
git mv docs/branching.md docs/guides/contributing/branching.md
git mv docs/architecture.md docs/architecture/overview.md
git mv docs/env.md docs/reference/configuration/environment.md
git mv docs/provider.md docs/reference/providers/overview.md
git mv docs/ai-workflow.md docs/guides/ai-assisted-development.md
git mv docs/PROGRESS.md docs/archive/progress/PROGRESS-legacy-2026-08-05.md
```

要求：

- 该提交只移动，不重写大段内容。
- 用 `git diff --summary` 确认 Git 识别为 rename。
- 不在同一提交批量格式化。

### 4.3 单独提交修复链接

执行：

```bash
rg -n "docs/(development|branching|architecture|env|provider|ai-workflow|PROGRESS)\.md" .
```

逐个更新链接。禁止用不受控的全仓替换。

### 4.4 重写 `docs/README.md`

目标不超过 100 行，推荐结构：

```markdown
# TradeMind 文档

## 用户
- 快速开始
- 部署
- 当前能力与限制

## 贡献者
- 开发环境
- 架构概览
- 分支与 PR
- 测试与验证

## 运维
- 配置与环境变量
- 本地/容器运行
- 备份、恢复、发布 Runbook

## 维护者
- API 约定
- Provider 参考
- ADR
- Agent 文档维护

## 历史资料
历史阶段报告和旧规则位于 `docs/archive/`，不代表当前行为。
```

禁止：

- 列出所有 P4–P9 文件。
- 把 screenshots/runs/fingerprints 当核心入口。
- 把历史验收报告标成“必读”。
- 复制 README 的产品介绍。

### 4.5 为文档添加统一 frontmatter

除根 README、许可证、NOTICE 外，current 文档建议使用：

```yaml
---
doc_type: guide
audience: contributor
status: current
owner: <team-or-maintainer>
source_of_truth:
  - package.json
review_cycle_days: 90
last_verified_commit: <commit>
---
```

允许的 `doc_type`：

- `guide`
- `reference`
- `runbook`
- `status`
- `adr`
- `archive`
- `generated`

允许的 `status`：

- `current`
- `deprecated`
- `historical`
- `generated`

规则：

- `generated` 文件顶部必须写“不要手工编辑”。
- `historical` 文件必须链接到当前替代文档。
- `current` 文件必须有 owner 和 truth source。
- archive 不参与过期检查。
- `last_verified_commit` 不得填审阅基线后长期不更新；实施时填真实验证 commit。

### 4.6 PR 4 验收

- 所有新路径链接有效。
- `docs/README.md` 不再列阶段报告。
- archive README 明确“不代表当前行为”。
- Git rename 历史可追踪。
- `pnpm docs:check` 初版通过。
- README/AGENTS/CONTRIBUTING 没有断链。

---

## PR 5：重写核心人类文档

### 5.1 替换 `docs/PROGRESS.md`

创建 `docs/status/current.md`，只保留：

1. 项目一句话定位。
2. 当前已验证能力矩阵。
3. 当前已知限制。
4. 生产使用前必须人工验证的事项。
5. 当前优先级或开放决策。
6. 每项声明的代码/测试/issue 来源。

模板：

```markdown
---
doc_type: status
audience: maintainer
status: current
owner: <owner>
review_cycle_days: 30
last_verified_commit: <commit>
---

# 当前能力与限制

## 能力矩阵

| 领域 | 当前能力 | 主要限制 | 事实来源 |
|---|---|---|---|
| 商品采集 | ... | ... | `collector/...`, tests |
| 商品草稿 | ... | ... | `backend/...`, `admin/...` |
| AI 内容 | ... | ... | ... |
| Ozon 刊登 | ... | 未完成真实受控验收时明确写出 | ... |
| 订单与库存 | ... | ... | ... |

## 当前高风险限制

- ...

## 下一步决策

- ...
```

禁止写入：

- `Stage update`。
- 每次测试的 105/105、13/13 等数量。
- 当前 worktree、分支、是否 commit/push。
- 某一天的故障排查流水。
- 一次性环境问题。
- 数百行时间线。

旧 `PROGRESS`：

- 冻结。
- 顶部加 historical banner。
- 移入 archive。
- 后续不再追加。

历史记录优先放：

- GitHub PR。
- release notes / `CHANGELOG.md`。
- 必要时按月放 `docs/status/history/YYYY-MM.md`，每条仅 1–3 行并链接 PR。

### 5.2 重写 README / README.en

删除当前 `Release Status` 表中的内部条目：

- Phase。
- P5/P6/P7/P10。
- Tag deferred。
- Release Candidate。
- Final acceptance deferred。
- 内部 closure/gate 术语。

替换为公开可理解的成熟度说明，例如：

```markdown
## 项目成熟度

TradeMind 仍处于快速演进阶段。自托管、二次开发和测试环境使用是当前主要场景。
连接真实店铺、执行商品刊登、库存同步或其他外部写操作前，请在隔离店铺和小批量数据上完成受控验证。
详细的当前能力与已知限制见 `docs/status/current.md`。
```

README 目标结构：

1. 产品一句话。
2. 主要使用场景。
3. 当前稳定能力。
4. 截图。
5. 架构概览。
6. 5–8 步快速开始。
7. 常用稳定命令。
8. 文档导航。
9. 安全/成熟度提醒。
10. 贡献与许可证。

禁止：

- 任何 P 阶段号。
- 每日进度。
- 临时分支或测试证据。
- 任意选取一个 `check:p4-r` 作为“常用命令”。
- 过长的完整端点、环境变量、Provider 列表。

### 5.3 拆分 API 文档

目标：

```text
docs/reference/api/
├── conventions.md
├── authentication.md
├── errors.md
├── routes.generated.md
└── platforms/
```

`conventions.md` 手工维护：

- `/api/v1` 基础路径。
- envelope。
- trace id。
- pagination 总原则。
- fail-closed。
- tenant 来源规则。
- 幂等与错误分类。

`authentication.md`：

- Bearer/JWT。
- tenant/admin 边界。
- OAuth state。
- 不返回 secrets。
- 过期/撤销语义。

`routes.generated.md`：

- 从实际 Gin routes 或 OpenAPI 生成。
- 不手工编辑。
- 每行包括 method、path、handler/module、auth classification。
- 不写 P5/P7 等阶段说明。

平台特殊语义：

- `providers/platforms/douyin.md`
- `providers/platforms/ozon.md`

历史发布 gate 和阶段结论：

- 移到 archive。
- 不混入 API 参考。

#### API 生成策略

先搜索是否已有 OpenAPI/Swagger：

```bash
rg -n "openapi|swagger|gin-swagger|Routes\(\)" backend docs scripts
```

分支处理：

- 已有 OpenAPI：以 OpenAPI 为真源，生成 Markdown。
- 无 OpenAPI：先实现只读 route snapshot，不强制本 PR 完成完整 schema。
- 不要用正则直接解析 Go 源码作为长期唯一方案；优先从 router 实例或明确 route registry 生成。
- 在 CI 中比较生成结果，发现 diff 即失败。

### 5.4 拆分 Provider 文档

`overview.md` 只讲：

- Provider 边界。
- 注册、配置、连接测试、错误映射、脱敏。
- 核心业务不得依赖第三方 SDK。
- provider capability 与实现状态的表达方法。

按领域拆分：

- AI。
- Image。
- Storage。
- Collector。
- Platform。

按平台拆分：

- Douyin。
- Ozon。
- 后续平台。

每个具体平台参考必须包括：

- 认证方式。
- 只读/写接口边界。
- 幂等。
- retryable / terminal / uncertain 结果。
- tenant/shop 归属。
- 测试 fake/server。
- 真实写入安全要求。
- 代码入口。

禁止在 overview 中继续追加“Phase 3–10.4”的时间线。

### 5.5 更新架构文档

`docs/architecture/overview.md` 必须从现行代码确认并覆盖：

- backend。
- admin。
- collector。
- browser-extension。
- PostgreSQL / Redis。
- Provider / adapter。
- 统一刊登中心与平台适配边界。
- API contract tests。
- architecture ratchet。
- artifacts / generated docs 的边界。
- 租户上下文来源。
- 写操作安全边界。

不要列“推荐目录”；列“当前目录”，并注明由 generated module map 补充。

重大设计原因写 ADR，例如：

- Provider 隔离。
- 测试禁止真实平台写入。
- 不可变刊登快照。
- tenant fail-closed。
- 文档真源与生成策略。

### 5.6 清理 development/env/branching/contributing

#### Development

保留：

- 前置依赖。
- 安装。
- 启动。
- 稳定 test/build/lint 命令。
- 常见本地问题。
- Windows 特别说明。

移出：

- P7/P9 性能闭环。
- 一次性恢复演练。
- 某个阶段证据生成。
- 临时容器名称。
- 某次 worktree 故障。

#### Environment

建议建立 `config/env.schema.json`：

```json
{
  "DATABASE_URL": {
    "scope": ["backend"],
    "required": true,
    "secret": true,
    "example": "postgres://...",
    "description": "..."
  }
}
```

生成：

- `.env.example`。
- `.env.docker.example` 的公共部分。
- `environment.generated.md`。

若暂时无法 schema 化：

- 先写比较脚本。
- 确保 docs 中变量必须存在于 example/config。
- 将 P7-V2 等测试专用变量移到性能 runbook。

#### Branching / CONTRIBUTING

- 去掉 P9 等专项命令。
- `CONTRIBUTING.md` 链接到 branching、verification、architecture。
- 不复制 AGENTS 全局规则。
- 不复制全部 test matrix。
- 保留 no secrets、PR、测试说明的贡献者版本即可。

#### PR 模板

替换为：

```markdown
## 变更类型

- [ ] 功能
- [ ] Bug 修复
- [ ] 重构
- [ ] 文档
- [ ] CI / 构建

## 影响范围

- 目标文件：
- `pnpm agent:context` 输出的 task/context：
- `pnpm docs:impact` 输出：

## 验证

- 已运行：
- 未运行及原因：

## 安全

- [ ] 未提交 secrets
- [ ] 未连接生产 DB/Redis
- [ ] 未执行真实第三方写请求，或已说明受控验证过程
- [ ] 未扩大 baseline

## 文档

- [ ] 真源/生成文档已更新
- [ ] 无文档影响
```

删除“必须同步 AGENTS、ai-workflow、ai-coding-rules、.cursorrules、Cursor README、Cursor usage”等多文件复制要求。

### 5.7 PR 5 验收

```bash
rg -n "Stage update|P[4-9](?:[-_A-Za-z0-9]*)?|Tag deferred|Release Candidate|Production Ready:" \
  README.md README.en.md AGENTS.md CONTRIBUTING.md docs/guides docs/reference docs/status/current.md
```

期望无结果；确有合法产品名时加精确 allowlist，不要宽泛忽略。

还必须：

- README 快速开始从干净环境可按步骤执行，或明确未实测。
- 当前状态每个关键声明有代码/测试来源。
- API 路由清单不再手工维护于 Agent 规则。
- Provider overview 不含阶段时间线。
- development 不再把性能阶段脚本当常用命令。

---

## PR 6：生成器、陈旧检测和 CI

### 6.1 新增链接检查

`scripts/docs/check-links.mjs`：

- 检查相对 Markdown 链接。
- 检查 anchor。
- 忽略 archive 中明确冻结的外部旧链接时必须有 allowlist。
- 检查大小写，保证 Linux CI 可用。
- 不进行无限网络抓取；外链检查可单独计划运行。

### 6.2 新增陈旧声明检查

`scripts/docs/check-stale-claims.mjs` 默认检查：

```text
AGENTS.md
README.md
README.en.md
CONTRIBUTING.md
docs/README.md
docs/guides/**
docs/reference/**
docs/architecture/**
docs/status/current.md
.cursor/rules/**
.agents/skills/**
```

默认禁止：

- `Stage update`
- `P4`–`P99` 阶段标记
- `Tag deferred`
- `Release Candidate`
- `Production Ready:`
- `codex/`
- `origin/main`
- `working tree`
- “未 commit、未 push”之类执行日志
- archive 文件作为“必读”
- active agent 文件中的完整 `/api/v1/...` 清单
- active agent 文件中的一次性测试通过数量

允许：

- archive。
- ADR 中作为历史引用的明确代码块。
- 必须有精确路径/行级 allowlist 和原因。

### 6.3 新增所有权检查

`scripts/docs/check-ownership.mjs`：

- current 文档必须有 `doc_type`、`status`、`owner`、`source_of_truth`。
- generated 文档必须声明 generator。
- historical 文档必须指向 replacement。
- review cycle 过期：
  - 核心文档直接失败。
  - 普通 guide 初期 warning，稳定后再 fail。
- `last_verified_commit` 必须是仓库可解析 commit，或由生成器写入 source hash。

### 6.4 生成命令索引

`scripts/docs/generate-command-reference.mjs`：

输入：

- 根 `package.json`
- `admin/package.json`
- `collector/package.json`
- 其他 workspace package

输出：

- 稳定开发命令。
- build/test/quality/architecture 命令。
- phase/evidence/legacy 命令单独分组。
- 每项标明 package、用途、是否 CI 使用。
- 不把 PowerShell-only 命令展示为跨平台默认。

建议为 package script 增加机器元数据，或维护 `scripts/catalog.json`：

```json
{
  "check:p4-r": {
    "category": "legacy-evidence",
    "document": "docs/archive/phases/p4/README.md"
  },
  "test:affected": {
    "category": "stable",
    "document": "docs/runbooks/testing.md"
  }
}
```

长期目标：

- 稳定命令使用稳定名称。
- 阶段命令逐步前缀为 `legacy:` / `evidence:`，但重命名必须另开兼容 PR。
- README 只展示 stable 分类。

### 6.5 生成模块影响图

`scripts/docs/generate-module-map.mjs` 输入：

- `config/agent/change-impact.json`
- `tests/architecture/module-boundaries.json`
- workspace package 信息
- 关键 route/provider registry

输出：

```text
docs/reference/generated/module-map.generated.md
```

原 `docs/module-map.md`：

- 先改成 redirect。
- 一个发布周期后删除。
- 不再手工堆几十个 Ozon 文件路径和实现细节。

### 6.6 环境变量校验/生成

至少实现：

- 每个 docs current env 变量存在于 schema/example/config。
- 每个 required config 变量出现在 docs 或明确 internal。
- secret 变量没有真实值。
- `.env.example` 与 `.env.docker.example` 的差异有解释。
- benchmark/test harness 变量不会出现在普通 quick start。

### 6.7 API route 生成/校验

最低可接受版本：

- 通过 Go route registry / router instance 获取 method/path。
- 生成 JSON snapshot 和 Markdown。
- CI 对 snapshot diff 失败。
- 新路由若无 auth classification 则失败。
- 删除路由必须产生可审阅 diff。

完整 OpenAPI 可作为后续独立工作，不阻塞本轮瘦身。

### 6.8 `docs:check` 聚合

`docs:check` 应依次运行：

1. context/schema validation。
2. link check。
3. frontmatter/ownership。
4. stale claims。
5. generated files `--check`。
6. secret scan。
7. Agent context budgets。
8. orphan current docs 检查。

不要让 `docs:check` 运行完整 Admin E2E 或后端集成测试。

### 6.9 更新 CI

在 `project-tests.yml` 或独立 `docs.yml` 中增加：

```yaml
- run: pnpm install --frozen-lockfile
- run: pnpm agent:check
- run: pnpm docs:check
- run: pnpm docs:generate:check
```

CI 要求：

- 只读检查。
- 生成器在 `--check` 下不修改工作树。
- 发现生成差异打印需要执行的命令。
- 不自动 commit 生成文件。
- archive 不触发 current 文档过期检查。
- docs-only PR 不运行无关重型 E2E。

### 6.10 PR 6 验收

- 故意改一个 package script，`docs:generate:check` 能发现 command index 漂移。
- 故意添加断链，link check 失败。
- 故意在 README 写 `P9`，stale claim check 失败。
- 故意把 archive 加到 required context，agent check 失败。
- 故意让 small UI 加载 5 个 context，budget check 失败。
- 恢复这些故意改动后全绿。

---

## PR 7：归档阶段报告与清理证据

### 7.1 分类规则

对 `docs/` 根目录每个文件逐一分类，不能仅凭文件名批量移动。

#### 归档到 `docs/archive/phases/<phase>/`

适合：

- `P4_*`、`P5_*`、`P6_*`、`P7_*`、`P8_*`、`P9_*`。
- closure、acceptance、final audit、stage report。
- 已完成阶段的叙事性 Markdown。
- 有长期追溯价值的决策过程，但不是当前 ADR。

#### 移到 CI artifact / 根 `artifacts/`

适合：

- JSON 运行输出。
- 截图。
- benchmark 结果。
- fingerprints。
- runs / regressions / baselines / currents。
- 浏览器 trace、视频、日志。

#### 保留为 current

只有同时满足：

- 被当前用户/开发/运维流程使用。
- 内容已与代码校验。
- 有 owner、truth source 和 review cycle。
- 不依赖阶段编号表达有效性。

### 7.2 每个文件的原子判断步骤

对每个候选文件执行：

1. `rg -n "<basename>|<relative-path>" .`
2. 检查是否被：
   - package script。
   - workflow。
   - README。
   - docs hub。
   - Agent rule。
   - test fixture。
3. 打开前 80 行和最后 40 行。
4. 判断是 current、history 还是 evidence。
5. 若被可执行脚本使用：
   - 先改脚本输出位置。
   - 再移动文件。
6. 若仅被旧文档引用：
   - 移动。
   - 更新链接。
7. 若无引用且是原始运行产物：
   - 确认 GitHub/CI artifact 是否已有替代。
   - 再决定删除或归档。
8. 用 `git mv` 保留历史。
9. 在 archive README 写索引，不把每个文件放回 docs 主导航。
10. 运行 link、workflow 和相关专项检查。

### 7.3 处理 `.workbuddy/memory`

1. 确认没有脚本或 Agent adapter读取。
2. 若只是日期记忆：
   - 有历史价值：`git mv` 到 `docs/archive/tool-memory/workbuddy/`。
   - 无历史价值：删除并在 `.gitignore` 加工具本地记忆路径。
3. 在 AGENTS/context map 中明确 archive/tool-memory 不加载。
4. 不把其他 AI 工具临时 scratchpad 继续提交进仓库。

### 7.4 `artifacts/` 保留策略

新增 `artifacts/README.md`：

- 什么可以入 Git。
- 什么只能作为 CI artifact。
- 保留时长。
- 是否含敏感信息。
- 命名规则。
- 生成命令。
- 不作为当前产品文档。

建议：

- schema、空目录 README、少量可复现基准可提交。
- 原始日志、trace、视频、批量截图默认不提交。
- GitHub Actions 设置 retention days。
- 含真实店铺、token、商品或租户数据的 artifact 禁止提交。

### 7.5 PR 7 验收

- `docs/` 根目录只保留导航和少量真正核心入口。
- docs hub 不列所有 archive 文件。
- package scripts/workflow 不再依赖旧 docs 输出路径。
- archive 文件顶部有 historical banner。
- raw evidence 不再污染 current reference。
- 所有链接检查通过。
- Git diff 不包含未经解释的大规模删除。

---

# 7. 段落级迁移算法

Agent 处理每个旧文档时，必须对每个标题/段落执行以下决策，不允许整篇复制到新路径。

## 7.1 第一步：判断内容类型

| 问题 | 是 | 否 |
|---|---|---|
| 这是所有任务都必须遵守的安全/权限规则吗？ | 放 AGENTS，赋 CORE ID | 继续 |
| 这是某领域执行步骤吗？ | 放对应 Skill | 继续 |
| 这是当前代码结构、端点、变量或命令清单吗？ | 生成或 reference | 继续 |
| 这是“为什么这样设计”吗？ | ADR | 继续 |
| 这是开发者操作步骤吗？ | guide/runbook | 继续 |
| 这是当前能力/限制吗？ | status/current | 继续 |
| 这是某次阶段、测试或故障历史吗？ | archive/history/evidence | 删除重复 |

## 7.2 第二步：找真源

每个 current 段落至少写出一个真源：

- 代码文件。
- config/schema。
- package script。
- workflow。
- test。
- ADR。

没有真源：

- 不得写成确定事实。
- 标为 `unverified`。
- 创建维护 issue 或移入历史。

## 7.3 第三步：移动而不是复制

- 新位置完整定义。
- 旧位置在过渡期只放 redirect。
- 其他文档链接新位置。
- 禁止保留两个可编辑副本。

## 7.4 第四步：去除快照型内容

从 Agent 文件移除：

- 当前支持的平台列表。
- 当前 API 路由列表。
- 当前目录树。
- 当前 UI 组件清单。
- 当前测试数量。
- 当前阶段。
- 当前分支。
- 当前端口之外的临时环境细节。

稳定端口或协议确属长期约束时，应放配置/reference，并由测试校验。

## 7.5 第五步：验证

每迁移一个文档：

```bash
pnpm docs:check
pnpm agent:check
git diff --check
```

涉及命令、env、API、模块图时再执行对应 generator check。

---

# 8. 推荐的新文档模板

## 8.1 Human guide

```markdown
---
doc_type: guide
audience: contributor
status: current
owner: <owner>
source_of_truth:
  - <code-or-config-path>
review_cycle_days: 90
last_verified_commit: <commit>
---

# 标题

## 目的

读完后能完成什么。

## 前置条件

只列必要条件。

## 步骤

1. 原子步骤。
2. 每步给验证结果。
3. 不复制完整命令索引，链接 generated reference。

## 常见问题

只保留可重复、当前有效的问题。

## 事实来源

- `...`
```

## 8.2 Reference

```markdown
---
doc_type: reference
audience: developer
status: current
owner: <owner>
source_of_truth:
  - <path>
review_cycle_days: 60
last_verified_commit: <commit>
---

# 标题

## 稳定约定

## 当前接口/结构

由 generated 内容或明确代码入口支持。

## 兼容性与限制

## 变更要求

改动哪类代码时必须更新/生成什么。
```

## 8.3 Runbook

```markdown
---
doc_type: runbook
audience: operator
status: current
owner: <owner>
source_of_truth:
  - <workflow-or-script>
review_cycle_days: 60
last_verified_commit: <commit>
---

# 操作名称

## 适用场景

## 风险

## 前置检查

## 执行步骤

每一步包含命令、预期结果、失败停止条件。

## 回滚

## 验证

## 证据保存位置
```

## 8.4 ADR

```markdown
---
doc_type: adr
status: current
owner: <owner>
decision_date: YYYY-MM-DD
---

# ADR-NNN：决策标题

## 背景

## 决策

## 备选方案

## 后果

## 代码落实位置

## 重新评估条件
```

## 8.5 Historical banner

```markdown
> [!WARNING]
> 本文件是历史记录，冻结于 YYYY-MM-DD，不代表当前系统行为。
> 当前文档：`<replacement path>`。
```

## 8.6 Generated banner

```markdown
> [!IMPORTANT]
> 本文件由 `<generator command>` 生成。不要手工编辑。
> 真源：`<source paths>`。
```

---

# 9. CI 和质量门禁的具体验收标准

## 9.1 上下文预算

| 场景 | 最大 required 文件 | 最大 required 字节 | 禁止项 |
|---|---:|---:|---|
| docs-only | 1 | 16 KiB | 代码/测试/UI Skill |
| 小型 Admin UI | 1 | 32 KiB | architecture、backend、完整 upstream design |
| Admin 交互/写路径 | 2–3 | 48 KiB | 无条件全 5 视口 |
| 后端普通 service | 1–2 | 32 KiB | UI/E2E |
| API/DTO | 2–3 | 48 KiB | 无关 collector/UI design |
| migration/worker/adapter | 2–4 | 64 KiB | 真实平台写 |
| 跨模块 | ≤6 | 70 KiB | 所有 Skill 无条件全载 |
| always loaded | — | 12 KiB | 项目状态、API 清单、历史 |

## 9.2 内容门禁

Active Agent 文档必须：

- 0 个内部阶段号。
- 0 个 Stage update。
- 0 个完整 endpoint inventory。
- 0 个 Skill 级联必读链。
- 1 个最终说明格式。
- 1 个全局规则所有者。
- 1 个 routing truth source。

Stable human docs 必须：

- 0 个工作树/branch 流水。
- 0 个一次性测试结果堆积。
- 0 个历史报告作为必读。
- 每个 current 文档有 owner 和 truth source。
- 生成内容无未提交 diff。

## 9.3 功能不回归

文档重构 PR 必须证明：

- 现有应用代码没有语义改动。
- 现有 test/quality/architecture script 名保持兼容或提供 alias。
- CI 不自动扩大 baseline。
- 无真实第三方写入。
- secrets scan 通过。
- 路由/环境/命令生成器为只读校验。

---

# 10. 建议的提交粒度

每个 PR 内仍应拆小提交：

```text
1. chore(docs): add context config schema
2. feat(workflow): add context selector
3. test(workflow): add v2 routing matrix
4. refactor(workflow): switch trigger checker to manifest
5. docs(agent): rewrite AGENTS router
6. docs(cursor): replace active rules with thin adapters
7. chore(archive): move legacy agent rules
```

文件移动类 PR：

```text
1. chore(docs): create target directories
2. chore(docs): move current guides without content edits
3. docs: update links after moves
4. docs: rewrite navigation
```

禁止：

- 一个提交同时改 100 个文件、重写内容、重命名和修脚本。
- 用“format docs”掩盖语义变化。
- 在 archive 移动中顺带改产品代码。
- 将生成文件和生成器分在不可独立验证的提交中。

---

# 11. 回滚策略

每个阶段都必须可独立回滚。

## PR 1 回滚

- 保留旧 `workflow:check` alias。
- v2 selector 失败时可切回 v1 checker。
- 不先删除旧 matrix。

## PR 2 回滚

- 旧 Cursor rules 已在 archive，可 `git mv` 恢复。
- `.cursorrules` 过渡指针保留一个周期。
- AGENTS 旧版可由单独 commit revert。

## PR 4/5 回滚

- 文件移动与内容重写分开，能只回滚重写而保留目录结构。
- 所有 redirect 保留至链接检查稳定。
- 不在同一 PR 删除旧文件和所有历史。

## 生成器回滚

- generator 与生成文件同 PR。
- `--check` 不写磁盘。
- 生成器异常时 current reference 仍可读取，但 PR 必须标注临时人工维护状态。

---

# 12. 执行时绝对不要做的事

1. 不要只改 `AGENTS.md` 而不改 `check-skill-triggers.mjs`。
2. 不要把 622 行 UI Skill直接复制到另一个文件。
3. 不要用新的“总规范.md”替代旧的多个大文件。
4. 不要把所有规则塞进 `context-map.json` 的长字符串。
5. 不要把 current status 再写成新的逐日流水账。
6. 不要简单把 `P7` 替换为 `P10`，应移除阶段语义。
7. 不要删除历史文件而丢失许可证、审计或安全证据。
8. 不要在 README 里放内部 closure/gate 表。
9. 不要手工维护可从代码生成的完整 API、命令、env 清单。
10. 不要让 docs-only PR 跑完整 Admin E2E。
11. 不要为跨平台兼容在通用文档写 `pnpm.cmd`；统一用 `pnpm`。
12. 不要因为旧规则说“未实现”就回退当前 Ozon、订单、库存、租户或图片能力描述。
13. 不要删除 `NOTICE` 或来源归属，除非完成许可证核对。
14. 不要自动 commit、push、创建 tag。
15. 不要在未拦截情况下测试真实第三方写接口。

---

# 13. 最终 Definition of Done

全部 PR 完成后，必须同时满足：

## Agent 体系

- [ ] `AGENTS.md` ≤100 行、≤10 KiB。
- [ ] 只有一个跨工具全局入口。
- [ ] 只有一个 `alwaysApply` Cursor adapter，或全部 always-apply 合计 ≤12 KiB。
- [ ] `context-map.json` 是唯一任务路由真源。
- [ ] `change-impact.json` 是唯一文档影响真源。
- [ ] docs-only required context 只有 `docs-maintenance`。
- [ ] small-admin-ui required context 最多 1 个。
- [ ] 普通后端 service required context 最多 2 个。
- [ ] Skill 不互相级联。
- [ ] 每个自动加载 Skill ≤160 行。
- [ ] locked `frontend-design` 已按 lock 策略处理。
- [ ] active Agent 文件没有内部阶段、API 清单和项目快照。
- [ ] selector/checker/matrix 在 CI 中通过。

## 人类文档

- [ ] README 无 P4–P10、closure、tag deferred、release candidate 等内部状态。
- [ ] `docs/README.md` ≤100 行并按受众导航。
- [ ] `docs/status/current.md` ≤120 行。
- [ ] 旧 `PROGRESS` 已冻结归档，不再追加。
- [ ] API conventions 与 generated routes 分开。
- [ ] Provider overview 与平台实现/历史分开。
- [ ] architecture 覆盖当前主要组件和边界。
- [ ] development 只展示稳定命令。
- [ ] env 与 example/config 可校验。
- [ ] module map、command index 至少一项已生成，其余有明确迁移 issue。
- [ ] archive 不在默认导航和 Agent 上下文。
- [ ] tool memory 不在 active path。
- [ ] README 中英文语义一致。
- [ ] terminology 明确 TradeAssist / TradeMind 用法。

## CI

- [ ] `pnpm agent:check` 通过。
- [ ] `pnpm docs:check` 通过。
- [ ] `pnpm docs:generate:check` 通过。
- [ ] 断链、陈旧声明、上下文超预算可被测试证明会失败。
- [ ] docs-only 路径不会触发无关重型测试。
- [ ] 无 baseline 扩大。
- [ ] 无 secrets。
- [ ] 无真实第三方写入。

---

# 14. 可直接交给 Agent 的总提示词

下面内容可复制给执行 Agent。建议每次只指定一个 PR，不要一次执行全部。

```text
你正在重构 123wusongzhi/tradeassist 的 Agent 说明与人类文档体系。

审阅基线是 main@b453069，但执行时必须先读取当前 HEAD、git status、代码、测试、package scripts 和 workflow，以当前代码事实为准。不要用旧文档反向修改现行业务。

先阅读《TradeAssist / TradeMind 文档与 Agent 上下文重构执行手册》，本次只执行其中的【PR <编号>：<名称>】。不要越界到后续 PR。

硬约束：
1. 不覆盖用户已有修改。
2. 未经要求不 commit、不 push。
3. 不改变 API、payload、权限、租户、状态机、数据库 schema、Provider 行为或 UI 业务语义。
4. 不连接生产 DB/Redis，不执行真实第三方平台写请求。
5. 不扩大任何 baseline，不用 skip/ignore 掩盖失败。
6. 先建立/使用机器路由，再删除旧规则。
7. 大规模 git mv 与内容重写分成不同提交级 diff。
8. 当前事实与文档冲突时，以代码、测试、配置为准。
9. 不确定事实标记 unverified，不猜。
10. 只运行与本 PR 相关的最小检查，并说明未运行项。

执行流程：
A. 输出当前 HEAD、工作树状态、目标文件。
B. 读取本 PR 所需最小上下文。
C. 列出 3–7 个原子步骤和每步验收。
D. 实施最小 diff。
E. 运行本手册指定检查。
F. 检查链接、敏感信息、上下文预算和生成文件。
G. 最终报告：
   - 改了什么
   - 为什么
   - 已运行检查及结果
   - 未运行检查及原因
   - 文档影响
   - 剩余风险
   - 下一 PR 的明确入口

本次目标：
<粘贴对应 PR 的步骤和验收条件>
```

---

# 15. 审阅依据索引

以下是本次判断的主要代码/文档入口，实施时应再次打开当前版本：

- [`AGENTS.md`](https://github.com/123wusongzhi/tradeassist/blob/main/AGENTS.md)
- [`.cursor/rules/README.md`](https://github.com/123wusongzhi/tradeassist/blob/main/.cursor/rules/README.md)
- [`.cursor/rules/00-project-overview.mdc`](https://github.com/123wusongzhi/tradeassist/blob/main/.cursor/rules/00-project-overview.mdc)
- [`.cursor/rules/01-architecture.mdc`](https://github.com/123wusongzhi/tradeassist/blob/main/.cursor/rules/01-architecture.mdc)
- [`.cursor/rules/05-ai-provider.mdc`](https://github.com/123wusongzhi/tradeassist/blob/main/.cursor/rules/05-ai-provider.mdc)
- [`.cursor/rules/07-collector-node-playwright.mdc`](https://github.com/123wusongzhi/tradeassist/blob/main/.cursor/rules/07-collector-node-playwright.mdc)
- [`.cursor/rules/08-api-db-security.mdc`](https://github.com/123wusongzhi/tradeassist/blob/main/.cursor/rules/08-api-db-security.mdc)
- [`.cursor/rules/09-dev-workflow.mdc`](https://github.com/123wusongzhi/tradeassist/blob/main/.cursor/rules/09-dev-workflow.mdc)
- [`.cursor/rules/15-external-docs-no-phase-status.mdc`](https://github.com/123wusongzhi/tradeassist/blob/main/.cursor/rules/15-external-docs-no-phase-status.mdc)
- [`.agents/skills/frontend-design/SKILL.md`](https://github.com/123wusongzhi/tradeassist/blob/main/.agents/skills/frontend-design/SKILL.md)
- [`skills-lock.json`](https://github.com/123wusongzhi/tradeassist/blob/main/skills-lock.json)
- [`scripts/workflow/check-skill-triggers.mjs`](https://github.com/123wusongzhi/tradeassist/blob/main/scripts/workflow/check-skill-triggers.mjs)
- [`tests/workflow/skill-trigger-matrix.json`](https://github.com/123wusongzhi/tradeassist/blob/main/tests/workflow/skill-trigger-matrix.json)
- [`docs/ai-workflow.md`](https://github.com/123wusongzhi/tradeassist/blob/main/docs/ai-workflow.md)
- [`docs/PROGRESS.md`](https://github.com/123wusongzhi/tradeassist/blob/main/docs/PROGRESS.md)
- [`docs/api.md`](https://github.com/123wusongzhi/tradeassist/blob/main/docs/api.md)
- [`docs/provider.md`](https://github.com/123wusongzhi/tradeassist/blob/main/docs/provider.md)
- [`docs/development.md`](https://github.com/123wusongzhi/tradeassist/blob/main/docs/development.md)
- [`docs/env.md`](https://github.com/123wusongzhi/tradeassist/blob/main/docs/env.md)
- [`docs/module-map.md`](https://github.com/123wusongzhi/tradeassist/blob/main/docs/module-map.md)
- [`docs/task-checklist.md`](https://github.com/123wusongzhi/tradeassist/blob/main/docs/task-checklist.md)
- [`README.md`](https://github.com/123wusongzhi/tradeassist/blob/main/README.md)
- [`package.json`](https://github.com/123wusongzhi/tradeassist/blob/main/package.json)
- [`CONTRIBUTING.md`](https://github.com/123wusongzhi/tradeassist/blob/main/CONTRIBUTING.md)
- [PR template](https://github.com/123wusongzhi/tradeassist/blob/main/.github/pull_request_template.md)

---

# 16. 一句话执行原则

> **把“必须长期遵守的少量不变量”留给 Agent，把“当前事实”交给代码和生成文档，把“为什么”交给 ADR，把“怎么操作”交给 Guide/Runbook，把“曾经发生过什么”移到 Archive。**
