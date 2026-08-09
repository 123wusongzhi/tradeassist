---
name: docs-maintenance
description: TradeMind 文档分类、真源、链接、生成文件与 archive 维护规范
---

# Docs Maintenance

## 适用条件

- 修改 `*.md`、`*.mdc`、`config/agent/**`、`docs/**`、文档生成脚本。
- 不修改业务代码语义、API、权限或测试 baseline。

## 不适用

- 纯代码缺陷修复（改用对应领域 context）。
- 需要改产品行为却只改文档来“对齐想象”。

## 输入

- 目标文件与改动意图
- `pnpm agent:context` 输出
- 相关真源：代码 / `package.json` / config / 测试

## 领域不变量

- [DOC-001] 一个事实只有一个所有者；其他文件只链接。
- [DOC-002] 代码、测试、配置优先于说明文档；冲突时修正文档。
- [DOC-003] 稳定导航与 Agent 默认上下文不得包含 archive、history、原始证据。
- [DOC-004] 生成文件不得手工改业务内容；只允许重新生成。
- [DOC-005] 不确定事实标记 `unverified`，禁止猜测。
- [DOC-006] 不在 README/AGENTS/current 文档写入内部阶段号流水账。

## 文档分类

| 类型 | 位置 | 用途 |
|---|---|---|
| guide | `docs/guides/` | 教程与协作流程 |
| reference | `docs/reference/` | 稳定约定与索引 |
| runbook | `docs/runbooks/` | 操作/排错 |
| status | `docs/status/current.md` | 当前能力与限制 |
| adr | `docs/architecture/adr/` | 架构决策原因 |
| archive | `docs/archive/` | 历史，不代表当前 |

## 原子步骤

1. 判断改动属于 guide / reference / status / archive / generated。
2. 找到唯一真源；只在所有者处改完整定义。
3. 旧路径过渡期只保留 redirect，不保留双份正文。
4. 更新链接；运行 `pnpm docs:check`。
5. 若影响命令/env/API/module map，运行对应 `docs:generate*` 或 `--check`。
6. 交付时说明文档影响与未验证项。

## 验证

- `pnpm docs:check`
- `pnpm agent:check`（若改了 Agent 入口或 context map）
- `pnpm quality:sensitive`

## 输出

使用 `AGENTS.md` 统一最终说明格式。
