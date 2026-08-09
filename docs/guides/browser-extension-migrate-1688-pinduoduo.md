---
doc_type: guide
audience: agent
status: current
owner: maintainers
source_of_truth:
  - browser-extension/src/**
  - collector/src/providers/source1688/**
  - collector/src/providers/sourcePinduoduo/**
  - backend/internal/modules/collect/browser_extension_task.go
  - docs/browser-extension-collector.md
  - docs/collector-engines.md
review_cycle_days: 30
---

# Agent 实施指南：将 1688 / 拼多多 Playwright 采集迁移到浏览器扩展

> **文档用途**：指导 Agent（或开发者）把 `1688`、`pinduoduo` 从 Playwright Collector
> 迁移为浏览器侧边栏扩展适配器。本文按**当前仓库代码事实**编写；实现时以代码为准。
>
> **参考基线**：淘宝/天猫已完成「Playwright 抽取逻辑 → 扩展 `MAIN` world 注入采集」迁移。
> 迁移 1688 / 拼多多时，**必须复用同一套协议与约束**，而不是发明第二条扩展通道。
>
> **范围**：本文是操作手册，**不替代**用户文档 `docs/browser-extension-collector.md` 与
> 引擎边界文档 `docs/collector-engines.md`。实现完成后须同步更新那两份真源中的「支持范围」描述。

---

## 0. 先读结论（Agent 决策树）

| 问题 | 答案 |
| --- | --- |
| 扩展是否替代 Playwright 后台任务？ | **否**。扩展只做「用户当前标签页单商品采集」。批量 / 无人值守仍走 Playwright。 |
| 扩展结果走哪条链路？ | 扩展 → Backend `browser-extension` API → 创建 `collect_task(engine=browser_extension)` → 导入商品草稿。**不经过 Redis / collect worker / Playwright / OpenCLI**。 |
| 抽取逻辑放哪？ | 全部在扩展 `browser-extension/src/adapters/<platform>.ts` 的 **自包含 `collect*` 函数**里；`chrome.scripting.executeScript({ world: 'MAIN' })` 注入。 |
| 能否直接 `import` Playwright 的 Node 解析模块到页面？ | **不能**。页面内函数会被序列化，闭包失效。须把页面侧逻辑内联；纯函数可在模块顶层做单测，再**复制**进 `collect` 体内。 |
| 后端硬限制？ | 当前 `CreateBrowserExtensionTask` **只允许** `source=taobao_tmall` 与淘宝/天猫 URL。迁移必须改 Backend 多 source 校验与 import normalize 分支。 |
| Playwright 源码是否删除？ | **首期不删**。扩展上线后 Playwright 仍作后台/批量入口；后续再评估是否降级或归档。 |

---

## 1. 目标与非目标

### 1.1 目标

1. 用户在已登录的 **1688 商品详情页** 或 **拼多多批发详情页** 打开扩展侧边栏，点「采集当前商品」，创建 TradeMind 商品草稿。
2. 输出字段与现有 `NormalizedProduct` 契约兼容（与 Playwright 成功采集结果可对照）。
3. 遵循淘宝/天猫扩展已验证的安全模型：设备令牌、精确 `host_permissions`、`credentials: 'omit'`、不申请 Cookie/debugger。
4. 保持 Playwright 路径可继续创建任务（不破坏批量与登录 Profile）。

### 1.2 非目标（首期禁止做）

- 扩展侧批量、定时、后台队列、跨标签页自动翻页。
- 把 1688/拼多多接到 OpenCLI Bridge（OpenCLI 当前仅淘宝/天猫）。
- 远程下发适配器 JS、任意域名 `host_permissions`、读取 Admin JWT。
- 在扩展里启动 Playwright 或二次打开无头浏览器。
- 用 `skip` / 扩大 baseline 掩盖失败测试。

---

## 2. 三条采集入口（迁移后应保持的边界）

```text
Admin / 用户浏览器
        │
        ├─ A. 浏览器扩展（本迁移目标）
        │     当前标签页 → executeScript(MAIN) → POST /api/v1/collect/browser-extension/*
        │     engine = browser_extension；不进 Redis
        │
        ├─ B. Playwright Collector :3001
        │     Backend worker 投递 → collector providers（1688 / pinduoduo / …）
        │
        └─ C. OpenCLI Bridge :3100
              仅 taobao_tmall；与本迁移无关
```

迁移完成后矩阵应变为：

| 来源 | Playwright | OpenCLI | 浏览器扩展（当前页单采） |
| --- | --- | --- | --- |
| `taobao_tmall` | 支持 | 支持 | **已支持** |
| `1688` | 支持 | 不支持 | **本迁移新增** |
| `pinduoduo` | 支持（批发详情） | 不支持 | **本迁移新增** |

---

## 3. 淘宝/天猫「已完成迁移」的代码地图（必须对照）

实现 1688 / 拼多多前，Agent **先完整读一遍**下列文件，再动手。

### 3.1 扩展端（参考实现）

| 文件 | 职责 | 迁移时如何仿写 |
| --- | --- | --- |
| `browser-extension/public/manifest.json` | 精确 `host_permissions`、权限集 | 为 1688 / 拼多多**追加**精确域名，禁止 `*://*/*` 页面权限 |
| `browser-extension/src/adapters/types.ts` | `BrowserCollectAdapter` 接口 | 新适配器实现同一接口 |
| `browser-extension/src/adapters/taobao-tmall.ts` | URL 判定 + 可测纯函数 + **自包含** `collectTaobaoTmallPage` | 拆成 `1688.ts` / `pinduoduo.ts` |
| `browser-extension/src/adapters/registry.ts` | `adapterForURL` / `supportedAdapters` | 注册新适配器 |
| `browser-extension/src/service-worker.ts` | `COLLECT_ACTIVE_TAB` → `executeScript({ world: 'MAIN', func: adapter.collect })` | **尽量不改协议**；错误文案改为多平台 |
| `browser-extension/src/api.ts` | 配对 / 建任务 / 提交结果 | `createTask` 的 `source` 必须随 URL 变化 |
| `browser-extension/src/sidepanel.ts` | UI：支持判定、建任务、读页、上传 | 支持判定改为 registry；文案通用化 |
| `browser-extension/src/types.ts` | `NormalizedProduct.source` 等 | 把 `source` 从字面量 `'taobao_tmall'` 扩成联合类型 |
| `browser-extension/src/adapters/registry.test.ts` 等 | 路由与纯函数单测 | 为新平台补测 |

### 3.2 淘宝扩展采集的硬约束（照抄，禁止违反）

以下摘自现有实现与 `docs/browser-extension-collector.md`，**1688/拼多多同样适用**：

1. **`world: 'MAIN'`**
   默认 ISOLATED world 读不到页面全局变量 / 部分脚本状态。淘宝靠 `window.__ICE_APP_CONTEXT__`；1688 常靠 `window.context` / 内嵌 script JSON。必须 MAIN。

2. **`collect` 必须是「完整自包含函数」**
   `chrome.scripting.executeScript` 只序列化函数源码。
   - ✅ `collect: collect1688Page`（直接引用具名函数）
   - ❌ `collect: () => collect1688Page()` 或引用模块顶层 helper
   顶层可导出纯函数做单元测试，但 **页面执行路径内必须再内联一份**（淘宝文件注释已写明）。

3. **错误码格式**
   页面内 `throw new Error('CODE: human message')`；`service-worker` 按第一个 `:` 切开变成 `errorCode` + `message`。

4. **尽量恢复滚动 / 不长期破坏用户页面状态**
   淘宝会记录 `scrollY`，详情滚动后复位。1688 需要懒加载滚动时同样处理。

5. **禁止在扩展日志 / raw 中写入 Cookie、完整 Token、账号密码**（`AGENTS.md` CORE-001 / COL-004）。

### 3.3 淘宝扩展端到端数据流（复制此流程）

```text
sidepanel.collectCurrentPage()
  1. api.createTask(url)          // POST .../tasks  { source, url }
  2. chrome.runtime.sendMessage({ type: 'COLLECT_ACTIVE_TAB', tabId, url, ... })
  3. service-worker: adapterForURL(url) → executeScript(adapter.collect)
  4. api.submitResult(taskId, product)  // 失败则 submitFailure
  5. 展示草稿链接
