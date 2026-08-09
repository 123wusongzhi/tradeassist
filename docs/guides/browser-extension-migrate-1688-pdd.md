# 迁移指南：1688 / 拼多多采集（Playwright → 浏览器扩展）

> 本文档是给执行迁移的 AI Agent / 开发者的**操作手册**，只描述"怎么改、改成什么样"，
> 本身不改动任何代码。所有代码片段均为实现指引（伪代码级），实际落地时以
> `browser-extension/src/adapters/taobao-tmall.ts`（已完成迁移的先例）为最高参照。
>
> 关联文档：[浏览器侧边栏采集](../browser-extension-collector.md)、
> [采集引擎与部署指南](../collector-engines.md)、[1688 采集已知问题](../collector-1688-pitfalls.md)。

## 1. 目标与现状

### 1.1 目标

把 `collector/src/providers/source1688/` 与 `collector/src/providers/sourcePinduoduo/`
两套 **Playwright 引擎**采集，迁移为 **TradeMind 浏览器扩展**的打包适配器，让用户在
自己已登录的 1688 / 拼多多批发（pifa）详情页上点击一次即可采集并创建商品草稿，
流程与现有淘宝/天猫扩展完全一致。

### 1.2 已完成的先例（迁移范本）

| 阶段 | 淘宝/天猫（已完成） | 本次目标 |
| --- | --- | --- |
| Playwright 源 | `collector/src/providers/sourceTaobaoTmall/` | `collector/src/providers/source1688/`、`sourcePinduoduo/` |
| 扩展适配器 | `browser-extension/src/adapters/taobao-tmall.ts` | 新增 `adapters/alibaba-1688.ts`、`adapters/pinduoduo.ts` |
| source 值 | `taobao_tmall` | `1688`、`pinduoduo` |
| 后端 task 支持 | `CreateBrowserExtensionTask` 仅放行 `taobao_tmall` | 放行新 source |

迁移方式是"移植"而非"复用"：扩展里 `chrome.scripting.executeScript` 会把采集函数
**序列化成源码后在页面上下文执行**，因此 collector 里所有"在页面内跑"的逻辑必须
内联进单个自包含函数；collector 里的 Node 侧解析逻辑也要搬进函数体内（或精简后
搬入）。这是本次迁移最核心、最反直觉的约束，见 §3。

### 1.3 三方现状速览

**扩展端（browser-extension/，约 2 千行 TS）**

| 文件 | 职责 | 迁移时需要动 |
| --- | --- | --- |
| `src/adapters/types.ts` | `BrowserCollectAdapter` 接口（id/label/supports/collect） | 不改 |
| `src/adapters/registry.ts` | 适配器注册表 `adapters[]` + `adapterForURL` + `supportedAdapters` | 注册两个新适配器 |
| `src/adapters/taobao-tmall.ts` | 淘宝/天猫适配器（模块级纯函数 + 自包含 `collectTaobaoTmallPage`） | 不动，作范本 |
| `src/service-worker.ts` | `chrome.scripting.executeScript` 注入（`world: 'MAIN'`）、错误解析 | 仅文案泛化 |
| `src/sidepanel.ts` | 侧边栏 UI：配对、页面支持判断、采集流程、SKU 分析 | `isSupportedTaobaoTmallURL` 调用点泛化 |
| `src/api.ts` | TradeMind API 客户端 | `createTask` 的 `source` 硬编码需参数化 |
| `src/types.ts` | `NormalizedProduct.source` 目前是字面量 `'taobao_tmall'` | 扩为联合类型 |
| `public/manifest.json` | MV3 清单，`host_permissions` 仅淘宝/天猫 6 域 + localhost | 增加 1688/pdd 域名 |
| `src/adapters/*.test.ts`、`api.test.ts`、`pairing.test.ts` | vitest 单测 | 新增/更新 |

**collector 端（待迁移源）**

- `source1688/`：`alibaba-1688.ts`（Provider 入口，sourceId=`'1688'`）、
  `browser-extract-1688.ts`（**页内纯 DOM 抽取**，`extract1688DomInPage` 一次
  `page.evaluate` 拿全部 DOM 数据）、`parser.ts` + `context-parse.ts`（**Node 侧**
  script JSON 挖掘与组装）、`price-extract.ts` / `sku-helpers.ts` / `image-extract.ts`
  （**Node 侧纯函数**）、`selectors.ts`（全部选择器常量）、`page-prep.ts`（等待+滚动）、
  `auth-detect.ts`（登录态检测）、`debug-snapshot.ts`（失败快照，扩展不需要）。
  特点：**零点击、零接口、零网络监听**，数据 100% 来自 DOM + 页面 script JSON。
- `sourcePinduoduo/`：`index.ts`（Provider 入口，sourceId=`'pinduoduo'`，注册别名
  `pdd`）、`wholesale-detail.ts`（批发详情编排）、`wholesale-detail-extract.ts`
  （**页内纯 DOM 抽取**，`extractPifaWholesaleDetailInPage`，布局启发式）、
  `wholesale-detail-gallery.ts`（**页内交互式图集**：模拟点击箭头/缩略图 + 滚动，
  helper 是手写字符串注入 `window.__pddGallery`）、`wholesale-detail-images.ts` /
  `wholesale-detail-shared.ts`（**Node 侧纯函数**：图片后处理/标题清洗/SKU 行转对象）、
  `access-detect.ts` / `auth-detect.ts`（登录/微信授权/验证码检测）、`url-type.ts` /
  `validate-url.ts`（URL 分类与校验）。
  特点：无 `page.click`/`waitForSelector`/网络监听，所有"点击"都是 `page.evaluate`
  内的 `element.click()`；**不读任何 XHR**。

**后端（backend/，必须配套改，否则扩展提交被拒）**

