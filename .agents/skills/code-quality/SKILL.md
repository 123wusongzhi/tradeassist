---
name: code-quality
description: TradeMind 代码质量轻量检查、高风险升级与 Baseline/Ratchet 约束
---

# Code Quality

## 适用条件

- 代码新增/修改/重构/Bug 修复的质量审查。
- 由 selector 作为 optional `deep-code-review` 或显式高风险时加载。
- 不通过本文再级联其他 Skill。

## 不适用

- 纯文档任务（用 docs-maintenance）。
- 用质量名义扩大无关重构。

## 输入

- diff 与目标文件
- 风险：auth、tenant、库存、发布、第三方写、事务、Redis、队列、上传、Token、envelope
- selector 指定检查

## 领域不变量

- [CQ-001] Diff hygiene：最小改动，无无关格式化，不回滚用户改动。
- [CQ-002] 错误不可吞；异步必须处理失败；日志脱敏。
- [CQ-003] 输入与外部响应需校验；外部请求有 timeout 与有界 retry。
- [CQ-004] 不用 skip/ignore/宽泛 allowlist 或自动扩大 baseline 掩盖失败。
- [CQ-005] 高风险路径（认证权限、租户、库存、刊登、第三方写、并发事务、上传、Token）必须升级深度审查。
- [CQ-006] 新代码不得新增 baseline 外的 TS/Go/lint/安全问题。
- [CQ-007] Bug 修复优先补回归测试；失败报告首个根因。
- [CQ-008] 禁止真实密钥、生产 DB/Redis、未拦截第三方写请求。

## 轻量检查

1. 范围是否最小，是否有 debug 残留、`.only`、无理由 `.skip`。
2. 类型与命名是否清楚；是否新增无意义 `any`/宽泛 disable。
3. 空值、错误、权限失败路径是否完整。
4. 是否误改 API/业务语义。
5. 是否运行 selector 指定的 quality/test。

## 深度升级

命中高风险时额外审查：事务边界、幂等、锁与租约、竞态、tenant fail-closed、第三方错误分类、回滚、缓存一致性、敏感数据面。

## Baseline/Ratchet

- 允许历史问题存在于 baseline。
- 禁止新增签名/计数上升。
- 更新 baseline 必须显式命令并说明原因；CI 不得自动扩大。

## 验证

以 selector / `quality:affected` 为准，不在此复制全量命令矩阵。

## 输出

使用 `AGENTS.md` 统一最终说明格式；发现按 Critical / High / Medium / Advisory 分级。