```

**禁止**改成：先采页再建任务、或绕过 Backend 直写 product API。

### 3.4 Backend 扩展任务链路（必须扩展的部分）

| 文件 | 当前行为 | 迁移必须改 |
| --- | --- | --- |
| `backend/internal/modules/collect/browser_extension_task.go` | `Create` 只接受 `taobao_tmall`；`Complete` 只走 `normalizeTaobaoTmallImport` | 多 source 分发 |
| `backend/internal/modules/collect/taobao_tmall_url_validate.go` | 淘宝 URL 校验 | 新增 1688 / 拼多多 URL 校验或复用已有 domain 包 |
| `backend/internal/modules/collect/pinduoduo_import_normalize.go` | Worker 路径已用 | Complete 扩展任务时也要调用 |
| `backend/internal/modules/collect/browser_extension_task_test.go` | 全是 taobao 用例 | 增加 1688 / pinduoduo 正反例 |
| `backend/internal/modules/collectextension/*` | 配对/设备令牌 | **一般不用改**（与 source 无关） |

当前硬编码（实现时搜索并改掉）：

```go
// browser_extension_task.go CreateBrowserExtensionTask
if !isTaobaoTmallCollectSource(source) {
    return zero, fmt.Errorf("browser extension only supports taobao_tmall source")
}
if err := validateTaobaoTmallCollectURL(url); err != nil {
    return zero, err
}

// CompleteBrowserExtensionTask
params, storedJSON = normalizeTaobaoTmallImport(task.Source, norm, in.ProductJSON)
```

Worker 路径 `service.go` 已有分发模式，扩展 Complete 应对齐：

```go
// service.go（worker 成功导入）示意——Complete 应镜像此分发
if pinduoduo { normalizePinduoduoImport(...) }
if taobao_tmall { normalizeTaobaoTmallImport(...) }
// 1688 当前 worker 无单独 normalize 函数，走通用 importParams；
// 扩展 Complete 对 1688 同样可先走通用解析，再按需补 1688 专用 normalize。
```

---

## 4. Playwright 源码地图（迁移抽取逻辑的真源）

### 4.1 1688

目录：`collector/src/providers/source1688/`

| 文件 | 作用 | 扩展迁移策略 |
| --- | --- | --- |
| `alibaba-1688.ts` | Provider 入口：URL 校验、导航、验证码/登录检测、组装 `NormalizedProduct` | **导航/Profile/batch-gate 不迁**；把「页面就绪 + 抽取 + 组装 + 错误码」迁到扩展 |
| `page-prep.ts` | 等待核心选择器 + 分段滚动懒加载 | 在 `collect1688Page` 内用 `sleep` + `scroll` 重写（无 Playwright API） |
| `browser-extract-1688.ts` | **`extract1688DomInPage`**：已是「自包含、可序列化」DOM 抽取 | **优先整段移植**为扩展 collect 的主体；参数（selectors）可写死在函数内 |
| `selectors.ts` | 标题/主图/详情/属性/SKU 选择器列表 | 随 extract 一起内联 |
| `parser.ts` + `context-parse.ts` + `price-extract.ts` + `image-extract.ts` + `sku-helpers.ts` | Node 侧：解析 script JSON、SKU 合并、价格/图片兜底 | 扩展里必须改为**页面内**完成（见 §6.4）；不能依赖 Node `Page` |
| `auth-detect.ts` / captcha 逻辑 in `alibaba-1688.ts` | 登录墙/滑块 | 扩展侧用页面文案 + URL 检测，失败码对齐 |
| `types.ts` | `BrowserExtractPayload` / `Parse1688Result` | 扩展最终输出统一 `NormalizedProduct`，中间结构可内部保留 |
| `docs/collector-1688-pitfalls.md` | 已知坑（价格误取 unitWeight、SKU 噪声、evaluate 注入） | **迁移时逐条对照**，把防回归规则写进扩展纯函数测试 |

**URL 规则（Playwright 事实）**：

- Host：`detail.1688.com` 或 `m.1688.com`
- 路径：含 `/offer/` 或 `offerId=` 等（`isLikelyOfferPath`）
- 规范化：`https://detail.1688.com/offer/{id}.html`

**输出要点**：

```ts
{
  source: '1688',
  sourceUrl,
  title,
  currency: 'CNY', // 或从页面推断
  mainDescription?,
  mainImages: string[],
  descriptionImages: string[],
  attributes: Record<string, string | number | boolean>,
  skus: ProductSku[],
  raw: { /* 调试快照、productPrice、warnings、finalUrl 等 */ }
}
```

### 4.2 拼多多

目录：`collector/src/providers/sourcePinduoduo/`

| 文件 | 作用 | 扩展迁移策略 |
| --- | --- | --- |
| `index.ts` | Provider：URL 分类、拒绝非批发详情、访问检测、组装 | 扩展 **首期只支持** `wholesale_detail`（与 Playwright 一致） |
| `url-type.ts` / `validate-url.ts` | 链接语义分类 | **整套移植**到扩展（纯函数，易测） |
| `wholesale-detail-extract.ts` | `extractPifaWholesaleDetailInPage`：已是页面自包含 DOM 抽取 | **优先整段移植** |
| `wholesale-detail-gallery.ts` | Playwright 点击缩略图 / 滚动详情图 | 扩展内用 DOM 点击 + 滚动 + sleep 重写 |
| `wholesale-detail-images.ts` | 主图/详情图分类与过滤 | 移植为页面内函数或 collect 内联 |
| `wholesale-detail-shared.ts` | 标题清洗、价区间、SKU 行转 SKU、warning 文案 | 移植；warning code 写入 `raw.warnings` |
| `wholesale-detail.ts` | `assemblePifaWholesaleProduct` + 质量门禁 | 移植组装逻辑到扩展 |
| `access-detect.ts` / `auth-detect.ts` | 登录/验证/拦截 | 扩展侧检测后 `LOGIN_REQUIRED` / `VERIFY_REQUIRED` |
| `parser.ts` | 按 URL 类型分发（批发 vs 其它） | 扩展首期只走批发分支 |

**首期支持 URL（必须与 Playwright 一致）**：

- ✅ `https://pifa.pinduoduo.com/goods/detail/?gid=*`（及带 `goods_id` 的批发详情）
- ❌ 移动端 `yangkeduo.com/goods.html?goods_id=*`（Playwright 明确 `UNSUPPORTED_PINDUODUO_URL:goods_detail`）
- ❌ 批发首页、登录页、微信授权、App 跳转