| 文件 | 现状 | 需改 |
| --- | --- | --- |
| `internal/modules/collect/browser_extension_task.go` | `CreateBrowserExtensionTask` 只放行 `isTaobaoTmallCollectSource`（:203）并校验 `validateTaobaoTmallCollectURL`（:206）；`CompleteBrowserExtensionTask` 再次校验 source（:286）并调 `normalizeTaobaoTmallImport`（:293） | source 白名单、URL 校验、导入归一化按 source 分发 |
| `internal/modules/collect/taobao_tmall_profile.go` | `isTaobaoTmallCollectSource`（:14，`taobao_tmall`\|`taobao`） | 扩展一个"浏览器扩展支持 source"判定 |
| `internal/modules/collect/taobao_tmall_url_validate.go` | `validateTaobaoTmallCollectURL` 用 `collectdomain.ClassifyTaobaoTmallURL` | 新增 1688/pdd 的 URL 校验函数 |
| `internal/modules/collect/taobao_tmall_import_normalize.go` | 图片 URL 清理（`_.jpg` 后缀等**淘宝专属**规则）、无 SKU 时默认规格回填、`mergeRawExtractProvider` | 新增 1688/pdd 的归一化（规则不同，不可复用淘宝图片清理） |
| `internal/pkg/collectdomain/platform.go` | `DetectPlatform` 已识别 `1688`/`pinduoduo`（:68-69） | 可直接复用；无需改 |

## 2. 数据流（迁移后形态）

```text
用户在 1688 / pifa.pinduoduo.com 详情页（已登录）
  │ 侧边栏点击“采集当前商品”
  ▼
sidepanel.ts ──createTask(url, source)──▶ backend（source 白名单 + URL 校验）
  │ chrome.runtime.sendMessage(COLLECT_ACTIVE_TAB)
  ▼
service-worker.ts ──executeScript({ func: adapter.collect, world: 'MAIN' })──▶ 页面
  │ 自包含函数在页面上下文执行（可读 DOM + 页面全局变量）
  ▼
NormalizedProduct（source: '1688' | 'pinduoduo'）
  │ submitResult(taskId, product)
  ▼
backend：按 source 归一化（图片清理规则不同）→ 导入商品草稿 → 任务成功
```

## 3. 三条硬约束（所有改动必须遵守）

1. **自包含序列化**：`chrome.scripting.executeScript` 把 `adapter.collect` 的函数体
   序列化后注入页面，**函数外的模块闭包、常量、import 全部不可达**。所以：
   - 所有页面内辅助逻辑（等待、DOM 抽取、JSON 解析、正则、常量）必须**定义在
     collect 函数体内**；
   - `adapter.collect` 必须**直接引用**完整函数（`collect: collectAlibaba1688Page`），
     禁止箭头包装（`collect: (o) => collectAlibaba1688Page(o)` 会触发
     ReferenceError / EMPTY_RESULT，先例见 `docs/browser-extension-collector.md` 与
     `taobao-tmall.ts:842-848` 的注释）。
   - 模块级可以**同时保留一份同名纯函数**（供 vitest 直接 import 测试），
     参照 `taobao-tmall.ts` 的 `buildSkusFromTaobaoSkuBase`（:63）与
     `collectTaobaoTmallPage` 内联版（:423-551）双份共存的模式。
2. **MAIN world**：读页面全局变量（1688 的 `window.context`、PDD 的
   `window.__INITIAL_STATE__/rawData/store`）必须 `world: 'MAIN'`
   （service-worker.ts:54 已设置，勿改）。
3. **精确域名权限**：manifest 只加目标平台精确 `host_permissions`；禁止
   `https://*/*` 常驻或远程下发脚本。新增域名必须随版本发布（安全边界，
   见 `docs/browser-extension-collector.md` 排错节）。

## 4. 第一步：通用骨架改造（不依赖具体平台，建议先做）

以下改动与淘宝先例兼容，全部是"参数化/泛化"，不改变现有行为。

### 4.1 `browser-extension/src/types.ts`

`NormalizedProduct.source` 目前是字面量 `'taobao_tmall'`，改为联合类型：

```ts
export type CollectSource = 'taobao_tmall' | '1688' | 'pinduoduo';
// NormalizedProduct.source: CollectSource
```

（若后端未来还有自定义平台，可再放宽为 `string`，但当前联合类型最安全。）

### 4.2 `browser-extension/src/adapters/registry.ts`

```ts
import { taobaoTmallAdapter } from './taobao-tmall.js';
import { alibaba1688Adapter } from './alibaba-1688.js';
import { pinduoduoAdapter } from './pinduoduo.js';

const adapters: BrowserCollectAdapter[] = [
  taobaoTmallAdapter,
  alibaba1688Adapter,   // 新增
  pinduoduoAdapter,     // 新增
];
```

`adapterForURL` / `supportedAdapters` 无需改逻辑，自动覆盖新平台。

### 4.3 `browser-extension/src/api.ts`

`createTask` 目前硬编码 `source: 'taobao_tmall'`，改为参数：

```ts
async createTask(url: string, source: string) {
  return this.request<CollectTask>('/api/v1/collect/browser-extension/tasks', {
    method: 'POST',
    body: JSON.stringify({ source, url }),
  });
}
```

调用点（sidepanel.ts 的 `collectCurrentPage`）改为
`api.createTask(activePage.url, adapterForURL(activePage.url)?.id ?? 'taobao_tmall')`
——注意 sidepanel 此时需要 import `adapterForURL` 而不是 `isSupportedTaobaoTmallURL`。

### 4.4 `browser-extension/src/sidepanel.ts`

- `refreshActivePage`（:261-275）里 `supported: isSupportedTaobaoTmallURL(tab.url)`
  改为 `supported: Boolean(adapterForURL(tab.url))`，可顺带记录
  `adapterId: adapterForURL(tab.url)?.id` 供 createTask 使用。
- `ui.pageBadge` 的“可以采集/不支持”文案无需改；但“不支持”时的提示语（若有）
  泛化为"仅支持淘宝/天猫、1688、拼多多批发商品详情页"。
- `collectCurrentPage` 中 `api.createTask(activePage.url)` 传 source（见 4.3）。
- 侧边栏 SKU 分析面板与 `skuPriceProbeInput` 保持现状：1688/pdd 适配器忽略
  `maxPriceProbes` 参数即可（无 SKU 价格探测，见 §5/§6）。

