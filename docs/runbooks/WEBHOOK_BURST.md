# Webhook Burst

Meaning: webhook receive rate or queue lag exceeds capacity.

Check: signature failures, duplicate event ratio, queue age, persist latency and shop concentration.

Mitigate: keep ACK path short, apply platform/shop burst policy, delay processing tasks and protect DB pool.

Scale: add worker capacity only if DB and provider limits allow.

Forbidden: do not drop already acknowledged events or skip signature/replay checks.

Recovery: webhook lag and duplicate storm metrics return to baseline.
