---
name: api-contract-testing
description: TradeMind API envelope、auth、tenant 与 DTO 兼容性契约规范
---

# API Contract Testing

## 适用条件

- API、DTO、envelope、auth、tenant 边界、Admin mock 与 contract fixture。
- selector 任务 `api-contract-change`。

## 领域不变量

- [API-001] 公共 envelope 与错误语义保持兼容；破坏性变更必须显式。
- [API-002] auth/tenant fail-closed；不返回 secrets。
- [API-003] Admin mock、contract tests 与后端 shape 同步。
- [API-004] 路由清单以代码/生成物为准，不在 Skill 维护全表。
- [API-005] 禁止以重构名义悄悄改 method/path/payload/权限。

## 原子步骤

1. 对照现有 contract fixture 与 handler 响应。
2. 更新单一真源（代码 + tests/contracts）。
3. 运行 `pnpm test:contracts` 与 selector 其他检查。

## 验证

- `pnpm test:contracts`
- 必要时 `pnpm test:backend` / 前端消费路径测试

## 输出

使用 `AGENTS.md` 统一最终说明格式。