### 4.5 `browser-extension/src/service-worker.ts`

- `collect()`（:30-68）无需改逻辑（已按 `adapterForURL` 分发、`world:'MAIN'`）。
- 错误文案"当前仅支持淘宝或天猫商品详情页"（:39）泛化为平台列表；
- `errorResult`（:13-28）的 `CODE: message` 解析保持——新适配器所有失败都抛
  `CODE: 中文提示` 格式的错误，与淘宝一致。

### 4.6 `browser-extension/public/manifest.json`

`host_permissions` 增加（最小权限，参考淘宝 6 域粒度）：

```jsonc
// 1688：详情页标准域。m.1688.com 可选（见 §5.5 说明）
"https://detail.1688.com/*",
"https://m.1688.com/*",            // 若需支持移动端 offer 链接
// 拼多多：只支持批发（pifa）详情，移动端 goods_detail 不在范围
"https://pifa.pinduoduo.com/*",
"https://*.pifa.pinduoduo.com/*",  // 若实际域名含子域则保留
```

同时更新 `manifest.json` 的 description 与 `public/sidepanel.html` 中任何写死
"淘宝/天猫"的文案（`grep -rn "淘宝\|天猫" browser-extension/src browser-extension/public`）。

### 4.7 后端通用放行（详见 §7）

- `browser_extension_task.go:203`：`isTaobaoTmallCollectSource` → 新的
  `isBrowserExtensionSupportedSource`（含 `taobao_tmall`/`taobao`/`1688`/`pinduoduo`/`pdd`）；
- `browser_extension_task.go:206`：URL 校验按 source 分发（§7.2）；
- `browser_extension_task.go:286-293`：source 白名单与导入归一化分发（§7.3）。

> 做完 §4 后必须能通过：`pnpm build:browser-extension`、`pnpm test:browser-extension`，
> 且淘宝/天猫真机采集回归通过（构建产物加载到 Chrome 手测一次）。

## 5. 第二步：1688 适配器（`browser-extension/src/adapters/alibaba-1688.ts`）

### 5.1 可复用资产清单（collector → 扩展的搬运方式）

| collector 文件:函数 | 角色 | 搬运方式 |
| --- | --- | --- |
| `browser-extract-1688.ts:33` `extract1688DomInPage` | 页内 DOM 抽取（标题/主图/详情图/属性/SKU 维度/价格文本/阻塞标记），**纯函数可序列化** | 整体内联进 collect 函数体；其内部 helper（`pickImgUrl`、噪声过滤、`text()` 等）一并内联。**先例**：`taobao-tmall.ts` 的内联复制模式 |
| `selectors.ts` | 全部选择器常量 | 复制为 collect 体内的 const 数组（或函数体内常量，不进模块闭包） |
| `page-prep.ts:7-18` | 随机等待 + 核心选择器等待 + 分段滚动懒加载 | 等待改为轮询循环（§5.3）；滚动逻辑原样内联（纯 window API） |
| `parser.ts`（`parseJsonFragmentsFromScripts`/`assembleParsedProduct` 等） | script JSON 挖掘 + 多源合并组装 | 内联，但**裁剪**：去掉 debug 字段（`extractDebug`、`scriptDigest`、候选原文）以控制函数体积与 raw 体积（3 MiB 上限） |
| `context-parse.ts`（`find1688ResultData` 等） | `result.data` 模块树提取价格/图/属性/SKU | 内联（深度优先遍历是纯逻辑） |
| `price-extract.ts` | 价格白名单/黑名单/递归读取 | 内联（**必须保留 `unitWeight` 黑名单**，见 `docs/collector-1688-pitfalls.md` §2） |
| `sku-helpers.ts` | SKU 桶键解析（`颜色:蓝;尺码:M`）、价格/库存提取、DOM 表补全 | 内联 |
| `image-extract.ts` | 图 URL 分类/去重/兜底 | 内联 |
| `auth-detect.ts` / `alibaba-1688.ts` 的阻塞检测 | 登录/验证码正则与判定 | 内联正则 + `location.href` 判断（§5.4） |
| `debug-snapshot.ts` | 失败 HTML/截图快照 | **不需要**：扩展无法写文件，raw 里的错误信息足够 |
| `alibaba-1688.ts` 的 `normalizeOfferNavUrl` | URL 归一化导航 | **不需要**：用户已在最终页面上，无需导航 |

### 5.2 Playwright API → 浏览器 API 替代映射（1688 用到的全部）

| Playwright | 1688 使用处 | 扩展替代 |
| --- | --- | --- |
| `page.goto(url, ...)` + fallback | alibaba-1688.ts:121,128 | **删除**（用户已在页面） |
| `page.waitForLoadState('networkidle')` | :236 | 删除或保留一次短等待（页面已加载完成） |
| `page.waitForTimeout(ms)` | page-prep.ts:9,17 | `const sleep = (ms) => new Promise(r => setTimeout(r, ms))`（函数体内定义，先例 taobao-tmall.ts:243） |
| `page.waitForSelector(sel, {timeout})` | page-prep.ts:11 | 轮询循环：`while (!document.querySelector(...) && Date.now()-start < 12000) await sleep(250)` |
| `page.evaluate(fn, arg)`（DOM 抽取） | browser-extract-1688.ts:427 | **直接调用**（collect 函数本身就在页面上下文） |
| `page.evaluate`（滚动） | page-prep.ts:21 | 原样 `window.scrollTo` |
| `page.url()` | alibaba-1688.ts:239,247 | `location.href` |
| `page.content()` / `page.screenshot()` | debug-snapshot.ts | 删除 |
| `page.setDefaultTimeout` 等 | session-manager | 删除（无超时框架，用轮询 deadline 自行控制） |

### 5.3 `collectAlibaba1688Page` 函数骨架（实现指引）

