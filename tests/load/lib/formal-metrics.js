import { Counter, Trend } from 'k6/metrics';

export const FORMAL_PHASE_TAG_KEY = 'phase';
export const FORMAL_STEADY_PHASE_VALUE = 'steady';
export const FORMAL_METRIC_REGISTRY_VERSION = 1;

export const formalRouteMetricDefinitions = [
  { metricId: 'productList', displayName: 'Product List', scenarioId: 'product_list', routeId: 'GET /api/v1/products', operationId: 'list', group: 'read-heavy', durationThresholds: ['p(95)<800', 'p(99)<1500'], requiredForBaseline: true, requiredForCurrent: true, requiredForSoak: true },
  { metricId: 'orderList', displayName: 'Order List', scenarioId: 'order_list', routeId: 'GET /api/v1/orders', operationId: 'list', group: 'read-heavy', durationThresholds: ['p(95)<800', 'p(99)<1500'], requiredForBaseline: true, requiredForCurrent: true, requiredForSoak: true },
  { metricId: 'inventoryList', displayName: 'Inventory List', scenarioId: 'inventory_list', routeId: 'GET /api/v1/inventory', operationId: 'list', group: 'read-heavy', durationThresholds: ['p(95)<800', 'p(99)<1500'], requiredForBaseline: true, requiredForCurrent: true, requiredForSoak: true },
  { metricId: 'taskList', displayName: 'Task List', scenarioId: 'task_list', routeId: 'GET /api/v1/task-center/failures', operationId: 'list', group: 'read-heavy', durationThresholds: ['p(95)<800', 'p(99)<1500'], requiredForBaseline: true, requiredForCurrent: true, requiredForSoak: true },
  { metricId: 'webhookEventList', displayName: 'Webhook Event List', scenarioId: 'webhook_event_list', routeId: 'GET /api/v1/webhook-events', operationId: 'list', group: 'read-heavy', durationThresholds: ['p(95)<800', 'p(99)<1500'], requiredForBaseline: true, requiredForCurrent: true, requiredForSoak: true },
  { metricId: 'operationLogList', displayName: 'Operation Log List', scenarioId: 'operation_log_list', routeId: 'GET /api/v1/operation-logs', operationId: 'list', group: 'read-heavy', durationThresholds: ['p(95)<800', 'p(99)<1500'], requiredForBaseline: true, requiredForCurrent: true, requiredForSoak: true },
  { metricId: 'webhookIngestion', displayName: 'Webhook Ingestion', scenarioId: 'webhook_ingestion', routeId: 'POST /api/v1/webhooks/internal-test/ping', operationId: 'ingest', group: 'mixed', durationThresholds: ['p(95)<1200', 'p(99)<2500'], requiredForBaseline: true, requiredForCurrent: true, requiredForSoak: true },
  { metricId: 'providerMockFlow', displayName: 'Provider Mock Flow', scenarioId: 'provider_mock_flow', routeId: 'GET /health/live', operationId: 'health_live', group: 'mixed', durationThresholds: ['p(95)<500', 'p(99)<1000'], requiredForBaseline: true, requiredForCurrent: true, requiredForSoak: true },
  { metricId: 'authInvalidLogin', displayName: 'Auth Invalid Login', scenarioId: 'auth_invalid_login', routeId: 'POST /api/v1/auth/login', operationId: 'login_invalid', group: 'auth-security', durationThresholds: ['p(95)<500', 'p(99)<1000'], requiredForBaseline: true, requiredForCurrent: true, requiredForSoak: true },
  { metricId: 'webhookInvalidSignature', displayName: 'Webhook Invalid Signature', scenarioId: 'webhook_invalid_signature', routeId: 'POST /api/v1/webhooks/internal-test/ping', operationId: 'signature_check', group: 'auth-security', durationThresholds: ['p(95)<500', 'p(99)<1000'], requiredForBaseline: true, requiredForCurrent: true, requiredForSoak: true },
];

export function getFormalRouteMetricDefinitions() {
  return formalRouteMetricDefinitions.map((definition) => ({
    ...definition,
    metricType: 'trend+counter',
    phaseTagKey: FORMAL_PHASE_TAG_KEY,
    steadyPhaseValue: FORMAL_STEADY_PHASE_VALUE,
    minimumSampleCount: 100,
    unit: 'ms',
    direction: 'lower_is_better',
    durationMetricName: `p7_${definition.scenarioId}_duration`,
    requestMetricName: `p7_${definition.scenarioId}_requests`,
    steadyDurationMetricName: `p7_${definition.scenarioId}_steady_duration`,
    steadyRequestMetricName: `p7_${definition.scenarioId}_steady_requests`,
    absoluteSloId: `${definition.scenarioId}_latency`,
  }));
}

export function getFormalMetricBinding(metricId) {
  return getFormalRouteMetricDefinitions().find((definition) => definition.metricId === metricId);
}

export function createFormalRouteMetrics() {
  const aggregateTrends = {};
  const aggregateCounters = {};
  const steadyTrends = {};
  const steadyCounters = {};
  for (const definition of getFormalRouteMetricDefinitions()) {
    aggregateTrends[definition.metricId] = new Trend(definition.durationMetricName);
    aggregateCounters[definition.metricId] = new Counter(definition.requestMetricName);
    steadyTrends[definition.metricId] = new Trend(definition.steadyDurationMetricName);
    steadyCounters[definition.metricId] = new Counter(definition.steadyRequestMetricName);
  }
  return { aggregateTrends, aggregateCounters, steadyTrends, steadyCounters };
}

export function metricTags(metricId, phase = FORMAL_STEADY_PHASE_VALUE, expectedStatusClass = '2xx') {
  const definition = getFormalMetricBinding(metricId);
  return {
    scenarioId: definition?.scenarioId || metricId,
    routeId: definition?.routeId || metricId,
    operationId: definition?.operationId || 'unknown',
    [FORMAL_PHASE_TAG_KEY]: phase,
    expectedStatusClass,
  };
}

export function recordFormalRouteMetric(metrics, metricId, duration, phase = FORMAL_STEADY_PHASE_VALUE, expectedStatusClass = '2xx') {
  const tags = metricTags(metricId, phase, expectedStatusClass);
  metrics.aggregateTrends[metricId]?.add(duration, tags);
  metrics.aggregateCounters[metricId]?.add(1, tags);
  if (phase === FORMAL_STEADY_PHASE_VALUE) {
    metrics.steadyTrends[metricId]?.add(duration, tags);
    metrics.steadyCounters[metricId]?.add(1, tags);
  }
}

export const formalThresholds = {
  http_req_failed: ['rate<0.01'],
  unexpected_401: ['count==0'],
  unexpected_403: ['count==0'],
  unexpected_404: ['count==0'],
  unexpected_5xx: ['rate<0.002'],
  auth_login_failures: ['count==0'],
  'http_req_duration{group:read-heavy}': ['p(95)<800', 'p(99)<1500'],
  'http_req_duration{group:mixed}': ['p(95)<1200', 'p(99)<2500'],
  'http_req_duration{group:auth}': ['p(95)<500', 'p(99)<1000'],
  ...Object.fromEntries(getFormalRouteMetricDefinitions().map((definition) => [definition.durationMetricName, definition.durationThresholds])),
};
