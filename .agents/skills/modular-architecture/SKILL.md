---
name: modular-architecture
description: TradeMind 模块边界、循环依赖与 Architecture Baseline/Ratchet
---

# Modular Architecture

## 适用条件

- 新模块、跨模块依赖、shared/common、adapter、worker/queue/scheduler、migration/repository、公共 type/API、大型拆分。
- selector 任务 `architecture-change`。

## 不适用

- 单文件局部 bug、纯文案、小 UI 样式。
- 以架构优化为名改业务语义。

## 输入

- 变更文件与依赖方向
- `tests/architecture/**` 与现有模块边界
- selector 输出

## 领域不变量

- [ARCH-001] handler → service → repository/provider/queue；禁止反向依赖。
- [ARCH-002] 核心业务不依赖第三方 SDK 细节；经 provider/adapter。
- [ARCH-003] shared/common 不得反向依赖业务模块。
- [ARCH-004] 禁止新增循环依赖与越层依赖。
- [ARCH-005] 不得以架构名义修改 API、payload、权限、状态机。
- [ARCH-006] 不自动扩大 architecture baseline；新违规不得进入。
- [ARCH-007] 未经确认不做大规模目录迁移。

## 原子步骤

1. 用代码与 architecture 配置确认当前边界，不凭旧文档目录树。
2. 将改动限制在目标模块；共享逻辑上移需有明确复用理由。
3. 新 adapter/worker/migration 明确归属与接口。
4. 运行 `architecture:affected` / selector 检查。
5. 若边界决策是长期约束，补充 ADR，而不是写阶段报告。

## 升级条件

- 对外 shape 变化 → api-contract
- 安全/并发高风险 → deep-code-review

## 验证

- `pnpm architecture:affected`
- 必要时 `pnpm architecture:check` / `pnpm architecture:test`
- 不把页面名、平台列表硬编码进本 Skill

## 输出

使用 `AGENTS.md` 统一最终说明格式。
