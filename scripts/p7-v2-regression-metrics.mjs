export const CORE_SCENARIOS = [
  'Product List',
  'Order List',
  'Inventory List',
  'Task List',
  'Webhook Event List',
  'Operation Log List',
  'Webhook Ingestion',
  'Provider Mock Flow',
  'Auth/Security',
];

export const SCENARIO_METRICS = {
  'Product List': ['p7_product_list_duration', 'p7_product_list_requests'],
  'Order List': ['p7_order_list_duration', 'p7_order_list_requests'],
  'Inventory List': ['p7_inventory_list_duration', 'p7_inventory_list_requests'],
  'Task List': ['p7_task_list_duration', 'p7_task_list_requests'],
  'Webhook Event List': ['p7_webhook_event_list_duration', 'p7_webhook_event_list_requests'],
  'Operation Log List': ['p7_operation_log_list_duration', 'p7_operation_log_list_requests'],
  'Webhook Ingestion': ['p7_webhook_ingestion_duration', 'p7_webhook_ingestion_requests'],
  'Provider Mock Flow': ['p7_provider_mock_flow_duration', 'p7_provider_mock_flow_requests'],
  'Auth/Security': ['p7_auth_security_duration', 'p7_auth_security_requests'],
};

export const METRIC_METADATA = {
  p95: { metricFamily: 'latency', unit: 'ms', direction: 'lower_is_better', zeroPolicy: 'valid_value', comparisonMode: 'relative_and_materiality', relativeThreshold: 0.1, materialityFloor: { type: 'absolute_delta', value: 2, unit: 'ms' }, minimumSampleCount: 100, missingPolicy: 'not_comparable' },
  p99: { metricFamily: 'latency', unit: 'ms', direction: 'lower_is_better', zeroPolicy: 'invalid_when_zero', comparisonMode: 'relative_and_materiality', relativeThreshold: 0.15, materialityFloor: { type: 'absolute_delta', value: 3, unit: 'ms' }, minimumSampleCount: 100, missingPolicy: 'not_comparable' },
  rps: { metricFamily: 'throughput', unit: 'rps', direction: 'higher_is_better', zeroPolicy: 'invalid_when_zero', comparisonMode: 'relative', relativeThreshold: 0.1, materialityFloor: null, minimumSampleCount: 100, missingPolicy: 'not_comparable' },
  errorRate: { metricFamily: 'rate', unit: 'ratio', direction: 'lower_is_better', zeroPolicy: 'valid_value', comparisonMode: 'absolute_budget', relativeThreshold: null, materialityFloor: { type: 'percentage_point', value: 0.002, unit: 'ratio' }, minimumSampleCount: 100, missingPolicy: 'not_comparable' },
  timeouts: { metricFamily: 'count', unit: 'count', direction: 'lower_is_better', zeroPolicy: 'valid_value', comparisonMode: 'absolute_budget', relativeThreshold: null, materialityFloor: null, minimumSampleCount: 100, missingPolicy: 'not_comparable' },
};
