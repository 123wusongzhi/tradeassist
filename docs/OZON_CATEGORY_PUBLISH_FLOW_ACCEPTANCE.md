# Ozon 类目与刊登流程竣工验收

> 验收日期：2026-08-03
> 参考任务：`019fc52d-f757-76e2-9919-7b2afc1ae374`
> 验收分支：`feat/ozon-category-publish-flow`
> 多 SKU 图片补充验收：2026-08-04（本工作树，未执行真实 Ozon 写入）
> 统一刊登中心补充验收：2026-08-04（本工作树，未执行真实 Ozon 写入）
> 多 SKU 变体属性补充验收：2026-08-04（本工作树，未执行真实 Ozon 写入）

## 1. 验收结论

| 验收层级 | 结论 | 说明 |
| --- | --- | --- |
| 开发竣工验收 | **通过** | 约定的类目、映射、商品配置、预检、提交任务、安全控制、文档与自动化回归已形成闭环；仓库既有格式基线例外见第 5 节。 |
| 真实只读联调 | **历史通过；当前需复验** | 初次验收已完成连接、类目、属性模板与字典搜索；故障复验时 Ozon 已报告当前 API Key 停用，更新凭证前不得继续宣称当前环境联调通过。 |
| 生产真实刊登验收 | **未执行 / Deferred** | 尚未获准用真实商品调用 Ozon 导入、库存或归档接口，也未验证平台异步审核与失败后人工核对流程。 |
| Production Ready | **No** | 不得把开发验收或只读联调表述为生产可用验收。 |

本次可签收的是 TradeMind 侧的开发交付。生产放量前仍须完成受控小量真实商品验收，并建立失败或结果未知时的人工核对与恢复流程。

## 2. 验收范围

| 能力 | 验收结果 | 关键边界 |
| --- | --- | --- |
| Ozon 类目同步 | 通过 | Redis 可用时创建异步运行；记录 `added` / `changed` / `deactivated` / `reactivated` 四态差异；截断结果为 `partial`，不误停用缓存类目。 |
| 来源类目映射 | 通过 | 按租户隔离；店铺映射优先于租户默认映射；推荐仅提供候选，人工确认后才落库。 |
| 商品 + 店铺刊登配置 | 通过 | Ozon 按商品 + 平台 + 店铺独立保存叶子类目、v3 动态属性、SKU 售价覆盖、图片和包裹配置；旧商品级行安全回退，多个 Ozon 店铺不会互相覆盖。 |
| 多 SKU 变体属性 | 补充实现，本地自动化验收通过 | 指定一个或多个 Ozon 普通属性作为维度，按本地 SKU UUID 独立保存词典/文本值；可从本地 SKU 属性严格匹配，未匹配项人工选择。缺值、重复组合、未知引用或只能由复杂属性表达时阻断，不会把商品级单值复制给全部 SKU。 |
| 多 SKU 图片配置 | 补充实现，本地自动化验收通过 | 每个 SKU 原始主图固定第 1；公共图仅按人工选择追加，支持批量应用后逐 SKU 调整；缺原图只能显式保存替代图，旧配置默认仅用 SKU 原图。 |
| 实时发布前校验 | 通过 | 重新读取店铺合同币种、活动类目与属性模板；返回逐 SKU 最终有效值、来源与错误。必填/多值/复杂属性、schema 漂移、价格、图片及包裹配置不合法时硬阻断。 |
| 批量分组确认 | 通过 | 按来源类目分组；限制请求规模和组间重复商品；保存全局映射时额外要求配置管理权限。 |
| 保存、检查与真实提交 | 通过 | “保存当前编辑（不提交）”只写 TradeMind 配置；发布前检查只调用 Ozon 只读接口；真实提交需二次确认并创建 TradeMind 处理记录，只有取得真实 product ID 后才显示商品已创建。 |
| 幂等与外部写安全 | 通过 | Ozon 提交强制 `Idempotency-Key`；任务与发布事实事务化落库；导入/库存写禁用传输层自动重试；失败或结果未知时仅 `retryable=true` 开放手动重试，否则必须人工核对且禁止换 key 绕过。 |
| 凭证与网络边界 | 通过 | 租户 `authConfig` 不可注入凭证或覆盖 Ozon API 主机；历史明文配置不回传、不参与运行；只允许进程内受信任测试覆盖。 |
| RBAC、审计与 UI | 通过 | 读取、店铺操作、配置管理权限分离；关键映射写入审计；专项 E2E 拦截全部写请求并覆盖五档视口。 |