签名与淘宝对齐：`async function collectAlibaba1688Page(options?: { maxPriceProbes?: number }): Promise<NormalizedProduct>`。
`maxPriceProbes` 仅用于兼容调用方，1688 无逐 SKU 探测，可忽略。

函数体内按顺序实现以下段落（每段都是"把 collector 对应代码内联 + 改 API"）：

1. **内联 helper 区**：`sleep`、`text`、`firstText`、`fail(code, msg)`（抛
   `new Error(\`${code}: ${msg}\`)`）、`pickImgUrl`、噪声过滤、`parsePrice`、
   `parseQuantity`、`normalizeImage`、`uniqueImages`——全部复制自
   `browser-extract-1688.ts` / `taobao-tmall.ts` 函数体内版本。
2. **宿主校验**：`location.hostname` 不是 `1688.com`/`*.1688.com` → `fail('UNSUPPORTED_PAGE', '当前标签页不是 1688 商品详情页')`。
3. **核心元素等待**：轮询 `h1, [class*="title"], [class*="price"], [class*="gallery"], [class*="sku"]`
   （selectors.ts 的 CORE_SELECTORS），12s 超时（可失败继续，与 collector 一致）。
4. **阻塞检测（URL 级）**：`location.href` 匹配
   `punish|x5secdata|captcha|_____tmd_____|sec\.1688\.com.*(verify|captcha)` 或
   （跳转 `passport.1688.com|login.1688.com` 且当前不是 offer 路径）→
   `fail('LOGIN_REQUIRED'|'VERIFY_REQUIRED', ...)`（对应 collector 的
   `isCaptchaRedirectUrl`，alibaba-1688.ts:78-92）。
5. **分段滚动懒加载**：`window.scrollTo(0, y)` 步进 0.75 视口高、每步 `sleep(350)`、
   最多 12000px，回到顶部（page-prep.ts:20-33 原样内联）。
6. **DOM 抽取**：把 `extract1688DomInPage` 整体内联为一个函数体内函数
   `extractDom()`，返回 payload（headingText/docTitle/meta/galleryUrls/detailUrls/
   paramPairs/domSkuDimensions/domSkuTableRows/domPriceTexts/scriptSnippets/
   `__blocked__`）。
7. **scriptSnippets 收集**：内联 browser-extract-1688.ts:309-364——
   `document.scripts` 遍历（含关键字 `skuMap|skuProps|...` 的截断进 snippets，
   最多 14 段 × 120KB）、`window.context` JSON.stringify、全局白名单
   `__INIT_DATA / __INITIAL_STATE__ / detailData / offerDetailData / iDetailConfig / OFFER_DETAIL`。
8. **JSON 解析与组装**：内联 `parser.ts` 的片段 JSON 解析（`tryParseLeadingJsonObject`）、
   `find1688ResultData`（context-parse.ts:17-45）、价格/图/属性/SKU 提取与合并
   （**裁剪 debug 输出**），产出 title/price/mainImages/descriptionImages/attributes/skus。
9. **主图空则重试一次**：`mainImages.length === 0` 时重复第 5-8 段一次
   （collector 的 alibaba-1688.ts:258-261）。
10. **二次阻塞检测（页面级）**：body 前 3500 字符匹配
    `安全验证|请完成验证|访问过于频繁|captcha|滑块验证|人机验证|nc-container|punish-page`
    → `fail('VERIFY_REQUIRED', ...)`；`请登录|账号登录` 且无商品内容 →
    `fail('LOGIN_REQUIRED', ...)`（对应 `isBlockedPage` + `isStrictCaptchaSurface`）。
11. **字段缺失判定与警告**：title/price/images/sku 四项缺失汇总（对应
    `fieldMissingSummary`），生成 `qualityWarnings` 数组（复用淘宝的代码风格：
    `PRICE_NOT_FOUND`、`MAIN_IMAGES_EMPTY`、`SKU_INCOMPLETE` 等，错误码沿用
    collector 1688 的语义）。
12. **返回**：

```ts
return {
  source: '1688',
  sourceUrl: location.href,
  title,
  currency: 'CNY',
  mainDescription: '',            // collector 1688 未填，保持空串
  mainImages,
  descriptionImages,
  attributes,
  skus,
  raw: {
    provider: 'browser_extension',
    schemaVersion: 2,
    capturedAt: new Date().toISOString(),
    pageTitle: document.title,
    finalUrl: location.href,
    productPrice,
    priceText,
    qualityWarnings,
    collectStatus,                // 'success' | 'partial_success'
    // 注意：不要带 scriptSnippets 原文 / extractDebug，控制 raw ≤ 3 MiB
  },
};
```

末尾导出：

```ts
export const alibaba1688Adapter: BrowserCollectAdapter = {
  id: '1688',
  label: '1688',
  supports: isSupportedAlibaba1688URL,   // 模块级纯函数，可用 vitest 测
  collect: collectAlibaba1688Page,       // 直接引用，禁止箭头包装
};
```

### 5.4 `supports()` 与 URL 校验（模块级纯函数，可单测）

对应 collector `alibaba-1688.ts:14-30`：

```ts
function is1688Host(hostname: string): boolean {
  return hostname === '1688.com' || hostname.endsWith('.1688.com');
}
function isLikelyOfferPath(pathname: string, search: string): boolean {
  return /\/offer\/?/i.test(pathname)
    || /offerId=/.test(search)
    || /\/offer(?:id)?\.html$/i.test(pathname);
}
export function isSupportedAlibaba1688URL(raw: string): boolean {
  try {
    const url = new URL(raw);
    return url.protocol === 'https:' && is1688Host(url.hostname.toLowerCase())
      && isLikelyOfferPath(url.pathname, url.search);
  } catch { return false; }
}
```

注意：与 collector 一致**只接受 https**；`http://` 一律 false（淘宝先例同款）。

### 5.5 注意事项（1688 特有）

- **SKU 无点击**：1688 全部从 script JSON + DOM 文本读 SKU（桶键
  `颜色:蓝;尺码:M`），不要引入任何规格点击逻辑；`maxPriceProbes` 忽略。
