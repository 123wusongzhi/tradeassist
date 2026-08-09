---
name: backend-testing
description: TradeMind 后端测试按 DB/Redis/queue/adapter 风险分类
---

# Backend Testing

## 适用条件

- Go 单元、HTTP、DB、Redis、队列、adapter 测试策略。
- selector 将 `backend-testing` 标为 optional/required 时。

## 领域不变量

- [BT-001] 表驱动单元测试优先覆盖纯逻辑与错误分支。
- [BT-002] HTTP 测试断言 envelope、状态码与权限失败。
- [BT-003] DB/Redis 测试可跳过环境缺失，但不得静默连生产。
- [BT-004] adapter 使用 fake/server；禁止真实平台写。
- [BT-005] worker/queue 覆盖幂等、重试上限、停止与租约。
- [BT-006] 不在 Skill 硬编码完整端点清单。

## 风险分类

| 风险 | 最低验证 |
|---|---|
| service 纯逻辑 | `test:backend` |
| repository/migration | backend + `test:db`（环境可用） |
| redis/queue/worker | backend + `test:redis`（环境可用） |
| API shape | + `test:contracts` |

## 输出

使用 `AGENTS.md` 统一最终说明格式。