## 3. 真实只读验收证据与故障复验

初次验收使用本地已授权 `mery` 店铺执行只读联调，结果如下：

- `test-connection` 成功。
- 递归同步得到 7,992 个类目节点，其中 7,424 个叶子类目。
- 选定三级叶子类目读取到 37 个属性，其中 3 个必填属性、15 个字典属性。
- 字典远程搜索返回真实候选值。
- 验收期间未调用 `/v3/product/import`、`/v2/products/stocks` 或 `/v1/product/archive`。

真实只读数据仅证明凭证、类目树、属性模板和字典查询链路可用，不证明真实商品导入或平台审核链路已通过。

同日后续针对管理端出现的 `Request failed with status code 400` 进行复验，确认故障链路为：

1. 用户选择叶类目后，属性模板同步请求先失败。
2. 前端仍保留失败类目，随后允许发起商品配置保存；后端因该类目没有属性模板再次返回 400。
3. 真实上游根因是当前店铺的 Ozon API Key 已停用。该凭证不能由 TradeMind 自动续期或替换，必须由有权限的运营人员在「店铺管理 → 授权配置」中更新。

本次修复后，HTTP 错误会保留后端安全错误信息；授权失效会明确引导更新 Ozon 凭证；新类目同步失败时不会应用该类目，而是恢复上一个可用类目或清除本次选择，避免继续保存无模板配置。修复未自动修改店铺状态、权限或任何真实平台数据。

补充权限与一致性回归：商品配置 Upsert 现在在事务内写入并使用全新变量按 `product_id + platform + config_scope_key` 回读，更新已有记录时返回真实稳定记录 ID；回读失败会回滚写入。Ozon scope 为所选店铺 UUID，迁移不会复制或改写旧 JSON 内容。全局管理员仍可跨租户查看，但不能把开发默认租户回退当作租户切换来保存配置或运行实时发布检查；Admin 会显示跨租户只读提示并禁用相关按钮。保存店铺级配置要求商品可见且拥有所选店铺操作权限；运行预检和真实提交继续保留既有“商品全部关联店铺可操作”保护，并额外校验所选店铺。仅查看授权返回明确 403，不可见或跨租户普通账号返回 404。Ozon 凭证拒绝、上游拒绝与临时故障分别使用安全的 502/503 领域错误，响应携带 `traceId` 且不包含凭证明文。

## 4. 自动化与工程门禁证据

| 检查 | 结果 |
| --- | --- |
| `go test ./... -count=1` | 通过，全量后端包无失败。 |
| `go vet ./...` | 通过。 |
| `go build ./...` | 通过。 |
| `pnpm test:frontend` | 通过，23 个文件、89 项测试；包含 Ozon 配置稳定 ID 与 HTTP 错误 envelope 回归。 |
| `pnpm test:contracts` | 通过，4 项契约测试。 |
| `pnpm build:admin` | 通过。 |
| Ozon 专项 Playwright E2E | 通过，10/10；新增覆盖已有配置更新与刷新保持、稳定 ID、结构化 403、跨租户只读禁用，并继续覆盖异步同步、四态差异、映射确认、未保存阻断、凭证停用提示与类目恢复、schema 漂移阻断、二次确认、幂等请求和五档视口。源码开发服务器与重建后 `:8000` Admin 容器各通过一轮，所有写请求均由守卫拦截。 |
| `ADMIN_E2E_PORT=8018 pnpm test:affected` | 通过；Admin smoke 6/6、前端 89/89、契约 4/4、全量后端测试均通过。默认端口 8001 的首次运行在进入用例前发生 WebServer 启动超时，隔离端口重跑通过。 |
| `pnpm architecture:affected` | 通过；架构测试 11/11，边界检查 0 个新增或扩大违规。 |
| `pnpm check:ui-copy --strict` | 通过。 |
| `pnpm quality:sensitive` | 通过，变更行高置信敏感信息发现数为 0。 |
| `pnpm check:dev` | 工具链与 Docker 均可用，但独立 worktree 未创建根 `.env`，因此命令按设计返回失败；未复制或生成敏感配置。 |
| Backend / Admin 容器与本地浏览器 | Backend、Admin 从当前 worktree 重建；Backend health 与 Admin/Backend HTTP 探针均为 200。Chrome 实际登录会话显示跨租户只读提示、预检按钮禁用且无 `internal error`/控制台错误；重建后目标配置 GET 为 200，日志中无 Ozon 商品导入调用。 |