**输出要点**：

```ts
{
  source: 'pinduoduo',
  sourceUrl,
  title,
  currency: 'CNY',
  mainDescription?,
  mainImages,
  descriptionImages,
  attributes,
  skus,
  raw: {
    extractProvider: 'pinduoduo', // 或 browser_extension + provider
    urlType: 'wholesale_detail',
    productPrice, priceMin, priceMax, priceText,
    warnings: WholesaleWarningCode[],
    qualityWarnings: string[],
    finalUrl, ...
  }
}
```

Backend `normalizePinduoduoImport` 会读 `raw.productPrice`、合成默认 SKU 等——扩展提交的 JSON **字段名应对齐 Playwright**，否则草稿价格/SKU 会丢。

---

## 5. 推荐实施顺序（严格按阶段，禁止一口气乱改）

```text
Phase 0  读代码 + 对照本文 + 列文件改动清单
Phase 1  扩展类型/Registry/manifest 骨架（还不做复杂抽取）
Phase 2  Backend 多 source 任务创建与 Complete 分发
Phase 3  1688 适配器（DOM + script JSON 抽取）
Phase 4  拼多多适配器（批发详情）
Phase 5  sidepanel / 文案 / Admin 提示
Phase 6  测试 + 文档同步 + 手动验收
```

每个 Phase 结束应可独立构建 / 跑单测。不要 Phase 3 没测就进 Phase 4。

---

## 6. Phase 1：扩展骨架（具体操作）

### 6.1 改 `NormalizedProduct.source` 类型

文件：`browser-extension/src/types.ts`

当前：

```ts
export type NormalizedProduct = {
  source: 'taobao_tmall';
  ...
};
```

改为（示例）：

```ts
export type BrowserCollectSource = 'taobao_tmall' | '1688' | 'pinduoduo';

export type NormalizedProduct = {
  source: BrowserCollectSource;
  sourceUrl: string;
  title: string;
  currency: 'CNY' | string;
  mainDescription?: string;
  mainImages: string[];
  descriptionImages: string[];
  attributes: Record<string, string | number | boolean>;
  skus: ProductSku[];
  raw: Record<string, unknown>;
};
```

### 6.2 新增适配器文件（空壳可先 supports + 抛 `NOT_IMPLEMENTED`）

创建：

- `browser-extension/src/adapters/1688.ts`
- `browser-extension/src/adapters/pinduoduo.ts`

每个文件至少导出：

