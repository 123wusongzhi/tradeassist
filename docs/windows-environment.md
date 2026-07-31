# Windows 系统环境摩擦与处理清单

> 审计时间：2026-07-31。来源：对 TradeMind 全部本地会话（7 个）的全文审计，识别由 Windows 系统环境（非代码）导致的重复摩擦，并给出已验证的系统状态与处理方法。

## 一、背景

以下三类会话曾反复出现系统环境导致的摩擦，均与代码本身无关：

| 会话 | 主题 | 主要环境摩擦 |
| --- | --- | --- |
| 2026-07-30 部署 | 从 GitHub 部署项目 | go/psql/redis-server 不在 PATH；README 中文按 GBK 乱码；Linux 命令被粘贴进 PowerShell |
| 2026-07-30 采集器/OpenCLI/Docker | Docker 链路、OpenCLI、仓库迁移 | `host.docker.internal:3100` connection refused；gofmt 不在 PATH；`max.cmd` 直接调用失败；双 pnpm 版本触发非交互确认 |
| 2026-07-31 SKU 识别 | 淘宝/天猫 SKU 探测、浏览器扩展 | **PowerShell 5.1 参数传递 bug**（`eval` 收到 5/34 个参数，改用 Base64 绕过）；`.CMD` shim 调用失败；403 次命令级 UTF-8 前缀；worktree 缺 node_modules/.env |

## 二、系统状态（2026-07-31 实测）

### 已解决

| 问题 | 状态 | 验证结果 |
| --- | --- | --- |
| PowerShell 5.1 参数/编码缺陷 | 已解决 | PS 7.6.4 已装；`~/.codex/config.toml` 已配置 `[shell] command = "pwsh"`；含双引号参数实测按 1 个参数传递 |
| 中文乱码（GBK 代码页） | 已解决 | 系统 ACP/OEMCP 均已切换为 65001（UTF-8）；pwsh 默认 utf-8 输出 |
| go/gofmt 不在 PATH | 已解决 | `C:\Program Files\Go\bin` 可用 |
| 多 pnpm 版本歧义 | 已解决 | 统一到 pnpm 9.15.4（与 package.json 的 packageManager 一致）；node v24.12.0、opencli 1.8.6 |
| 项目缺 `.env` / node_modules | 已解决 | 根目录与 worktree `f6f2` 均已有 `.env` + `node_modules` |
| Docker 全栈与采集链路 | 正常 | 5 个容器 Up/healthy：postgres 5433、redis 6380、collector 3001、backend 8081、admin 8000；宿主机 OpenCLI bridge `3100/health` 返回 200 |

### 未解决与处理方法

| 问题 | 处理方法 |
| --- | --- |
| `psql` / `redis-cli` 不在 PATH（本机未装原生 PostgreSQL/Redis） | 无需安装原生服务，使用 Docker 内命令（已验证可用）：`docker exec trademind-full-postgres-1 psql -U trademind -d trademind`、`docker exec trademind-full-redis-1 redis-cli ping`。后续可提供 `psql.cmd`/`redis-cli.cmd` 转发脚本或写入 README |
| `scripts/check-env.ps1` 环境自检脚本 | 待落地：把本清单的检查项固化为一条命令，输出红绿灯，并挂到 `pnpm check:dev` 前置 |
| `opencli:doctor` / `pnpm setup:local` | 待落地：前者检查 CLI/Daemon/扩展/端口 3100；后者一键复制 `.env.example → .env` + `pnpm install` |
| 项目根未跟踪文件 `.workbuddy/`、`collector/package-lock.json` | 均未提交。pnpm workspace 内不应出现 npm lockfile，建议确认来源后删除或加入 .gitignore |

## 三、Windows 环境操作规范（开发者与 AI Agent 通用）

1. 统一使用 PowerShell 7（`pwsh`），不再依赖 PS 5.1 行为（参数传递、编码均有历史缺陷）。
2. 读写中文文件必须显式编码：读取 `Get-Content -Encoding UTF8`，写入 `Set-Content -Encoding utf8`；命令输出固定 UTF-8（`$OutputEncoding`、`[Console]::OutputEncoding`、`chcp 65001`）。
3. 不直接 `& xxx.cmd` / `xxx.ps1` 调用 node_modules shim，优先 `pnpm --filter <pkg> <script>` / `pnpm exec`。
4. 不把 Linux 命令（`runuser`、`systemctl` 等）原样粘贴进 PowerShell。
5. 并行执行多个 pnpm 检查时加 `CI=true` 或改为串行，避免非交互依赖目录确认。
6. Docker 场景：宿主机采集链路使用 `host.docker.internal` + `extra_hosts: host-gateway`；OpenCLI Bridge 建议常驻（Windows 计划任务开机自启），避免端口 3100 偶发断连。
7. 数据库/缓存客户端统一走 Docker：`docker exec trademind-full-postgres-1 psql ...`、`docker exec trademind-full-redis-1 redis-cli ...`。

## 四、快速自检命令

```powershell
# 1. PowerShell 版本（应为 7.x Core）
$PSVersionTable.PSVersion
# 2. 工具链
go version; node --version; pnpm --version; opencli --version
# 3. Docker 全栈健康
docker ps --filter name=trademind-full --format '{{.Names}} {{.Status}}'
# 4. OpenCLI Bridge
Invoke-WebRequest -UseBasicParsing http://127.0.0.1:3100/health
# 5. 数据库/缓存探活
docker exec trademind-full-postgres-1 pg_isready
docker exec trademind-full-redis-1 redis-cli ping
```

