import fs from 'node:fs';
import path from 'node:path';

const repoRoot = process.cwd();
const checks = [];

function rel(p) {
  return path.join(repoRoot, p);
}

function read(p) {
  try {
    return fs.readFileSync(rel(p), 'utf8');
  } catch {
    return '';
  }
}

function exists(p) {
  return fs.existsSync(rel(p));
}

function add(id, ok, message) {
  checks.push({ id, status: ok ? 'passed' : 'failed', message });
  console.log(`[${ok ? 'PASS' : 'FAIL'}] ${id} - ${message}`);
}

const demoAuto = read('scripts/demo-auto-acceptance.ps1');
const seedData = read('scripts/seed-demo-data.ps1');
const seedPerm = read('scripts/seed-demo-permissions.ps1');
const aiImageTrial = read('scripts/ai-image-trial-run.ps1');

add('go-test-isolated-env', exists('scripts/lib/clean-test-env.mjs') && demoAuto.includes('go-test-isolated.mjs'), 'demo:auto-acceptance runs go test through isolated env');
add('go-test-no-root-dotenv', !demoAuto.includes('Import-DotEnv (Join-Path $repoRoot ".env")'), 'demo:auto-acceptance does not import root .env before go test');
add('seed-data-version', seedData.includes('DEMO_DATASET_VERSION') && seedData.includes('p4-r-v1'), 'demo data seed declares P4-R dataset version');
add('seed-data-exit-json', seedData.includes('Write-SeedResult') && seedData.includes('environment_blocked'), 'demo data seed has structured exit result');
add('seed-permission-version', seedPerm.includes('DEMO_PERMISSION_TEMPLATE_VERSION') && seedPerm.includes('p4-r-v1'), 'permission seed declares P4-R template version');
add('seed-permission-exit-json', seedPerm.includes('Write-PermissionSeedResult') && seedPerm.includes('manual_action_required'), 'permission seed has structured exit result');
add('verify-demo-data', exists('scripts/verify-demo-data.mjs'), 'demo data verifier script exists');
add('verify-demo-permissions', exists('scripts/verify-demo-permissions.mjs'), 'demo permissions verifier script exists');
add('production-demo-seed-forbidden', seedData.includes('DEMO_SEED_FORBIDDEN_IN_PRODUCTION') && seedPerm.includes('DEMO_SEED_FORBIDDEN_IN_PRODUCTION'), 'seed scripts forbid production/staging by default');
add('ai-image-timeout-classification', aiImageTrial.includes('Classify-TrialError') && aiImageTrial.includes('lastCompletedStage'), 'AI image trial records timeout/error classification and progress stage');
add('acceptance-result-model', demoAuto.includes('codeFailed') && demoAuto.includes('nonAiFailed') && demoAuto.includes('deferred'), 'acceptance report distinguishes failed/blocked/deferred/code failed');

for (const p of [
  'docs/P4_R_DEMO_REGRESSION_AUDIT.md',
  'docs/P4_R_DEMO_SEED_ROOT_CAUSE.md',
  'docs/P4_R_PERMISSION_SEED_ROOT_CAUSE.md',
  'docs/P4_R_DEMO_DATA_VERSIONING.md',
  'docs/P4_R_PERMISSION_TEMPLATE_VERSIONING.md',
  'docs/P4_R_AI_IMAGE_TIMEOUT_ANALYSIS.md',
  'docs/P4_R_ACCEPTANCE_STEP_CLASSIFICATION.md',
]) {
  add(`doc-${path.basename(p)}`, exists(p), `${p} exists`);
}

const failed = checks.filter((c) => c.status === 'failed').length;
const report = {
  generatedAt: new Date().toISOString(),
  phase: 'P4-R',
  summary: {
    total: checks.length,
    passed: checks.length - failed,
    failed,
  },
  checks,
};

fs.writeFileSync(rel('docs/p4-r-demo-regression-report.json'), `${JSON.stringify(report, null, 2)}\n`);

const md = [
  '# Phase P4-R Demo Regression Closure Check',
  '',
  `> Generated: ${report.generatedAt}`,
  '',
  `Result: **${failed === 0 ? 'passed' : 'failed'}**`,
  '',
  '| Check | Status | Message |',
  '| --- | --- | --- |',
  ...checks.map((c) => `| ${c.id} | ${c.status} | ${c.message} |`),
  '',
  'Real Douyin credential E2E, real environment security verification, production gray release, tag creation, and Production Ready marking remain deferred.',
  '',
].join('\n');
fs.writeFileSync(rel('docs/P4_R_DEMO_REGRESSION_REPORT.md'), md);

process.exit(failed > 0 ? 1 : 0);
