# Admin E2E 测试

本目录保存 TradeMind Admin 的 Playwright Test 自动化回归。开发过程中的动态验收使用 Playwright MCP；仓库内可重复运行和 CI 使用 Playwright Test。

## 安装浏览器

```bash
pnpm --filter @trademind/admin exec playwright install chromium
```

CI 使用：

```bash
pnpm --filter @trademind/admin exec playwright install --with-deps chromium
```

## 本地运行

如果 `http://localhost:8001` 或 `http://127.0.0.1:8001` 已有 Admin dev server，Playwright 会复用。否则会通过配置自动运行：

```bash
pnpm dev:admin -- --host 127.0.0.1 --port 8001
```

常用命令：

```bash
pnpm test:e2e
pnpm test:e2e:smoke
pnpm test:e2e:product-draft
pnpm test:e2e:publish-safety
pnpm test:e2e:contracts
pnpm test:e2e:headed
pnpm test:e2e:ui
pnpm test:e2e:report
```

## 目录

- `fixtures/`：Playwright fixture，统一安装 auth、Mock、Guard。
- `mocks/`：API envelope 与业务 Mock。
- `pages/`：稳定 Page Object，只封装页面级动作。
- `specs/`：按回归主题拆分。
- `utils/`：Network Write Guard、Console Guard、断言和路由。

## Mock 规范

所有 `/api/v1/**` 业务接口由测试 Mock，CI 不需要后端服务。统一响应结构是：

```ts
{ code: number; message: string; data: T; traceId?: string }
```

`GET /api/v1/image/providers` 的 `data` 必须是 `ImageProviderCapability[]`，不能返回 `{ list: [] }` 包装。

## 写请求安全

默认阻断所有 API 非 GET 请求。测试必须显式 allow 写接口，并断言 method、URL、payload、次数和额外写请求。取消操作必须为 0 请求，确认操作必须为 1 请求，快速重复点击不得重复提交。

## Console allowlist

默认 pageerror、console.error、新增 React warning、新增 AntD warning 失败。allowlist 只能加入精确、稳定、已确认的既有 warning，并说明原因。

## 新增页面测试

新增 Admin 页面必须补 route smoke、auth Mock、normal/loading/empty/error/readonly、桌面和 375px、根节点 overflow、Console guard、写请求 Mock、取消 0 请求、单次提交和关键 payload。若页面有 URL 状态，还要补 deep-link 和 refresh restore。

## 更新 API Mock

修改 service、response envelope 或 payload 时，先确认真实 `admin/src/services/**` 类型和 `admin/src/services/request.ts` envelope，再更新 `mocks/` 和 contract tests。不得修改生产代码迁就错误 Mock。

## 调试失败

- 查看 `playwright-report/admin-e2e`。
- 查看 `test-results/admin-e2e` 中 trace、video、screenshot。
- 优先定位首个真实根因。
- Mock 错误修 Mock；真实生产缺陷记录并最小修复。
- 禁止直接 skip 或删除失败测试。

## CI 行为

`.github/workflows/admin-e2e.yml` 在 Admin 相关 PR、dev push、手动触发和每日定时回归运行。CI 不连接生产数据库、真实 Redis、真实平台、真实店铺或真实 API；所有业务请求由测试 Mock。

## 禁止

不得使用生产凭据、真实账号、真实店铺、真实 API Token 或真实平台写接口。不得提交 `playwright-report/`、`test-results/`、`blob-report/`、`.playwright/` 或临时认证状态。

## 已知限制

首期仅覆盖 Chromium 和 P0 核心回归，不做像素级视觉快照基线。P1/P2 可在后续页面迭代中持续补充。
