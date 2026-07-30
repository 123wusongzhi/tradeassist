# 熔断与限流设计（P2）

> Provider 维度 **Circuit Breaker** 与 HTTP **429 Retry-After** 协同，避免雪崩并尊重平台限速。

## 熔断器状态机

实现：`internal/pkg/httpclient.CircuitBreaker`

```text
closed ──(连续失败 ≥ threshold)──► open
open ──(openDuration 过后)──► half_open
half_open ──(探测成功)──► closed
half_open ──(探测失败)──► open
```

| 状态 | `Allow()` 行为 |
| --- | --- |
| `closed` | 允许所有请求 |
| `open` | 拒绝，直至 `openUntil` |
| `half_open` | 最多 `halfOpenAllowed`（默认 2）个探测请求 |

### 默认参数

- `threshold = 5` 连续失败（连接错误或 HTTP ≥500）。
- `openDuration = 30s`。
- 成功响应（<500）调用 `RecordSuccess()` → 重置为 `closed`。
- `half_open` 内失败立即重新 `open`。

### 与 Client 集成

```text
Do() 前 Allow() == false → 直接错误 circuit_open
Do() 后 status >= 500 或 net err → RecordFailure()
否则 → RecordSuccess()
```

配置状态中心展示 **熔断状态** 项（Provider 级 closed/open/half_open）。

## 限流（HTTP 429）

### 分类

`taskretry.Classify`：

- 错误含 `rate limit` / `429` → `CodeRateLimited`，`Retryable=true`。
- `httpStatus == 429` 同上。

### Retry-After 解析

`taskretry.ParseRetryAfter(header)` 支持：

1. 整数秒数（如 `120`）。
2. HTTP-date（RFC 7231）。

`httpclient.DoWithRetry` 在 429 时：

```text
wait = max(policy backoff, Retry-After)
```

再进入下一轮 attempt，直至 `MaxAttempts` 或 context 取消。

### 平台错误码

抖店等 Provider 将平台限流映射为 `DOUYIN_RATE_LIMITED` / `*_RATE_LIMITED`，任务 Worker 标记 `retrying` + `next_retry_at`。

库存模块另有滚动分钟计数 `InventoryRateObserveStarted`（店铺级节流观测，非全局熔断）。

## 重试 vs 熔断

| 机制 | 作用范围 | 目的 |
| --- | --- | --- |
| 指数/阶梯退避 | 单次调用 / 单任务 | 恢复瞬时故障 |
| 熔断器 | Provider 全实例共享（进程内） | 停止向已故障依赖施压 |
| 429 Retry-After | 单次调用 | 遵守平台限速 |

详见 `PROVIDER_RESILIENCE_DESIGN.md`、`TASK_RELIABILITY_DESIGN.md`。
