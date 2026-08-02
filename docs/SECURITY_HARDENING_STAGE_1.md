# 安全加固第一阶段收尾说明

- 日期：2026-08-03
- 分支：`fix/p0-p2-security-hardening`

## 阶段结论

本阶段按“优先恢复并保证用户正常使用”的决定收尾。当前完整 Docker 栈已使用本阶段代码重建并重启，Admin、Backend、Collector、PostgreSQL、Redis 均正常运行；Admin 首页、Backend readiness、Collector health 以及 Admin 到 Backend 的反向代理探针均返回 HTTP 200。

本阶段不是生产发布签收，也不代表剩余 P0/P1/P2 已全部清零。所有已确认并完成的修复已经保留，进一步的深层并发一致性审查按项目负责人决定延后。

## 本阶段完成内容

- 认证与会话：加强 JWT、Refresh Token、会话撤销、生产配置 fail-fast、登录身份与密码策略；补齐开发环境精确回环来源的 CSRF 放行，恢复 `localhost` / `127.0.0.1` 下的浏览器登录，生产白名单不变。
- 租户与权限：为 Admin API、商品、订单、库存、刊登、采集、AI 任务、文件和运维入口补齐可信租户、店铺范围和写权限约束。
- 文件与网络安全：加强上传隔离、扫描/隔离状态机、受控静态访问、图片解码与下载 SSRF 防护。
- Worker 与任务状态：为采集、图片任务、订单同步、客户同步、库存同步和商品刊登补充租约恢复、重试 CAS、取消竞态保护及 lease 丢失后的副作用阻断。
- Collector：增加内部 Bearer Token、非回环监听保护、浏览器/Profile 会话隔离、响应体与 URL 安全限制。
- Admin：补齐安全会话退出、权限菜单/路由/写操作保护及相关单元与 E2E 回归。
- Scripts：修复命令注入、误判成功和敏感差异检查问题。

## 审查方式

- 使用 OpenCodeReview delegation 模式，只执行确定性的文件选择与规则解析。
- 已先执行 `ocr scan --preview`，当时得到 3487 个 changed 项、2411 个 review candidate；4 个超过 2 MiB 的文档图片/JSON 证据文件被跳过，均非运行源码。
- 已分别解析 backend、admin、collector、scripts 的审查规则。
- 实际代码审查与判断由 Codex 完成，未配置或调用外部 LLM。

## 验证结果

- `go test ./... -count=1 -timeout=300s`：通过。
- 高风险模块定向测试（productpublish、ordersync、customersync、inventory、imagetask）：通过。
- `pnpm architecture:affected`：448 个 affected files，7 类检查，0 failures；覆盖 Admin 单测、Collector 单测、API 契约、Backend 全量测试、架构 ratchet 与敏感差异检查。
- Admin 与 Collector 构建：通过；本阶段 Docker 镜像中的 Admin、Backend、Collector 均重新构建成功。
- `docker compose -f docker-compose.full.yml up -d --build`：重建和重启成功，未删除数据卷。
- 运行探针：Admin `:8000/`、Backend `:8081/health/ready`、Collector `:3001/health`、Admin 代理 `/health/ready` 均返回 200。
- 登录回归：真实浏览器使用首启管理员账号成功进入 `/dashboard/product-operations`，刷新后安全会话仍有效；带 `Origin: http://127.0.0.1:8000` 的代理登录返回业务码 `0`。

## 本地配置迁移

Docker 内部 Backend 与 Collector 现在要求共享 `COLLECTOR_INTERNAL_TOKEN`。本机 `.env` 已补充随机长值以恢复当前部署；`.env` 被 Git 忽略，令牌不会提交。其他部署升级时必须按 `.env.docker.example` 配置独立随机值，并保证两端一致。

## 延后事项

下一阶段继续审计时优先处理：

1. productpublish panic 路径和 worker 起始 `publish_status` 更新的精确租约 CAS。
2. productpublish 任务终态、publication、SKU 映射多步写的一致事务与数据库错误传播。
3. ordersync / inventory 的 stale 标记和 progress touch 对当前执行租约的更严格约束。
4. 重新执行完整 P0/P1/P2 终审并形成最终清零结论；本阶段不作清零声明。
