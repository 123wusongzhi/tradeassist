# 浏览器侧边栏采集

> 状态：2026-07-31。当前为源码预览版，首版支持在 Chrome / Edge 当前打开的
> 淘宝、天猫商品详情页中采集。正式面向普通用户分发前，还需要发布签名扩展包或上架
> Chrome Web Store / Edge Add-ons。

## 适用场景

浏览器侧边栏是淘宝/天猫日常单商品采集的推荐入口。用户继续使用自己已经登录的平台
页面，在侧边栏点击一次即可创建 TradeMind 商品草稿，不需要：

- 启动 OpenCLI daemon 或宿主机 Bridge；
- 配置 `host.docker.internal:3100`；
- 维护第二套浏览器登录态；
- 让 Backend 主动打开或聚焦浏览器。

Playwright Collector 仍然保留，用于后台任务、批量采集和其他平台；OpenCLI 继续作为
可选兼容引擎，不是安装浏览器扩展的前置条件。

## 数据流与故障边界

```text
当前淘宝/天猫标签页
  │  用户在侧边栏点击“采集当前商品”
  ▼
TradeMind 浏览器扩展（打包内置适配器）
  │  专用设备令牌；HTTPS / 当前 Admin 同源地址
  ▼
TradeMind Backend
  ├─ 校验设备、租户、任务归属和归一化结果
  └─ 创建 collect_task 与商品草稿
```

扩展采集不经过 Redis、Playwright Collector 或 OpenCLI Bridge。扩展失效只影响用户
当前页采集，不影响 Playwright 后台任务；Collector 容器失效也不阻止已连接扩展提交
当前页结果。

## 构建与安装

在仓库根目录执行：

```bash
pnpm install
pnpm build:browser-extension
```

构建产物位于 `browser-extension/dist`。

Chrome：

1. 打开 `chrome://extensions`。
2. 打开“开发者模式”。
3. 选择“加载已解压的扩展程序”。
4. 选择仓库中的 `browser-extension/dist`。

Edge 使用 `edge://extensions`，其余步骤相同。重新构建后，在扩展管理页点击刷新。

## 首次连接

1. 登录 TradeMind Admin。
2. 打开“采集 → 浏览器扩展”。
3. 点击“生成连接信息”，再点击“复制连接信息”。
4. 点击浏览器工具栏中的 TradeMind 扩展，打开侧边栏。
5. 粘贴连接信息并点击“连接”。

连接信息包含当前 TradeMind 地址和一次性配对码。配对码 10 分钟内有效且只能交换
一次；扩展得到一个 90 天有效、可随时撤销的专用设备令牌，不会获得或保存 Admin
登录令牌。

## 日常采集

1. 在已连接扩展的同一浏览器中打开淘宝或天猫商品详情页。
2. 完成登录、验证码或页面确认，确保页面内容已经正常显示。
3. 打开 TradeMind 侧边栏，点击“采集当前商品”。
4. 等待“读取页面 → 提交结果 → 创建草稿”完成，点击结果链接打开商品草稿。

首版会采集标题、价格、主图、详情图、属性和可识别的 SKU 组合。动态页面或平台结构
变化可能产生 warning；采集前后扩展会尽量恢复滚动位置和已选 SKU，但不应把扩展
当作无人值守批量机器人。

## 本地与 Docker

| TradeMind 部署 | 扩展连接地址 | 额外采集进程 |
| --- | --- | --- |
| 本地开发 | 当前 Admin 地址，例如 `http://127.0.0.1:8000` | 无 |
| Docker Compose | 当前 Admin 地址，例如 `http://127.0.0.1:8000` | 无 |
| HTTPS / 远程部署 | 用户实际打开的 Admin HTTPS 地址 | 无 |

Admin 会把自身 `window.location.origin` 写入连接信息。Docker 的 Admin Nginx 已将
`/api` 代理到 Backend，因此本地与 Docker 使用同一套配对和采集协议，不需要改
Compose，也没有新增环境变量。

远程连接时，浏览器会针对该 TradeMind 地址请求一次网络权限。生产或公网部署必须
使用 HTTPS；不要让用户连接到来源不明的 TradeMind 地址。

## 安全边界

- 一次性配对码和设备令牌在服务端都只保存 SHA-256 哈希。
- 首次配对交换只允许精确接口上的无 Cookie 请求；携带 Cookie 的未知扩展来源仍受
  CSRF 来源校验。
- 设备令牌限定到创建它的租户和 Admin 用户，可在“已连接设备”中立即撤销。
- 配对、撤销和设备采集都要求该 Admin 用户当前仍处于启用状态并拥有商品写入权限；
  只读账号不能借助旧设备令牌创建草稿。
- 任务同时校验租户、设备 ID 与任务归属；其他设备不能提交该任务的结果。
- 单次归一化结果最大 3 MiB，并限制标题、图片、属性、SKU 和 raw 快照大小。
- 扩展不申请 Cookie、下载、调试器权限，也不读取 Admin JWT。
- 页面读取权限只覆盖已支持的淘宝/天猫域名；远程 TradeMind 地址在配对时按具体
  origin 单独申请。
- 非 loopback 地址强制使用 HTTPS；可选网络权限不包含任意远程 HTTP。
- 平台适配器随扩展构建产物发布，不允许运行时远程加载 JavaScript。

## 当前限制

- 只支持淘宝/天猫当前页单商品采集。
- 不支持扩展侧批量、定时或无人值守采集。
- 后端任务页的通用“重试”不会重新控制用户页面；失败后应回到扩展再次点击采集。
- 源码安装需要手动加载 `dist`。面向非技术用户的真正“一次安装”需要后续完成
  扩展商店上架、签名发布、版本更新和隐私说明。

## 排错

### 提示“当前页面暂不支持”

确认当前标签页是 `https` 的淘宝/天猫商品详情页，而不是搜索、登录、验证码或店铺
首页。平台新增域名时必须通过代码发布增加精确权限，不能远程下发脚本绕过审核。

### 连接失败

1. 确认 TradeMind Admin 与 Backend 可正常访问。
2. 重新生成连接信息；旧配对码可能已过期或已使用。
3. 远程部署确认使用 HTTPS，且已允许扩展访问该具体地址。
4. 在 Admin“已连接设备”确认设备未撤销、未过期。

### 页面能打开但结果不完整

先在当前标签页完成登录或验证并滚动到商品内容。若页面结构已变化，保留任务错误码和
页面类型信息，更新 `browser-extension/src/adapters/` 中的打包适配器并补回归测试。

## 开发与验证

```bash
pnpm build:browser-extension
pnpm test:browser-extension
pnpm quality:browser-extension
```

新增平台时应新增独立适配器和精确 `host_permissions`，同步 Backend source 校验、
API 契约、Admin 文案、文档和测试。详细三路采集边界见
[采集引擎与部署指南](collector-engines.md)。
