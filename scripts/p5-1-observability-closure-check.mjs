#!/usr/bin/env node
/**
 * Phase P5.1 observability closure scan.
 * This is intentionally stricter than P5: metric registration alone is not
 * treated as business instrumentation.
 */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..');
const checks = [];

function read(rel) {
  const p = path.join(root, rel);
  return fs.existsSync(p) ? fs.readFileSync(p, 'utf8') : '';
}
function exists(rel) {
  return fs.existsSync(path.join(root, rel));
}
function pass(id, message, detail = '') {
  checks.push({ id, status: 'passed', message, detail });
}
function fail(id, message, detail = '') {
  checks.push({ id, status: 'failed', message, detail });
}
function warn(id, message, detail = '') {
  checks.push({ id, status: 'warning', message, detail });
}
function requireText(id, rel, needles, message) {
  const text = read(rel);
  const missing = needles.filter((n) => !text.includes(n));
  if (missing.length === 0) pass(id, message || rel);
  else fail(id, message || `${rel} missing ${missing.join(', ')}`, rel);
}

requireText('db-runtime-collector', 'backend/internal/pkg/observability/db.go', [
  'StartDBStatsCollector',
  '.Stats()',
  'DBConnectionsOpen',
], 'DB runtime collector exists');
requireText('db-query-wrapper', 'backend/internal/pkg/observability/db.go', [
  'InstrumentedDB',
  'QueryContext',
  'BeginTx',
  'db_slow_query',
], 'DB query / transaction wrapper exists');
requireText('db-runtime-wired', 'backend/cmd/server/main.go', [
  'StartDBStatsCollector',
], 'DB stats collector wired in server');

requireText('otlp-http-exporter', 'backend/internal/pkg/tracing/tracing.go', [
  'httpSpanExporter',
  'ExportSpans',
  'normalizeEndpoint',
], 'OTLP HTTP exporter implemented without genproto dependency');
requireText('otlp-mock-test', 'backend/internal/pkg/tracing/tracing_test.go', [
  'TestHTTPExporterSendsSpanToMockCollector',
], 'Mock collector test exists');
requireText('telemetry-failure-safe', 'backend/internal/pkg/observability/observability.go', [
  'RecordTelemetryExportFailure',
  'RecordTelemetryDropped',
  'OnExportError',
], 'Telemetry failure-safe callbacks exist');

requireText('alert-evaluator', 'backend/internal/modules/alerting/execution.go', [
  'EvaluateRules',
  'AlertEvaluationRun',
  'StartEvaluatorWorker',
], 'Alert evaluator worker exists');
requireText('alert-delivery', 'backend/internal/modules/alerting/execution.go', [
  'AlertDelivery',
  'DeliverPending',
  'IdempotencyKey',
  'StartDeliveryWorker',
], 'Alert delivery worker exists');
requireText('alert-wired', 'backend/cmd/server/main.go', [
  'StartEvaluatorWorker',
  'StartDeliveryWorker',
], 'Alert evaluator/delivery wired in server');
requireText('alert-test', 'backend/internal/modules/alerting/alerting_test.go', [
  'TestAlertEvaluatorDeliveryAndRecovery',
], 'Alert trigger/delivery/recovery test exists');

requireText('slo-evaluator', 'backend/internal/modules/observabilitymod/slo.go', [
  'EvaluateSLOs',
  'ObserveSLO',
  'calculateSLO',
  'insufficient_data',
], 'SLO evaluator and budget/burn-rate logic exists');
requireText('slo-wired', 'backend/cmd/server/main.go', [
  'StartSLOEvaluatorWorker',
  'EnsureDefaultSLOs',
], 'SLO evaluator wired in server');
requireText('slo-test', 'backend/internal/modules/observabilitymod/slo_test.go', [
  'TestSLOErrorBudgetAndBurnRate',
], 'SLO calculation tests exist');

