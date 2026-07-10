#!/usr/bin/env node
/**
 * Phase P2.1 domain idempotency + task lease static scan.
 * Output: docs/P2_1_DOMAIN_IDEMPOTENCY_REPORT.md + docs/p2-1-domain-idempotency-report.json
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

function pass(id, message, detail) {
  checks.push({ id, status: 'passed', message, detail: detail ?? null });
}
function fail(id, message, detail) {
  checks.push({ id, status: 'failed', message, detail: detail ?? null });
}
function warn(id, message, detail) {
  checks.push({ id, status: 'warning', message, detail: detail ?? null });
}

function requireFile(id, rel, label) {
  if (exists(rel)) pass(id, `${label} exists`, rel);
  else fail(id, `${label} missing`, rel);
}

function requireContent(id, rel, needles, label) {
  const text = read(rel);
  if (!text) {
    fail(id, `${label}: file missing`, rel);
    return;
  }
  const missing = needles.filter((n) => !text.includes(n));
  if (missing.length === 0) pass(id, label, rel);
  else fail(id, `${label}: missing ${missing.join(', ')}`, rel);
}

function requireAnyContent(id, rel, needles, label) {
  const text = read(rel);
  if (!text) {
    fail(id, `${label}: file missing`, rel);
    return;
  }
  if (needles.some((n) => text.includes(n))) pass(id, label, rel);
  else fail(id, `${label}: expected one of ${needles.join(' | ')}`, rel);
}

// --- Core idempotency module ---
requireFile('idem-module-service', 'backend/internal/modules/idempotency/service.go', 'idempotency.Service');
requireContent('idem-module-scopes', 'backend/internal/modules/idempotency/scope.go', [
  'ScopeOrderSync',
  'ScopeOrderImport',
  'ScopeInventory',
  'ScopeInventoryPush',
  'ScopePublish',
  'ScopeCustomerSend',
  'ScopeAIText',
  'ScopeAIImage',
  'ScopeWebhook',
], 'idempotency business scopes');
requireContent('idem-module-keys', 'backend/internal/modules/idempotency/keys.go', [
  'OrderSyncJob',
  'OrderImport',
  'InventoryDeduct',
  'InventoryPush',
  'PublishBatch',
  'PublishEnqueue',
  'CustomerSend',
  'AITextBatch',
  'AIImageBatch',
  'Webhook',
], 'idempotency key builders');
requireContent('idem-module-decision', 'backend/internal/modules/idempotency/decision.go', ['Classify', 'DecisionAcquired'], 'idempotency Classify');
requireContent('idem-module-execute', 'backend/internal/modules/idempotency/execute.go', ['Execute', 'Complete'], 'idempotency Execute helper');
requireContent('idem-model-unique', 'backend/internal/modules/idempotency/model.go', ['ux_idempotency_scope_key'], 'idempotency unique constraint');

// --- Router wiring ---
const router = read('backend/internal/api/router.go');
if (router.includes('idempotencySvc := &idempotency.Service')) {
  pass('router-idempotency-svc', 'router creates shared idempotency.Service');
} else {
  fail('router-idempotency-svc', 'router missing idempotencySvc');
}

const wiredServices = [
  ['router-wire-ordersync', 'orderSyncSvc := &ordersync.Service', 'Idempotency: idempotencySvc', 'ordersync'],
  ['router-wire-order', 'orderSvc := &order.Service', 'Idempotency: idempotencySvc', 'order import'],
  ['router-wire-inventory', 'inventorySvc := &inventory.Service', 'Idempotency: idempotencySvc', 'inventory'],
  ['router-wire-publish', 'productPublishSvc := &productpublish.Service', 'Idempotency: idempotencySvc', 'productpublish'],
  ['router-wire-customer', 'customerChatSvc := &customerchat.Service', 'Idempotency: idempotencySvc', 'customerchat'],
  ['router-wire-ai-text', 'aiProductTextSvc := &aiproducttext.Service', 'Idempotency: idempotencySvc', 'aiproducttext'],
  ['router-wire-ai-image', 'aiProductImageSvc := &aiproductimage.Service', 'Idempotency: idempotencySvc', 'aiproductimage'],
];
for (const [id, start, inject, label] of wiredServices) {
  const idx = router.indexOf(start);
  const slice = idx >= 0 ? router.slice(idx, idx + 800) : '';
  if (idx >= 0 && slice.includes(inject)) pass(id, `router wires idempotency to ${label}`);
  else fail(id, `router missing Idempotency injection for ${label}`);
}

if (router.includes('webhook')) {
  pass('router-wire-webhook', 'webhook handler wired in router');
} else {
  warn('router-wire-webhook', 'webhook HTTP route not wired in router (service-level idempotency only)');
}

// --- Domain path integrations ---
const domainPaths = [
  {
    id: 'path-order-sync',
    file: 'backend/internal/modules/ordersync/idempotency_create.go',
    scope: 'ScopeOrderSync',
    key: 'OrderSyncJob',
    acquire: 'Acquire',
    complete: 'Complete',
    label: 'order sync job create',
  },
  {
    id: 'path-order-import',
    file: 'backend/internal/modules/order/idempotency_import.go',
    scope: 'ScopeOrderImport',
    key: 'OrderImport',
    acquire: 'Acquire',
    complete: 'Complete',
    label: 'order import/upsert',
  },
  {
    id: 'path-inventory-deduct',
    file: 'backend/internal/modules/inventory/idempotency_deduct.go',
    scope: 'ScopeInventory',
    key: 'InventoryDeduct',
    acquire: 'Acquire',
    complete: 'Complete',
    label: 'inventory deduct',
  },
  {
    id: 'path-inventory-push',
    file: 'backend/internal/modules/inventory/idempotency_push.go',
    scope: 'ScopeInventoryPush',
    key: 'InventoryPush',
    acquire: 'Acquire',
    complete: 'Complete',
    label: 'inventory push',
  },
  {
    id: 'path-publish-batch',
    file: 'backend/internal/modules/productpublish/idempotency_batch.go',
    scope: 'ScopePublish',
    key: 'PublishBatch',
    acquire: 'Acquire',
    complete: 'Complete',
    label: 'publish batch / enqueue',
  },
  {
    id: 'path-customer-send',
    file: 'backend/internal/modules/customerchat/send_platform.go',
    scope: 'ScopeCustomerSend',
    key: 'CustomerSend',
    acquire: 'Acquire',
    complete: 'Complete',
    label: 'customer message send',
  },
  {
    id: 'path-ai-text-batch',
    file: 'backend/internal/modules/aiproducttext/service.go',
    scope: 'ScopeAIText',
    key: 'acquireTextBatch',
    acquire: 'Acquire',
    complete: 'Complete',
    label: 'AI text batch create',
  },
  {
    id: 'path-ai-image-batch',
    file: 'backend/internal/modules/aiproductimage/service.go',
    scope: 'ScopeAIImage',
    key: 'acquireImageBatch',
    acquire: 'Acquire',
    complete: 'Complete',
    label: 'AI image batch create',
  },
  {
    id: 'path-webhook',
    file: 'backend/internal/modules/webhook/service.go',
    scope: 'ScopeWebhook',
    key: 'Webhook',
    acquire: 'Acquire',
    complete: 'Complete',
    label: 'webhook ingest',
  },
];

for (const p of domainPaths) {
  requireFile(`${p.id}-file`, p.file, p.label);
  requireContent(`${p.id}-acquire`, p.file, [p.scope, p.acquire], `${p.label} Acquire via ${p.scope}`);
  requireAnyContent(`${p.id}-complete`, p.file, ['Complete', 'Fail'], `${p.label} Complete/Fail lifecycle`);
  if (p.key && !read(p.file).includes(p.key)) {
    fail(`${p.id}-key`, `${p.label}: key helper ${p.key} not referenced`, p.file);
  } else if (p.key) {
    pass(`${p.id}-key`, `${p.label} uses key pattern`);
  }
}

// publish-enqueue sub-path
requireContent('path-publish-enqueue', 'backend/internal/modules/productpublish/idempotency_batch.go', ['PublishEnqueue'], 'publish enqueue idempotency');

// --- Task lease package ---
requireFile('tasklease-pkg', 'backend/internal/pkg/tasklease/lease.go', 'tasklease package');
requireContent('tasklease-try-claim', 'backend/internal/pkg/tasklease/lease.go', ['TryClaim', 'execution_id', 'heartbeat_at', 'lock_version'], 'tasklease TryClaim fields');
requireContent('tasklease-renew', 'backend/internal/pkg/tasklease/lease.go', ['RenewHeartbeat', 'ValidateLease', 'StartRenewal'], 'tasklease renewal API');
requireContent('tasklease-takeover', 'backend/internal/pkg/tasklease/lease.go', ['TakeoverExpired'], 'tasklease stale takeover');

const leaseModules = [
  ['tasklease-ordersync', 'backend/internal/modules/ordersync/lease.go', 'order sync worker lease'],
  ['tasklease-inventory', 'backend/internal/modules/inventory/lease.go', 'inventory sync worker lease'],
  ['tasklease-publish', 'backend/internal/modules/productpublish/lease.go', 'product publish worker lease'],
];
for (const [id, rel, label] of leaseModules) {
  requireContent(id, rel, ['tasklease.TryClaim', 'StartRenewal'], label);
}

const leaseModels = [
  ['task-model-ordersync', 'backend/internal/modules/ordersync/model.go', 'order_sync_tasks model lease fields'],
  ['task-model-inventory', 'backend/internal/modules/inventory/model.go', 'inventory_sync_tasks model lease fields'],
  ['task-model-publish', 'backend/internal/modules/productpublish/model.go', 'product_publish_tasks model lease fields'],
];
for (const [id, rel, label] of leaseModels) {
  requireContent(id, rel, ['HeartbeatAt', 'ExecutionID', 'LockVersion', 'LockedUntil'], label);
}

requireContent('migrate-p21', 'backend/internal/database/migrate_p2_1.go', ['heartbeat_at', 'execution_id', 'migrateP21Reliability'], 'P2.1 migration');

// P2.1 columns on extended task tables (migration only; worker adoption may lag)
for (const table of ['collect_tasks', 'image_tasks', 'customer_message_sync_tasks']) {
  const mig = read('backend/internal/database/migrate_p2_1.go');
  if (mig.includes(table) && mig.includes('heartbeat_at')) {
    pass(`migrate-p21-${table}`, `P2.1 migration adds heartbeat_at to ${table}`);
  } else {
    warn(`migrate-p21-${table}`, `P2.1 migration may not cover ${table} heartbeat columns`);
  }
}

// inventory business_event_key unique index
if (read('backend/internal/database/migrate_p2_1.go').includes('ux_inventory_change_business_event_key')) {
  pass('inventory-event-key-index', 'inventory business_event_key partial unique index');
} else {
  fail('inventory-event-key-index', 'inventory business_event_key index missing in migrate_p2_1');
}

// --- Secondary / future paths (warn if not integrated) ---
const aiTextSvc = read('backend/internal/modules/aiproducttext/service.go');
if (aiTextSvc.includes('AITextApply') || aiTextSvc.includes('ai-text-apply')) {
  pass('path-ai-text-apply', 'AI text apply idempotency referenced');
} else {
  warn('path-ai-text-apply', 'AI text apply path not yet wired (batch create only; keys reserved in keys.go)');
}
const aiImageSvc = read('backend/internal/modules/aiproductimage/service.go');
if (aiImageSvc.includes('AIImageApply') || aiImageSvc.includes('ai-image-apply')) {
  pass('path-ai-image-apply', 'AI image apply idempotency referenced');
} else {
  warn('path-ai-image-apply', 'AI image apply path not yet wired (batch create only; keys reserved in keys.go)');
}

// --- Docs ---
const docs = [
  'docs/IDEMPOTENCY_DESIGN.md',
  'docs/P2_1_IDEMPOTENCY_ADOPTION_MATRIX.md',
  'docs/DOMAIN_IDEMPOTENCY_INTEGRATION.md',
  'docs/TASK_LEASE_AND_HEARTBEAT_DESIGN.md',
  'docs/STALE_WORKER_PROTECTION.md',
  'docs/CONCURRENT_WRITE_SAFETY.md',
];
for (const doc of docs) {
  if (exists(doc)) pass(`doc-${path.basename(doc)}`, `${doc} exists`);
  else fail(`doc-${path.basename(doc)}`, `${doc} missing`);
}

// --- Tests ---
if (exists('backend/internal/modules/idempotency/concurrency_test.go')) {
  pass('idem-concurrency-test', 'idempotency concurrency test exists');
} else {
  warn('idem-concurrency-test', 'idempotency concurrency test missing');
}
if (exists('backend/internal/pkg/tasklease/lease_test.go')) {
  pass('tasklease-test', 'tasklease unit test exists');
} else {
  warn('tasklease-test', 'tasklease unit test missing');
}

const failed = checks.filter((c) => c.status === 'failed').length;
const warnings = checks.filter((c) => c.status === 'warning').length;
const passed = checks.filter((c) => c.status === 'passed').length;
const overall = failed === 0 ? (warnings ? 'passed_with_warning' : 'passed') : 'failed';

const report = {
  generatedAt: new Date().toISOString(),
  phase: 'P2.1',
  overall,
  summary: { passed, warnings, failed, total: checks.length },
  domainPaths: domainPaths.map((p) => ({ id: p.id, label: p.label, file: p.file })),
  checks,
};

const jsonPath = path.join(root, 'docs/p2-1-domain-idempotency-report.json');
const mdPath = path.join(root, 'docs/P2_1_DOMAIN_IDEMPOTENCY_REPORT.md');
fs.writeFileSync(jsonPath, JSON.stringify(report, null, 2));

const md = [
  '# P2.1 Domain Idempotency Scan Report',
  '',
  `Generated: ${report.generatedAt}`,
  '',
  `**Overall:** ${overall} (${passed} passed, ${warnings} warnings, ${failed} failed)`,
  '',
  '> Phase P2.1 validates unified `idempotency.Service` on critical write paths and `tasklease` heartbeat/execution identity on async workers. This scan is static; it does not imply Production Ready or full acceptance green.',
  '',
  '## Domain paths',
  '',
  '| ID | Path | Source file |',
  '| --- | --- | --- |',
  ...domainPaths.map((p) => `| ${p.id} | ${p.label} | \`${p.file}\` |`),
  '',
  '## Checks',
  '',
  '| ID | Status | Message |',
  '| --- | --- | --- |',
  ...checks.map((c) => `| ${c.id} | ${c.status} | ${c.message} |`),
  '',
  '## Run',
  '',
  '```bash',
  'node scripts/p2-1-domain-idempotency-check.mjs',
  '```',
  '',
].join('\n');
fs.writeFileSync(mdPath, md);

console.log(`P2.1 scan: ${overall} — ${jsonPath}`);
process.exit(failed > 0 ? 1 : 0);
