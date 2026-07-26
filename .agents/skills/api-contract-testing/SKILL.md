---
name: api-contract-testing
description: TradeMind API 契约、前后端 DTO、Admin Mock、response envelope 和关键 endpoint 漂移检测规范
---

# TradeMind API 契约测试规范

## 自动适用

涉及 API URL、HTTP method、query、payload、DTO、response envelope、错误结构、前端 service/types、后端 handler/DTO、Admin E2E mock 时自动适用。

## 当前仓库状态

当前没有完整 OpenAPI/Swagger 文件。不要强行一次补齐全项目 OpenAPI；先用关键 endpoint 契约清单、共享 fixtures 和后端/前端测试共同校验。

## 契约范围

至少覆盖：

- `GET /api/v1/auth/profile`
- `GET /api/v1/image/providers`
- `GET /api/v1/products/:id`
- `GET /api/v1/products/:id/readiness`
- `GET /api/v1/products/:id/publications`
- `GET /api/v1/product-publications/:id/douyin/sku-bindings`
- `GET /api/v1/products/:id/publish-targets`
- `POST /api/v1/products/:id/platform-configs/douyin_shop/create-draft`
- `POST /api/v1/products/:id/publish`

## 必须验证

- method、URL、path params、query、request body。
- success envelope：`{ code, message, data, traceId? }`。
- error envelope：`code !== 0`、message、data/null、traceId。
- data shape、pagination、nullable、enum、业务错误 code/message。
- Admin E2E mock 与真实后端 route 的 method/data/envelope 一致。

## 实施方式

- 契约清单位于 `tests/contracts/**`。
- 前端 unit 测试验证 service 对契约的 URL/payload/envelope 处理。
- Go 测试验证 handler/DTO/envelope 或至少验证 route 表/关键 shape。
- Admin Playwright `@contract` 继续验证浏览器 Mock 消费契约。

## 禁止项

不得把“前端 Mock 可运行”视为真实后端契约正确。不得只验证 HTTP 200。不得手工复制大量易漂移 schema；无法共享 runtime schema 时只固定关键 endpoint fixtures。
