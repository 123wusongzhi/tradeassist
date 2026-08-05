---
name: admin-e2e-testing
description: TradeMind Admin E2E 写请求拦截、console、深链与响应式风险规范
---

# Admin E2E Testing

## 适用条件

- Admin 交互 Bug、路由/深链、Modal/Drawer、写操作、响应式布局风险。
- selector 任务 `admin-interaction-or-write`。

## 不适用

- 纯文案且无 DOM/交互变化。
- 后端契约定义（用 api-contract）。

## 领域不变量

- [E2E-001] 默认拦截所有非 GET；真实写仅人工受控环境。
- [E2E-002] 监听 console/page error；无宽泛 allowlist 掩盖。
- [E2E-003] 深链与刷新后关键状态可恢复。
- [E2E-004] 写操作验证防重复提交与结果反馈。
- [E2E-005] 不把完整页面清单/tab 数硬编码为本 Skill 真源。
- [E2E-006] 失败报告首个根因与未覆盖风险。

## 原子步骤

1. 确认风险：写路径 / 路由 / 响应式 / 弹层。
2. 用现有 Playwright fixture 与 mock，不访问生产。
3. 断言关键用户路径，而非整站巡检。
4. 运行 selector 检查（常见 `test:e2e:smoke`）。

## 验证

以 selector 为准；需要完整套件时再显式运行 `test:e2e`。

## 输出

使用 `AGENTS.md` 统一最终说明格式。
