---
name: frontend-unit-testing
description: TradeMind Admin TypeScript 单元测试、React 组件测试、hooks、请求转换和共享 UI 测试规范
---

# TradeMind 前端单元与组件测试规范

## 自动适用

涉及 `admin/src/**` 中 TypeScript、React 组件、hooks、service、常量、URL 状态、请求 payload、状态映射、共享 UI 或测试配置时自动适用。

## 技术栈

- Runner：Vitest。
- 环境：`jsdom` 用于 React 组件，Node 环境用于纯工具时可复用同一配置。
- 组件测试：React Testing Library、`@testing-library/jest-dom`、`@testing-library/user-event`。
- 不引入 Jest，不引入 Cypress。MSW 仅在模块级网络 Mock 确有必要时使用。

## 测试目标

优先覆盖：

- API envelope 解包与业务错误。
- URL tab/section/query 解析、写入 helper。
- readiness 状态、结果、分组、severity 映射。
- publish / draft / batch payload 构造。
- rowKey fallback、状态 Tag、长文本显示。
- Write Guard 或测试工具纯逻辑。
- `TmPageContainer`、`EmptyState`、`StatusTag` 等共享 UI 基础行为。

## 环境处理

Vitest setup 必须处理：

- `@` alias 指向 `admin/src`。
- Umi `@umijs/max` 的 `request`、`history` 等最小 mock。
- CSS/LESS import。
- `matchMedia`、`ResizeObserver`、`IntersectionObserver`、localStorage、history、URLSearchParams。
- Ant Design Portal/动画导致的异步行为。

不得用过度宽泛全局 Mock 掩盖真实错误；只 mock Umi runtime、浏览器缺失 API 和外部网络边界。

## 断言原则

- 测用户可见文本、角色、class/结构边界和真实业务输出。
- 不测试 Ant Design 内部 DOM。
- 不写无意义 snapshot。
- 失败信息应能定位业务行为。

## 变更测试选择

- 纯 TS 工具：相关单测 + type/build。
- 组件/页面：相关单测/组件测试 + Admin E2E smoke 或受影响 spec。
- service/request/payload：service 单测 + API contract + 受影响 E2E。
- shared UI/layout：组件测试 + Admin smoke + 响应式/overflow 相关 E2E。

## Bug 回归

前端 Bug 修复优先补一个能复现用户可见问题的测试，再做最小修复。无法自动化时报告原因。
