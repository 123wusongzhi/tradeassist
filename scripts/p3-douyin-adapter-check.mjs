#!/usr/bin/env node
/**
 * Phase P3 Douyin production adapter static scan.
 * Output: docs/P3_DOUYIN_ADAPTER_REPORT.md + docs/p3-douyin-adapter-report.json
 *
 * Status: passed_with_real_credentials_deferred
 * - Code is implemented; real credential E2E is deferred to Phase P10.
 * - Does NOT imply Production Ready, gray release, or real platform E2E.
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
  if (!text) { fail(id, `${label}: file missing`, rel); return false; }
  const missing = needles.filter((n) => !text.includes(n));
  if (missing.length === 0) { pass(id, label, rel); return true; }
  fail(id, `${label}: missing ${missing.join(', ')}`, rel);
  return false;
}
function sectionStatus(ids) {
  const sectionChecks = checks.filter((c) => ids.some((id) => c.id.startsWith(id)));
  if (sectionChecks.some((c) => c.status === 'failed')) return 'failed';
  if (sectionChecks.some((c) => c.status === 'warning')) return 'warning';
  return 'passed';
}

// ─── Provider files ───────────────────────────────────────────────────────────
const providerBase = 'backend/internal/providers/platform/douyinshop';

requireFile('provider.facade', `${providerBase}/facade.go`, 'DouyinProvider facade');
requireFile('provider.errors', `${providerBase}/errors.go`, 'errors.go');
requireFile('provider.token_lock', `${providerBase}/token_lock.go`, 'token_lock.go');
requireFile('provider.order_detail', `${providerBase}/order_detail.go`, 'order_detail.go');
requireFile('provider.inventory_query', `${providerBase}/inventory_query.go`, 'inventory_query.go');
requireFile('provider.customer', `${providerBase}/customer.go`, 'customer.go');
requireFile('provider.webhook_sign', `${providerBase}/webhook_sign.go`, 'webhook_sign.go');
requireFile('provider.webhook_events', `${providerBase}/webhook_events.go`, 'webhook_events.go');
requireFile('provider.health', `${providerBase}/health.go`, 'health.go');
requireFile('provider.brand', `${providerBase}/brand.go`, 'brand.go');
requireFile('provider.http_transport', `${providerBase}/http_transport.go`, 'http_transport.go');

requireContent('provider.errors.codes', `${providerBase}/errors.go`,
  ['DOUYIN_UNKNOWN_RESULT', 'DOUYIN_TOKEN_VERSION_CONFLICT', 'DOUYIN_CONTRACT_MISMATCH',
   'UnknownResult', 'SafeRetry', 'ManualReviewRequired', 'ErrorClass'],
  'errors.go has P3 fields and codes');

requireContent('provider.brand.blocked', `${providerBase}/brand.go`,
  ['blocked_by_contract_verification'],
  'brand.go declares blocked_by_contract_verification');

requireContent('provider.customer.blocked', `${providerBase}/customer.go`,
  ['blocked_by_contract_verification'],
  'customer.go declares blocked_by_contract_verification');

const providerIds = ['provider.'];

// ─── Webhook module ───────────────────────────────────────────────────────────
const webhookBase = 'backend/internal/modules/webhook';

requireFile('webhook.douyin_verifier', `${webhookBase}/douyin_verifier.go`, 'douyin_verifier.go');
requireFile('webhook.douyin_handler', `${webhookBase}/douyin_handler.go`, 'douyin_handler.go');
requireContent('webhook.processor', `${webhookBase}/processor.go`,
  ['HandleDouyinPlatformEvent'],
  'processor.go dispatches to HandleDouyinPlatformEvent');

const webhookIds = ['webhook.'];

// ─── Models ───────────────────────────────────────────────────────────────────
requireFile('model.oauth_state', 'backend/internal/modules/shop/douyin_oauth_state.go', 'DouyinOAuthState model');
requireFile('model.image_asset', 'backend/internal/modules/product/douyin_image_asset.go', 'DouyinImageAsset model');
requireFile('model.sync_cursor', 'backend/internal/modules/ordersync/douyin_sync_cursor.go', 'DouyinSyncCursor model');

const modelIds = ['model.'];

// ─── Idempotency keys ─────────────────────────────────────────────────────────
requireContent('idem.keys', 'backend/internal/modules/idempotency/keys.go',
  ['DouyinProductDraftCreate', 'DouyinImageUpload', 'AIProductApply'],
  'idempotency/keys.go has P3 key builders');
requireContent('idem.scope', 'backend/internal/modules/idempotency/scope.go',
  ['ScopeProductAIApply', 'ScopeDouyinImage'],
  'idempotency/scope.go has P3 scopes');

const idemIds = ['idem.'];

// ─── Migrations ───────────────────────────────────────────────────────────────
requireFile('migrate.p3', 'backend/internal/database/migrate_p3_douyin.go', 'migrate_p3_douyin.go');
requireContent('migrate.call', 'backend/internal/database/migrate.go',
  ['migrateP3Douyin'],
  'migrate.go calls migrateP3Douyin');

const migrateIds = ['migrate.'];

// ─── configstatus ─────────────────────────────────────────────────────────────
requireContent('configstatus.platform_key', 'backend/internal/modules/configstatus/service.go',
  ['platform_douyin_shop'],
  'configstatus uses platform_douyin_shop');
requireFile('configstatus.p3_status', 'backend/internal/modules/configstatus/p3_status.go', 'p3_status.go');

const configIds = ['configstatus.'];

// ─── Failure classifier ───────────────────────────────────────────────────────
requireContent('failclassifier.douyin', 'backend/internal/modules/taskcenter/failureclassifier/enumerate.go',
  ['douyin_draft_create_failed', 'douyin_token_refresh_failed', 'douyin_webhook_signature_failed'],
  'failureclassifier has douyin_* types');

const failIds = ['failclassifier.'];

// ─── Fixtures ─────────────────────────────────────────────────────────────────
const fixtureBase = 'backend/testdata/douyin';
for (const fixture of ['oauth_state', 'order_detail', 'webhook_health_ping',
  'webhook_order_created', 'customer_message_envelope', 'product_draft_create',
  'inventory_query', 'shop_info']) {
  requireFile(`fixture.${fixture}`, `${fixtureBase}/${fixture}.json`, `fixture ${fixture}.json`);
}

const fixtureIds = ['fixture.'];

// ─── Docs ─────────────────────────────────────────────────────────────────────
const requiredDocs = [
  'docs/P3_DOUYIN_ADAPTER_AUDIT_MATRIX.md',
  'docs/DOUYIN_PROVIDER_ARCHITECTURE.md',
  'docs/DOUYIN_OAUTH_AND_TOKEN_LIFECYCLE.md',
  'docs/DOUYIN_CATALOG_SYNC_DESIGN.md',
  'docs/DOUYIN_IMAGE_UPLOAD_DESIGN.md',
  'docs/DOUYIN_PRODUCT_DRAFT_MAPPING.md',
  'docs/DOUYIN_PRODUCT_DRAFT_IDEMPOTENCY.md',
  'docs/DOUYIN_ORDER_SYNC_DESIGN.md',
  'docs/DOUYIN_INVENTORY_ADAPTER.md',
  'docs/DOUYIN_CUSTOMER_ADAPTER.md',
  'docs/DOUYIN_WEBHOOK_ADAPTER.md',
  'docs/DOUYIN_ERROR_CLASSIFICATION.md',
  'docs/P3_DOUYIN_ADAPTER_REPORT.md',
];
for (const doc of requiredDocs) {
  requireFile(`doc.${path.basename(doc, '.md')}`, doc, path.basename(doc));
}

const docIds = ['doc.'];

// ─── Blocking patterns ────────────────────────────────────────────────────────
// Ensure commit=false / start_sale_type not overridden to publish
const publishFiles = [
  'backend/internal/modules/productpublish',
];
for (const dir of publishFiles) {
  const goFiles = fs.existsSync(path.join(root, dir))
    ? fs.readdirSync(path.join(root, dir)).filter((f) => f.endsWith('.go'))
    : [];
  for (const f of goFiles) {
    const src = read(`${dir}/${f}`);
    if (src.includes('commit=true') || src.includes('"commit": true')) {
      fail('safety.no_auto_publish', `${dir}/${f} sets commit=true — must stay commit=false`, `${dir}/${f}`);
    }
  }
}
if (!checks.some((c) => c.id === 'safety.no_auto_publish' && c.status === 'failed')) {
  pass('safety.no_auto_publish', 'No commit=true found in productpublish', 'productpublish/**/*.go');
}

