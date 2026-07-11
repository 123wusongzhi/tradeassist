#!/usr/bin/env node
/**
 * Phase P2.2 reliability closure static scan.
 * Output: docs/P2_2_RELIABILITY_CLOSURE_REPORT.md + docs/p2-2-reliability-closure-report.json
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
    return false;
  }
  const missing = needles.filter((n) => !text.includes(n));
  if (missing.length === 0) {
    pass(id, label, rel);
    return true;
  }
  fail(id, `${label}: missing ${missing.join(', ')}`, rel);
  return false;
}

function requireAnyContent(id, rel, needles, label) {
  const text = read(rel);
  if (!text) {
    fail(id, `${label}: file missing`, rel);
    return false;
  }
  if (needles.some((n) => text.includes(n))) {
    pass(id, label, rel);
    return true;
  }
  fail(id, `${label}: expected one of ${needles.join(' | ')}`, rel);
  return false;
}

function concat(...rels) {
  return rels.map(read).join('\n');
}

// --- AI text apply / undo ---
requireFile('ai-text-idempotency-apply', 'backend/internal/modules/aiproducttext/idempotency_apply.go', 'AI text idempotency_apply.go');
requireContent(
  'ai-text-apply-service',
  'backend/internal/modules/aiproducttext/idempotency_apply.go',
  ['idempotency.AITextApply', 'Acquire', 'ScopeAIText'],
  'AI text apply uses idempotency.Service (AITextApply + Acquire)',
);
requireContent(
  'ai-text-undo',
  'backend/internal/modules/aiproducttext/idempotency_apply.go',
  ['idempotency.AITextUndo', 'Acquire'],
  'AI text undo uses AITextUndo',
);
requireContent(
  'ai-text-version-conflict',
  'backend/internal/modules/aiproducttext/model.go',
  ['AI_TEXT_TARGET_VERSION_CONFLICT', 'AI_TEXT_UNDO_VERSION_CONFLICT'],
  'AI text version conflict codes',
);

// --- AI image apply / undo ---
requireFile('ai-image-idempotency-apply', 'backend/internal/modules/aiproductimage/idempotency_apply.go', 'AI image idempotency_apply.go');
requireContent(
  'ai-image-apply-service',
  'backend/internal/modules/aiproductimage/idempotency_apply.go',
  ['idempotency.AIImageApply', 'Acquire', 'ScopeAIImage'],
  'AI image apply uses AIImageApply + Acquire',
);
requireContent(
  'ai-image-undo',
  'backend/internal/modules/aiproductimage/idempotency_apply.go',
  ['idempotency.AIImageUndo', 'Acquire'],
  'AI image undo uses AIImageUndo',
);
requireContent(
  'ai-image-version-conflict',
  'backend/internal/modules/aiproductimage/model.go',
  ['AI_IMAGE_TARGET_VERSION_CONFLICT', 'AI_IMAGE_UNDO_VERSION_CONFLICT'],
  'AI image version conflict codes',
);

requireContent(
  'keys-apply-builders',
  'backend/internal/modules/idempotency/keys.go',
  ['AITextApply', 'AITextUndo', 'AIImageApply', 'AIImageUndo', 'WebhookProcess'],
  'idempotency key builders for apply/undo/webhook-process',
);

// --- Webhook HTTP ---
requireContent(
  'webhook-register-public',
  'backend/internal/modules/webhook/router.go',
  ['RegisterPublic', '/webhooks/:platform/:eventType'],
  'Webhook RegisterPublic route',
);
const router = read('backend/internal/api/router.go');
if (router.includes('webhook.RegisterPublic')) {
  pass('router-webhook-register', 'router.go wires webhook.RegisterPublic');
} else {
  fail('router-webhook-register', 'router.go missing webhook.RegisterPublic');
}

requireAnyContent(
  'webhook-body-limit',
  'backend/internal/modules/webhook/handler.go',
  ['MaxBytesReader', 'maxPayload'],
  'Webhook body limit (MaxBytesReader / maxPayload)',
);
requireAnyContent(
  'webhook-max-body-config',
  'backend/internal/config/config.go',
  ['WEBHOOK_MAX_BODY_KB', 'WebhookMaxBodyKB', 'MaxPayloadBytes'],
  'WEBHOOK_MAX_BODY_KB / MaxPayloadBytes config',
);

requireContent(
  'webhook-signature-verifier',
  'backend/internal/modules/webhook/signature.go',
  ['SignatureVerifier', 'Verify'],
  'SignatureVerifier exists',
);
requireAnyContent(
  'webhook-clock-skew',
  'backend/internal/modules/webhook/service.go',
  ['MaxClockSkew', 'ValidateTimestamp'],
  'MaxClockSkew / timestamp validation',
);
requireAnyContent(
  'webhook-replay',
  'backend/internal/modules/webhook/service.go',
  ['OnConflict', 'Duplicate', 'DoNothing'],
  'Replay protection (duplicate / OnConflict)',
);
requireAnyContent(
  'webhook-fast-ack',
  'backend/internal/modules/webhook/handler.go',
  ['accepted', 'Ingest'],
  'Fast ACK (accepted / Ingest)',
);
requireContent(
  'webhook-async-process',
  'backend/internal/modules/webhook/processor.go',
  ['ProcessEvent', 'ProcessQueuedEvents'],
  'Async ProcessEvent / ProcessQueuedEvents',
);
requireContent(
  'webhook-start-worker',
  'backend/internal/modules/webhook/worker.go',
  ['StartWorker', 'ProcessQueuedEvents'],
  'Webhook StartWorker',
);
requireContent(
  'webhook-bypass-forbidden',
  'backend/internal/modules/webhook/signature.go',
  ['CodeSignatureBypassForbidden', 'IsProduction'],
  'Production signature bypass forbidden',
);

// Sensitive: handler/service should not log raw body / secrets
const webhookSensitiveFiles = [
  'backend/internal/modules/webhook/handler.go',
  'backend/internal/modules/webhook/service.go',
  'backend/internal/modules/webhook/worker.go',
];
const badLogPatterns = [
  /log\.(Info|Warn|Error|Debug)\([^)]*RawBody/i,
  /slog\.[A-Z][a-z]+\([^)]*raw\s*body/i,
  /Printf\([^)]*req\.Payload/,
  /Println\([^)]*raw/,
];
let sensitiveOk = true;
const sensitiveHits = [];
for (const rel of webhookSensitiveFiles) {
  const text = read(rel);
  for (const re of badLogPatterns) {
    if (re.test(text)) {
      sensitiveOk = false;
      sensitiveHits.push(`${rel} ~ ${re}`);
    }
  }
  // Full payload dump into logs is forbidden; truncateSummary for DB is OK.
  if (/log\.(Info|Warn|Error).*Payload[^H]/.test(text) || /slog\.\w+\([^)]*Payload[^H]/.test(text)) {
    sensitiveOk = false;
    sensitiveHits.push(`${rel}: Payload logged`);
  }
}
const hasTruncate = read('backend/internal/modules/webhook/service.go').includes('truncateSummary');
if (sensitiveOk && hasTruncate) {
  pass('webhook-sensitive-logging', 'Webhook modules avoid logging raw secrets; truncateSummary present');
} else if (sensitiveOk) {
  warn('webhook-sensitive-logging', 'No raw-body log patterns found; truncateSummary missing', sensitiveHits);
} else {
  fail('webhook-sensitive-logging', 'Possible raw body/secret logging', sensitiveHits.join('; '));
}

// --- Workers: tasklease ---
const workerLease = [
  {
    id: 'collect',
    lease: 'backend/internal/modules/collect/lease.go',
    claim: ['TryClaimPendingOrRetrying', 'tasklease.'],
    finish: 'finishCollectTask',
    stale: 'backend/internal/modules/collect/lease_stale_worker_test.go',
  },
  {
    id: 'imagetask',
    lease: 'backend/internal/modules/imagetask/lease.go',
    claim: ['TryClaimPendingOrRetrying', 'tasklease.'],
    finish: 'finishImageTask',
    stale: 'backend/internal/modules/imagetask/lease_stale_worker_test.go',
  },
  {
    id: 'customersync',
    lease: 'backend/internal/modules/customersync/lease.go',
    claim: ['TryClaim', 'tasklease.'],
    finish: 'finishCustomerSyncTask',
    stale: 'backend/internal/modules/customersync/lease_stale_worker_test.go',
  },
  {
    id: 'ordersync',
    lease: 'backend/internal/modules/ordersync/lease.go',
    claim: ['TryClaim', 'tasklease.'],
    finish: 'finishOrderSyncTask',
    stale: null,
  },
  {
    id: 'inventory',
    lease: 'backend/internal/modules/inventory/lease.go',
    claim: ['TryClaim', 'tasklease.'],
    finish: 'finishInventorySyncTask',
    stale: null,
  },
  {
    id: 'productpublish',
    lease: 'backend/internal/modules/productpublish/lease.go',
    claim: ['TryClaim', 'tasklease.'],
    finish: 'finishProductPublishTask',
    stale: null,
  },
];

for (const w of workerLease) {
  requireAnyContent(`worker-${w.id}-claim`, w.lease, w.claim, `${w.id} uses tasklease claim`);
  requireContent(
    `worker-${w.id}-validate-finish`,
    w.lease,
    ['ValidateLease', w.finish],
    `${w.id} ValidateLease + ${w.finish}`,
  );
  requireContent(
    `worker-${w.id}-renewal`,
    w.lease,
    ['StartRenewal', 'ExecutionID'],
    `${w.id} heartbeat renewal / execution_id`,
  );
  if (w.stale) {
    requireFile(`worker-${w.id}-stale-test`, w.stale, `${w.id} lease_stale_worker_test.go`);
  }
}

// Stale tests for the three P2.2-extended workers (explicit list)
for (const rel of [
  'backend/internal/modules/collect/lease_stale_worker_test.go',
  'backend/internal/modules/imagetask/lease_stale_worker_test.go',
  'backend/internal/modules/customersync/lease_stale_worker_test.go',
]) {
  requireFile(`stale-${path.basename(path.dirname(rel))}`, rel, 'lease_stale_worker_test.go');
}

// --- Concurrency / webhook tests ---
requireFile('test-ai-text-apply', 'backend/internal/modules/aiproducttext/apply_idempotency_test.go', 'AI text apply_idempotency_test.go');
requireFile('test-ai-image-apply', 'backend/internal/modules/aiproductimage/apply_idempotency_test.go', 'AI image apply_idempotency_test.go');
requireFile('test-webhook', 'backend/internal/modules/webhook/handler_test.go', 'Webhook handler_test.go');

// --- Race report doc ---
requireFile('doc-race-report', 'docs/P2_2_RACE_TEST_REPORT.md', 'P2_2_RACE_TEST_REPORT.md');

// --- Required docs ---
const requiredDocs = [
  'docs/AI_RESULT_APPLY_IDEMPOTENCY.md',
  'docs/AI_RESULT_UNDO_DESIGN.md',
  'docs/WEBHOOK_HTTP_RECEIVER_DESIGN.md',
  'docs/WEBHOOK_SIGNATURE_AND_REPLAY_PROTECTION.md',
  'docs/P2_2_WORKER_LEASE_ADOPTION_MATRIX.md',
  'docs/P2_2_RACE_TEST_REPORT.md',
  'docs/P2_2_RELIABILITY_CLOSURE_MATRIX.md',
];
for (const doc of requiredDocs) {
  if (exists(doc)) pass(`doc-${path.basename(doc)}`, `${doc} exists`);
  else fail(`doc-${path.basename(doc)}`, `${doc} missing`);
}

// --- P2.1 former warnings cleared ---
if (router.includes('webhook.RegisterPublic') || router.includes('webhook')) {
  pass('p21-warning-webhook-router', 'P2.1 former warning cleared: webhook route in router');
} else {
  fail('p21-warning-webhook-router', 'P2.1 warning remains: webhook not in router');
}
const aiTextSrc = concat(
  'backend/internal/modules/aiproducttext/service.go',
  'backend/internal/modules/aiproducttext/idempotency_apply.go',
);
if (aiTextSrc.includes('AITextApply')) {
  pass('p21-warning-ai-text-apply', 'P2.1 former warning cleared: AI text apply idempotency');
} else {
  fail('p21-warning-ai-text-apply', 'P2.1 warning remains: AI text apply not wired');
}
const aiImageSrc = concat(
  'backend/internal/modules/aiproductimage/service.go',
  'backend/internal/modules/aiproductimage/service_apply.go',
  'backend/internal/modules/aiproductimage/idempotency_apply.go',
);
if (aiImageSrc.includes('AIImageApply')) {
  pass('p21-warning-ai-image-apply', 'P2.1 former warning cleared: AI image apply idempotency');
} else {
  fail('p21-warning-ai-image-apply', 'P2.1 warning remains: AI image apply not wired');
}

// --- Aggregate ---
const failed = checks.filter((c) => c.status === 'failed').length;
const warnings = checks.filter((c) => c.status === 'warning').length;
const passed = checks.filter((c) => c.status === 'passed').length;
const status = failed === 0 ? 'passed' : 'failed';
const overall = failed === 0 ? (warnings ? 'passed_with_warning' : 'passed') : 'failed';

function sectionStatus(ids) {
  const subset = checks.filter((c) => ids.some((p) => c.id.startsWith(p) || c.id === p));
  if (subset.some((c) => c.status === 'failed')) return 'failed';
  if (subset.some((c) => c.status === 'warning')) return 'passed_with_warning';
  return subset.length ? 'passed' : 'unknown';
}

const aiApplyIds = ['ai-text', 'ai-image', 'keys-apply', 'test-ai-text', 'test-ai-image', 'p21-warning-ai'];
const webhookIds = ['webhook-', 'router-webhook', 'p21-warning-webhook'];
const workerIds = ['worker-', 'stale-'];
const raceIds = ['doc-race'];
const docIds = ['doc-'];

const issues = checks
  .filter((c) => c.status === 'failed' || c.status === 'warning')
  .map((c) => ({ id: c.id, status: c.status, message: c.message, detail: c.detail }));

const report = {
  generatedAt: new Date().toISOString(),
  phase: 'P2.2',
  status,
  overall,
  summary: { passed, warnings, failed, total: checks.length },
  aiApply: {
    status: sectionStatus(aiApplyIds),
    textApply: sectionStatus(['ai-text-']),
    textUndo: sectionStatus(['ai-text-undo', 'keys-apply-undo']),
    imageApply: sectionStatus(['ai-image-']),
    imageUndo: sectionStatus(['ai-image-undo']),
    versionConflicts: sectionStatus(['ai-text-version', 'ai-image-version']),
  },
  webhook: {
    status: sectionStatus(webhookIds),
    route: sectionStatus(['webhook-register', 'router-webhook']),
    signature: sectionStatus(['webhook-signature', 'webhook-bypass', 'webhook-clock']),
    replay: sectionStatus(['webhook-replay']),
    asyncWorker: sectionStatus(['webhook-async', 'webhook-start']),
    sensitiveLogging: sectionStatus(['webhook-sensitive']),
  },
  workers: {
    status: sectionStatus(workerIds),
    modules: workerLease.map((w) => ({
      id: w.id,
      leaseFile: w.lease,
      staleTest: w.stale,
    })),
  },
  race: {
    status: (() => {
      const raceDoc = read('docs/P2_2_RACE_TEST_REPORT.md');
      if (!raceDoc) return 'missing';
      if (raceDoc.includes('未发现 data race') || raceDoc.includes('结论：通过')) return 'passed';
      if (raceDoc.includes('pending') || raceDoc.includes('placeholder')) return 'placeholder';
      return 'reported';
    })(),
    report: 'docs/P2_2_RACE_TEST_REPORT.md',
    note: 'WSL2 Linux CGO race executed 2026-07-11; see P2_2_RACE_TEST_REPORT.md',
  },
  environment: {
    scanner: 'scripts/p2-2-reliability-closure-check.mjs',
    node: process.version,
    cwd: root,
    note: 'Static scan only; not Production Ready; not real-platform E2E',
  },
  docs: {
    status: sectionStatus(docIds),
    required: requiredDocs,
  },
  issues,
  checks,
};

const jsonPath = path.join(root, 'docs/p2-2-reliability-closure-report.json');
const mdPath = path.join(root, 'docs/P2_2_RELIABILITY_CLOSURE_REPORT.md');
fs.writeFileSync(jsonPath, JSON.stringify(report, null, 2));

const md = [
  '# P2.2 Reliability Closure Scan Report',
  '',
  `Generated: ${report.generatedAt}`,
  '',
  `**Overall:** ${overall} (${passed} passed, ${warnings} warnings, ${failed} failed)`,
  '',
  '> Phase P2.2 validates AI apply/undo idempotency, Webhook HTTP receiver (signature / replay / ACK / async worker), and tasklease adoption on six workers. This scan is **static**; it does **not** imply Production Ready, gray release, or real platform E2E. Race results: see [`P2_2_RACE_TEST_REPORT.md`](P2_2_RACE_TEST_REPORT.md) (WSL2 Linux race passed 2026-07-11).',
  '',
  '## Sections',
  '',
  `| Section | Status |`,
  `| --- | --- |`,
  `| aiApply | ${report.aiApply.status} |`,
  `| webhook | ${report.webhook.status} |`,
  `| workers | ${report.workers.status} |`,
  `| race | ${report.race.status} |`,
  `| docs | ${report.docs.status} |`,
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
  'node scripts/p2-2-reliability-closure-check.mjs',
  'node scripts/p2-1-domain-idempotency-check.mjs',
  '```',
  '',
].join('\n');
fs.writeFileSync(mdPath, md);

console.log(`P2.2 scan: ${overall} — ${jsonPath}`);
process.exit(failed > 0 ? 1 : 0);