- **价格黑名单**：`unitWeight`/`canBookCount`/`amountOnSale`/`skuId` 等非价格键
  必须保留（collector 采过 `unitWeight: 39` 的坑，见 pitfalls 文档 §2）。
- **SKU 键解析**：`parseComboKey` 的 `;；#` 分段 + `>`/`»` 两段式 + 维度名推断逻辑
  必须原样保留（pitfalls §3/§4）。
- **`m.1688.com`**：collector 支持 `normalizeOfferNavUrl` 把 m 站链接归一到
  detail 域再导航；扩展里用户停留在哪个域就采哪个域——`supports()` 可同时接受
  `m.1688.com`（pathname 含 `/offer/`），manifest 相应加 `https://m.1688.com/*`。
  若首版只支持 detail 域，`supports()` 只认 `detail.1688.com` 即可，README 里注明。
- **登录态**：用户在浏览器已登录 1688 即视为已登录；采集函数内不做 cookie 检测。
- **失败快照**：不做 HTML/截图快照（collector 的 debug-snapshot 职责在扩展中
  由"错误码 + 侧边栏 toast"替代）。

## 6. 第三步：拼多多适配器（`browser-extension/src/adapters/pinduoduo.ts`）

### 6.1 可复用资产清单

| collector 文件:函数 | 角色 | 搬运方式 |
| --- | --- | --- |
| `wholesale-detail-extract.ts:35` `extractPifaWholesaleDetailInPage` | 页内 DOM 抽取（标题候选/价格文本/主图容器启发式/SKU 行/属性/intro 文本），纯函数 | 整体内联（含 `findMainGalleryRoot`、`collectMediaInRoot`、`pickImgUrl`、垃圾图过滤等 helper） |
| `wholesale-detail-gallery.ts`（`GALLERY_HELPERS` :13-271、`waitForMainGalleryReady` :277-302、`collectInteractiveGalleryImages` :304-385、`scrollAndCollectDetailImages` :387-433） | 交互式图集：等待画廊 → 点"下一页"箭头 ≤5 次 + 缩略图 ≤12 次 → 点详情 tab + 滚动 10 轮 | 逻辑内联（helper 字符串机制不需要了：函数体里直接定义闭包函数，点 `element.click()` + `sleep`）；`waitForFunction` → 轮询 |
| `wholesale-detail-images.ts`（`classifyRegionImages` :403-567、`normalizePddImageList` :581-600、`upgradePddImageSize` :101-106） | Node 侧图片后处理：尺寸升级（imageView2）、去重、分类、兜底链 | 内联（纯字符串/数组逻辑） |
| `wholesale-detail-shared.ts`（`cleanProductTitle`、`parsePriceRangeText`、`wholesaleRowsToSkus`、`buildMainDescription`、警告码） | Node 侧工具 | 内联 |
| `wholesale-detail.ts`（`extractAndAssemblePifaWholesale` :283-298、`mergePayloadImages`、`assemblePifaWholesaleProduct` :139-281、`validateWholesaleCollectQuality` :300-331） | 编排与组装 | 按顺序内联进 collect 函数体（无 playwright 依赖） |
| `access-detect.ts`（`detectPinduoduoAccessStatus` :93-175 + `PIFA_LOGIN_TEXT_RE` 等） | 登录/微信授权/验证码/下架/App 引导检测 | 内联正则 + `location.href`/`document.body.innerText` 判断（§6.4） |
| `url-type.ts` / `validate-url.ts` | URL 分类/校验（只收 `wholesale_detail`） | 内联进 `supports()`（模块级纯函数） |
| `parser.ts`（通用路径） | **不可达分支**（canHandle 只放行 wholesale_detail） | **不迁移** |
| `auth-detect.ts`（登录态检测接口用） | 供 session-manager，不参与采集 | **不迁移** |

### 6.2 Playwright API → 浏览器 API 替代映射（PDD 用到的全部）

| Playwright | PDD 使用处 | 扩展替代 |
| --- | --- | --- |
| `page.goto` + 跳转后 URL 再分类 | index.ts:107,117 | 删除；`location.href` 做一次最终分类检查即可 |
| `page.waitForLoadState('networkidle')` / `waitForTimeout(800)` | index.ts:114-115 | 删除/保留一次短 sleep |
| `page.evaluate`（注入 helper + 点击 + 滚动） | gallery.ts:274-433 全部 | 直接调用（函数体即页面上下文）；`element.click()` + `sleep` 原样 |
| `page.waitForFunction(fn, {timeout})` | gallery.ts:287-299 | 轮询循环（与 1688 的 waitForSelector 替代同款） |
| `page.url()` | index.ts:117,163；access-detect.ts:98 | `location.href` |
| `context.route`（SSRF 守卫） | index.ts:196 | **不需要**（扩展不发起任意网络请求；manifest 精确 host 权限即边界） |
| `page.close()` / `context.close()` | index.ts:204-205 | 删除 |

### 6.3 `collectPinduoduoWholesalePage` 函数骨架

签名与淘宝对齐：`async function collectPinduoduoWholesalePage(options?: { maxPriceProbes?: number }): Promise<NormalizedProduct>`（同样忽略 `maxPriceProbes`）。

1. **内联 helper 区**：`sleep`、`text`、`fail`、`pickImgUrl`（extract.ts:101-130）、
   垃圾图正则（:138-154）、`isIrrelevantHint` 等。
2. **宿主校验**：`location.hostname` 不是 `pifa.pinduoduo.com`（或
   `*.pifa.pinduoduo.com`）→ `fail('UNSUPPORTED_PAGE', '当前标签页不是拼多多批发商品详情页')`；
   path 不含 `goods` 或 `gid/goods_id` 非数字 → 同 fail（对应 url-type.ts
   `wholesale_detail` 判定）。
