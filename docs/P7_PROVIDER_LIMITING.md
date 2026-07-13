# P7 Provider Limiting

Status: design placeholder, implementation pending.

Required interface:

```go
type ProviderLimiter interface {
    Wait(ctx context.Context, provider, operation string) error
    Observe(result ProviderResult)
}
```

Provider 429, Retry-After, adaptive slowdown and circuit-breaker coordination must be verified with mock providers before P7 closure.
