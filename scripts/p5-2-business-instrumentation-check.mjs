#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const checks = [];

function read(rel) {
  const p = path.join(root, rel);
  return fs.existsSync(p) ? fs.readFileSync(p, 'utf8') : '';
}

function exists(rel) {
  return fs.existsSync(path.join(root, rel));
}

function add(status, id, message, detail = '') {
  checks.push({ status, id, message, detail });
}

function requireText(id, rel, needles, message) {
  const text = read(rel);
  const missing = needles.filter((n) => !text.includes(n));
  if (missing.length === 0) add('passed', id, message || rel);
  else add('failed', id, `${message || rel} missing ${missing.join(', ')}`, rel);
}

const modules = [
  ['httpclient', 'backend/internal/pkg/httpclient/client.go', 'backend/internal/pkg/httpclient/observability_test.go', ['ObserveProvider', 'provider_requests_total']],
  ['webhook', 'backend/internal/modules/webhook/observability.go', 'backend/internal/modules/webhook/observability_test.go', ['ObserveWebhook', 'webhook_events_processed_total']],
  ['ordersync', 'backend/internal/modules/ordersync/observability.go', 'backend/internal/modules/ordersync/observability_test.go', ['ObserveOrder', 'order_sync_runs_total']],
  ['inventory', 'backend/internal/modules/inventory/observability.go', 'backend/internal/modules/inventory/observability_test.go', ['ObserveInventory', 'inventory_adjustments_total']],
  ['ai-text', 'backend/internal/modules/aiproducttext/observability.go', 'backend/internal/modules/aiproducttext/observability_test.go', ['ObserveAIText', 'ai_text_requests_total']],
  ['ai-image', 'backend/internal/modules/aiproductimage/observability.go', 'backend/internal/modules/aiproductimage/observability_test.go', ['ObserveAIImage', 'ai_image_provider_timeouts_total']],
  ['file-scan', 'backend/internal/modules/files/observability.go', 'backend/internal/modules/files/observability_test.go', ['ObserveFileScan', 'file_scan_tasks_total']],
  ['security', 'backend/internal/modules/securitymod/observability.go', 'backend/internal/modules/securitymod/observability_test.go', ['ObserveSecurity', 'security_events_total']],
  ['auth', 'backend/internal/modules/auth/observability.go', 'backend/internal/modules/auth/observability_test.go', ['ObserveAuth', 'auth_login_attempts_total']],
];

for (const [id, impl, test, needles] of modules) {
  requireText(`business-${id}`, impl, [needles[0]], `${id} real instrumentation facade`);
  requireText(`business-${id}-test`, test, [needles[1], 'SnapshotValues'], `${id} non-zero metrics test`);
}

for (const [metric, rel] of [
  ['provider_requests_total', 'backend/internal/pkg/metrics/catalog.go'],
  ['webhook_events_processed_total', 'backend/internal/pkg/metrics/catalog.go'],
  ['order_sync_orders_created_total', 'backend/internal/pkg/metrics/catalog.go'],
  ['inventory_unknown_results_total', 'backend/internal/pkg/metrics/catalog.go'],
  ['ai_text_provider_timeouts_total', 'backend/internal/pkg/metrics/catalog.go'],
  ['ai_image_provider_timeouts_total', 'backend/internal/pkg/metrics/catalog.go'],
  ['file_scan_results_total', 'backend/internal/pkg/metrics/catalog.go'],
  ['audit_chain_mismatch_total', 'backend/internal/pkg/metrics/catalog.go'],
  ['auth_refresh_reuse_detected_total', 'backend/internal/pkg/metrics/catalog.go'],
]) {
  requireText(`catalog-${metric}`, rel, [metric], metric);
}

requireText('router-catalog-wiring', 'backend/internal/api/router.go', ['metricCatalog', 'Metrics:     metricCatalog'], 'shared catalog wired into services');
requireText('alert-real-snapshot-source', 'backend/cmd/server/main.go', ['SnapshotValues', 'StartEvaluatorWorker'], 'alert evaluator reads registry snapshot');
requireText('ai-image-alert-rule', 'backend/internal/modules/alerting/rules.go', ['ai_image_provider_timeouts_total'], 'AI image timeout alert rule');
requireText('auth-refresh-reuse-alert-rule', 'backend/internal/modules/alerting/rules.go', ['auth_refresh_reuse_detected_total'], 'Auth refresh reuse alert rule');
requireText('otlp-report', 'docs/P5_2_SPAN_EXPORT_PROTOCOL_REPORT.md', ['protocol', 'Custom HTTP Span Exporter Ready'], 'span export protocol boundary report');
requireText('dashboard-validation', 'docs/P5_2_DASHBOARD_METRIC_VALIDATION.md', ['application-overview.json', 'ai-providers.json', 'security.json'], 'dashboard metric validation report');
requireText('race-report', 'docs/P5_2_RACE_TEST_REPORT.md', ['Linux Race Verification'], 'race verification report');
requireText('final-report', 'docs/P5_2_FINAL_OBSERVABILITY_REPORT.md', ['Phase P5.2', 'Real Environment Telemetry Verification: deferred'], 'final observability report');

const forbiddenLabels = ['tenant_id', 'shop_id', 'user_id', 'task_id', 'order_id', 'product_id', 'sku_id', 'event_id', 'object_key', 'raw_url'];
const catalog = read('backend/internal/pkg/metrics/catalog.go');
for (const key of forbiddenLabels) {
  if (catalog.includes(`"${key}"`)) add('failed', `forbidden-label-${key}`, `catalog uses forbidden high-cardinality label ${key}`);
  else add('passed', `forbidden-label-${key}`, `${key} absent from metric labels`);
}

const passed = checks.filter((c) => c.status === 'passed').length;
const failed = checks.filter((c) => c.status === 'failed').length;
const report = {
  phase: 'P5.2',
  status: failed === 0 ? 'passed_with_real_environment_telemetry_verification_deferred' : 'incomplete',
  summary: { passed, warnings: 0, failed },
  realEnvironmentTelemetryVerification: 'deferred',
  unclosedGates: [
    'standard_otlp_http_exporter_pending_p5_v_final_verification',
    'linux_race_verification_deferred_on_windows',
    'demo_auto_acceptance_deferred',
    'real_telemetry_backend_and_external_alert_channel_verification_deferred',
  ],
  issues: checks.filter((c) => c.status === 'failed'),
};

fs.writeFileSync(path.join(root, 'docs/p5-2-final-observability-report.json'), JSON.stringify(report, null, 2));
console.log(JSON.stringify(report.summary));
process.exit(failed > 0 ? 1 : 0);