3. **访问检测**：按 access-detect.ts:107-174 的顺序内联判定：
   - `location.href` 命中 `open.weixin.qq.com|weixin.qq.com/connect|wx.qq.com` →
     `fail('WECHAT_AUTH_REQUIRED', '需要微信扫码授权登录后重试')`；
   - host/path 含 `passport|login|auth` → `fail('LOGIN_REQUIRED', ...)`；
   - body/title 前 5000 字符命中 `PIFA_LOGIN_TEXT_RE`
     （`请登录|登录后|需要登录|账号登录|手机登录|验证码登录`）且 body 无
     `goods_id|商品详情|¥|￥` → `fail('LOGIN_REQUIRED', ...)`；
   - `验证码|滑块|安全验证|人机验证|访问受限` → `fail('VERIFY_REQUIRED', ...)`；
   - `商品不存在|已下架|找不到商品|404|商品已售罄` 且 body < 1200 → 
     `fail('PRODUCT_NOT_FOUND', ...)`；
   - `打开app|请在app|下载拼多多|去app内` 且 body < 3500 → 
     `fail('PAGE_BLOCKED_OR_VERIFY_REQUIRED', '请在浏览器中打开商品页后重试')`。
4. **等待画廊就绪**：滚动到画廊区域 + 轮询首图（
   `findMainGalleryRoot` + 容器内 `img` 有 `naturalWidth > 0`），10s 超时
   （对应 gallery.ts:277-302）。
5. **交互式主图采集**：内联 `collectInteractiveGalleryImages`——
   找"下一页"箭头（类名/aria-label 含 `next|right|arrow|下一|›|»|chevron-right|icon-right`）
   最多点 5 次；找可点缩略图最多 12 次，每次 `click()` + `sleep(300~400)` 后取大图
   （`findMainLargeImage`）；同时扫 `window.__INITIAL_STATE__/rawData/store` 的
   `url|src|imageUrl|thumbUrl|picUrl|hdUrl|originUrl|gallery|images|viewImageData|detailGallery`
   键收集图 URL（最多 30 条）。
6. **详情图采集**：内联 `scrollAndCollectDetailImages`——点
   `商品介绍|图文详情|详情` tab（`element.click()` + sleep），滚动 10 轮，
   每轮收集详情区 `img`/背景图（`collectDetailSectionImages`）。
7. **DOM 抽取**：内联 `extractPifaWholesaleDetailInPage` → payload
   （标题候选/价格文本/主图三桶/详情图/intro 文本/SKU 行/属性/specButtonCount）。
8. **图片后处理**：内联 `classifyRegionImages`（og:image → SKU 图 → 详情首图 →
   unknown 池 → 全页池兜底链；`upgradePddImageSize` 去掉 `imageView2` 参数与
   `_NxN` 后缀）、`mergePayloadImages` 三路合并、`normalizePddImageList`。
9. **SKU 组装**：内联 `wholesaleRowsToSkus`（行文本 → `{ properties: {规格: 名称},
   price, image }`；价格缺失用 `priceMin` 兜底并告警 `sku_price_fallback_to_min_price`；
   "仅剩 N 件"→ stock；上限 80 条）；`parsePriceRangeText` 求最低价；
   `cleanProductTitle` 清洗标题；`buildMainDescription` 拼 intro 文本。
10. **质量校验**：内联 `validateWholesaleCollectQuality`——无价格且无图/无 SKU →
    `fail('PARSE_FAILED', ...)`；无价格 → `fail('PARSE_FAILED_PRICE_MISSING', ...)`；
    否则收集 warnings（`SKU_INCOMPLETE`、`SKU_PARSE_FAILED`、
    `DESCRIPTION_IMAGES_INCOMPLETE` 等，沿用 collector 的 19 种警告码语义）。
11. **返回**：

```ts
return {
  source: 'pinduoduo',
  sourceUrl: location.href,
  title,
  currency: 'CNY',
  mainDescription,                  // 批发路径有 intro 文本，不要置空
  mainImages,
  descriptionImages,
  attributes,
  skus,
  raw: {
    provider: 'browser_extension',
    schemaVersion: 2,
    capturedAt: new Date().toISOString(),
    pageTitle: document.title,
    finalUrl: location.href,
    productPrice, priceText,
    priceMin, priceMax, priceRange,
    qualityWarnings,
    collectStatus,                  // 'success' | 'partial_success'
    skuRowCount, specButtonCount, introFound,
    // 不要带 imageDebug / titleCandidates 等大对象，控制 raw ≤ 3 MiB
  },
};
```

末尾导出：

```ts
export const pinduoduoAdapter: BrowserCollectAdapter = {
  id: 'pinduoduo',
  label: '拼多多',
  supports: isSupportedPinduoduoWholesaleURL,
  collect: collectPinduoduoWholesalePage,
};
```

### 6.4 `supports()`（模块级纯函数，可单测）

严格对齐 collector 只收批发详情的现状（`validate-url.ts:18-21` + `url-type.ts:44-49`）：

```ts
export function isSupportedPinduoduoWholesaleURL(raw: string): boolean {
  try {
    const url = new URL(raw);
    const host = url.hostname.toLowerCase();
    const isPifa = host === 'pifa.pinduoduo.com' || host.endsWith('.pifa.pinduoduo.com');
    if (url.protocol !== 'https:' || !isPifa) return false;
    const gid = url.searchParams.get('gid') ?? url.searchParams.get('goods_id') ?? '';
    const hasGoodsPath = /goods/i.test(url.pathname);
    return (hasGoodsPath && /^\d+$/.test(gid)) || /goods\/detail/i.test(url.pathname);
  } catch { return false; }
}
```

> 注意：**不要**把 `mobile.yangkeduo.com/goods.html?goods_id=*` 纳入 supports。
> collector 的 meta 声明支持移动端但 `canHandle` 实际拒绝（index.ts:95），
> 扩展侧保持与"实际可用路径"一致，避免半吊子支持。

### 6.5 注意事项（PDD 特有）

- **SKU 无 skuId**：批发页 SKU 只有 `properties`/`price`/`image`，没有 skuCode、
  没有库存数字（只有"仅剩 N 件"）。后端导入侧不得为 PDD 要求 skuCode。