```ts
export function isSupported1688URL(raw: string): boolean { ... }
export async function collect1688Page(): Promise<NormalizedProduct> { ... }
export const alibaba1688Adapter: BrowserCollectAdapter = {
  id: '1688',
  label: '1688',
  supports: isSupported1688URL,
  collect: collect1688Page, // 禁止箭头包装
};
```

`supports` 实现应对齐 Playwright：

**1688**（对照 `alibaba-1688.ts`）：

```ts
// 伪代码
const u = new URL(raw);
if (u.protocol !== 'https:' && u.protocol !== 'http:') return false; // 建议扩展只接受 https
const host = u.hostname.toLowerCase();
const isHost = host === 'detail.1688.com' || host === 'm.1688.com';
const pathOk = /\/offer\//i.test(u.pathname) || /offerId=/i.test(u.search);
return isHost && pathOk;
```

**拼多多**（对照 `validate-url.ts` + `url-type.ts`）：

```ts
// 仅 wholesale_detail 返回 true
classifyPinduoduoUrl(raw) === 'wholesale_detail'
```

### 6.3 注册适配器

文件：`browser-extension/src/adapters/registry.ts`

```ts
const adapters: BrowserCollectAdapter[] = [
  taobaoTmallAdapter,
  alibaba1688Adapter,
  pinduoduoAdapter,
];
```

更新 `registry.test.ts`：对 1688 / pifa 样例 URL 断言 `adapterForURL(url)?.id`。

### 6.4 更新 `manifest.json`

文件：`browser-extension/public/manifest.json`

在 `host_permissions` **追加**（按实际支持 host 精修，以下为起点）：

```json
"https://detail.1688.com/*",
"https://m.1688.com/*",
"https://pifa.pinduoduo.com/*"
```

注意：

- Chrome MV3 权限只显式列出 `detail.1688.com`、`m.1688.com`，不申请 `*.1688.com` 全站权限。
- **不要**为了省事加 `https://*/*` 到页面权限；远程 Backend 继续用 `optional_host_permissions: ["https://*/*"]`。
- 同步改 `description` / `name` 文案，避免仍写「仅淘宝/天猫」。

### 6.5 改 `api.createTask` 动态 source

文件：`browser-extension/src/api.ts`

当前写死：

```ts
body: JSON.stringify({ source: 'taobao_tmall', url }),
```

改为：

```ts
async createTask(url: string, source: string) {
  return this.request<CollectTask>('/api/v1/collect/browser-extension/tasks', {
    method: 'POST',
    body: JSON.stringify({ source, url }),
  });
}
```

`sidepanel.ts` 中：

```ts
const adapter = adapterForURL(activePage.url);
// ...
task = await api.createTask(activePage.url, adapter.id);
```

提交结果时 `product.source` 规范化后必须与 task.source **一致**；`taobao` 会统一为
`taobao_tmall`，Backend 以已验证的 task source/sourceUrl 作为草稿落库值。

### 6.6 改 `service-worker.ts` 文案

把：

```ts
message: '当前仅支持淘宝或天猫商品详情页',
```

改为通用：

```ts
message: '当前页面不在已支持的采集站点内（淘宝/天猫/1688/拼多多批发）',
```

协议字段 `COLLECT_ACTIVE_TAB` **不要改名**。

### 6.7 改 `sidepanel.ts` 页面支持判定

当前：

```ts
supported: isSupportedTaobaoTmallURL(tab.url),
```

改为：

```ts
import { adapterForURL } from './adapters/registry.js';
// ...
supported: Boolean(adapterForURL(tab.url)),
```

UI 标题「淘宝 / 天猫商品」改为「当前商品页」或根据 adapter.label 动态显示。

SKU 价格探测上限控件：

- 淘宝：保留。
- 1688 / 拼多多：可隐藏或忽略 `skuPriceProbeMax`（仅淘宝 adapter 使用 options）。

`collect` 的 `args` 对非淘宝可传 `{}`。

---

## 7. Phase 2：Backend 多 source（具体操作）

### 7.1 任务创建校验

文件：`backend/internal/modules/collect/browser_extension_task.go`

建议抽出：

```go
func validateBrowserExtensionSourceAndURL(source, url string) error {
    switch strings.ToLower(strings.TrimSpace(source)) {
    case "taobao_tmall", "taobao":
        if !isTaobaoTmallCollectSource(source) { ... }
        return validateTaobaoTmallCollectURL(url)
    case "1688":
        return validate1688CollectURL(url) // 新建或复用已有 validate
    case "pinduoduo", "pdd":
        return validatePinduoduoWholesaleCollectURL(url) // 仅批发详情
    default:
        return fmt.Errorf("browser extension unsupported source %q", source)
    }
}
```

**URL 校验实现来源**：

- 搜索 Backend 是否已有 1688 / 拼多多 URL 分类（Admin 有 `admin/src/utils/pinduoduoUrl.ts`，Backend collect 创建任务路径也有类似校验）。优先复用 **Go 侧已有** 函数，避免前后端两套语义漂移。
- 若仅有 Playwright 侧 TS 规则，则在 Go 中实现等价规则，并补表驱动测试。

### 7.2 Complete 导入 normalize 分发

同一文件 `CompleteBrowserExtensionTask`：

```go
params := norm.importParams(in.ProductJSON)
storedJSON := in.ProductJSON

switch {
case isTaobaoTmallCollectSource(task.Source):
    params, storedJSON = normalizeTaobaoTmallImport(task.Source, norm, in.ProductJSON)
case strings.EqualFold(task.Source, "pinduoduo"), strings.EqualFold(task.Source, "pdd"):
    params, storedJSON = normalizePinduoduoImport(task.Source, norm, in.ProductJSON)
case strings.EqualFold(task.Source, "1688"):
    // 若暂无专用 normalize：保持 importParams；并强制 mainImages 非空策略与 worker 一致
    if len(norm.MainImages) == 0 {
        return zero, fmt.Errorf("missing main images")
    }
default:
    return zero, fmt.Errorf("browser extension unsupported source %q", task.Source)
}
```

