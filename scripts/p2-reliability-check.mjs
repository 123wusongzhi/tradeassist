#!/usr/bin/env node
/**
 * Phase P2 reliability static scan.
 * Output: docs/P2_RELIABILITY_REPORT.md + docs/p2-reliability-report.json
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

function pass(id, message, detail) {
  checks.push({ id, status: 'passed', message, detail });
}
function fail(id, message, detail) {
  checks.push({ id, status: 'failed', message, detail });
}
function warn(id, message, detail) {
  checks.push({ id, status: 'warning', message, detail });
}

// Idempotency module
if (fs.existsSync(path.join(root, 'backend/internal/modules/idempotency/service.go'))) {
  pass('idempotency-module', 'idempotency module exists');
} else {
  fail('idempotency-module', 'idempotency module missing');
}

const idemModel = read('backend/internal/modules/idempotency/model.go');
if (idemModel.includes('ux_idempotency_scope_key')) pass('idempotency-unique', 'idempotency unique constraint');
else fail('idempotency-unique', 'idempotency unique index missing');

// P1 storage fail-fast
const validate = read('backend/internal/config/validate.go');
if (validate.includes('ErrCodeStorageProviderInvalid') && validate.includes('validateStorageProvider')) {
  pass('p1-storage-failfast', 'P1 local storage fail-fast in Validate()');
} else {
  fail('p1-storage-failfast', 'storage fail-fast missing');
}

// Task lease fields
for (const f of ['collect/model.go', 'ordersync/model.go', 'inventory/model.go']) {
  const t = read('backend/internal/modules/' + f);
  if (t.includes('LockedBy') && t.includes('LockedUntil')) pass('task-lease-' + f, f + ' has lease fields');
  else fail('task-lease-' + f, f + ' missing lease fields');
}

// dead_letter
if (read('backend/internal/modules/collect/model.go').includes('dead_letter')) {
  pass('dead-letter-status', 'dead_letter status exists');
} else {
  fail('dead-letter-status', 'dead_letter status missing');
}

// retry classification
if (read('backend/internal/pkg/taskretry/classify.go').includes('rate_limited')) {
  pass('retry-classify', 'retryable error classification');
} else {
  fail('retry-classify', 'retry classification missing');
}

// Retry-After
if (read('backend/internal/pkg/taskretry/classify.go').includes('ParseRetryAfter')) {
  pass('retry-after', 'Retry-After parsing');
} else {
  fail('retry-after', 'Retry-After missing');
}

// Circuit breaker
if (read('backend/internal/pkg/httpclient/client.go').includes('CircuitBreaker')) {
  pass('circuit-breaker', 'circuit breaker implementation');
} else {
  fail('circuit-breaker', 'circuit breaker missing');
}

// CORS
const cors = read('backend/internal/middleware/cors.go');
if (cors.includes('CORS') && cors.includes('wildcard')) {
  pass('cors-middleware', 'CORS middleware with wildcard guard');
} else {
  fail('cors-middleware', 'CORS middleware missing');
}

// Migration lock
if (read('backend/internal/database/migration_lock.go').includes('advisory_lock')) {
  pass('migration-lock', 'PostgreSQL migration advisory lock');
} else {
  fail('migration-lock', 'migration lock missing');
}

// Provider HealthCheck
if (read('backend/internal/pkg/providerhealth/health.go').includes('HealthCheck')) {
  pass('provider-health', 'Provider HealthCheck registry');
} else {
  fail('provider-health', 'Provider HealthCheck missing');
}

// Customer clientMessageId
if (read('backend/internal/modules/customerchat/model.go').includes('ClientMessageID')) {
  pass('customer-client-msg-id', 'customer clientMessageId field');
} else {
  fail('customer-client-msg-id', 'clientMessageId missing');
}

// Inventory business event key
if (read('backend/internal/modules/inventory/model.go').includes('BusinessEventKey')) {
  pass('inventory-event-key', 'inventory business event key');
} else {
  fail('inventory-event-key', 'inventory business event key missing');
}

// Webhook
if (fs.existsSync(path.join(root, 'backend/internal/modules/webhook/service.go'))) {
  pass('webhook-idempotency', 'webhook idempotency base');
} else {
  fail('webhook-idempotency', 'webhook module missing');
}

// Docs
for (const doc of [
  'docs/IDEMPOTENCY_DESIGN.md',
  'docs/TASK_RELIABILITY_DESIGN.md',
  'docs/PROVIDER_RESILIENCE_DESIGN.md',
  'docs/CORS_PRODUCTION_GUIDE.md',
  'docs/MIGRATION_LOCK_DESIGN.md',
]) {
  if (fs.existsSync(path.join(root, doc))) pass('doc-' + doc, doc + ' exists');
  else fail('doc-' + doc, doc + ' missing');
}

// Sensitive log scan (basic)
const httpclient = read('backend/internal/pkg/httpclient/client.go');
if (httpclient.includes('RedactURL')) pass('log-redact', 'URL redaction in httpclient');
else warn('log-redact', 'httpclient redaction may be incomplete');

const failed = checks.filter((c) => c.status === 'failed').length;
const warnings = checks.filter((c) => c.status === 'warning').length;
const passed = checks.filter((c) => c.status === 'passed').length;
const overall = failed === 0 ? (warnings ? 'passed_with_warning' : 'passed') : 'failed';

const report = {
  generatedAt: new Date().toISOString(),
  phase: 'P2',
  overall,
  summary: { passed, warnings, failed, total: checks.length },
  checks,
};

const jsonPath = path.join(root, 'docs/p2-reliability-report.json');
const mdPath = path.join(root, 'docs/P2_RELIABILITY_REPORT.md');
fs.writeFileSync(jsonPath, JSON.stringify(report, null, 2));

const md = [
  '# P2 Reliability Scan Report',
  '',
  `Generated: ${report.generatedAt}`,
  '',
  `**Overall:** ${overall} (${passed} passed, ${warnings} warnings, ${failed} failed)`,
  '',
  '| ID | Status | Message |',
  '| --- | --- | --- |',
  ...checks.map((c) => `| ${c.id} | ${c.status} | ${c.message} |`),
  '',
].join('\n');
fs.writeFileSync(mdPath, md);

console.log(`P2 scan: ${overall} — ${jsonPath}`);
process.exit(failed > 0 ? 1 : 0);
