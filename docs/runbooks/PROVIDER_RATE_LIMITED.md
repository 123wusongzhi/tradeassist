# Provider Rate Limited

Meaning: provider 429 or Retry-After ratio exceeds baseline.

Check: provider/operation, retry-after values, circuit breaker state, task type and tenant/shop concentration.

Mitigate: obey Retry-After, lower concurrency, delay retries and classify unknown write results for reconciliation.

Scale: request provider quota only after local burst and retry behavior are verified.

Forbidden: do not blind-retry external writes.

Recovery: 429 ratio drops and provider wait returns to baseline.