本机验收时 `TEST_DATABASE_URL` 与 `TEST_REDIS_URL` 均未配置；现有运行栈的 PostgreSQL/Redis 不是专用破坏性测试环境，因此未把外部 PostgreSQL/Redis 集成套件记为已执行。已有单元、HTTP、SQLite、队列状态与 Worker 回归通过；生产前应在安全隔离的 PostgreSQL/Redis 环境补跑 `pnpm test:backend:integration`、`pnpm test:db` 和 `pnpm test:redis`。

### 4.1 2026-08-04 多 SKU 图片补充验收

| 检查 | 结果 |
| --- | --- |
| `TEST_AFFECTED_BASE=HEAD GOFLAGS=-p=1 ADMIN_E2E_PORT=18096 pnpm test:affected` | 通过；Admin smoke 6/6、前端 23 个文件 91/91、契约 4/4、全量后端均通过。Go 并行度只用于避免本机内存争用，不改变测试集合。 |
| Ozon 图片专项 Playwright E2E | 通过，10/10；覆盖多 SKU 独立主图、公共图批量应用与逐 SKU 调整、保存请求、具体 SKU 缺图、显式替代图、失效选择恢复和五档视口；全部 Ozon 写请求均拦截。 |
| `pnpm build:admin` | 通过。 |
| `pnpm architecture:test` / `TEST_AFFECTED_BASE=HEAD pnpm architecture:affected` | 通过；架构测试 11/11，扫描 456 个 TypeScript 文件，0 个新增或扩大违规，受影响门禁失败数 0。 |
| `pnpm quality:baseline:admin-ts` | 通过；当前 24 项 / 17 个签名，相对 30 项 / 18 个签名基线无新增或增加。 |
| `pnpm quality:sensitive` / `pnpm check:ui-copy --strict` | 通过；敏感信息发现数 0，UI 文案检查无问题。 |
| `go vet -p=1 ./...` / `go test -p 1 ./...` | 通过；本次变更的 19 个 Go 文件单独执行 `gofmt -l`，结果为空。 |
| `pnpm test:redis` | 通过。 |
| `pnpm check:dev` | 未通过；工具链与 Docker 均正常，仅本工作树未创建根 `.env` 或 `backend/.env`。未为验收复制或提交环境配置。 |

补充验收未调用真实 `/v3/product/import`、库存或归档接口。该次 `pnpm quality:affected` 曾被当前 Windows 工作树全仓 `gofmt -l .` 报告的 1,248 个行尾文件阻断；后续逐文件诊断确认这些报告项均为系统 `core.autocrlf=true` 在缺少仓库行尾规则时产生的 CRLF checkout，归一化后与 `gofmt` 输出一致，真实格式差异为 0。仓库现已通过根目录 `.gitattributes` 固定 Go 文件为 LF，完整 `pnpm quality:affected` 已恢复通过，未扩大 baseline。一次冗余的默认并行 Go 全量复跑曾因本机内存耗尽失败，限制编译并行度后同一全量测试集合通过。

### 4.2 2026-08-04 统一刊登中心补充验收

- 旧 `/product/ozon-publish` 深链保留 `productId` / `shopId` 并跳转到 `/product/publishing-center`；商品菜单只显示通用“刊登中心”和“刊登进度”。
- Admin E2E 11/11 通过：覆盖店铺级配置保存与刷新回显、库存不进入保存 payload、v3 多值与可重复复杂属性、只读预检逐 SKU 最终值、取消确认 0 次提交、确认后恰好 1 次拦截提交、类目维护折叠、loading/empty/error/readonly、五档视口无根横向溢出和移动端检查区前置，以及 `retryable` 重试门禁。所有写请求均由 `NetworkWriteGuard` 拦截，未调用真实 Ozon 商品、库存或归档接口。
- 后端新增迁移、配置解析、属性规范化、预检、任务快照和 Adapter 回归；API 契约新增 `shopId` 查询、`ozonListing`、`ozonPreview` 与 `resolvedOzon` 字段校验。具体全量门禁结果以本工作树 `docs/PROGRESS.md` 最新条目为准。