对齐 worker：`service.go` 对 1688 在 mainImages 为空时直接 `PARSE_FAILED`。

### 7.3 校验 product.source 与 task.Source

建议在 Complete 时：

- 解析出的 `norm.Source`（若 JSON 含 source 字段）与 `task.Source` 忽略大小写一致；
- 不一致则拒绝，防止任务是 1688、payload 却标 taobao。

### 7.4 测试清单（Backend）

文件：`browser_extension_task_test.go` 扩展：

| 用例 | 期望 |
| --- | --- |
| Create `source=1688` + 合法 offer URL | 成功，status=running，engine=browser_extension |
| Create `source=1688` + 淘宝 URL | 失败 |
| Create `source=pinduoduo` + pifa detail | 成功 |
| Create `source=pinduoduo` + yangkeduo goods | 失败（与 Playwright 一致） |
| Create `source=custom` | 失败 |
| Complete 1688 合法 product JSON | 成功导入草稿 |
| Complete pinduoduo 无 SKU 但有 productPrice | 走 normalize 合成默认规格（若走 pinduoduo normalize） |
| Complete 设备不匹配 / 二次提交 | 保持现有错误 |

运行（按仓库实际命令，常见）：

```bash
cd backend && go test ./internal/modules/collect/ -count=1 -run BrowserExtension
```

---

## 8. Phase 3：1688 适配器实现细节

### 8.1 总体算法（扩展页内）

对齐 Playwright：`prepare → extract DOM+scripts → assemble → quality gate`。

```text
collect1688Page():
  1. 校验 hostname / offer 路径；否则 UNSUPPORTED_PAGE
  2. 检测 captcha / 登录 / 商品不存在（文案 + URL）
  3. 等待核心选择器出现（超时 ~10–12s）
  4. 分段滚动触发懒加载（对照 page-prep.ts），记录并恢复 scrollY
  5. 执行「DOM 抽取」= 移植 extract1688DomInPage 逻辑
  6. 在页面内解析 script 片段 / window.context 等 JSON 根
  7. 合并标题、主图、详情图、属性、SKU、价格
  8. 质量门禁：无标题 / 无主图 → 硬失败；缺价或缺 SKU → warning + partial 仍可提交（对齐 resolveCollectOutcome）
  9. return NormalizedProduct
```

### 8.2 从 Playwright 移植 `extract1688DomInPage` 的操作步骤

源文件：`collector/src/providers/source1688/browser-extract-1688.ts`

1. 复制函数体到 `collect1688Page` **内部**（或作为同一文件内、collect 再调用的内联函数——注意 executeScript 只能注入一个顶层 func，因此 **只能有一个导出 collect 入口**，所有 helper 写在 collect 函数体内）。
2. 把 `Extract1688DomArg` 参数改为函数内常量：

```ts
const titleSelectors = [ 'h1.d-title', ... ]; // 从 selectors.ts 抄
const mainSel = [ ... ];
// attrSel / skuSectionSel / skuTableSel 同理
const snippetMax = 120_000;
const maxFragments = 14;
```

3. Playwright 调用方式是 `page.evaluate(extract1688DomInPage, arg)`；扩展则是 collect 直接在页面执行，**不再需要** `page.evaluate`。
4. 返回结构 `BrowserExtractPayload` 在页面内继续处理，不要 postMessage 回 Node。

### 8.3 Node 侧 `parser.ts` / `context-parse.ts` 如何「页面化」

Playwright 架构是：

```text
Browser: 抽 DOM + script 文本片段
Node:    JSON.parse、walk 模块树、SKU map、价格 key 白名单
```

扩展没有 Node。两种合法策略（按推荐顺序）：

#### 策略 A（推荐）：页面内完成组装

把下列能力改写为 collect 体内函数（可先从 Node 源复制再删 Node 依赖）：

| Node 能力 | 源文件 | 页面内注意点 |
| --- | --- | --- |
| `find1688ResultData` | `context-parse.ts` | 从 `window.context`、`window.__INIT_DATA__`、script 文本 JSON 根 walk |
| `extractMainImagesFrom1688Data` / detail / attributes | 同上 | 使用 `normalizeImageUrl` 逻辑：`//` → `https:` |
| `mineSkusFrom1688Data` / `walkSkuPropArrays` | 同上 + `sku-helpers.ts` | 过滤 junk 规格值（含 `¥`、`库存`、维度名自身） |
| `extractDefaultOfferPrice` / DOM 价格 | `price-extract.ts` | **严禁**把 `unitWeight` 当价格（见 pitfalls §2） |
| DOM SKU 表 enrich | `sku-helpers.ts` | 表格行价/库存文案解析 |
| 图片 junk 过滤 | `utils.ts` `isLikelyJunkImage` | 过滤 icon/服务图 |

#### 策略 B（不推荐首期）：Browser 只抽原始 payload，扩展 service-worker 再解析

service-worker 虽是扩展 JS 环境，可 import 模块，但：

- 需要把 payload 从 MAIN world 返回到 SW；
- 与淘宝「单函数返回 NormalizedProduct」模式不一致；
- 增大协议复杂度。

**首期请用策略 A**，与淘宝一致。

### 8.4 纯函数单测（模块顶层）

对可确定性逻辑在模块顶层导出并测试（与淘宝 `buildSkusFromTaobaoSkuBase` 相同模式）：

建议测试文件：`browser-extension/src/adapters/1688-*.test.ts`