const business = [
  ['httpclient', 'backend/internal/pkg/httpclient/client.go', ['ObserveProvider', 'provider_requests_total']],
  ['webhook', 'backend/internal/modules/webhook', ['ObserveWebhook', 'webhook_events_processed_total']],
  ['ordersync', 'backend/internal/modules/ordersync', ['ObserveOrder', 'order_sync_runs_total']],
  ['inventory', 'backend/internal/modules/inventory', ['ObserveInventory', 'inventory_adjustments_total']],
  ['ai-text', 'backend/internal/modules/aiproducttext', ['ObserveAIText', 'ai_text_requests_total']],
  ['ai-image', 'backend/internal/modules/aiproductimage', ['ObserveAIImage', 'ai_image_provider_timeouts_total']],
  ['file-scan', 'backend/internal/modules/files', ['ObserveFileScan', 'file_scan_tasks_total']],
  ['security', 'backend/internal/modules/securitymod', ['ObserveSecurity', 'security_events_total']],
  ['auth', 'backend/internal/modules/auth', ['ObserveAuth', 'auth_login_attempts_total']],
];
for (const [id, rel, needles] of business) {
  const p = path.join(root, rel);
  let text = '';
  if (fs.existsSync(p) && fs.statSync(p).isDirectory()) {
    for (const file of fs.readdirSync(p).filter((f) => f.endsWith('.go'))) {
      text += read(path.join(rel, file)) + '\n';
    }
  } else {
    text = read(rel);
  }
  if (needles.some((n) => text.includes(n))) pass(`business-${id}`, `${id} real instrumentation detected`);
  else fail(`business-${id}`, `${id} real business instrumentation is still missing`, rel);
}

const catalog = read('backend/internal/pkg/metrics/catalog.go');
for (const metric of [
  'db_connections_open',
  'db_query_duration_seconds',
  'alert_deliveries',
  'slo_error_budget_remaining_ratio',
  'slo_burn_rate',
  'telemetry_dropped_items_total',
]) {
  if (catalog.includes(metric) || read('backend/internal/modules/alerting/execution.go').includes(metric)) {
    pass(`catalog-${metric}`, metric);
  } else {
    fail(`catalog-${metric}`, `${metric} missing`);
  }
}

if (exists('deploy/observability/dashboards/alerts-and-slo.json')) pass('dashboard-alerts-slo', 'alerts-and-slo dashboard exists');
else fail('dashboard-alerts-slo', 'alerts-and-slo dashboard missing');

for (const doc of [
  'docs/P5_1_EXECUTION_CLOSURE_AUDIT.md',
  'docs/P5_1_BUSINESS_INSTRUMENTATION.md',
  'docs/P5_1_DATABASE_OBSERVABILITY.md',
  'docs/P5_1_OTLP_DEPENDENCY_RESOLUTION.md',
  'docs/P5_1_ALERT_EXECUTION_REPORT.md',
  'docs/P5_1_SLO_EVALUATION_REPORT.md',
  'docs/P5_1_RACE_TEST_REPORT.md',
  'docs/P5_1_DEVELOPMENT_ACCEPTANCE_REPORT.md',
]) {
  if (exists(doc)) pass(`doc-${path.basename(doc)}`, doc);
  else warn(`doc-${path.basename(doc)}`, `${doc} missing`);
}

const passed = checks.filter((c) => c.status === 'passed').length;
const warnings = checks.filter((c) => c.status === 'warning').length;
const failed = checks.filter((c) => c.status === 'failed').length;
const status = failed === 0
  ? 'passed_with_real_environment_telemetry_verification_deferred'
  : 'incomplete';

const report = {
  phase: 'P5.1',
  status,
  summary: { passed, warnings, failed },
  realEnvironmentTelemetryVerification: 'deferred',
  externalAlertChannelVerification: 'deferred',
  productionSLOVerification: 'deferred',
  issues: checks.filter((c) => c.status === 'failed').map((c) => ({
    id: c.id,
    message: c.message,
    detail: c.detail,
  })),
};

fs.mkdirSync(path.join(root, 'docs'), { recursive: true });
fs.writeFileSync(path.join(root, 'docs/p5-1-observability-closure-report.json'), JSON.stringify(report, null, 2));
fs.writeFileSync(path.join(root, 'docs/P5_1_OBSERVABILITY_CLOSURE_REPORT.md'), `# P5.1 Observability Closure Report

Phase: P5.1
Status: ${status}
Real Environment Telemetry Verification: deferred
External Alert Channel Verification: deferred
Production SLO Verification: deferred

## Summary
- passed: ${passed}
- warnings: ${warnings}
- failed: ${failed}

## Checks
${checks.map((c) => `- [${c.status}] ${c.id}: ${c.message}`).join('\n')}

## Conclusion
${failed === 0 ? 'Phase P5.1 code-level closure passed. Do not mark Production Ready; real telemetry, external channels, and production SLO verification remain deferred.' : 'Phase P5.1 Incomplete. Phase P5 Closure Verification Incomplete. Do not mark Business Instrumentation Ready or Phase P5 Fully Closed until failed items are fixed and validated.'}
`);

console.log(JSON.stringify(report.summary));
process.exit(failed > 0 ? 1 : 0);
