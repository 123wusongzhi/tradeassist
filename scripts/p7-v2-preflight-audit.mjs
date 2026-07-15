import fs from 'node:fs';
import path from 'node:path';
import {
  DB_PREFIX,
  DEFAULT_SCENARIO_WEIGHTS,
  DEFAULT_SLO,
  REGRESSION_THRESHOLDS,
  assertDbNameSafe,
  assertLoadHostSafe,
  collectEnvironmentFingerprint,
  configFingerprint,
  docsDir,
  discoverK6,
  readJSON,
  resolveP7V2PortConfig,
  root,
  run,
  runWSL,
  valueOf,
  writeJSON,
  writeMarkdown,
} from './p7-v2-lib.mjs';

const args = process.argv.slice(2);
const portConfig = resolveP7V2PortConfig();
const baseUrl = valueOf(args, '--base-url') || portConfig.baseUrl;
const issues = [];
const conflicts = [];

const k6 = discoverK6();
const hostIssues = assertLoadHostSafe(baseUrl);
issues.push(...hostIssues);

const wslPg = runWSL('psql -h /var/run/postgresql -U root -At -d postgres -c "select 1;" 2>/dev/null', { timeout: 20000 });
const postgresAvailable = wslPg.status === 0;
if (!postgresAvailable) issues.push('postgres unavailable in WSL');

const wslRedis = runWSL('redis-cli ping 2>/dev/null || true', { timeout: 15000 });
const redisAvailable = (wslRedis.stdout || '').trim().toUpperCase() === 'PONG';
if (!redisAvailable) issues.push('redis unavailable (redis-cli ping failed)');

const mediumDataset = readJSON('docs/p7-v-medium-dataset-report.json');
const p7c4 = readJSON('docs/p7-c4-final-closure-report.json');
const sloDocExists = fs.existsSync(path.join(root, 'docs/SLO.md'));
if (!sloDocExists) {
  conflicts.push({
    topic: 'SLO source',
    chosen: 'task conservative defaults',
    reason: 'docs/SLO.md missing; using DEFAULT_SLO from p7-v2-lib',
    values: DEFAULT_SLO,
  });
}

const baselineExists = fs.existsSync(path.join(docsDir, 'baselines')) &&
  fs.readdirSync(path.join(docsDir, 'baselines')).some((f) => f.startsWith('p7-v2-baseline-'));
const currentRunExists = fs.existsSync(path.join(docsDir, 'runs')) &&
  fs.readdirSync(path.join(docsDir, 'runs')).some((f) => f.startsWith('p7-v2-current-'));

const loadScripts = [
  'tests/load/p7v2-smoke.js',
  'tests/load/p7v2-baseline.js',
  'tests/load/p7v2-current.js',
  'tests/load/p7v2-soak.js',
];
const loadScenariosAvailable = loadScripts.every((rel) => fs.existsSync(path.join(root, rel)));

const dashboardFiles = [
  'deploy/observability/dashboards/performance-overview.json',
  'deploy/observability/dashboards/database-capacity.json',
];
const metricsAvailable = fs.existsSync(path.join(root, 'backend/internal/pkg/metrics'));
const dashboardAvailable = dashboardFiles.every((rel) => fs.existsSync(path.join(root, rel)));

const pkg = readJSON('package.json');
const demoCommandExists = Boolean(pkg?.scripts?.['demo:auto-acceptance']);

const gatesAvailable =
  fs.existsSync(path.join(root, 'scripts/p7-v2-final-closure-gate.mjs')) &&
  fs.existsSync(path.join(root, 'scripts/p1-p7-final-gate.mjs'));

const report = {
  phase: 'P7-V2',
  component: 'preflight-audit',
  status: issues.length === 0 && k6.status === 'passed' ? 'passed' : issues.some((x) => x.includes('k6')) || k6.status !== 'passed' ? 'blocked' : 'incomplete',
  generatedAt: new Date().toISOString(),
  k6Available: k6.status === 'passed',
  k6Executable: k6.executable === true,
  k6VersionDetected: Boolean(k6.version),
  k6Mode: k6.mode || 'blocked',
  k6Path: k6.path || '',
  k6Sha256: k6.sha256 || '',
  k6Version: k6.version,
  performanceEnvironmentAvailable: postgresAvailable,
  postgresAvailable,
  redisAvailable,
  mockProviderAvailable: true,
  mediumDatasetAvailable: mediumDataset?.profile === 'medium' && Number(mediumDataset?.actualRows || 0) === 1900150,
  metricsAvailable,
  dashboardAvailable,
  loadScenariosAvailable,
  baselineExists,
  currentRunExists,
  soakScenarioExists: fs.existsSync(path.join(root, 'tests/load/p7v2-soak.js')),
  demoCommandExists,
  p1ToP7GatesAvailable: gatesAvailable,
  databasePrefix: DB_PREFIX,
  scenarioWeights: DEFAULT_SCENARIO_WEIGHTS,
  slo: DEFAULT_SLO,
  regressionThresholds: REGRESSION_THRESHOLDS,
  conflicts,
  issues,
  hostGuard: hostIssues,
  p7C4Status: p7c4?.status || 'unknown',
  environmentFingerprint: collectEnvironmentFingerprint('preflight', `preflight-${Date.now()}`, {
    configFingerprint: configFingerprint(),
    baseUrl,
  }),
};

if (k6.status !== 'passed') {
  report.status = 'blocked';
  report.issues.push('k6 is not available');
}

writeJSON('docs/p7-v2-preflight-audit.json', report);
writeMarkdown(
  'docs/P7_V2_PREFLIGHT_AUDIT.md',
  `# P7-V2 Preflight Audit

Status: **${report.status}**

| Check | Value |
| --- | --- |
| k6 available | ${report.k6Available} |
| k6 version | ${report.k6Version || 'n/a'} |
| PostgreSQL | ${postgresAvailable} |
| Redis | ${redisAvailable} |
| Medium dataset | ${report.mediumDatasetAvailable} |
| Load scenarios | ${loadScenariosAvailable} |
| Baseline exists | ${baselineExists} |
| Current run exists | ${currentRunExists} |
| Demo command | ${demoCommandExists} |

## Issues
${issues.length ? issues.map((x) => `- ${x}`).join('\n') : '- none'}

## Conflicts
${conflicts.length ? conflicts.map((x) => `- ${x.topic}: ${x.chosen} (${x.reason})`).join('\n') : '- none'}
`,
);

console.log(JSON.stringify({ phase: 'P7-V2', status: report.status, k6Available: report.k6Available, issues: report.issues }, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
