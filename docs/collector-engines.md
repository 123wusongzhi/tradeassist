# 采集引擎与部署指南

> 状态：2026-07-31。本文是浏览器侧边栏扩展、Playwright Collector 与 OpenCLI
> Bridge 三条采集入口的权威说明。端口、路由、启动方式或引擎支持范围发生变化时，
> 应优先更新本文，再同步 `README`、开发、Docker、环境变量、Provider 与 API 文档。

## 先看结论

- Playwright Collector 是基础采集服务，固定使用 `3001`。本地部署访问
  `http://127.0.0.1:3001`，Docker backend 访问 `http://collector:3001`。
- OpenCLI 是可选的宿主机 Bridge，默认使用 `3100`。本地 backend 访问
  `http://127.0.0.1:3100`，Docker backend 访问
  `http://host.docker.internal:3100`。
- backend 按每个任务已经持久化的 `engine` 路由，不再用一个
  `COLLECTOR_BASE_URL` 在 Playwright 与 OpenCLI 之间整体切换。
- OpenCLI 当前只支持淘宝/天猫。Playwright 仍承载全部当前可用或 Beta 的既有来源，
  也是淘宝/天猫的可选备用引擎。
- “备用”表示用户可以重新选择 Playwright，并不表示 OpenCLI 运行失败后会自动切换。
  已开始执行的任务禁止静默跨引擎回退，避免结果和排错信息失真。
- 普通本地安装和完整 Docker Compose 都默认关闭 OpenCLI，只使用 Playwright 即可。
  开启 OpenCLI 后，Docker 仍是“容器核心服务 + 宿主机 Bridge”的轻量混合部署。

因此，本地与 Docker 的核心功能没有两套实现：两者使用同一个 backend、同一个任务
模型和同一个 Playwright Collector。唯一部署差异是 OpenCLI 必须复用宿主机上的
OpenCLI、Chrome 扩展连接和登录态，目前不放进 Compose。

## 三条采集入口：都可选，用户按场景三选一

| 入口 | 运行位置 | 适用场景 | 前置依赖 |
| --- | --- | --- | --- |
| 浏览器侧边栏扩展 | 用户当前 Chrome / Edge 标签页 | 淘宝/天猫单商品、当前页面即时采集 | 只需 TradeMind backend；不需要 Playwright Collector、OpenCLI Bridge 或第二套浏览器 |
| Playwright Collector | 本地进程或 Compose `collector` 容器（`3001`） | 后台任务、批量采集、全平台来源 | backend 可达即可，不要求浏览器扩展或 OpenCLI |
| OpenCLI Bridge | 宿主机可选进程（`3100`） | 复用宿主机 Chrome 登录态的淘宝/天猫采集 | 宿主机已装 `opencli` 并完成适配器同步 |

三条入口互不强制、互不依赖，用户按场景选择其一或组合使用：

- 只用扩展：不启动 Playwright Collector、不开启 OpenCLI Bridge，扩展即可在
  当前商品页采集并提交 backend 创建草稿。
- 只用 Playwright：不安装浏览器扩展、不装 OpenCLI，后台任务与批量采集照常运行。
- 只用 OpenCLI：Playwright 未启动时，淘宝/天猫显式选 `engine=opencli` 的任务
  仍可执行；其他采集来源不受影响。
- 混合使用：三个入口共享同一个 backend 与任务模型，可同时存在；每个任务或场景
  单独选择入口，没有全局强制启用项。

## 运行结构

```text
Admin
  │
  ▼
Go backend / collect worker
  │
  ├─ engine=playwright
  │    ├─ 本地：http://127.0.0.1:3001
  │    └─ Docker：http://collector:3001
  │
  └─ engine=opencli（当前仅 taobao_tmall）
       ├─ 本地：http://127.0.0.1:3100
       └─ Docker：http://host.docker.internal:3100
```

两个 HTTP 服务彼此独立：

| 服务 | 默认端口 | 运行位置 | 作用 |
| --- | --- | --- | --- |
| Playwright Collector | `3001` | 本地进程或 Compose `collector` 容器 | 全部既有采集来源 |
| OpenCLI Bridge | `3100` | 宿主机可选进程 | 当前仅淘宝/天猫 OpenCLI 采集 |

