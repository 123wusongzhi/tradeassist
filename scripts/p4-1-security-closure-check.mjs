#!/usr/bin/env node
/**
 * Phase P4.1 security closure static scan.
 * Output: docs/P4_1_SECURITY_CLOSURE_REPORT.md + docs/p4-1-security-closure-report.json
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
function add(id, status, message, detail = '') {
  checks.push({ id, status, message, detail });
}
function pass(id, message, detail) { add(id, 'passed', message, detail); }
function fail(id, message, detail) { add(id, 'failed', message, detail); }
function warn(id, message, detail) { add(id, 'warning', message, detail); }

// Production tenant fallback
const validate = read('backend/internal/config/validate.go');
const tenantCfg = read('backend/internal/config/tenant_config.go');
if (validate.includes('validateTenantIsolation') && tenantCfg.includes('PRODUCTION_TENANT_FALLBACK_FORBIDDEN')) {
  pass('production-tenant-fallback', 'Production tenant fallback forbidden in config.Validate');
} else fail('production-tenant-fallback', 'Missing production tenant fallback validation');

// TenantContext
const tenant = read('backend/internal/pkg/security/tenant.go');
if (tenant.includes('AuthSource') && tenant.includes('RequireTenantContext')) pass('tenant-context', 'TenantContext extended');
else fail('tenant-context', 'TenantContext incomplete');

// Repository tenant scope
const repoScope = read('backend/internal/pkg/repository/tenant_scope.go');
if (repoScope.includes('FindByID') && repoScope.includes('tenant_id')) pass('repository-tenant-scope', 'Repository tenant helpers');
else fail('repository-tenant-scope', 'Repository tenant helpers missing');

// System repository naming
if (repoScope.includes('SystemFindByID')) pass('system-repository', 'SystemFindByID isolated');
else fail('system-repository', 'SystemFindByID missing');

// Worker tenant
const taskTenant = read('backend/internal/pkg/tasktenant/context.go');
if (taskTenant.includes('RequireTaskTenant') && taskTenant.includes('BuildWorkerContext')) pass('worker-tenant', 'Worker tenant context');
else fail('worker-tenant', 'Worker tenant context missing');

// IDOR tests
const idorTest = 'backend/internal/securitytests/idor/idor_test.go';
if (exists(idorTest)) {
  const idor = read(idorTest);
  const cases = (idor.match(/func TestIDOR_/g) || []).length;
  if (cases >= 20) pass('idor-tests', `${cases} IDOR test cases`);
  else fail('idor-tests', `Only ${cases} IDOR cases, need 20+`);
} else fail('idor-tests', 'IDOR test suite missing');

// Key rotation
const rotation = read('backend/internal/modules/securitymod/rotation.go');
const rotModels = read('backend/internal/modules/securitymod/models.go');
if (rotation.includes('ProcessReencryptBatch') && rotation.includes('VerifyRotation')) pass('key-rotation-worker', 'Reencrypt + verify');
else fail('key-rotation-worker', 'Rotation execution incomplete');
if (rotModels.includes('KeyRotationJob')) pass('key-rotation-model', 'Rotation state model');
else fail('key-rotation-model', 'Rotation model missing');

const secHandler = read('backend/internal/modules/securitymod/handler.go');
if (secHandler.includes('rotation/:id/verify') && secHandler.includes('KeyReferences')) pass('key-rotation-api', 'Rotation API closure');
else fail('key-rotation-api', 'Rotation API incomplete');

// File scanner
const scanner = read('backend/internal/pkg/filescanner/scanner.go');
const imgScan = read('backend/internal/pkg/filescanner/image_decode.go');
if (scanner.includes('FileScanner') && imgScan.includes('ImageDecodeScanner')) pass('file-scanner', 'FileScanner + image decode');
else fail('file-scanner', 'FileScanner missing');

const stateMachine = read('backend/internal/modules/files/state_machine.go');
if (stateMachine.includes('pending_scan') && stateMachine.includes('CanTransition')) pass('file-state-machine', 'File security state machine');
else fail('file-state-machine', 'State machine missing');

const fileAccess = read('backend/internal/modules/files/access.go');
if (fileAccess.includes('CreateDownloadURL') && fileAccess.includes('LoadForDownload')) pass('private-file-access', 'Private object access');
else fail('private-file-access', 'Private access missing');

// Legacy auth
const authCfg = read('backend/internal/config/auth_config.go');
if (authCfg.includes('INSECURE_LEGACY_AUTH_MODE_FORBIDDEN')) pass('legacy-auth-failfast', 'Production legacy auth forbidden');
else fail('legacy-auth-failfast', 'Legacy auth fail-fast missing');

// Security center UI
if (exists('admin/src/pages/Settings/Security/index.tsx')) pass('security-center-ui', 'Security settings page exists');
else fail('security-center-ui', 'Security center UI missing');

const sessions = read('backend/internal/modules/auth/sessions_handler.go');
if (sessions.includes('ListSessions') && sessions.includes('RevokeOthers')) pass('session-management-ui', 'Session management API');
else fail('session-management-ui', 'Session API incomplete');

// Docs
for (const doc of [
  'docs/P4_1_TENANT_ENFORCEMENT_AUDIT.md',
  'docs/P4_1_TENANT_DATA_MIGRATION.md',
  'docs/P4_1_REPOSITORY_TENANT_ENFORCEMENT.md',
]) {
  if (exists(doc)) pass('doc-' + doc.split('/').pop(), doc);
  else fail('doc-' + doc.split('/').pop(), doc + ' missing');
}

// Migration
if (exists('backend/internal/database/migrate_p4_1.go')) pass('migrate-p4-1', 'P4.1 migration');
else fail('migrate-p4-1', 'migrate_p4_1.go missing');

const failed = checks.filter((c) => c.status === 'failed').length;
const report = {
  phase: 'P4.1',
  status: failed === 0 ? 'passed_with_real_environment_verification_deferred' : 'failed',
  tenantIsolation: {
    repositoryCoverage: checks.find((c) => c.id === 'repository-tenant-scope')?.status || 'unknown',
    workerCoverage: checks.find((c) => c.id === 'worker-tenant')?.status || 'unknown',
    idorTests: checks.find((c) => c.id === 'idor-tests')?.status || 'unknown',
    productionFallback: checks.find((c) => c.id === 'production-tenant-fallback')?.status === 'passed' ? 'disabled' : 'unknown',
  },
  keyRotation: {
    reencryptWorker: checks.find((c) => c.id === 'key-rotation-worker')?.status || 'unknown',
    verify: checks.find((c) => c.id === 'key-rotation-api')?.status || 'unknown',
  },
  fileSecurity: {
    scanner: checks.find((c) => c.id === 'file-scanner')?.status || 'unknown',
    stateMachine: checks.find((c) => c.id === 'file-state-machine')?.status || 'unknown',
    privateAccess: checks.find((c) => c.id === 'private-file-access')?.status || 'unknown',
  },
  ui: {
    securityCenter: checks.find((c) => c.id === 'security-center-ui')?.status || 'unknown',
    sessions: checks.find((c) => c.id === 'session-management-ui')?.status || 'unknown',
  },
  race: { status: 'deferred_on_windows' },
  realEnvironmentVerification: 'deferred',
  issues: checks.filter((c) => c.status === 'failed').map((c) => c.message),
};

const md = [
  '# P4.1 Security Closure Report',
  '',
  `**Status:** ${report.status}`,
  `**Generated:** ${new Date().toISOString()}`,
  '',
  '| Check | Status | Message |',
  '| --- | --- | --- |',
  ...checks.map((c) => `| ${c.id} | ${c.status} | ${c.message} |`),
  '',
  '## Deferred',
  '- Real environment security verification',
  '- Linux race tests (run on WSL2/CI)',
  '- Real Douyin credential E2E',
  '',
].join('\n');

fs.mkdirSync(path.join(root, 'docs'), { recursive: true });
fs.writeFileSync(path.join(root, 'docs', 'P4_1_SECURITY_CLOSURE_REPORT.md'), md);
fs.writeFileSync(path.join(root, 'docs', 'p4-1-security-closure-report.json'), JSON.stringify(report, null, 2));

console.log(md);
process.exit(failed > 0 ? 1 : 0);
