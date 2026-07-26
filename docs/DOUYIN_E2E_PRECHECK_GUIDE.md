# 抖店 E2E 前置检查指南（H1.3 / P3）

> **抖店 Release Candidate** · **Douyin Production Adapter Implemented** · 真实 E2E **未执行**（Real Credential Verification Deferred） · Tag deferred · 非 Production Ready

## 边界说明

- Phase P3 已完成适配器**代码**：OAuth、Token、类目、图片、平台草稿、订单、库存、Webhook 签名与事件路由
- 本阶段**不执行**真实抖店 E2E、不自动写不可逆接口、不直接上架
- 凭证缺失状态：`blocked_by_real_credentials` / `environment_required`（**不是**系统 P0/P1 失败）
- 创建本地草稿 ≠ 平台真实草稿；创建抖店草稿 ≠ 商品上架
- **代码实现完成 ≠ 真实 E2E 已通过**

## 前置条件清单

| 检查项 | 通过标准 | 缺失时状态 |
| --- | --- | --- |
| App Key / App Secret | 已配置（脱敏展示） | 待真实凭证 |
| OAuth 授权 | ≥1 已授权店铺 | 未授权 |
| Token 有效性 | 结构检查 / 可选 liveTest | 需人工确认 |
| 类目 / 属性同步 | 有缓存数据 | 当前跳过 |
| Storage `public_base` | 公网 HTTPS 可访问 | 待公网 Storage |
| 抖店图片上传 | 前置检查通过 | 待公网 Storage |
| Release Candidate 标识 | 保留 | — |

## 页面入口

| 页面 | 用途 |
| --- | --- |
| `/settings/platforms?platform=douyin_shop` | 抖店接入 + 生产预检面板 |
| `/settings/config-status` | 凭证 / Storage / 发布能力总览 |
| `/product/drafts` | 刊登前商品准备 |
| `/product/publish-batches` | 批量刊登边界提示 |
| `/ops/task-center/failures?platform=douyin_shop` | 抖店相关失败 |

## 脚本（不自动执行真实 E2E）

```bash
# 前置检查（exit 3 = blocked_by_real_credentials）
./scripts/douyin-e2e-preflight.sh

# 只读探针（需凭证）
./scripts/douyin-e2e-readonly.sh

# 写探针（需 ALLOW_DOUYIN_WRITE_TEST=true）
./scripts/douyin-e2e-write.sh

# 汇总 Markdown 报告
./scripts/douyin-e2e-report.sh
```

报告字段：`passed` / `passed_with_warning` / `skipped` / `blocked_by_real_credentials` / `blocked_by_environment` / `failed`

## 凭证缺失文案（统一）

> 当前未配置抖店真实凭证，系统只能完成本地 Demo 与前置检查，不能执行真实抖店 E2E。

按钮：去配置平台凭证 · 查看 E2E 准备清单 · 查看失败任务

## 相关文档

- [`DOUYIN_E2E_CHECKLIST.md`](DOUYIN_E2E_CHECKLIST.md)
- [`DOUYIN_RELEASE_GATE.md`](DOUYIN_RELEASE_GATE.md)
- [`STORAGE_PUBLIC_URL_GUIDE.md`](STORAGE_PUBLIC_URL_GUIDE.md)
