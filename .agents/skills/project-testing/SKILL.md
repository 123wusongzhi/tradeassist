---
name: project-testing
description: TradeMind 测试选择原则与门禁约束（命令由 selector 决定）
---

# Project Testing

## 适用条件

- 需要决定测什么、禁止什么。
- 测试基础设施或跨层回归策略变更。

## 不适用

- 替代领域实现 Skill。
- 在本文维护完整命令大全（见 generated command index / selector）。

## 领域不变量

- [TEST-001] 测试分层：单元 → 契约 → 集成 → E2E；按风险选择，不默认全跑。
- [TEST-002] `test:affected` / selector 决定命令；Skill 不硬编码全项目矩阵。
- [TEST-003] 禁止生产 DB/Redis、未拦截真实第三方写、自动扩大 baseline。
- [TEST-004] 失败定位首个根因；不得 skip 掩盖。
- [TEST-005] 必要检查未跑不得宣称完成，必须说明原因。
- [TEST-006] 契约与行为测试优先于实现细节快照。

## 选择原则

1. 只改纯函数/组件逻辑 → 对应单元测试。
2. DTO/envelope/API → contract tests。
3. DB/Redis/queue → 集成测试（环境可用时）。
4. Admin 交互/写路径/深链 → E2E smoke 或专项。
5. 架构边界变更 → architecture checks。

## 验证

运行 selector 输出的 checks；需要时再读领域测试 Skill。

## 输出

使用 `AGENTS.md` 统一最终说明格式。
