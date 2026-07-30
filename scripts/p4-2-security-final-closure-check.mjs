#!/usr/bin/env node
/**
 * Phase P4.2 security final closure static scan.
 * Output: docs/P4_2_SECURITY_FINAL_CLOSURE_REPORT.md + docs/p4-2-security-final-closure-report.json
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
function pass(id, message, detail) { add(id, 'passed', message, detail); }
function fail(id, message, detail) { add(id, 'failed', message, detail); }
function warn(id, message, detail) { add(id, 'warning', message, detail); }

// --- tasktenant package ---
const taskCtx = read('backend/internal/pkg/tasktenant/context.go');
const workerGate = read('backend/internal/pkg/tasktenant/worker_gate.go');
const taskResolve = read('backend/internal/pkg/tasktenant/resolve.go');
if (
  taskCtx.includes('RequireTaskTenant') &&
  taskCtx.includes('BuildWorkerContext') &&
  workerGate.includes('BeginWorker') &&
  taskResolve.includes('ResolveShopTenant')
) {
  pass('tasktenant-package', 'tasktenant context + worker gate + resolve');
} else {
  fail('tasktenant-package', 'tasktenant package incomplete');
}

// --- Production workers with tasktenant ---
const workerFiles = [
  ['worker-collect', 'backend/internal/modules/collect/worker.go', 'tasktenant.BeginWorker'],
  ['worker-ordersync', 'backend/internal/modules/ordersync/worker.go', 'tasktenant.BeginWorker'],
  ['worker-customersync', 'backend/internal/modules/customersync/worker.go', 'tasktenant.BeginWorker'],
  ['worker-productpublish', 'backend/internal/modules/productpublish/worker_consumer.go', 'tasktenant.BeginWorker'],
  ['worker-inventory', 'backend/internal/modules/inventory/worker_consumer.go', 'tasktenant.BeginWorker'],
  ['worker-file-scan', 'backend/internal/modules/files/scan_worker.go', 'tasktenant.BeginWorker'],
  ['worker-webhook', 'backend/internal/modules/webhook/processor.go', 'tasktenant.BeginWorker'],
];
for (const [id, file, needle] of workerFiles) {
  const body = read(file);
  if (body.includes(needle)) pass(id, `${path.basename(file)} uses tasktenant`);
  else fail(id, `${file} missing ${needle}`);
}

// --- security_secret_reencrypt worker ---
const reencryptWorker = read('backend/internal/modules/securitymod/reencrypt_worker.go');
const workerModel = read('backend/internal/modules/worker/model.go');
const mainGo = read('backend/cmd/server/main.go');
if (
  reencryptWorker.includes('StartReencryptWorker') &&
  reencryptWorker.includes('ProcessReencryptBatch') &&
  workerModel.includes('TypeSecuritySecretReencrypt') &&
  mainGo.includes('securitymod.StartReencryptWorker')
) {
  pass('security-secret-reencrypt-worker', 'security_secret_reencrypt worker registered');
} else {
  fail('security-secret-reencrypt-worker', 'security_secret_reencrypt worker incomplete');
}

// --- file_security_scan worker ---
const scanWorker = read('backend/internal/modules/files/scan_worker.go');
if (
  scanWorker.includes('StartScanWorker') &&
  scanWorker.includes('file:security:scan') &&
  workerModel.includes('TypeFileSecurityScan') &&
  mainGo.includes('files.StartScanWorker')
) {
  pass('file-security-scan-worker', 'file_security_scan worker + queue');
} else {
  fail('file-security-scan-worker', 'file_security_scan worker incomplete');
}

// --- Tenant scope helpers ---
const tenantQuery = read('backend/internal/pkg/tenantquery/scope.go');
const taskcenterScope = read('backend/internal/modules/taskcenter/tenant_scope.go');
if (
  tenantQuery.includes('ScopeTenant') &&
  tenantQuery.includes('ScopeShopTenant') &&
  taskcenterScope.includes('applyListTenantScope')
) {
  pass('tenant-scope-helpers', 'tenantquery + taskcenter tenant scope');
} else {
  fail('tenant-scope-helpers', 'tenant scope helpers missing');
}

// --- P4.2 models with tenant_id ---
const p42Models = [
  'backend/internal/modules/inventory/model.go',
  'backend/internal/modules/ordersync/model.go',
  'backend/internal/modules/customersync/model.go',
  'backend/internal/modules/productpublish/model.go',
  'backend/internal/modules/collect/model.go',
  'backend/internal/modules/exportmod/model.go',
  'backend/internal/modules/files/model.go',
  'backend/internal/modules/aiproducttext/model.go',
  'backend/internal/modules/aiproductimage/model.go',
  'backend/internal/modules/customerchat/model.go',
  'backend/internal/modules/taskcenter/model.go',
];
let modelHits = 0;
for (const m of p42Models) {
  const body = read(m);
  if (body.includes('TenantID')) modelHits++;
}
if (modelHits >= p42Models.length - 1) {
  pass('p42-model-tenant-columns', `${modelHits}/${p42Models.length} P4.2 models carry tenant_id`);
} else {
  fail('p42-model-tenant-columns', `Only ${modelHits}/${p42Models.length} models have tenant_id`);
}

// --- Migration ---
if (exists('backend/internal/database/migrate_p4_2.go') && read('backend/internal/database/migrate.go').includes('migrateP42Security')) {
  pass('migrate-p4-2', 'migrate_p4_2.go wired in migrate.go');
} else {
  fail('migrate-p4-2', 'P4.2 migration missing or not wired');
}

// --- Secret targets ---
const secretTargets = read('backend/internal/modules/securitymod/secret_targets.go');
if (
  secretTargets.includes('SettingsSecretTarget') &&
  secretTargets.includes('ShopAuthTokenTarget') &&
  secretTargets.includes('AllReencryptTargets')
) {
  pass('secret-target-coverage', 'settings + shop_auth_tokens reencrypt targets');
} else {
  fail('secret-target-coverage', 'secret target adapters incomplete');
}

// --- IDOR tests (40+) ---
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
if (idorCases >= 40) pass('idor-tests', `${idorCases} IDOR test cases`);
else if (idorCases >= 20) warn('idor-tests', `${idorCases} IDOR cases (target 40+)`, 'expand automated matrix in P4.2 backlog');
else fail('idor-tests', `Only ${idorCases} IDOR cases, need 20+`);

// --- Shop scope tests (20+) ---
const shopCases = countTestsInDir('backend/internal/securitytests/shopscope', 'ShopScope_');
if (shopCases >= 20) pass('shop-scope-tests', `${shopCases} shop scope test cases`);
else if (shopCases >= 5) warn('shop-scope-tests', `${shopCases} shop scope cases (target 20+)`, 'expand order/export/product shop denial tests');
else fail('shop-scope-tests', `Only ${shopCases} shop scope cases, need 5+`);

// --- Security center UI sections ---
const secUI = read('admin/src/pages/Settings/Security/index.tsx');
const sessions = read('backend/internal/modules/auth/sessions_handler.go');
const secHandler = read('backend/internal/modules/securitymod/handler.go');
const uiSections = [
  '安全中心',
  '运行概览',
  '认证与会话',
  '租户隔离状态',
  '主密钥轮换',
  '文件安全',
  '审计完整性',
  '传输安全',
  '回调签名校验',
];
const missingSections = uiSections.filter((s) => !secUI.includes(s));
if (missingSections.length === 0 && exists('admin/src/pages/Settings/Security/index.tsx')) {
  pass('security-center-ui-sections', `Security center sections: ${uiSections.length} panels`);
} else {
  fail('security-center-ui-sections', `Missing UI sections: ${missingSections.join(', ') || 'page'}`);
}
if (secUI.includes('fetchSecurityOverview') && secUI.includes('startKeyRotation') && secUI.includes('fetchFileSecurityStats')) {
  pass('security-center-ui-api', 'Security center wired to overview/rotation/file APIs');
} else {
  fail('security-center-ui-api', 'Security center API wiring incomplete');
}
if (sessions.includes('ListSessions') && sessions.includes('RevokeOthers')) {
  pass('session-management-api', 'Session list + revoke-others API');
} else {
  fail('session-management-api', 'Session API incomplete');
}
if (secHandler.includes('keys/rotation/start') && secHandler.includes('keys/references')) {
  pass('key-rotation-api', 'Key rotation + references API');
} else {
  fail('key-rotation-api', 'Key rotation API incomplete');
}

// --- Race tests (Linux/WSL only) ---
const isWindows = platform() === 'win32';
if (isWindows) {
  warn('race-tests', 'Race detector deferred on Windows', 'Run go test -race on Linux/WSL2/CI');
} else {
  warn('race-tests', 'Race tests not executed by this script', 'Run backend-race CI job manually');
}

// --- P4.2 documentation ---
const p42Docs = [
  'docs/P4_2_FULL_TENANT_AND_SECURITY_WORKER_AUDIT.md',
  'docs/P4_2_REPOSITORY_TENANT_COVERAGE.md',
  'docs/P4_2_ALL_WORKER_TENANT_CONTEXT.md',
  'docs/P4_2_WEBHOOK_TENANT_PROCESSING.md',
  'docs/P4_2_SECRET_TARGET_COVERAGE.md',
  'docs/P4_2_SECRET_REENCRYPT_EXECUTION.md',
  'docs/P4_2_FILE_SCAN_WORKER.md',
  'docs/P4_2_SECURITY_CENTER_UI.md',
  'docs/P4_2_IDOR_TEST_REPORT.md',
  'docs/P4_2_SHOP_SCOPE_TEST_REPORT.md',
  'docs/P4_2_RACE_TEST_REPORT.md',
];
for (const doc of p42Docs) {
  if (exists(doc)) pass('doc-' + doc.split('/').pop().replace('.md', ''), doc);
  else fail('doc-' + doc.split('/').pop().replace('.md', ''), doc + ' missing');
}

const failed = checks.filter((c) => c.status === 'failed').length;
const warnings = checks.filter((c) => c.status === 'warning').length;
const report = {
  phase: 'P4.2',
  status:
    failed === 0
      ? warnings > 0
        ? 'passed_with_warnings'
        : 'passed_with_real_environment_verification_deferred'
      : 'failed',
  generatedAt: new Date().toISOString(),
  platform: platform(),
  tenantIsolation: {
    tasktenantPackage: checks.find((c) => c.id === 'tasktenant-package')?.status || 'unknown',
    workerCoverage: workerFiles.every((w) => checks.find((c) => c.id === w[0])?.status === 'passed')
      ? 'passed'
      : 'partial',
    modelColumns: checks.find((c) => c.id === 'p42-model-tenant-columns')?.status || 'unknown',
    idorTests: checks.find((c) => c.id === 'idor-tests')?.status || 'unknown',
    shopScopeTests: checks.find((c) => c.id === 'shop-scope-tests')?.status || 'unknown',
  },
  securityWorkers: {
    secretReencrypt: checks.find((c) => c.id === 'security-secret-reencrypt-worker')?.status || 'unknown',
    fileScan: checks.find((c) => c.id === 'file-security-scan-worker')?.status || 'unknown',
    secretTargets: checks.find((c) => c.id === 'secret-target-coverage')?.status || 'unknown',
  },
  ui: {
    securityCenterSections: checks.find((c) => c.id === 'security-center-ui-sections')?.status || 'unknown',
    sessions: checks.find((c) => c.id === 'session-management-api')?.status || 'unknown',
    keyRotation: checks.find((c) => c.id === 'key-rotation-api')?.status || 'unknown',
  },
  race: {
    status: isWindows ? 'deferred_on_windows' : 'not_run_by_static_scan',
    note: 'Execute go test -race on Linux/WSL2/CI (backend-race job)',
  },
  migration: checks.find((c) => c.id === 'migrate-p4-2')?.status || 'unknown',
  realEnvironmentVerification: 'deferred',
  checks,
  issues: checks.filter((c) => c.status === 'failed').map((c) => c.message),
  warnings: checks.filter((c) => c.status === 'warning').map((c) => c.message),
};

const md = [
  '# P4.2 Security Final Closure Report',
  '',
  `**Status:** ${report.status}`,
  `**Generated:** ${report.generatedAt}`,
  `**Platform:** ${report.platform}`,
  '',
  '| Check | Status | Message |',
  '| --- | --- | --- |',
  ...checks.map((c) => `| ${c.id} | ${c.status} | ${c.message} |`),
  '',
  '## Summary',
  '',
  `- Tenant worker gate: **${report.tenantIsolation.workerCoverage}**`,
  `- IDOR automated tests: **${report.tenantIsolation.idorTests}**`,
  `- Shop scope tests: **${report.tenantIsolation.shopScopeTests}**`,
  `- Secret reencrypt worker: **${report.securityWorkers.secretReencrypt}**`,
  `- File scan worker: **${report.securityWorkers.fileScan}**`,
  `- Race tests: **${report.race.status}**`,
  '',
  '## Deferred',
  '- Real environment security verification',
  '- Linux race tests (run on WSL2/CI when on Windows)',
  '- Expand IDOR/shop-scope automated matrix to closure targets',
  '',
].join('\n');

fs.mkdirSync(path.join(root, 'docs'), { recursive: true });
fs.writeFileSync(path.join(root, 'docs', 'P4_2_SECURITY_FINAL_CLOSURE_REPORT.md'), md);
fs.writeFileSync(path.join(root, 'docs', 'p4-2-security-final-closure-report.json'), JSON.stringify(report, null, 2));

console.log(md);
process.exit(failed > 0 ? 1 : 0);