OpenCLI Bridge 停止时，只有实际引擎为 `opencli` 的任务会失败；backend 健康检查、
Playwright 任务和其他采集来源不应访问 `3100`。

## 引擎支持范围

| 采集来源 | Playwright | OpenCLI | 未显式选择时 |
| --- | --- | --- | --- |
| 淘宝/天猫 `taobao_tmall` | 支持 | 支持 | Bridge 已启用且默认值为 `opencli` 时使用 OpenCLI，否则使用 Playwright |
| 1688 `1688` | 支持 | 不支持 | Playwright |
| 拼多多 `pinduoduo` | 支持 | 不支持 | Playwright |
| AliExpress `aliexpress` | 支持 | 不支持 | Playwright |
| SHEIN / Temu `shein_temu` | 规划中，当前不可创建任务 | 不支持 | 当前不可用 |
| 自定义规则 `custom` | 支持 | 不支持 | Playwright |

浏览器侧边栏扩展是第三条独立入口，不走 backend 的 `engine` 路由：扩展任务由
扩展直接创建/完成，不投递 Redis，也不经过 Playwright 或 OpenCLI，只用于用户
当前打开的淘宝/天猫商品页。它与 Playwright / OpenCLI 互不依赖，选择扩展采集时
无需为它准备任何采集服务。

对非淘宝/天猫来源显式传 `engine=opencli` 时，backend 会返回
`COLLECT_ENGINE_SOURCE_UNSUPPORTED`，不会改走 Playwright。

## 选择部署方式

| 场景 | Playwright | OpenCLI | 推荐用途 |
| --- | --- | --- | --- |
| 本地默认 | 本地 `3001` | 关闭 | 开发、普通使用、最少依赖 |
| 本地双引擎 | 本地 `3001` | 宿主机 `127.0.0.1:3100` | 以 OpenCLI 采集淘宝/天猫，同时保留 Playwright |
| Docker 默认 | Compose `collector:3001` | 关闭 | 最轻松的完整部署 |
| Docker 双引擎 | Compose `collector:3001` | 宿主机 `0.0.0.0:3100` | Docker 核心栈 + OpenCLI 淘宝/天猫主引擎 |

如果不采集淘宝/天猫，或暂时不需要 OpenCLI，保持
`OPENCLI_BRIDGE_ENABLED=false` 即可；不要为了消除 OpenCLI 错误而修改
Playwright 的地址。

## 本地部署

### 仅使用 Playwright

```bash
pnpm install
pnpm install:collector:browsers
pnpm dev
```

`.env` 保持以下关键值：

```env
COLLECTOR_HTTP_ADDR=:3001
COLLECTOR_PLAYWRIGHT_BASE_URL=http://127.0.0.1:3001
OPENCLI_BRIDGE_ENABLED=false
```

这也是未安装 OpenCLI 时的正常模式。淘宝/天猫未显式选择引擎的任务会使用
Playwright。

### 启用 OpenCLI

先保证 `opencli` 命令可用，并确认 OpenCLI 扩展连接的是准备采集的 Chrome 与登录态。
然后执行：

```bash
pnpm opencli:install-adapter
pnpm opencli:doctor
```

在 `.env` 中启用：

```env
OPENCLI_BRIDGE_ENABLED=true
OPENCLI_BRIDGE_BASE_URL=http://127.0.0.1:3100
OPENCLI_BRIDGE_HTTP_ADDR=127.0.0.1:3100
COLLECT_DEFAULT_ENGINE_TAOBAO_TMALL=opencli
```

推荐直接运行：

```bash
pnpm dev
```

`pnpm dev` 会把 Bridge 作为可选子进程启动。Bridge 启动失败不会结束 backend、
admin 或 Playwright Collector。

需要分别调试进程时，可以单独运行：

```bash
pnpm dev:opencli-bridge
```

不要在 `pnpm dev` 已经启动 Bridge 后再次运行这个命令，否则会产生 `3100`
端口占用。

Bridge 启动时会幂等同步仓库中的
`collector/opencli-adapters/tmall/` 到用户 OpenCLI 目录。它只更新带 TradeMind
受管标记的适配器；检测到无标记的同名用户适配器时会停止并提示，不会直接覆盖。