- **无价格探测**：不要引入任何逐 SKU fetch/点击探测；`maxPriceProbes` 忽略。
- **画廊交互的节流**：点箭头/缩略图之间的 `sleep(300-400ms)` 必须保留，防止触发
  平台风控；每次点击后先确认大图已变化再收集。
- **详情 tab 点击**：找不到"商品介绍/图文详情"tab 时**不要失败**，降级为直接滚动
  收集（collector 行为一致，gallery.ts:390-402）。
- **微信授权**：批发页常见微信扫码登录墙，检测命中时给用户明确提示
  （`WECHAT_AUTH_REQUIRED`），让用户在浏览器完成授权后重试。

## 7. 第四步：后端配套改动（backend/）

> 后端不改，扩展提交的 1688/pdd 任务会被
> `CreateBrowserExtensionTask` 直接拒绝（`browser_extension_task.go:203-204`）。

### 7.1 source 白名单（`internal/modules/collect/`）

在 `taobao_tmall_profile.go` 旁新增一个通用判定（或就地扩展）：

```go
// browserExtensionSourceSupported 放行浏览器扩展支持的 source
func isBrowserExtensionSupportedSource(source string) bool {
    s := strings.TrimSpace(strings.ToLower(source))
    switch s {
    case "taobao_tmall", "taobao", "1688", "pinduoduo", "pdd":
        return true
    }
    return false
}
```

`browser_extension_task.go:203` 与 `:286` 两处把 `isTaobaoTmallCollectSource` 换成它。
注意：`isTaobaoTmallCollectSource` 在 `batch.go`/`service.go`/`engine_router.go` 等处
还有大量调用，**不要动那些**（它们是 Playwright 批量/后台路径，与扩展无关）。

### 7.2 URL 校验按 source 分发

新增 `browser_extension_url_validate.go`（或扩展现有文件）：

```go
func validateBrowserExtensionCollectURL(source, urlStr string) error {
    switch strings.TrimSpace(strings.ToLower(source)) {
    case "taobao_tmall", "taobao":
        return validateTaobaoTmallCollectURL(urlStr)
    case "1688":
        return validateAlibaba1688CollectURL(urlStr)
    case "pinduoduo", "pdd":
        return validatePinduoduoCollectURL(urlStr)
    default:
        return fmt.Errorf("UNSUPPORTED_SOURCE:不支持的采集来源")
    }
}
```

- `validateAlibaba1688CollectURL`：`collectdomain` 的 `HostnameFromURL` +
  `DetectPlatform` 判 `Platform1688`（platform.go:68-69 已有），再校验
  `/offer/` 路径或 `offerId=` 参数；错误信息参照 collector 的
  `INVALID_URL:not_a_1688_product_url` 文案。
- `validatePinduoduoCollectURL`：仅接受 `pifa.pinduoduo.com` 批发详情
  （`gid`/`goods_id` 纯数字 + path 含 `goods`），拒绝移动端
  `yangkeduo.com`；错误信息参照 collector `validate-url.ts` 语义。

### 7.3 导入归一化按 source 分发（`taobao_tmall_import_normalize.go`）

`normalizeTaobaoTmallImport`（:31-70）目前在 `CompleteBrowserExtensionTask` 中被
无条件调用。改造方式：函数开头按 `source` 分发到各自归一化，或新增两个函数：

- **1688**：`normalizeAlibaba1688Import`——图片 URL 清理**不能用淘宝的
  `normalizeTaobaoTmallImageURL`**（`_.jpg` 后缀是淘宝 CDN 规则，1688 的
  `cbu01.alicdn.com` 等是 `?x-oss-process` 参数，只需去掉 query 与 `data:`）；
  `currency` 缺省补 `CNY`；无 SKU 时用 `raw.productPrice` 回填默认规格
  （与淘宝同款逻辑，:50-56）；`mergeRawExtractProvider(fullJSON, "1688", ...)`。
- **pinduoduo**：`normalizePinduoduoImport`——图片 URL 清理调用 PDD 规则：
  去掉 `imageView2` 参数与 `_NxN` 后缀（搬运 `wholesale-detail-images.ts`
  `upgradePddImageSize` 的等价 Go 实现）；`currency` 补 `CNY`；无 SKU 回填默认
  规格；`mergeRawExtractProvider(fullJSON, "pinduoduo", ...)`。
- **保持淘宝路径行为不变**：`normalizeTaobaoTmallImport` 内部对非淘宝 source
  的 `return params, fullJSON`（:33-35）分支保留。

> 检查 `mergeRawExtractProvider` 的实现：确认它接受 extractProvider 字符串参数
> （`browser_extension_task.go` 只用了淘宝，1688/pdd 传各自 source 即可）。

### 7.4 不需要改的部分（确认清单）

- `collectextension/handler.go` / `router.go`：source 从 body 传入，handler 透传，
  service 层校验；**无需改**。
- CSRF 白名单（`security` 包）：按 `chrome-extension://` origin + 精确路由放行，
  与 source 无关；**无需改**。
- `collectdomain`：`DetectPlatform` 已支持 1688/pdd；**无需改**。
- 设备、配对、令牌逻辑：与 source 无关；**无需改**。

## 8. 测试与验证

### 8.1 单元测试（vitest，`browser-extension/src/adapters/`）

| 文件 | 覆盖点 |
| --- | --- |
| `registry.test.ts`（更新） | `adapterForURL` 路由 1688/pdd 详情页；`supportedAdapters()` 断言追加 `{id:'1688'}`、`{id:'pinduoduo'}`；拒绝 `http://`、`example.com`、`chrome://` |
| `alibaba-1688.test.ts`（新增） | `isSupportedAlibaba1688URL`（offer 路径/`offerId=`/m 站/拒绝非 https）；模块级纯函数：价格黑名单（`unitWeight: 39` 不当价格）、SKU 桶键解析（`颜色:蓝;尺码:M`、`蓝色【F106】>内长12`）、图 URL 分类去重 |
| `pinduoduo.test.ts`（新增） | `isSupportedPinduoduoWholesaleURL`（`pifa` 详情收、移动端 goods.html 拒、`gid` 非数字拒）；价格区间解析、SKU 行文本解析（`¥12.8\n仅剩 3 件\n白色`）、图片兜底链 |