### 4.3 2026-08-04 多 SKU 变体属性补充验收

- Admin 为每个 SKU 展示本地属性候选和 Ozon 词典/文本输入；自动匹配只接受同名/明确别名与词典精确值，不猜测近似值。选择某属性为 SKU 维度时会从商品公共属性中移除，避免同一属性以公共值和逐 SKU 值重复提交。
- `platformAttributes.version=3` 按店铺保存 `skuVariantAttributeIds` 和 `skuAttributeOverrides`；旧单值/v2 配置可读取，旧多 SKU 配置没有明确映射时可继续编辑但不能进入提交。
- `ResolveOzonListing` 为每个 SKU 生成最终属性与来源，发布前检查校验缺值和组合唯一性，任务快照保存精确结果；Adapter 对实时 schema 再校验，并对缺映射的旧多 SKU 快照保证 `/v3/product/import` 调用次数为 0。
- 专项 Playwright E2E 13/13 通过，包含逐 SKU 白色/黑色词典映射保存、旧表单兼容、五档视口和全部写请求拦截；前端服务单测 19/19、API 契约 4/4、Adapter 零写入回归通过。未执行真实 Ozon 商品、库存或归档写请求。

## 5. 已知基线与剩余风险

此前的 Go 格式/换行基线已关闭：根目录 `.gitattributes` 将 `*.go`、`go.mod`、`go.sum`、`go.work` 和 `go.work.sum` 固定为 LF，当前 1,279 个 Go 文件不再包含 CRLF，仓库级 `gofmt -l .` 返回 0 项。`pnpm quality:backend` 和完整 `pnpm quality:affected` 均已通过；规范化没有产生历史业务代码 diff，也没有扩大任何 baseline。

仍需生产前关闭的风险：

1. 当前 `mery` 店铺的 Ozon API Key 已停用；更新凭证并重新完成连接、类目、属性和字典只读复验前，当前环境不可用于 Ozon 刊登。
2. 未执行真实 Ozon 商品导入、库存写入或归档，无法确认当前真实商品数据与合同配置下的平台业务校验结果。
3. Ozon 导入和审核具有异步性。任务失败或调用结果未知时不自动重试；只有服务端明确判定 `retryable=true` 才提供手动重试，其他情况仍依赖运营人员在 Ozon 后台与刊登进度人工核对。
4. 尚未提供经生产演练验证的“核对后恢复/重新提交”专用流程；在该流程落地前不得自动放量。
5. 外部 PostgreSQL/Redis 集成套件本次因安全测试环境未配置而未执行。
6. 多 SKU 图片补充实现没有执行真实 `/v3/product/import`；自动化仅使用 Go `httptest` Ozon server 与 Admin 写请求拦截，生产真实图片抓取和平台审核仍属于 Deferred 验收。
7. 当前只允许普通属性作为 SKU 变体维度；`attribute_complex_id > 0` 的字段继续按商品级复杂组合提交。如果目标类目要求用复杂属性区分 SKU，系统会明确阻断，需先与 Ozon 类目语义核对或将商品拆分为单品，不能绕过检查静默发送。

## 6. 发布与回滚边界

- 数据库变更为新增同步运行、差异、类目映射记录及商品平台配置字段，并把商品平台配置唯一键扩展为店铺 scope；迁移只回填 `config_scope_key`、保留旧 JSON 和 legacy 回退，不包含删除业务数据。
- 建议先以 Beta、小店铺、小批量和人工值守方式发布；首批真实提交必须逐条核对 TradeMind 任务与 Ozon 后台结果。
- 应用回滚时可回退到发布前镜像或分支；新增表和字段保留，避免破坏已记录的同步、映射与提交审计数据。
- 任何失败或结果未知任务都不得直接换 `Idempotency-Key` 重提；先核对外部事实，再由明确的人工恢复流程处理。

## 7. 签收口径

本次结论为：**Ozon 类目与刊登流程开发竣工验收通过；真实只读联调曾通过，但当前凭证已失效、更新后必须复验；生产真实刊登验收 Deferred，非 Production Ready。**
