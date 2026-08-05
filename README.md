<h1 align="center">贸灵 TradeMind</h1>

<p align="center">
  <strong>开源 AI 跨境电商运营平台</strong>
</p>

<p align="center">
  聚焦 商品采集 → 商品草稿 → AI 内容优化 → 商品刊登 → 订单与库存协同
</p>

<p align="center">
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-Apache--2.0-blue.svg"></a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white">
  <img alt="React" src="https://img.shields.io/badge/React-18+-61DAFB?logo=react&logoColor=111">
  <img alt="TypeScript" src="https://img.shields.io/badge/TypeScript-5+-3178C6?logo=typescript&logoColor=white">
  <img alt="Docker" src="https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white">
  <img alt="Self-hosted" src="https://img.shields.io/badge/Self--hosted-supported-2EA043">
</p>

<p align="center">
  简体中文 | <a href="README.en.md">English</a>
</p>

<p align="center">
  <a href="#快速开始">快速开始</a> ·
  <a href="#界面预览">界面预览</a> ·
  <a href="#核心能力">核心能力</a> ·
  <a href="#架构与技术栈">架构与技术栈</a> ·
  <a href="docs/README.md">文档中心</a>
</p>

<p align="center">
  <img src="docs/assets/img/readme-hero-zh.png" alt="TradeMind 产品预览" width="100%" />
</p>

TradeMind 是一个面向跨境卖家与开发团队的开源 AI 运营平台，优先解决“采集、整理、优化、刊登、同步”这条主链路。项目当前聚焦两条主线：`AI 商品运营工具` 与 `多平台跨境 ERP MVP`。

与传统重型 ERP 不同，TradeMind 当前不追求多仓、采购、财务、WMS / OMS 或复杂 BI 的一次性全量覆盖，而是提供一个可私有化部署、可二次开发、可通过 Provider 扩展的平台底座。

## 项目成熟度

TradeMind 仍处于快速演进阶段。自托管、二次开发和测试环境使用是当前主要场景。

连接真实店铺、执行商品刊登、库存同步或其他外部写操作前，请在隔离店铺和小批量数据上完成受控验证。详细的当前能力与已知限制见 [`docs/status/current.md`](docs/status/current.md)。

## 项目定位

| 方向 | TradeMind 的做法 |
| --- | --- |
| AI 商品运营 | 围绕商品采集、草稿管理、AI 标题与描述、图片处理、发布前检查构建高频运营链路。 |
| 跨平台 ERP MVP | 优先打磨店铺授权、订单同步、SKU 匹配、库存同步、商品刊登等可运行闭环。 |
| 私有化与扩展 | 通过 AI / Storage / Image / Platform / Collector Provider 抽象扩展，适合自部署与二次开发。 |

## 界面预览

以下截图来自本地开发环境，展示当前最成熟的主线能力：**商品采集 → 商品草稿 → AI 内容优化**。

<table>
  <tr>
    <td width="50%" align="center">
      <img src="docs/assets/img/2.png" alt="采集中心" width="100%" />
      <br />
      <sub><strong>采集中心</strong>：采集器入口与批量采集</sub>
    </td>
    <td width="50%" align="center">
      <img src="docs/assets/img/3.png" alt="采集任务" width="100%" />
      <br />
      <sub><strong>采集任务</strong>：链接提交、状态追踪与草稿关联</sub>
    </td>
  </tr>
  <tr>
    <td width="50%" align="center">
      <img src="docs/assets/img/4.png" alt="采集监控" width="100%" />
      <br />
      <sub><strong>采集监控</strong>：Worker、任务与批次状态分布</sub>
    </td>
    <td width="50%" align="center">
      <img src="docs/assets/img/1.png" alt="AI 描述生成" width="100%" />
      <br />
      <sub><strong>AI 描述生成</strong>：生成卖点、规格与描述并应用到草稿</sub>
    </td>
  </tr>
</table>

## 核心能力

### AI 商品运营

- 商品采集：支持 1688、拼多多、淘宝/天猫与自定义规则采集；淘宝/天猫支持完整
  SKU 价格与库存识别（浏览器侧边栏扩展一键采集当前商品）。
- 商品草稿：统一管理商品、SKU、图片、库存阈值、采集告警与发布前检查。
- AI 内容：支持标题优化、描述生成、Prompt 模板、结果对比、人工应用与撤销。
- AI 图片：支持 remove.bg、OpenAI Image、ComfyUI 等 Provider，并通过异步任务队列执行。

### 多平台跨境 ERP MVP

- 店铺授权：已具备 Douyin Shop OAuth 闭环、Ozon 店铺级 API Key 接入、敏感配置加密与连接测试。
- 订单协同：支持订单同步、SKU 匹配、异常工作台等基础能力。
- 库存协同：支持库存镜像、预警与平台同步任务。
- 商品刊登：提供统一“刊登中心 / 刊登进度”入口；首期完整支持 Ozon 的商品 + 店铺独立配置、SKU 专属售价、本地库存引用、逐 SKU 图片顺序、包裹参数、多值/复杂类目属性、只读发布前检查、不可变提交快照和真实 Adapter 链路。其他平台只展示已真实接入的能力，不伪造可用字段；真实提交始终二次确认。
- AI 客服：支持建议回复与人工确认外发，避免 MVP 阶段自动外发风险。

