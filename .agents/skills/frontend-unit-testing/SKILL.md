---
name: frontend-unit-testing
description: TradeMind Admin 前端单元测试边界、mock 与断言规范
---

# Frontend Unit Testing

## 适用条件

- Admin 组件、hooks、service 转换、纯函数单测。
- selector 将 `frontend-unit` 标为 optional/required 时。

## 领域不变量

- [FUT-001] 测用户可见行为与状态转换，不锁死私有实现细节。
- [FUT-002] mock 与真实 API envelope/契约一致，不发明字段。
- [FUT-003] 不连真实后端；不触发真实写。
- [FUT-004] 不用 `.only`、无理由 `.skip`、弱断言。
- [FUT-005] 时间、随机、网络要可注入/可稳定。

## 原子步骤

1. 定位现有 test 文件或同目录新建。
2. 覆盖主路径 + 关键失败/空态。
3. 运行 `pnpm test:frontend` 或 selector 指定范围。

## 输出

使用 `AGENTS.md` 统一最终说明格式。