| 测试 | 输入 | 期望 |
| --- | --- | --- |
| `isSupported1688URL` | `https://detail.1688.com/offer/123.html` | true |
| | `https://item.taobao.com/...` | false |
| 价格提取 | JSON 含 `unitWeight: 39` 与真实 `price: 720` | 价=720 不是 39 |
| SKU junk | 规格值含 `1.2mm¥790库存` | 过滤掉 |
| 图片 | `//cbu01.alicdn.com/xxx.jpg` | `https://...` |

**重要**：页面内 collect 使用的实现必须与测试导出函数**逻辑同步**（复制或构建时生成；禁止只测顶层、页面内却是旧拷贝长期漂移——可在文件头注释「双份同步清单」）。

### 8.5 1688 错误码建议（对齐现有风格）

| 条件 | errorCode | 用户可读 message |
| --- | --- | --- |
| 非 1688 offer | `UNSUPPORTED_PAGE` | … |
| 滑块/安全验证 | `VERIFY_REQUIRED` / `PAGE_BLOCKED_OR_VERIFY_REQUIRED` | 请完成验证后重试 |
| 登录页 | `LOGIN_REQUIRED` | 请在当前浏览器登录 1688 后重试 |
| 无标题 | `TITLE_NOT_FOUND` / `missing_title` | … |
| 无主图 | `MAIN_IMAGES_EMPTY` / `missing_main_images` | … |
| 空结果 | 由 SW 报 `EMPTY_RESULT` | collect 返回 undefined 时 |

`raw.provider` 建议：`'browser_extension'`，并加 `extractProvider: '1688'`，便于 Admin 告警组件识别。

### 8.6 不迁移的 Playwright 专属能力

以下**留在 Collector**，扩展不做：

- `with1688BatchGate` 批量限速
- `BrowserManager.with1688Page` Profile / 用户数据目录
- `open-login-browser` / `check-login` HTTP API
- 失败 debug snapshot 落盘（`debug-snapshot.ts`）
- 导航 `page.goto`（用户已打开页面）

---

## 9. Phase 4：拼多多适配器实现细节

### 9.1 总体算法

```text
collectPinduoduoPage():
  1. classifyPinduoduoUrl(location.href)
     - 非 wholesale_detail → 明确错误（对照 unsupportedPinduoduoUrlMessage）
  2. 登录/验证/微信授权文案检测
  3. 等待标题/主图区域
  4. （可选）点击缩略图收集主图：对照 wholesale-detail-gallery.ts
  5. 滚动详情区：scrollAndCollectDetailImages 的页面版
  6. extractPifaWholesaleDetailInPage() 逻辑
  7. assemblePifaWholesaleProduct 逻辑（含 warning codes）
  8. validateWholesaleCollectQuality：失败 throw，部分成功带 warnings
  9. return NormalizedProduct source='pinduoduo'
```

### 9.2 优先整段移植的函数

| 源 | 目标 |
| --- | --- |
| `wholesale-detail-extract.ts` → `extractPifaWholesaleDetailInPage` | collect 体内 |
| `wholesale-detail-shared.ts` → `cleanProductTitle` / `wholesaleRowsToSkus` / `parsePriceRangeText` | 体内 + 顶层单测副本 |
| `wholesale-detail-images.ts` → `classifyRegionImages` | 体内 |
| `wholesale-detail.ts` → `assemblePifaWholesaleProduct` | 体内（去掉 Playwright `Page` 参数） |

### 9.3 Gallery 交互（Playwright → DOM）

`wholesale-detail-gallery.ts` 使用 Playwright `page.click` / `page.evaluate`。扩展改写原则：

```ts
// 伪代码：点击可见缩略图
const thumbs = document.querySelectorAll('<与源码相同的缩略图选择器>');
for (const thumb of thumbs) {
  (thumb as HTMLElement).click();
  await sleep(200–400);
  // 读取当前大图 src 推入 main candidates
}
// 滚动详情容器
detailRoot.scrollIntoView();
for (let i = 0; i < N; i++) {
  window.scrollBy(0, step);
  await sleep(260);
}
```

完成后 `window.scrollTo(0, originalScrollY)`。

### 9.4 价格与 SKU 对齐 Backend

`normalizePinduoduoImport` 依赖：

- `raw.productPrice` / `priceText` / `priceMin` / `priceMax`
- 无 SKU 时用 `productPrice` 合成「默认规格」

因此组装时必须设置：

```ts
raw: {
  extractProvider: 'pinduoduo',
  productPrice: price,
  priceMin, priceMax, priceText,
  warnings: warningCodes,          // 机器码
  qualityWarnings: humanMessages,  // 或与 Playwright 一致
  urlType: 'wholesale_detail',
  finalUrl: location.href,
  provider: 'browser_extension',
  schemaVersion: 1,
  capturedAt: new Date().toISOString(),
}
```

Admin 侧 `admin/src/utils/pinduoduoCollectAlerts.ts` 会读 raw warning——保持 code 集合与 `WholesaleWarningCode` 一致可减少 UI 改动。

### 9.5 拼多多错误码

| 条件 | errorCode |
| --- | --- |
| 移动端商品页 | `UNSUPPORTED_PINDUODUO_URL` |
| 批发首页 | 同上 |
| 需登录 | `LOGIN_REQUIRED` |
| 微信授权 | `WECHAT_AUTH_REQUIRED` |
| 验证码 | `VERIFY_REQUIRED` |
| 质量门禁失败 | 沿用 `validateWholesaleCollectQuality` 的 error 字符串格式 |

---

## 10. Phase 5：UI / Admin / 文档文案

### 10.1 扩展 UI

| 位置 | 改动 |
| --- | --- |
| `sidepanel.html` | 标题、hint 从「淘宝/天猫」改为多平台；SKU 探测说明可标注「仅淘宝/天猫」 |
| `sidepanel.ts` | 动态 label；不支持页提示列出支持站点 |
| `manifest.json` description | 同上 |
| `browser-extension/README.md` | 使用步骤增加 1688 / 拼多多 |