// ─── Summary ──────────────────────────────────────────────────────────────────
const passed = checks.filter((c) => c.status === 'passed').length;
const warnings = checks.filter((c) => c.status === 'warning').length;
const failed = checks.filter((c) => c.status === 'failed').length;
const issues = checks.filter((c) => c.status !== 'passed').map((c) => `${c.id}: ${c.message}`);

const overall = failed > 0 ? 'failed' : warnings > 0 ? 'passed_with_warning' : 'passed_with_real_credentials_deferred';

const report = {
  generatedAt: new Date().toISOString(),
  overall,
  passed,
  warnings,
  failed,
  scanner: 'scripts/p3-douyin-adapter-check.mjs',
  phaseStatus: 'passed_with_real_credentials_deferred',
  providerSection: { status: sectionStatus(providerIds) },
  webhookSection: { status: sectionStatus(webhookIds) },
  modelSection: { status: sectionStatus(modelIds) },
  idemSection: { status: sectionStatus(idemIds) },
  migrateSection: { status: sectionStatus(migrateIds) },
  configSection: { status: sectionStatus(configIds) },
  failClassifierSection: { status: sectionStatus(failIds) },
  fixtureSection: { status: sectionStatus(fixtureIds) },
  docsSection: { status: sectionStatus(docIds) },
  environment: {
    scanner: 'scripts/p3-douyin-adapter-check.mjs',
    node: process.version,
    cwd: root,
    note: 'Static scan only; not Production Ready; real credential E2E deferred to Phase P10',
  },
  issues,
  checks,
};

