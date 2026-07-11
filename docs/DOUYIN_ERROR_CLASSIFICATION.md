# 抖店错误分类设计

## ErrorClass 枚举

| ErrorClass | 含义 | 典型错误码 | SafeRetry | ManualReview |
|-----------|------|-----------|-----------|--------------|
| `auth_error` | 鉴权/token 失效 | DOUYIN_AUTH_EXPIRED, DOUYIN_REAUTHORIZATION_REQUIRED | false | false |
| `rate_limited` | 频率限制 | DOUYIN_RATE_LIMITED | true | false |
| `timeout` | 超时（读侧） | DOUYIN_REQUEST_TIMEOUT | true | false |
| `unknown_result` | 写入超时，结果未知 | DOUYIN_UNKNOWN_RESULT | false | **true** |
| `contract_mismatch` | 接口形状不符/合同未核查 | DOUYIN_CONTRACT_MISMATCH | false | false |
| `validation` | 请求参数校验失败 | DOUYIN_VALIDATION_FAILED | false | false |
| `permission` | 权限拒绝 | DOUYIN_PERMISSION_DENIED | false | false |
| `not_found` | 资源不存在 | DOUYIN_RESOURCE_NOT_FOUND, DOUYIN_PRODUCT_NOT_FOUND | false | false |
| `network` | 网络层错误 | — | true | false |
| `system` | 系统/配置错误 | DOUYIN_NOT_CONFIGURED | false | false |

## unknown_result 处理规则

写操作（product.addV2, sku.syncStock, image upload）超时后：

1. `UnknownResult = true`
2. `SafeRetry = false`（禁止自动重试）
3. `ManualReviewRequired = true`（需人工核查）
4. 任务状态标记为 `unknown_result`
5. 重试前调用 `tryRecoverDouyinDraftFromPlatform` 确认操作结果

## ClassifyError 函数

```go
errClass := douyinshop.ClassifyError(err)
```

优先使用 `Error.ErrorClass` 字段，回退到 flag 推断（AuthExpired, RateLimited, PermissionDenied），最后做字符串匹配。

## 日志安全

以下字段禁止出现在日志：
- `access_token`
- `refresh_token`
- `app_secret`
- `password`
- `cookie`

`SanitizeErrorText` 通过 `safefields.RedactString` 过滤敏感词。