## Docker 部署

### 仅使用 Playwright

```bash
cp .env.docker.example .env
docker compose -f docker-compose.full.yml up -d --build
```

Windows PowerShell：

```powershell
Copy-Item .env.docker.example .env
docker compose -f docker-compose.full.yml up -d --build
```

默认 `OPENCLI_BRIDGE_ENABLED=false`。Compose 会启动 Playwright Collector，
backend 始终通过 `http://collector:3001` 访问它，不依赖宿主机 `3100`。

### Docker 同时启用 OpenCLI

OpenCLI Bridge 不由 Compose 启动。宿主机需要 Node.js、pnpm、OpenCLI 和已连接
扩展的 Chrome。先在 `.env` 配置：

```env
OPENCLI_BRIDGE_ENABLED=true
OPENCLI_BRIDGE_BASE_URL=http://127.0.0.1:3100
OPENCLI_BRIDGE_DOCKER_BASE_URL=http://host.docker.internal:3100
OPENCLI_BRIDGE_HTTP_ADDR=0.0.0.0:3100
OPENCLI_BRIDGE_TOKEN=请替换为随机长字符串
COLLECT_DEFAULT_ENGINE_TAOBAO_TMALL=opencli
```

非回环监听必须设置 `OPENCLI_BRIDGE_TOKEN`。建议使用至少 32 字节的随机值，并用
本机防火墙限制 `3100`，不要直接暴露到公网。

先在宿主机执行：

```bash
pnpm install
pnpm opencli:install-adapter
pnpm opencli:doctor
pnpm dev:opencli-bridge
```

再启动或重启 Compose：

```bash
docker compose -f docker-compose.full.yml up -d --build
```

Compose 会把 `OPENCLI_BRIDGE_DOCKER_BASE_URL` 作为容器内的
`OPENCLI_BRIDGE_BASE_URL` 注入 backend。Linux 原生 Docker 使用 Compose 中的
`host-gateway` 映射，不需要把 Playwright 地址改成宿主机地址。

## 任务如何选择引擎

管理端的单任务和批量采集页面会展示引擎状态，并在淘宝/天猫来源下允许选择
OpenCLI 或 Playwright。任务创建后，实际引擎会写入任务快照并显示在任务列表中；
重试和进程重启不会改变它。

单任务请求示例：

```json
{
  "source": "taobao_tmall",
  "url": "https://detail.tmall.com/item.htm?id=123456",
  "engine": "opencli"
}
```

批量请求示例：

```json
{
  "source": "taobao_tmall",
  "urls": [
    "https://detail.tmall.com/item.htm?id=123456",
    "https://item.taobao.com/item.htm?id=789012"
  ],
  "engine": "playwright"
}
```

省略 `engine` 时：

1. 淘宝/天猫在 `OPENCLI_BRIDGE_ENABLED=true` 且
   `COLLECT_DEFAULT_ENGINE_TAOBAO_TMALL=opencli` 时使用 OpenCLI。
2. OpenCLI 未启用时使用 Playwright。
3. 其他来源始终使用 Playwright。

显式选择 OpenCLI 但 Bridge 未启用时，创建任务返回 HTTP `503` 和
`OPENCLI_BRIDGE_DISABLED`。Bridge 已启用但稍后离线时，已创建的 OpenCLI 任务会
按 OpenCLI 失败，不会在执行中偷偷改用 Playwright。

## 启动后验证

### 基础健康检查

本地或宿主机：

```bash
curl http://127.0.0.1:3001/health
curl http://127.0.0.1:3100/health
```

第二条只在启用并启动 OpenCLI Bridge 后才应成功。Bridge 配置 Token 后，运行状态
接口需要 Bearer Token：

```bash
curl -H "Authorization: Bearer <OPENCLI_BRIDGE_TOKEN>" \
  http://127.0.0.1:3100/v1/opencli/status
```

登录 TradeMind 后，还可以通过
`GET /api/v1/collect/engines/status` 或管理端采集页查看两个引擎的
`enabled`、`configured`、`reachable`、`ready` 与有效默认引擎。该 API 不返回
Bridge 地址、Token、CLI 原始输出或本机路径。