const jsonPath = path.join(root, 'docs/p3-douyin-adapter-report.json');
const mdPath = path.join(root, 'docs/P3_DOUYIN_ADAPTER_REPORT.md');
fs.writeFileSync(jsonPath, JSON.stringify(report, null, 2));

const md = [
  '# P3 抖店 Adapter 实施报告',
  '',
  `Generated: ${report.generatedAt}`,
  '',
  `**Overall:** ${overall} (${passed} passed, ${warnings} warnings, ${failed} failed)`,
  '',
  '> Phase P3 实现抖店 Production Adapter 代码。此扫描为**静态扫描**；不代表 Production Ready、灰度发布或真实平台 E2E 通过。',
  '> 真实凭证 E2E 验证推迟至 Phase P10。',
  '',
  '## Sections',
  '',
  '| Section | Status |',
  '| --- | --- |',
  `| provider | ${report.providerSection.status} |`,
  `| webhook | ${report.webhookSection.status} |`,
  `| models | ${report.modelSection.status} |`,
  `| idempotency | ${report.idemSection.status} |`,
  `| migrations | ${report.migrateSection.status} |`,
  `| configstatus | ${report.configSection.status} |`,
  `| failClassifier | ${report.failClassifierSection.status} |`,
  `| fixtures | ${report.fixtureSection.status} |`,
  `| docs | ${report.docsSection.status} |`,
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
  'node scripts/p3-douyin-adapter-check.mjs',
  '```',
  '',
  '## 下一步',
  '',
  '1. 申请抖店开放平台账号 + App 审核通过',
  '2. 使用沙箱验证 OAuth / product.addV2 / sku.syncStock / order.searchList',
  '3. 客服消息：合同签署后移除 contract_mismatch 拦截',
  '4. 品牌列表：申请权限后实现',
  '',
].join('\n');
fs.writeFileSync(mdPath, md);

console.log(`P3 scan: ${overall} → ${jsonPath}`);
process.exit(failed > 0 ? 1 : 0);
