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
2. 运行：`pnpm agent:context -- --files-from-git` 或 `pnpm agent:context -- --intent <task-id>`
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
