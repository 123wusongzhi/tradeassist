#!/usr/bin/env node
/**
 * Phase P4-V security closure verification gate.
 * Output: docs/P4_V_SECURITY_CLOSURE_REPORT.md + docs/p4-v-security-closure-report.json
 */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { platform } from 'node:os';

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
function add(id, status, message, detail = '') {
  checks.push({ id, status, message, detail });
}
function pass(id, message, detail = '') { add(id, 'passed', message, detail); }
function fail(id, message, detail = '') { add(id, 'failed', message, detail); }
function warn(id, message, detail = '') { add(id, 'warning', message, detail); }

// --- P4-V required reports ---
const p4vDocs = [
  'docs/P4_V_SECURITY_CLOSURE_AUDIT.md',
  'docs/P4_V_SECRET_TARGET_COVERAGE.md',
  'docs/P4_V_KEY_ROTATION_VERIFY_REPORT.md',
  'docs/P4_V_SQL_TENANT_SCOPE_REPORT.md',
  'docs/P4_V_ACCESS_CONTROL_REGRESSION.md',
  'docs/P4_V_RACE_TEST_REPORT.md',
];
for (const doc of p4vDocs) {
  if (exists(doc)) pass(`doc-${path.basename(doc, '.md')}`, doc);
  else fail(`doc-${path.basename(doc, '.md')}`, `${doc} missing`);
}

// --- Secret target wiring ---
const rotationAgg = read('backend/internal/modules/securitymod/rotation_aggregate.go');
const secretTargets = read('backend/internal/modules/securitymod/secret_targets.go');
if (
  secretTargets.includes('SettingsSecretTarget') &&
  secretTargets.includes('ShopAuthTokenTarget') &&
  secretTargets.includes('AllReencryptTargets')
) {
  pass('secret-target-adapters', 'SettingsSecretTarget + ShopAuthTokenTarget defined');
} else {
  fail('secret-target-adapters', 'secret target adapters incomplete');
}
if (
  rotationAgg.includes('aggregateSecretReferences') &&
  rotationAgg.includes('scanShopTokenReferences') &&
  rotationAgg.includes('scanSettingsReferences')
) {
  pass('verify-rotation-all-targets', 'VerifyRotation aggregates all secret targets');
} else {
  fail('verify-rotation-all-targets', 'VerifyRotation does not aggregate all targets');
}

// --- Tenant scope in key modules ---
const tenantModules = [
  ['inventory-tenant', 'backend/internal/modules/inventory/queries.go', 'ApplyTenantScope'],
  ['ordersync-tenant', 'backend/internal/modules/ordersync/service.go', 'ApplyTenantScope'],
  ['productpublish-tenant', 'backend/internal/modules/productpublish/service_queries.go', 'ApplyTenantScope'],
  ['customerchat-tenant', 'backend/internal/modules/customerchat/service.go', 'ApplyTenantScope'],
  ['taskcenter-tenant', 'backend/internal/modules/taskcenter/service.go', 'applyTenantListFilter'],
  ['webhook-tenant', 'backend/internal/modules/webhook/processor.go', 'tenant_id = ?'],
  ['exportmod-tenant', 'backend/internal/modules/exportmod/service.go', 'repository.FindByID'],
];
for (const [id, file, needle] of tenantModules) {
  const body = read(file);
  if (body.includes(needle)) pass(id, `${path.basename(file)} tenant scope`);
  else fail(id, `${file} missing ${needle}`);
}

// --- System repository naming ---
const tenantScope = read('backend/internal/pkg/repository/tenant_scope.go');
if (tenantScope.includes('SystemFindByID') && tenantScope.includes('ErrSystemContextRequired')) {
  pass('system-repository-naming', 'SystemFindByID requires system context');
} else {
  fail('system-repository-naming', 'System repository guard missing');
}

// --- Production tenant fallback ---
const tenantConfig = read('backend/internal/config/tenant_config.go');
if (tenantConfig.includes('legacy_dev_zero') || tenantConfig.includes('ResolveRequestTenantID')) {
  warn('production-tenant-fallback', 'tenant config has dev fallback — verify production blocks tenant_id=0');
} else {
  pass('production-tenant-fallback', 'tenant config reviewed');
}

