---
name: backend-development
description: TradeMind Go 后端分层、事务、错误边界与 Provider 实现规范
---

# Backend Development

## 适用条件

- `backend/**/*.go` 业务实现、handler、service、repository、provider、queue。

## 不适用

- 纯 Admin 文案/样式。
- 不改后端的文档任务。

## 输入

- 目标包/文件
- 是否触及 API、DB、队列、平台 adapter、租户
- selector 输出

## 领域不变量

- [BE-001] 分层：handler → service → repository / provider / queue；handler 不堆领域规则。
- [BE-002] 第三方平台、AI、存储、图片、采集 SDK 不得泄漏进核心业务层。
- [BE-003] 租户/权限 fail-closed；缺 tenant 不得默认真源数据。
- [BE-004] 外部 HTTP 必须 timeout；retry 有上限；区分 retryable / terminal / uncertain。
- [BE-005] 写路径考虑幂等；异步任务可停止、可观察、不把失败标成功。
- [BE-006] 日志脱敏：禁止完整密钥、Token、Cookie、生产凭证。
- [BE-007] error 必须带上下文包装；可恢复错误不用 panic。
- [BE-008] 默认数据库为 PostgreSQL；测试不得连生产 DB/Redis。

## 原子步骤

1. 定位现有模块入口与测试，确认契约。
2. 在正确层修改；不顺手改无关模块。
3. DB/事务/锁变更时明确边界与回滚。
4. 平台调用经 adapter/provider，并补充 fake/test。
5. 运行 selector 指定检查（通常 `test:backend`）。

## 升级条件

- migration / repository / 新模块 / adapter / worker → architecture-change
- envelope、DTO、auth、tenant 边界 → api-contract
- 高风险并发/安全 → deep-code-review（optional）

## 验证

- `pnpm test:backend` 或 selector 输出
- 涉及 DB/Redis 时按环境运行 `test:db` / `test:redis`
- 不执行真实第三方写请求

## 输出

使用 `AGENTS.md` 统一最终说明格式。