### 10.2 Admin

| 位置 | 改动 |
| --- | --- |
| `admin/src/pages/Collect/BrowserExtension/index.tsx` | 说明文案：支持站点列表 |
| 采集中心各平台入口（若写死「扩展仅淘宝」） | 搜索文案并更新 |

**不要**改变 Admin 创建 Playwright 任务的 API 语义。

### 10.3 文档真源（实现完成后必须改，非本指南替身）

| 文档 | 更新内容 |
| --- | --- |
| `docs/browser-extension-collector.md` | 支持 1688 / 拼多多批发；限制与排错 |
| `docs/collector-engines.md` | 三条入口矩阵表 |
| `docs/status/current.md` | 能力表一行 |
| `browser-extension/README.md` | 安装验收步骤 |

本文件 `docs/guides/browser-extension-migrate-1688-pinduoduo.md` 在迁移完成后可标注 `status: historical` 或移入 archive（由维护者决定），避免与 current 能力双源。

---

## 11. Phase 6：验证与验收

### 11.1 自动化（本地）

```bash
# 扩展
pnpm build:browser-extension
pnpm test:browser-extension
pnpm quality:browser-extension   # 若仓库提供

# Collector 回归（确保未破坏 Playwright）
pnpm test:collector

# Backend
cd backend && go test ./internal/modules/collect/ -count=1 -run 'BrowserExtension|Pinduoduo|Taobao'

# 影响面
pnpm docs:impact -- --files-from-git
pnpm quality:affected
pnpm test:affected
```

按 `pnpm agent:context -- --files ...` 输出补齐 checks。

### 11.2 手动验收矩阵

在真实浏览器加载 `browser-extension/dist`，配对 Admin 后：

| # | 页面 | 操作 | 期望 |
| --- | --- | --- | --- |
| 1 | 天猫详情（回归） | 采集 | 草稿成功，SKU 分析仍可用 |
| 2 | 淘宝详情（回归） | 采集 | 同上 |
| 3 | `detail.1688.com/offer/{id}.html` 已登录 | 采集 | 标题/主图/属性/SKU 基本正确；草稿 source=1688 |
| 4 | 1688 未登录或验证码 | 采集 | 明确 LOGIN/VERIFY 错误，任务 failure |
| 5 | `pifa.pinduoduo.com/goods/detail/?gid=` 已登录 | 采集 | 批发标题/价/主图/SKU；草稿 source=pinduoduo |
| 6 | `mobile.yangkeduo.com/goods.html?goods_id=` | 侧边栏 | 显示不支持，不能创建成功任务 |
| 7 | 无关站点 | 侧边栏 | 不支持 |
| 8 | 断网 / 撤销设备 | 采集 | 失败提示重新配对 |

对照同一商品用 Playwright 采一份，人工 diff：`title`、`mainImages` 数量级、`skus` 条数、价格量级。

### 11.3 验收完成定义（DoD）

- [ ] 扩展可对 1688 offer、拼多多批发详情创建草稿
- [ ] 淘宝/天猫扩展路径无回归
- [ ] Playwright 1688 / 拼多多单测与关键路径仍可用
- [ ] Backend 拒绝非法 source/URL
- [ ] `host_permissions` 仅为精确域名
- [ ] 无密钥/Cookie 进入仓库或日志
- [ ] 文档矩阵已更新
- [ ] 本指南中「禁止事项」未被违反

---

## 12. 代码改动清单（文件级 checklist）

### 12.1 必改（扩展）

- [ ] `browser-extension/public/manifest.json`
- [ ] `browser-extension/src/types.ts`
- [ ] `browser-extension/src/adapters/types.ts`（若 collect options 需扩展）
- [ ] `browser-extension/src/adapters/1688.ts`（新建）
- [ ] `browser-extension/src/adapters/pinduoduo.ts`（新建）
- [ ] `browser-extension/src/adapters/registry.ts`
- [ ] `browser-extension/src/adapters/registry.test.ts`
- [ ] `browser-extension/src/adapters/1688*.test.ts`（新建）
- [ ] `browser-extension/src/adapters/pinduoduo*.test.ts`（新建）
- [ ] `browser-extension/src/api.ts`（createTask source）
- [ ] `browser-extension/src/service-worker.ts`（文案）
- [ ] `browser-extension/src/sidepanel.ts` / `public/sidepanel.html`
- [ ] `browser-extension/README.md`

### 12.2 必改（Backend）

- [ ] `backend/internal/modules/collect/browser_extension_task.go`
- [ ] `backend/internal/modules/collect/browser_extension_task_test.go`
- [ ] 1688 / 拼多多 URL validate（新建或复用）
- [ ] Complete 路径调用 `normalizePinduoduoImport`（及 1688 策略）

### 12.3 可能改（Admin / 文档）

- [ ] `admin/src/pages/Collect/BrowserExtension/index.tsx`
- [ ] `docs/browser-extension-collector.md`
- [ ] `docs/collector-engines.md`
- [ ] `docs/status/current.md`

### 12.4 明确不要改（首期）

- [ ] `collector/src/providers/source1688/**` 删除或停用
- [ ] `collector/src/providers/sourcePinduoduo/**` 删除或停用
- [ ] OpenCLI adapter / Bridge 端口
- [ ] Redis collect worker 投递逻辑（扩展任务本就不入队）
- [ ] 生产密钥与 Profile 目录结构

---

## 13. 实现模板：新适配器文件结构（推荐骨架）