单元测试只测**模块级纯函数**（不 mock DOM）；`collect` 函数体无法在 vitest 中
直接执行（依赖真实页面），靠 8.3 真机手测。

### 8.2 构建与后端测试

```bash
pnpm build:browser-extension      # 必须通过（tsc 严格模式）
pnpm test:browser-extension       # 必须通过
go test ./internal/modules/collect/... ./internal/modules/collectextension/... ./internal/pkg/collectdomain/...
```

后端新增/修改的校验与归一化函数补 Go 单测：
- `validateAlibaba1688CollectURL` / `validatePinduoduoCollectURL`（正反例表）；
- 1688/pdd 的归一化（图片清理、默认规格回填）；
- 契约测试（`collectextension` 的现有测试文件）补充 `source: '1688'` /
  `source: 'pinduoduo'` 的建任务-提交-失败全链路。

### 8.3 真机手测矩阵（构建产物加载到 Chrome，逐项打勾）

| 场景 | 1688 | 拼多多 pifa |
| --- | --- | --- |
| 已登录正常商品（含多规格） | ✅ | ✅ |
| 标题/主图/详情图/属性齐全 | ✅ | ✅ |
| SKU 含价格与库存（1688 桶数据 / pdd "仅剩 N 件"） | ✅ | ✅ |
| 未登录（登录墙）→ 提示 `LOGIN_REQUIRED` | ✅ | ✅ |
| 验证码/滑块 → 提示 `VERIFY_REQUIRED` | ✅ | ✅ |
| 已下架/404 商品 → `PRODUCT_NOT_FOUND` | ✅ | ✅ |
| 无规格商品 → 默认规格回填 | ✅ | ✅ |
| 详情图懒加载（滚动后出现） | ✅ | ✅ |
| 侧边栏 SKU 分析面板正常展示 | ✅ | ✅ |
| 提交后草稿图片/价格/SKU 入库正确 | ✅ | ✅ |
| 结果不完整时 warnings 可见、任务仍可提交 | ✅ | ✅ |

手测时重点核对**图片 URL 入库结果**：1688 的 `?x-oss-process` 参数、PDD 的
`imageView2` 参数是否被正确清理（§7.3 的归一化职责）。

## 9. 文档同步（收尾必做）

- `docs/browser-extension-collector.md`：适用场景、数据流图、支持平台列表、
  排错文案从"淘宝/天猫"扩展到三平台；
- `docs/collector-engines.md`：三路采集边界表补充 1688/pdd 扩展入口；
- `browser-extension/README.md` 与 `public/manifest.json` description 同步；
- 根 `README.md` 功能列表如有提及扩展平台则同步；
- 完成后运行 `pnpm docs:check`。

## 10. 风险与常见坑（迁移执行时对照）

1. **executeScript 序列化**（最高频坑）：collect 函数体内引用了任何模块级标识符
   （哪怕是 `const MAX = 200`），注入后即为 `ReferenceError` → 页面返回
   `EMPTY_RESULT`。写完 1688/pdd 适配器后，人工检查函数体是否"零外部引用"；
   淘宝先例的注释（taobao-tmall.ts:238-241）写明过此坑。
2. **MAIN world**：`service-worker.ts` 的 `world: 'MAIN'` 是读
   `window.context`/`__INITIAL_STATE__` 的前提，不要改成 ISOLATED。
3. **raw 体积**：1688 的 `scriptSnippets`/`extractDebug`、PDD 的 `imageDebug` 在
   扩展里必须裁剪；后端单次结果上限 3 MiB（`handler.go:241`），超限整个任务失败。
4. **后端 source 校验遗漏**：只改扩展不改后端 → 建任务 400；只改 `Create` 不改
   `Complete` 里的二次校验（browser_extension_task.go:286）→ 提交结果被拒。
5. **图片清理规则不能串**：淘宝 `_.jpg` 后缀规则套到 1688/PDD 会误伤合法 URL；
   三家各用各的归一化（§7.3）。
6. **PDD 无 skuCode**：前端 SKU 分析面板与后端导入都按"无 skuCode"处理，不要
   因此报错或丢弃 SKU。
7. **域名权限最小化**：manifest 只加 `detail.1688.com`、`m.1688.com`（可选）、
   `pifa.pinduoduo.com`（可选 `*.pifa.pinduoduo.com`）；不加 `*.1688.com` 全量。
8. **平台风控**：1688/pdd 都忌讳高频请求；扩展是"用户主动点击"的低频场景，但
   PDD 画廊点击节流（300-400ms）与 1688 的重试次数（仅 1 次）保持 collector 原值。
9. **`supports()` 与后端校验必须一致**：扩展判"可以采集"的 URL，后端
   `validateBrowserExtensionCollectURL` 必须放行，否则用户在页面点了采集却在
   建任务时 400。三处口径（supports / 后端 URL 校验 / collector canHandle）
   以 collector `canHandle` 为真源对齐。
10. **不做无人值守**：扩展场景始终是"用户当前页 + 用户已登录"，不实现批量/
    定时；失败后提示用户手动处理验证/登录后重试（与淘宝扩展一致）。

## 11. 验收定义（Done 标准）

- [ ] `pnpm build:browser-extension`、`pnpm test:browser-extension` 通过；
- [ ] 后端 `go test`（collect/collectextension/collectdomain）通过，含新增单测；
- [ ] Chrome 加载 dist 后：1688 与 pifa 详情页各完成 ≥5 种场景真机手测（§8.3）；
- [ ] 淘宝/天猫扩展回归无退化；
- [ ] `docs/` 相关文档与文案同步，`pnpm docs:check` 通过；
- [ ] 未使用 `skip`/宽泛 allowlist/自动放宽 baseline 掩盖任何失败。
