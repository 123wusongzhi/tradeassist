# P5 Observability Architecture

```text
HTTP / Webhook / Worker
  → Context Correlation (request_id, trace_id, tenant_id, task_id)
  → Structured Logger (JSON production)
  → Metrics Registry (Prometheus, low-cardinality labels)
  → Tracer (OpenTelemetry, safe attributes)
  → Exporter /internal/metrics (internal only)
  → Dashboard + Alert Rules + Admin Observability Center
```

统一门面：`backend/internal/pkg/observability`。禁止第二套日志/指标系统。