引擎状态读取必须是无副作用的：Bridge 使用 `opencli daemon status` 被动读取 daemon、
扩展和 Profile 状态，不调用会执行真实浏览器连接的 `opencli doctor`。`doctor`
只用于用户显式运行的诊断命令。普通页面导航、点击和状态刷新不得租用、打开或聚焦
OpenCLI 控制的 Chrome 窗口。

### 人工验收清单

1. 提交一个淘宝/天猫 OpenCLI 任务，确认任务列表“实际引擎”为 OpenCLI。
2. 提交一个淘宝/天猫 Playwright 任务，确认“实际引擎”为 Playwright。
3. 停止 OpenCLI Bridge，再提交或重试 OpenCLI 任务，确认出现明确的 Bridge 错误。
4. Bridge 停止期间执行 Playwright 或 1688 任务，确认仍能正常访问 `3001`。
5. 重新启动 Bridge，检查引擎状态恢复，再使用一条真实商品链接验证标题、主图、
   SKU、详情图与商品草稿。

淘宝/天猫真实链接的字段级验收见
[collector-taobao-tmall-test-links.md](collector-taobao-tmall-test-links.md)。

## 淘宝/天猫 SKU 价格与库存识别

新版天猫 SSR 详情页默认隐藏 per-SKU 价格（`skuItem.hideOtherPrice=true`），初始
`skuCore.sku2info` 只返回每个 SKU 的库存（`quantity` / `quantityText`），没有价格；
实测点击规格选项也不会触发价格查询，页面始终显示“券后￥X起”。这是第三方服务商
普遍无法识别 SKU 价格的根因。

实际数据规律（2026-07-31 实测确认）：

- 每个 SKU 的券后价/原价只在该商品页带 `skuId` 参数重新请求时，由服务端 SSR 在
  `__ICE_APP_CONTEXT__.loaderData.home.data.res.skuCore.sku2info[skuId]` 返回
  （`subPrice` = 券后、`price` = 优惠前），一次请求只返回一个 SKU 的价格。
- 初始加载的 `sku2info` 已包含全部 SKU 库存，不需要额外请求；`quantity: 0` 表示
  缺货，`quantityText` 提供“有货 / 即将售罄 / 无货”等文案。

OpenCLI 适配器与 Playwright 采集器已内置处理：

1. 解析 `skuBase.skus`（`propPath` + `skuId`）得到规格组合，并与
   `skuCore.sku2info` 按 `skuId` 合并库存，不再产生 `STOCK_UNKNOWN`。
2. 对缺少价格的 SKU，在已登录页面内用同源 `fetch`（携带 Cookie）按 `skuId`
   **串行探测** SSR 价格，默认最多 24 条；管理端“采集设置 → 淘宝/天猫 → SKU
   价格探测上限”可配置（1–48，0 关闭，底层透传 `--sku-price-max`）。单条之间
   **300–800ms 随机延迟**（固定间隔更易被识别），连续失败自动停止；全程只复用
   当前标签页（同源 fetch），绝不并发、不新开窗口。
3. 老版淘宝/天猫页面（点击后价格会更新）保留点击采价回退；探测无结果时不打扰用户。

风控提醒：逐 SKU 探测本质上是多次访问商品页，短时间内大量采集容易触发滑块验证；
若页面出现“验证码拦截”，需在连接的 Chrome 手动完成一次验证后再继续。批量任务建议
降低探测数量或拉大任务间隔。

## 常见问题

### `host.docker.internal:3100: connect: connection refused`

这表示 Docker backend 正在执行 OpenCLI 任务，但宿主机 Bridge 没有监听或无法访问。

按顺序检查：

1. 在任务列表确认“实际引擎”确实是 OpenCLI。
2. 宿主机执行 `pnpm opencli:doctor`。
3. 宿主机执行 `pnpm dev:opencli-bridge`，检查
   `http://127.0.0.1:3100/health`。
4. Docker 场景确认 `OPENCLI_BRIDGE_HTTP_ADDR=0.0.0.0:3100`，且 backend 与
   Bridge 使用相同的 `OPENCLI_BRIDGE_TOKEN`。
5. 检查本机防火墙是否允许 Docker Desktop 访问 `3100`。