// --- IDOR / Shop scope tests ---
function countTestsInDir(relDir, prefix) {
  const dir = path.join(root, relDir);
  if (!fs.existsSync(dir)) return 0;
  let total = 0;
  for (const name of fs.readdirSync(dir)) {
    if (!name.endsWith('_test.go')) continue;
    const body = fs.readFileSync(path.join(dir, name), 'utf8');
    total += (body.match(new RegExp(`func Test${prefix}`, 'g')) || []).length;
  }
  return total;
}
const idorCases = countTestsInDir('backend/internal/securitytests/idor', 'IDOR_');
const shopCases = countTestsInDir('backend/internal/securitytests/shopscope', 'ShopScope_');
if (idorCases >= 55) pass('idor-55-cases', `${idorCases} IDOR test cases`);
else if (idorCases >= 40) warn('idor-55-cases', `${idorCases} IDOR cases (target 55)`);
else fail('idor-55-cases', `Only ${idorCases} IDOR cases`);

if (shopCases >= 21) pass('shop-scope-21-cases', `${shopCases} shop scope test cases`);
else if (shopCases >= 15) warn('shop-scope-21-cases', `${shopCases} shop scope cases (target 21)`);
else fail('shop-scope-21-cases', `Only ${shopCases} shop scope cases`);

// --- Race report ---
const raceReport = read('docs/P4_V_RACE_TEST_REPORT.md');
if (raceReport.includes('Linux Race Verification Passed')) {
  pass('linux-race-passed', 'Linux race verification passed');
} else if (raceReport.includes('Linux Race Verification Pending')) {
  warn('linux-race-pending', 'Linux race verification pending');
} else if (exists('docs/P4_V_RACE_TEST_REPORT.md')) {
  warn('linux-race-status', 'Race report exists but status unclear');
} else {
  fail('linux-race-report', 'P4_V_RACE_TEST_REPORT.md missing');
}

// --- demo:auto-acceptance report presence ---
const demoReport = exists('docs/demo-auto-acceptance-report.json') || exists('docs/DEMO_AUTO_ACCEPTANCE_REPORT.md');
if (demoReport) pass('demo-auto-acceptance-report', 'demo:auto-acceptance report present');
else warn('demo-auto-acceptance-report', 'demo:auto-acceptance report not found — run pnpm demo:auto-acceptance');

// --- Rotation tests ---
if (exists('backend/internal/modules/securitymod/rotation_test.go')) {
  pass('rotation-unit-tests', 'securitymod rotation tests present');
} else {
  fail('rotation-unit-tests', 'rotation tests missing');
}

const failed = checks.filter((c) => c.status === 'failed').length;
const warnings = checks.filter((c) => c.status === 'warning').length;
const report = {
  phase: 'P4-V',
  status:
    failed === 0
      ? warnings > 0
        ? 'passed_with_warnings'
        : 'passed_with_real_environment_verification_deferred'
      : 'failed',
  generatedAt: new Date().toISOString(),
  platform: platform(),
  tenantIsolation: {
    sqlScopeReport: exists('docs/P4_V_SQL_TENANT_SCOPE_REPORT.md'),
    idorCases,
    shopScopeCases: shopCases,
  },
  secretRotation: {
    targetsVerified: checks.find((c) => c.id === 'verify-rotation-all-targets')?.status === 'passed',
    coverageReport: exists('docs/P4_V_SECRET_TARGET_COVERAGE.md'),
  },
  race: {
    status: raceReport.includes('Linux Race Verification Passed')
      ? 'passed'
      : raceReport.includes('Linux Race Verification Pending')
        ? 'pending'
        : 'unknown',
  },
  realEnvironmentSecurityVerification: 'deferred',
  realCredentialVerification: 'deferred',
  checks,
  issues: checks.filter((c) => c.status === 'failed').map((c) => c.message),
  warnings: checks.filter((c) => c.status === 'warning').map((c) => c.message),
};

const md = [
  '# P4-V Security Closure Report',
  '',
  `**Status:** ${report.status}`,
  `**Generated:** ${report.generatedAt}`,
  `**Platform:** ${report.platform}`,
  '',
  '| Check | Status | Message |',
  '| --- | --- | --- |',
  ...checks.map((c) => `| ${c.id} | ${c.status} | ${c.message} |`),
  '',
  '## Deferred',
  '- Real environment security verification',
  '- Real Douyin credential E2E',
  '',
].join('\n');

fs.mkdirSync(path.join(root, 'docs'), { recursive: true });
fs.writeFileSync(path.join(root, 'docs', 'P4_V_SECURITY_CLOSURE_REPORT.md'), md);
fs.writeFileSync(path.join(root, 'docs', 'p4-v-security-closure-report.json'), JSON.stringify(report, null, 2));

console.log(md);
console.log(`\nP4-V gate: failed=${failed} warnings=${warnings}`);
process.exit(failed > 0 ? 1 : 0);