### 工程化与扩展

- Provider 架构：AI、存储、图片、平台、采集能力均通过 Provider 抽象扩展。
- 自部署友好：默认 PostgreSQL + Redis，支持本地开发和 Docker Compose 完整部署。
- Monorepo 协作：backend、admin、collector 与文档规则统一维护，适合团队协作与持续演进。
- 可靠性地基：关键写路径统一幂等，AI 结果应用/撤销保护，Webhook 快速 ACK，异步 Worker 租约防止陈旧写回。

## 架构与技术栈

| 层级 | 技术栈 |
| --- | --- |
| Backend | Go + Gin + GORM |
| Admin | React + TypeScript + Ant Design Pro |
| Collector | Node.js + TypeScript + Playwright；可选宿主机 OpenCLI Bridge |
| Data | PostgreSQL + Redis |
| Deploy | pnpm workspace + Docker Compose |
| Extension Points | AI / Storage / Image / Platform / Collector Providers |

## 快速开始

### 本地开发

```bash
pnpm install
pnpm install:collector:browsers
pnpm dev
```

常用命令：

```bash
pnpm check:dev
pnpm dev:infra
pnpm dev:backend
pnpm dev:admin
pnpm dev:collector
pnpm opencli:install-adapter
pnpm dev:opencli-bridge
pnpm opencli:doctor
pnpm build:admin
pnpm build:collector
pnpm build:browser-extension
pnpm seed:demo-data
pnpm seed:demo-permissions
pnpm verify:demo-data
pnpm verify:demo-permissions
pnpm check:p4-r
```

Playwright Collector 固定监听 `3001`。OpenCLI 是可选的宿主机 Bridge，默认监听
`127.0.0.1:3100`；在 `.env` 设置 `OPENCLI_BRIDGE_ENABLED=true` 后，`pnpm dev`
会同时启动它。Bridge 启动失败只影响 OpenCLI，Playwright 采集仍可使用。
TradeMind 的淘宝/天猫 OpenCLI 适配器随项目维护；Bridge 启动时会安全同步该适配器。
也可先运行 `pnpm opencli:install-adapter`。已有非 TradeMind 同名适配器不会被覆盖。
OpenCLI 当前只支持淘宝/天猫，且任务执行失败不会自动切换到 Playwright；完整路由、
部署选择和排错说明见 [采集引擎与部署指南](docs/collector-engines.md)。

### 浏览器侧边栏扩展

淘宝/天猫单商品采集还可直接使用随仓库维护的浏览器侧边栏扩展：在已登录的
Chrome / Edge 商品详情页点击一次即可采集标题、图片、属性与完整 SKU（券后价、
原价、库存、发货时间），无需额外浏览器或 OpenCLI Bridge。构建、安装、配对与
风控说明见 [浏览器侧边栏采集](docs/browser-extension-collector.md)。它与
Playwright Collector、OpenCLI Bridge 是三条互不强制、互不依赖的采集入口，按场景
三选一即可。

### Docker 部署

```bash
cp .env.docker.example .env
docker compose -f docker-compose.full.yml up -d --build
```

Windows PowerShell：

```powershell
Copy-Item .env.docker.example .env
docker compose -f docker-compose.full.yml up -d --build
```

默认访问地址：

| 服务 | 地址 |
| --- | --- |
| Admin | <http://127.0.0.1:8000> |
| Backend Health | <http://127.0.0.1:8080/health> |
| Playwright Collector Health | <http://127.0.0.1:3001/health> |

更多说明：

- [本地开发](docs/development.md)
- [Docker 部署](docs/docker-deployment.md)
- [采集引擎与部署](docs/collector-engines.md)
- [环境变量](docs/env.md)

## 文档导航

- [docs/README.md](docs/README.md)：完整文档入口。
- [docs/development.md](docs/development.md)：本地开发、调试与常用命令。
- [docs/docker-deployment.md](docs/docker-deployment.md)：Docker Compose 完整部署与运维说明。
- [docs/browser-extension-collector.md](docs/browser-extension-collector.md)：浏览器侧边栏采集扩展说明。
- [docs/api.md](docs/api.md)：API 契约、统一返回与鉴权说明。
- [docs/provider.md](docs/provider.md)：Provider 扩展机制与安全约束。
- [docs/architecture.md](docs/architecture.md)：系统架构、分层与数据流说明。
- [docs/branching.md](docs/branching.md)：分支策略与 PR 规则。

## 贡献与社区

- 贡献代码或文档前，请先阅读 [CONTRIBUTING.md](CONTRIBUTING.md)。
- 安全问题请参考 [SECURITY.md](SECURITY.md)。
- 如果你愿意补充更好的截图、示例数据或文档，也非常欢迎提交 PR。
- 赞助方式见 [docs/sponsor.md](docs/sponsor.md)。

## License

本项目基于 [Apache License 2.0](LICENSE) 开源。