如果任务实际引擎是 Playwright，它不应访问 `3100`；应检查是否仍在运行旧 backend
镜像，或是否误把旧的全局 `COLLECTOR_BASE_URL` 指向 Bridge。

### 浏览器可以打开商品，但采集仍失败

“页面能打开”只证明浏览器导航成功，不代表页面已经向适配器提供完整的结构化数据。
还需要检查：

- OpenCLI 扩展是否连接到同一个 Chrome。
- 淘宝/天猫是否已登录，页面是否要求人机验证。
- 页面是否被限流、软拦截或返回了不同结构。
- TradeMind 受管适配器是否已同步。
- 商品的 SKU、详情图等区域是否需要交互或延迟加载。

OpenCLI 的 `EMPTY_RESULT` 统一视为可恢复的解析失败，不等同于商品下架。只有适配器
明确返回 `ITEM_NOT_FOUND` 时，任务才归类为商品不存在。

### 普通页面点击会弹出 OpenCLI 控制的浏览器

旧版 Bridge 的 `/v1/opencli/status` 会执行 `opencli doctor`。OpenCLI Doctor 包含
一次真实 `BrowserBridge.connect()` 探测，因此管理端导航或打开弹窗时的状态 GET
也可能租用并聚焦浏览器窗口。当前 Bridge 已改为被动执行
`opencli daemon status`；若仍出现该现象，请确认宿主机 Bridge 已重启或热重载到
最新代码，而不是继续运行旧的构建产物。

### OpenCLI 不可用时为什么不自动用 Playwright

自动回退会让同一个任务在不同引擎间产生不可解释的结果，也会掩盖登录态、扩展连接
和适配器故障。当前策略是保留 Playwright 作为可见、可手动选择的备用方案，并让失败
任务保留原始实际引擎和错误。

### Docker 是否必须安装 OpenCLI

不需要。默认 Compose 只用 Playwright，功能和此前的 Docker 主链路一致。只有要用
OpenCLI 采集淘宝/天猫时，才需要在宿主机安装并启动 Bridge。

### 管理端“采集服务监听地址”为什么可能仍显示 `:3100`

当前管理端采集设置页保留了一个历史 `collector_http_addr` 展示/保存项；数据库没有
该项时，前端的旧回退值仍可能显示 `:3100`。它不会改写正在运行的 Collector 进程，
也不是双引擎路由来源。实际 Playwright 监听地址以进程环境变量
`COLLECTOR_HTTP_ADDR=:3001` 和 `/health` 结果为准，backend 地址以
`COLLECTOR_PLAYWRIGHT_BASE_URL` 为准。

这是已知的 UI 默认值漂移，后续代码任务应把展示默认值同步为 `:3001`，或移除这个
不能控制运行时的配置项。在该代码修复前，不要依据设置页的 `:3100` 把 Playwright
改回旧端口。

## 从旧配置迁移

旧版曾使用一个 `COLLECTOR_BASE_URL` 切换整个采集服务，且部分文档把通用 Collector
写成 `3100`。新配置按以下规则迁移：

| 旧配置 | 新配置 |
| --- | --- |
| `COLLECTOR_BASE_URL=http://127.0.0.1:3100` | `COLLECTOR_PLAYWRIGHT_BASE_URL=http://127.0.0.1:3001` |
| Docker backend 指向宿主机通用 Collector | Playwright 固定为 `http://collector:3001` |
| 用全局 URL 选择 OpenCLI | 使用任务 `engine=opencli` 与独立 `OPENCLI_BRIDGE_BASE_URL` |
| 宿主机 Collector `3100` | Playwright 改为 `3001`；`3100` 只保留给 OpenCLI Bridge |

`COLLECTOR_BASE_URL` 目前只作为 Playwright 地址的旧变量兼容入口；新部署应使用
`COLLECTOR_PLAYWRIGHT_BASE_URL`。不要把兼容变量指向 OpenCLI Bridge。

## 相关文档

- [development.md](development.md)：本地开发与分服务调试。
- [docker-deployment.md](docker-deployment.md)：完整 Compose 部署与运维。
- [env.md](env.md)：全部环境变量与敏感配置规则。
- [provider.md](provider.md)：Collector Provider 与引擎扩展边界。
- [api.md](api.md)：任务、批次与引擎状态 API 契约。
