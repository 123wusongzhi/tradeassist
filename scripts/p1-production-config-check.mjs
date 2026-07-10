#!/usr/bin/env node
/**
 * Phase P1 production config static scan.
 * Output: docs/P1_PRODUCTION_CONFIG_REPORT.md + docs/p1-production-config-report.json
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

function add(id, status, message, detail) {
  checks.push({ id, status, message, detail });
}

function pass(id, message, detail) {
  add(id, 'passed', message, detail);
}
function fail(id, message, detail) {
  add(id, 'failed', message, detail);
}
function warn(id, message, detail) {
  add(id, 'warning', message, detail);
}

// Status copy
for (const f of ['README.md', 'docs/PROGRESS.md']) {
  const t = read(f);
  if (t.includes('Ready for demo tag')) fail('status-no-demo-tag', `${f} must not say Ready for demo tag`);
  else pass('status-no-demo-tag', `${f} ok`);
  if (!t.includes('Tag deferred')) warn('status-tag-deferred', `${f} missing Tag deferred`);
  if (!t.includes('非 Production Ready') && !t.includes('非 Production Ready')) warn('status-not-prod', `${f} missing 非 Production Ready`);
  if (t.includes('Production Capability Development') || t.includes('Phase P1')) pass('status-p1', `${f} reflects P1`);
  else warn('status-p1', `${f} may need P1 status line`);
}

// Production demo seed guard
const router = read('backend/internal/api/router.go');
if (router.includes('EnableDemoSeed') && router.includes('IsProduction')) pass('demo-seed-guard', 'Demo seed gated by EnableDemoSeed + !production');
else fail('demo-seed-guard', 'Demo seed production guard missing');

const validate = read('backend/internal/config/validate.go');
if (validate.includes('ENABLE_DEMO_SEED')) pass('config-demo-seed', 'production validates ENABLE_DEMO_SEED');
else fail('config-demo-seed', 'production ENABLE_DEMO_SEED validation missing');

if (validate.includes('CONFIG_INSECURE_DEFAULT')) pass('config-insecure-default', 'insecure default error code present');
else fail('config-insecure-default', 'missing CONFIG_INSECURE_DEFAULT');

if (validate.includes('validateStorageProvider')) pass('storage-validate-failfast', 'STORAGE_PROVIDER fail-fast in Validate()');
else fail('storage-validate-failfast', 'validateStorageProvider missing in validate.go');

// Storage local production
const prodStatus = read('backend/internal/modules/configstatus/production_status.go');
if (prodStatus.includes('AllowsLocalStorage')) pass('storage-local-boundary', 'local storage production boundary');
else fail('storage-local-boundary', 'missing local storage boundary');

const pubValidate = read('backend/internal/pkg/storagepublic/validate.go');
if (pubValidate.includes('ValidatePublicBase')) pass('public-base-validate', 'ValidatePublicBase exists');
else fail('public-base-validate', 'ValidatePublicBase missing');

// Deploy assets
for (const f of [
  'deploy/nginx/trademind.conf',
  'deploy/systemd/trademind-api.service',
  'deploy/scripts/check-readiness.sh',
  '.env.production.example',
]) {
  if (fs.existsSync(path.join(root, f))) pass('deploy-' + f, f + ' exists');
  else fail('deploy-' + f, f + ' missing');
}

const nginx = read('deploy/nginx/trademind.conf');
if (nginx.includes('try_files') && nginx.includes('/index.html')) pass('nginx-fallback', 'Admin history fallback present');
else fail('nginx-fallback', 'nginx fallback missing');

// Health probes
if (read('backend/internal/health/health.go').includes('/health/live')) pass('health-live', 'liveness route');
else fail('health-live', 'liveness missing');

// gitignore
const gi = read('.gitignore');
if (gi.includes('.env.production') && gi.includes('!.env.production.example')) pass('gitignore-env', '.gitignore covers production env');
else warn('gitignore-env', '.gitignore env patterns incomplete');

// Admin source map note
const umirc = read('admin/.umirc.ts');
if (umirc.includes('devtool') || umirc.includes('sourceMap')) pass('admin-sourcemap', 'admin build source map policy set');
else warn('admin-sourcemap', 'admin .umirc.ts should set production source map policy');

const failed = checks.filter((c) => c.status === 'failed').length;
const warnings = checks.filter((c) => c.status === 'warning').length;
const passed = checks.filter((c) => c.status === 'passed').length;
const overall = failed === 0 ? (warnings ? 'passed_with_warning' : 'passed') : 'failed';

const report = {
  generatedAt: new Date().toISOString(),
  phase: 'P1',
  overall,
  summary: { passed, warnings, failed, total: checks.length },
  checks,
};

const jsonPath = path.join(root, 'docs/p1-production-config-report.json');
const mdPath = path.join(root, 'docs/P1_PRODUCTION_CONFIG_REPORT.md');
fs.writeFileSync(jsonPath, JSON.stringify(report, null, 2));

const md = [
  '# P1 Production Config Scan Report',
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

console.log(`P1 scan: ${overall} — ${jsonPath}`);
process.exit(failed > 0 ? 1 : 0);
