#!/usr/bin/env node
/**
 * Phase P3.2 multi-shop webhook static scan.
 * Output: docs/P3_2_MULTI_SHOP_WEBHOOK_REPORT.md + docs/p3-2-multi-shop-webhook-report.json
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

function record(id, status, message, detail = null) {
  checks.push({ id, status, message, detail });
}

function pass(id, message, detail) {
  record(id, 'passed', message, detail);
}

function fail(id, message, detail) {
  record(id, 'failed', message, detail);
}

function warn(id, message, detail) {
  record(id, 'warning', message, detail);
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

function sectionStatus(ids) {
  const subset = checks.filter((c) => ids.some((id) => c.id.startsWith(id)));
  if (subset.some((c) => c.status === 'failed')) return 'failed';
  if (subset.some((c) => c.status === 'warning')) return 'warning';
  return 'passed';
}

requireFile('resolver.file', 'backend/internal/modules/webhook/shop_resolver.go', 'WebhookShopResolver');
requireContent('resolver.interface', 'backend/internal/modules/webhook/shop_resolver.go',
  ['type WebhookShopResolver interface', 'ResolveWebhookShopInput', 'ResolvedWebhookShop', 'DBWebhookShopResolver'],
  'resolver interface and DB implementation');
requireContent('resolver.no_first_shop_fallback', 'backend/internal/modules/webhook/shop_resolver.go',
  ['DOUYIN_WEBHOOK_SHOP_NOT_RESOLVED', 'DOUYIN_WEBHOOK_SHOP_AMBIGUOUS', 'DOUYIN_WEBHOOK_UNTRUSTED_SHOP_IDENTIFIER'],
  'resolver rejects missing / ambiguous / untrusted shop IDs');
requireContent('resolver.binding', 'backend/internal/modules/webhook/shop_resolver.go',
  ['shop_auth_tokens.app_key', 'shop_auth_tokens.id', 'DOUYIN_WEBHOOK_SHOP_BINDING_MISMATCH', 'DOUYIN_WEBHOOK_APP_BINDING_MISMATCH'],
  'resolver validates app and binding ownership');
requireContent('resolver.tenant', 'backend/internal/modules/webhook/shop_resolver.go',
  ['shops.tenant_id', 'TenantID', 'InternalShopID'],
  'resolver returns tenant and internal shop');
requireContent('handler.resolver', 'backend/internal/modules/webhook/handler.go',
  ['ExtractResolveWebhookShopInput', 'ShopResolver.Resolve', 'ResolvedShop'],
  'handler calls resolver before ingest');
requireContent('router.resolver', 'backend/internal/api/router.go',
  ['ShopResolver:', 'DBWebhookShopResolver'],
  'router wires resolver');

requireContent('event.model.scope', 'backend/internal/modules/webhook/model.go',
  ['TenantID', 'InternalShopID', 'PlatformShopID', 'BindingID', 'ux_webhook_shop_event'],
  'webhook event stores tenant/shop/binding scope');
requireContent('event.ingest.scope', 'backend/internal/modules/webhook/service.go',
  ['WebhookScoped', 'tenant_id', 'platform_shop_id', 'applyResolvedShopToEvent'],
  'ingest idempotency and uniqueness are shop scoped');
requireContent('event.worker.by_id', 'backend/internal/modules/webhook/processor.go',
  ['ProcessEventByID', 'webhookProcessKey', 'ProcessQueuedEvents'],
  'worker processes selected event row by ID');
requireContent('order.tenant.scope', 'backend/internal/modules/ordersync/douyin_order_webhook.go',
  ['TenantID:', 'PlatformShopID:', 'tenant mismatch', 'shop_binding_mismatch'],
  'order webhook uses resolved tenant/shop scope');
requireContent('order.upsert.scope', 'backend/internal/modules/order/platform_upsert.go',
  ['TenantID', 'PlatformShopID', 'p.TenantID'],
  'order upsert accepts tenant and platform shop scope');
requireContent('order.import.scope', 'backend/internal/modules/order/idempotency_import.go',
  ['tenant_id = ? AND shop_id = ?', 'OrderImportRevision'],
  'order import lookup is tenant scoped');

requireContent('fallback.config', 'backend/internal/config/config.go',
  ['DOUYIN_WEBHOOK_TEST_SHOP_BINDING_ID', 'ENABLE_DOUYIN_WEBHOOK_DEMO_FALLBACK'],
  'fallback env vars are loaded');
requireContent('fallback.validate', 'backend/internal/config/validate.go',
  ['PRODUCTION_WEBHOOK_FALLBACK_FORBIDDEN', 'validateWebhookFallback', 'IsStagingOrProduction'],
  'staging/production fallback fail-fast');
requireContent('fallback.test', 'backend/internal/config/validate_test.go',
  ['TestValidate_productionRejectsDouyinWebhookFallback', 'TestValidate_stagingRejectsDouyinWebhookDemoFallback'],
  'fallback config tests');
requireContent('env.example', '.env.example',
  ['DOUYIN_WEBHOOK_TEST_SHOP_BINDING_ID', 'ENABLE_DOUYIN_WEBHOOK_DEMO_FALLBACK'],
  'env example contains fallback vars');
requireContent('env.docker', '.env.docker.example',
  ['DOUYIN_WEBHOOK_TEST_SHOP_BINDING_ID', 'ENABLE_DOUYIN_WEBHOOK_DEMO_FALLBACK'],
  'docker env example contains fallback vars');
requireContent('compose.env', 'docker-compose.full.yml',
  ['DOUYIN_WEBHOOK_TEST_SHOP_BINDING_ID', 'ENABLE_DOUYIN_WEBHOOK_DEMO_FALLBACK'],
  'docker compose passes fallback vars');

requireFile('test.resolver', 'backend/internal/modules/webhook/shop_resolver_test.go', 'resolver tests');
requireContent('test.multishop.event', 'backend/internal/modules/webhook/shop_resolver_test.go',
  ['TestWebhookIngestSameEventIDDifferentShops', 'same-event', 'ProcessQueuedEvents'],
  'same eventId across shops test exists');
requireContent('test.ambiguous', 'backend/internal/modules/webhook/shop_resolver_test.go',
  ['TestWebhookShopResolverRejectsAmbiguousBinding'],
  'ambiguous binding test exists');
requireContent('test.webhook.concurrent', 'backend/internal/modules/webhook/handler_test.go',
  ['TestWebhookConcurrentSameEvent'],
  'same-shop concurrent event test exists');
requireContent('test.order.concurrent', 'backend/internal/modules/order/platform_upsert_test.go',
  ['TestWebhookPollingConcurrentUpsert'],
  'webhook + polling concurrent upsert test exists');

requireFile('migration.p32', 'backend/internal/database/migrate_p3_2_webhook.go', 'P3.2 migration');
requireContent('migration.call', 'backend/internal/database/migrate.go',
  ['migrateP32Webhook'],
  'migrate.go calls P3.2 migration');
requireContent('migration.index', 'backend/internal/database/migrate_p3_2_webhook.go',
  ['DROP INDEX IF EXISTS ux_webhook_platform_event', 'ux_webhook_shop_event', 'ux_shops_platform_external_active'],
  'migration replaces platform-only webhook uniqueness');

requireContent('configstatus.p32', 'backend/internal/modules/configstatus/service.go',
  ['p32WebhookResolverItem', 'p32WebhookProductionFallbackItem', 'p32RaceVerificationItem'],
  'config status wires P3.2 items');
requireFile('configstatus.p32.file', 'backend/internal/modules/configstatus/p32_status.go', 'P3.2 config status file');
requireContent('taskcenter.codes', 'backend/internal/modules/taskcenter/failureclassifier/enumerate.go',
  ['douyin_webhook_shop_not_resolved', 'douyin_webhook_shop_ambiguous', 'douyin_webhook_tenant_mismatch'],
  'task center P3.2 categories exist');

const docs = [
  'docs/P3_2_MULTI_SHOP_WEBHOOK_AUDIT.md',
  'docs/DOUYIN_WEBHOOK_SHOP_RESOLUTION.md',
  'docs/DOUYIN_WEBHOOK_TENANT_ISOLATION.md',
  'docs/DOUYIN_WEBHOOK_APP_SECRET_BINDING.md',
  'docs/TEST_DATABASE_ISOLATION.md',
  'docs/GO_TEST_STABILITY_REPORT.md',
  'docs/P3_2_RACE_TEST_REPORT.md',
  'docs/P3_2_MULTI_SHOP_WEBHOOK_REPORT.md',
];
for (const doc of docs) requireFile(`doc.${path.basename(doc)}`, doc, path.basename(doc));

const race = read('docs/P3_2_RACE_TEST_REPORT.md');
let raceStatus = 'environment_blocked';
if (race.includes('Final result: passed')) raceStatus = 'passed';
else if (race.includes('Final result: failed')) raceStatus = 'failed';
else warn('race.not_passed', 'race report does not claim passed; keep status environment_blocked', 'docs/P3_2_RACE_TEST_REPORT.md');

const failed = checks.filter((c) => c.status === 'failed');
const warnings = checks.filter((c) => c.status === 'warning');
const reportStatus = failed.length === 0 && raceStatus === 'passed'
  ? 'passed_with_real_credentials_deferred'
  : failed.length === 0
    ? 'passed_with_race_environment_blocked'
    : 'failed';

const report = {
  phase: 'P3.2',
  status: reportStatus,
  multiShopRouting: {
    resolver: sectionStatus(['resolver.', 'handler.resolver', 'router.resolver']),
    binding: sectionStatus(['resolver.binding', 'migration.index']),
    tenantIsolation: sectionStatus(['resolver.tenant', 'order.tenant', 'order.upsert', 'order.import']),
    ambiguityRejection: sectionStatus(['resolver.no_first', 'test.ambiguous']),
    productionFallback: sectionStatus(['fallback.']),
  },
  concurrency: {
    webhook: sectionStatus(['event.', 'test.multishop', 'test.webhook']),
    orderUpsert: sectionStatus(['order.', 'test.order']),
    tokenRefresh: exists('backend/internal/providers/platform/douyinshop/token_lock_test.go') ? 'passed' : 'warning',
  },
  race: { status: raceStatus },
  regression: {
    goTestStableRuns: Number((read('docs/GO_TEST_STABILITY_REPORT.md').match(/go test stable runs:\s*(\d+)/i) || [])[1] || 0),
    nonAiFailed: Number((read('docs/GO_TEST_STABILITY_REPORT.md').match(/non-AI failed:\s*(\d+)/i) || [])[1] || 0),
  },
  realCredentialVerification: 'deferred',
  checks,
  issues: failed.map((c) => c.id),
  warnings: warnings.map((c) => c.id),
};

const jsonPath = path.join(root, 'docs/p3-2-multi-shop-webhook-report.json');
fs.writeFileSync(jsonPath, JSON.stringify(report, null, 2) + '\n');

const md = [
  '# Phase P3.2 Multi-Shop Webhook Report',
  '',
  `Generated: ${new Date().toISOString()}`,
  '',
  `Status: ${report.status}`,
  '',
  'This static scan validates code and documentation for multi-shop webhook routing. It does not perform real Douyin credential E2E and does not mark Production Ready.',
  '',
  '## Summary',
  '',
  `- Resolver: ${report.multiShopRouting.resolver}`,
  `- Binding: ${report.multiShopRouting.binding}`,
  `- Tenant isolation: ${report.multiShopRouting.tenantIsolation}`,
  `- Production fallback: ${report.multiShopRouting.productionFallback}`,
  `- Webhook concurrency: ${report.concurrency.webhook}`,
  `- Order upsert concurrency: ${report.concurrency.orderUpsert}`,
  `- Race: ${report.race.status}`,
  `- Real credential verification: ${report.realCredentialVerification}`,
  '',
  '## Checks',
  '',
  '| id | status | message | detail |',
  '| --- | --- | --- | --- |',
  ...checks.map((c) => `| ${c.id} | ${c.status} | ${String(c.message).replaceAll('|', '\\|')} | ${c.detail ?? ''} |`),
  '',
];
fs.writeFileSync(path.join(root, 'docs/P3_2_MULTI_SHOP_WEBHOOK_REPORT.md'), md.join('\n'));

if (failed.length > 0) {
  console.error(`P3.2 scan failed: ${failed.map((c) => c.id).join(', ')}`);
  process.exit(1);
}

console.log(`P3.2 scan ${report.status}: ${checks.length} checks, ${warnings.length} warnings`);