```ts
// browser-extension/src/adapters/1688.ts
import type { BrowserCollectAdapter } from './types.js';
import type { NormalizedProduct } from '../types.js';

// ===== 可单测纯函数（模块作用域）=====
export function isSupported1688URL(raw: string): boolean { /* ... */ }
export function parse1688Price(raw: unknown): number | undefined { /* ... */ }
// ... 与页面逻辑保持同步的构建函数 ...

// ===== 注入页面的唯一入口：全部 helper 写在函数体内 =====
export async function collect1688Page(): Promise<NormalizedProduct> {
  const sleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));
  const fail = (code: string, message: string): never => {
    throw new Error(`${code}: ${message}`);
  };

  // 1) URL / 登录 / 验证
  // 2) wait + scroll
  // 3) DOM extract（自包含）
  // 4) JSON roots + SKU/price/images merge
  // 5) quality gate
  return {
    source: '1688',
    sourceUrl: location.href,
    title,
    currency: 'CNY',
    mainDescription,
    mainImages,
    descriptionImages,
    attributes,
    skus,
    raw: {
      provider: 'browser_extension',
      extractProvider: '1688',
      schemaVersion: 1,
      capturedAt: new Date().toISOString(),
      finalUrl: location.href,
      productPrice,
      qualityWarnings,
    },
  };
}

export const alibaba1688Adapter: BrowserCollectAdapter = {
  id: '1688',
  label: '1688',
  supports: isSupported1688URL,
  collect: collect1688Page,
};
```

拼多多文件同构，`source: 'pinduoduo'`，`id: 'pinduoduo'`。

---

## 14. 常见失败模式与排查（给 Agent 的 debug 手册）

| 现象 | 可能原因 | 处理 |
| --- | --- | --- |
| `EMPTY_RESULT` | collect 箭头包装 / 引用外部函数导致页面 ReferenceError | 直接引用具名函数；看扩展 service worker 控制台 |
| 只有 1 个默认 SKU | 未使用 `world: 'MAIN'` 或未读到页面 JSON | 确认 SW 传 `world: 'MAIN'` |
| 价格全是 39 一类错价 | 误读 unitWeight | 对照 `docs/collector-1688-pitfalls.md` §2 |
| SKU 属性值含「库存299件」 | DOM textContent 过宽 | 对照 pitfalls §3；用 isJunkSkuValue |
| 扩展提示不支持但 URL 正确 | manifest 未加 host 或 supports 过严 | 检查 permissions 与 registry |
| Create task 400 | Backend 仍只允许 taobao_tmall | 完成 Phase 2 |
| 草稿无图 / 无价 | Complete 未走对应 normalize 或 raw 字段名不对 | 对齐 Playwright raw 字段 |
| 拼多多移动端被采进 | supports 误放行 goods_detail | 强制 `wholesale_detail` only |
| CORS / 401 | 配对失效或 credentials 错误 | 保持 `credentials: 'omit'` + Bearer 设备令牌 |

---

## 15. 与 Playwright 的字段对照表（提交 Backend 前自检）

| 字段 | 淘宝扩展 | 1688 Playwright | 拼多多 Playwright | 扩展迁移要求 |
| --- | --- | --- | --- | --- |
| `source` | `taobao_tmall` | `1688` | `pinduoduo` | 必须正确 |
| `sourceUrl` | 当前页 URL | 任务 URL | 任务 URL | 用用户页 URL |
| `title` | 必填 | 必填 | 必填，清洗平台标题 | 必填 |
| `currency` | CNY | CNY | CNY | 默认 CNY |
| `mainImages` | 必填 | 强约束 | 可 warning | 1688 建议硬性非空 |
| `descriptionImages` | 可空+warning | 可空 | 可空+warning | 可空 |
| `attributes` | object | object | object | 可空+warning |
| `skus[].properties` | 规格字典 | 规格字典 | 规格字典 | 键值均为展示名 |
| `skus[].price` | number 元 | number 元 | number 元 | **元**，非分 |
| `skus[].stock` | number，0=缺货 | number | number | 同语义 |
| `raw.productPrice` | 有 | 有 | 有 | 拼多多 normalize 依赖 |
| `raw.qualityWarnings` / `warnings` | 有 | 有 | 有 | 保留 |

统一商品结构真源：`collector/src/types/product.ts` 与 Backend `parseNormalized`。

---

## 16. Agent 工作纪律（本仓库）

1. 先跑 `pnpm agent:context -- --files browser-extension/src/** collector/src/providers/**`，只读 requiredContexts。
2. 小步提交逻辑清晰的改动；**未经用户要求不 commit / 不 push**（CORE-002）。
3. 不把重构/UI 美化混进迁移 PR（CORE-003）。
4. 测试不打真实电商写接口（CORE-005 / COL-005）。
5. 交付说明使用 `AGENTS.md` 最终说明格式：改了什么 / 为什么 / 已跑检查 / 未跑检查 / 文档影响 / 风险。
6. 实现完成后运行 `pnpm docs:impact -- --files-from-git`，只更新受影响真源。

---

## 17. 附录：关键符号与搜索词

在仓库根目录用搜索快速定位：

```text
COLLECT_ACTIVE_TAB
adapterForURL
collectTaobaoTmallPage
world: 'MAIN'
CreateBrowserExtensionTask
normalizePinduoduoImport
extract1688DomInPage
extractPifaWholesaleDetailInPage
classifyPinduoduoUrl
browser_extension
```

---

## 18. 附录：建议 PR 拆分

| PR | 内容 | 可单独合并？ |
| --- | --- | --- |
| PR1 | Backend 多 source + 测试（扩展仍可只发 taobao） | 是 |
| PR2 | 扩展 registry/manifest/api/sidepanel 骨架 + 1688 adapter | 是（依赖 PR1） |
| PR3 | 拼多多 adapter | 是 |
| PR4 | 文档与 Admin 文案 | 是 |

禁止把「删除 Playwright 1688/拼多多」放进同一 PR。

---

**文档结束。** 实现时若发现代码与本文冲突，**以代码与测试为准**，并回写修正本文对应章节。
