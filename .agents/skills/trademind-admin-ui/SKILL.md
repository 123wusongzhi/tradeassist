---
name: trademind-admin-ui
description: TradeMind Admin 项目级 UI 不变量与风险分级验收
---

# TradeMind Admin UI

## 适用条件

- `admin/src/pages/**`、`admin/src/components/**`、Admin 样式与布局改动。
- 小型文案、状态展示、局部样式、组件交互。

## 不适用

- API / DTO / 权限 / 状态机变更（升级 api-contract / backend）。
- 深度视觉体系重设计（可选阅读 upstream frontend-design，非默认必读）。

## 输入

- 目标页面/组件
- 是否改布局、路由、写操作、Modal/Drawer
- selector 输出与风险标记

## 领域不变量

- [UI-001] 默认不修改 API URL、method、payload、权限、readonly 或业务协议。
- [UI-002] loading / empty / error / readonly / submitting 状态必须可区分。
- [UI-003] 写操作需要明确触发与防重复提交；高风险操作需二次确认。
- [UI-004] 页面根节点不得横向溢出；表格与工具栏在窄屏可退化。
- [UI-005] 优先复用共享页面壳与表格/空态/抽屉组件，避免平行实现。
- [UI-006] 用户可见文案默认中文，术语保持一致。
- [UI-007] 未拦截时不得触发真实写请求（E2E/本地验收）。

## 风险分级视口

| 风险 | 必测视口 |
|---|---|
| 纯文案/图标/局部状态且不改布局 | 当前桌面 + 一个窄屏 |
| 布局、表格、Modal/Drawer、Tabs、Toolbar | 桌面、平板、手机（3 档） |
| shared 组件、全局布局、关键写路径 | 现有完整多档 |
| 无 DOM/样式变化 | 不做视觉视口验收 |

## 原子步骤

1. 确认是 UI 行为还是业务契约；契约变更先停并升级 context。
2. 最小范围修改页面/组件。
3. 检查状态完整性与窄屏溢出。
4. 按风险跑前端单测；写操作/路由/交互升级 E2E context。
5. 运行 selector 指定检查。

## 升级条件

仅在以下情况读取 optional reference：

- 全新页面信息架构或共享设计 token 大改 → frontend-design-deep
- 交互回归、深链、写路径 → admin-e2e
- 复杂状态逻辑单测不足 → frontend-unit

## 验证

命令以 `pnpm agent:context` 输出为准，常见：

- `pnpm test:frontend`
- `pnpm quality:affected`
- `pnpm test:affected`

## 输出

使用 `AGENTS.md` 统一最终说明格式。
