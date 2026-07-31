# TradeMind Chrome 侧边栏采集扩展

这是 TradeMind 内置的 Manifest V3 浏览器采集入口。它在用户当前打开的淘宝/天猫商品页中读取商品信息，并把结果提交给 TradeMind Backend 创建商品草稿。

它不依赖 OpenCLI daemon、`host.docker.internal:3100` 或额外启动的浏览器进程。Playwright Collector 仍保留，用于后台采集和备用路径。

当前为源码预览版。面向普通用户的一键安装仍需要后续发布签名扩展包或上架
Chrome Web Store / Edge Add-ons。

## 本地构建

```bash
pnpm install
pnpm build:browser-extension
```

构建产物位于 `browser-extension/dist`。在 Chrome 或 Edge 的扩展管理页打开“开发者模式”，选择“加载已解压的扩展程序”，然后选择该目录。

本地和 Docker 使用同一套连接方式：扩展连接 Admin 当前 origin，Docker Admin
通过 `/api` 反向代理 Backend，不需要修改 Compose 或增加环境变量。

## 配对

1. 登录 TradeMind 管理端。
2. 打开“采集 → 浏览器扩展”。
3. 生成一次性连接信息并复制。
4. 打开扩展侧边栏，粘贴后点击“连接”。
5. 打开淘宝或天猫商品详情页，点击“采集当前商品”。

配对码 10 分钟内有效且只能使用一次。扩展得到的是单独、可撤销、90 天有效的设备令牌，不会读取或保存 Admin 登录令牌。

## 权限边界

- 页面权限仅覆盖已支持的淘宝/天猫商品详情域名。
- 默认网络权限仅覆盖 `localhost` 与 `127.0.0.1`。
- 连接远程 TradeMind 时必须使用 HTTPS，Chrome 会针对对应地址请求一次额外权限。
- 扩展不申请 Cookie、下载、调试器或全站读写权限。
- 所有适配器均随扩展打包，不远程加载 JavaScript。

完整使用、限制和排错说明见
[`docs/browser-extension-collector.md`](../docs/browser-extension-collector.md)。
