---
name: collector-development
description: TradeMind Collector / 浏览器扩展采集实现与引擎隔离规范
---

# Collector Development

## 适用条件

- `collector/**`、`browser-extension/**` 采集、归一化、引擎路由、扩展协议。

## 不适用

- Admin 纯展示改动。
- 后端 ERP 业务状态机（除非采集结果契约变更）。

## 输入

- 目标引擎（Playwright / OpenCLI / extension）
- 是否改输出 schema、登录态、资源生命周期
- selector 输出

## 领域不变量

- [COL-001] Playwright 与 OpenCLI Bridge 故障隔离；一侧失败不得拖垮另一侧。
- [COL-002] 浏览器上下文、页面、下载、临时文件必须释放。
- [COL-003] 归一化输出保持稳定字段；价格/库存避免不安全浮点。
- [COL-004] 不把完整 Cookie、Token、账号密码写入日志或仓库。
- [COL-005] 测试不访问真实电商写接口；不依赖生产站点稳定性作为单元断言。
- [COL-006] 引擎能力差异写 reference/runbook，不在 Skill 硬编码平台清单快照。

## 原子步骤

1. 确认改的是引擎、归一化、协议还是配置。
2. 保持输出契约兼容；破坏性变更同步契约测试/文档。
3. 补充纯函数或 adapter 单测。
4. 运行 `pnpm test:collector` 与 selector 检查。

## 升级条件

- 采集结果影响后端 DTO/API → api-contract
- 新共享包或跨模块边界 → architecture-change

## 验证

- `pnpm test:collector`
- `pnpm quality:affected`
- `pnpm test:affected`

## 输出

使用 `AGENTS.md` 统一最终说明格式。
