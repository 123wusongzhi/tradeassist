# 抖店 Provider 架构设计

> 阶段：P3 | 状态：代码已实现，等待真实凭证 E2E 验证

## 层次结构

```
shop/douyin_oauth.go          ← OAuth 授权入口、token 持久化
    ↓
douyinshop/client.go          ← HTTP 客户端 + token 刷新 + singleflight
    ↓
douyinshop/facade.go          ← DouyinProvider 接口（所有能力统一入口）
    ↓
douyinshop/{category,image,product,order,inventory,customer,brand}.go
                               ← 各能力具体实现
    ↓
douyinshop/sign.go            ← 请求签名（App Key + 时间戳 + 参数 MD5）
douyinshop/request.go         ← HTTP 请求构建
douyinshop/response.go        ← 响应解析、平台错误映射
douyinshop/errors.go          ← 统一错误类型 + ErrorClass
```

## 关键设计决策

### 1. HTTPDoer 接口
Client 使用 `HTTPDoer` 接口注入 HTTP 实现，便于测试 mock。默认使用 `*http.Client`。

### 2. Token 刷新保护
- `EnsureFreshAccessSingleflight`：同一店铺同时只有一次 token 刷新请求
- `TokenVersion`（P3 新增）：多实例场景下防止旧版本 token 覆盖新版本

### 3. 错误分类
所有错误通过 `*Error` 类型传播，包含：
- `ErrorClass`：`auth_error`, `rate_limited`, `timeout`, `unknown_result`, `contract_mismatch` 等
- `UnknownResult`：写入超时后设为 true，禁止自动重试
- `SafeRetry`：幂等安全时为 true（只读操作）
- `RetryAfter`：来自 Retry-After 响应头

### 4. 写入幂等性
- 商品草稿：`douyin-product-draft-create:{shopId}:{draftId}:{version}`
- 图片上传：`douyin-image-upload:{shopId}:{objectKey}:{contentHash}`
- 超时后先执行 `tryRecoverDouyinDraftFromPlatform` 再决定是否重试

### 5. 外部依赖限制
- 客服消息 API：`blocked_by_contract_verification`
- 品牌列表 API：`blocked_by_contract_verification`
- 所有 OpenAPI 调用：需真实 App Key + 已授权店铺 token
