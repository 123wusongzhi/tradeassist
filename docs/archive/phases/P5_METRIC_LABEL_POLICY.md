# P5 Metric Label Policy

## 禁止 Label
request_id, trace_id, user_id, tenant_id, shop_id, task_id, order_id, product_id, sku_id, event_id, object_key, raw_url, error_message

## 允许 Label
method, route_template, status_class, module, operation, provider, platform, result, error_class, task_type, event_group, scanner, severity

实现：backend/internal/pkg/metrics/label_policy.go + 自动测试。
