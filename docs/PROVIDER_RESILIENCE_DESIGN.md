# Provider 韧性设计（P2）

> 第三方平台、采集服务、AI、存储等出站调用统一经 **`httpclient` + 超时 + 可选熔断 + 健康缓存** 保护主业务。

## HTTP Client（`internal/pkg/httpclient`）

### 默认配置 `DefaultConfig()`

| 项 | 默认值 |
| --- | --- |
| `ConnectTimeout` | 10s |
| `RequestTimeout` | 60s |
| `ResponseHeaderTimeout` | 30s |
| `MaxResponseBytes` | 32 MiB |
| `MaxRedirects` | 3 |
| `RetryPolicy` | `taskretry.DefaultPolicy()`（5 次） |
| `UserAgent` | `TradeMind/1.0` |

### 核心方法

- `Do(ctx, req)`：单次请求；可选信号量 `maxConcurrent` 限流。
- `DoWithRetry(ctx, build)`：对 5xx / 429 / 网络错误自动退避重试。
- `ReadLimitedBody(resp)`：防止超大响应 OOM。
- `RedactURL(url)`：日志脱敏 query 凭证。

### 熔断器挂载

```go
client.SetCircuitBreaker(httpclient.NewCircuitBreaker(threshold, openDuration))
```

默认 threshold **5**，open 窗口 **30s**（见 `CIRCUIT_BREAKER_AND_RATE_LIMIT.md`）。

熔断打开时 `Do` 返回 `circuit_open: provider temporarily unavailable`，**不发起网络请求**。

## 错误分类联动

`taskretry.Classify(err, httpStatus)` 驱动：

- 是否重试（`DoWithRetry` / 任务 Worker）。
- 失败任务中心展示码（`rate_limited`、`provider_5xx` 等）。
- 幂等 `Fail(retryable)` 决策。

## Provider Health（`internal/pkg/providerhealth`）

| 状态 | 含义 |
| --- | --- |
| `available` | 探测成功 |
| `degraded` | 可用但延迟/部分失败 |
| `rate_limited` | 限流中 |
| `circuit_open` | 熔断打开 |
| `unauthorized` | 凭证问题 |
| `not_configured` | 未配置 Checker |
| `temporary_unavailable` | 短暂不可用 |
| `manual_required` | 需人工处理 |

- `Registry` 缓存 TTL 默认 **5 分钟**。
- `Get(ctx, provider, capability, force)` 过期后重新探测。
- 配置状态中心 **Provider Health** 项：缓存检查，失败不影响 `/health/live`。

## 分层约定

```text
Handler → Service → Provider 接口 → httpclient → 第三方 API
```

- Provider 实现内构造 `httpclient.Client`，不得绕过超时。
- 平台 SDK（COS/OSS）使用各自超时配置，语义与 httpclient 对齐。
- 日志禁止输出完整 API Key / Token；使用 `RedactURL` 与 raw 截断。

## 生产检查

staging/production 须非 local `STORAGE_PROVIDER`；各平台 `timeout_sec` 与 httpclient 取较小值。详见 `provider.md`、`CIRCUIT_BREAKER_AND_RATE_LIMIT.md`。
