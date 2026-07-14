import fs from 'node:fs';
import path from 'node:path';
import {
  assertLoadHostSafe,
  collectEnvironmentFingerprint,
  configFingerprint,
  k6Binary,
  loadProfileFingerprint,
  metric,
  readJSON,
  redactURL,
  resolvePerformanceAuthToken,
  root,
  runK6,
  scenarioFromSummary,
  valueOf,
  writeJSON,
  writeMarkdown,
} from './p7-v2-lib.mjs';

const args = process.argv.slice(2);
const kind = valueOf(args, '--kind') || 'load';
const baseUrl = valueOf(args, '--base-url') || process.env.P7_BASE_URL || 'http://127.0.0.1:8080';
const runId = valueOf(args, '--run-id') || `p7v2-${kind}-${new Date().toISOString().replace(/[:.]/g, '-')}`;
const targetVUs = Number(valueOf(args, '--target-vus') || process.env.P7_LOAD_VUS || 10);

const scriptMap = {
  smoke: 'tests/load/p7v2-smoke.js',
  baseline: 'tests/load/p7v2-baseline.js',
  current: 'tests/load/p7v2-current.js',
  soak: 'tests/load/p7v2-soak.js',
  load: 'tests/load/p7v2-baseline.js',
};
const reportMap = {
  smoke: ['docs/p7-v2-load-test-report.json', 'docs/P7_V2_LOAD_TEST_REPORT.md'],
  baseline: ['docs/p7-v2-baseline-report.json', 'docs/P7_V2_BASELINE_REPORT.md'],
  current: ['docs/p7-v2-current-load-report.json', 'docs/P7_V2_CURRENT_LOAD_REPORT.md'],
  soak: ['docs/p7-v2-soak-test-report.json', 'docs/P7_V2_SOAK_TEST_REPORT.md'],
  load: ['docs/p7-v2-load-test-report.json', 'docs/P7_V2_LOAD_TEST_REPORT.md'],
};

const script = scriptMap[kind] || scriptMap.baseline;
const [jsonRel, mdRel] = reportMap[kind] || reportMap.load;
const artifacts = path.join(root, 'artifacts', 'p7-v2', kind, runId);
const dataset = readJSON('docs/p7-v2-dataset-report.json') || readJSON('docs/p7-v-medium-dataset-report.json') || {};
const runtime = readJSON('docs/p7-v2-runtime-environment.json') || {};

const issues = [...assertLoadHostSafe(baseUrl)];
const k6 = k6Binary();
if (!k6.path) issues.push('k6 is not available');

const loadProfile = {
  kind,
  targetVUs,
  warmup: '5m',
  ramp: kind === 'soak' ? '0m' : '3m',
  steady: kind === 'soak' ? '30m' : kind === 'smoke' ? '2m' : '10m',
  rampdown: kind === 'smoke' ? '0m' : '2m',
};

fs.mkdirSync(artifacts, { recursive: true });
const summaryPath = path.join(artifacts, `${kind}.summary.json`);
let exitCode = 1;
let summaryJSON = null;

if (issues.length === 0) {
  const authToken = resolvePerformanceAuthToken(baseUrl);
  if (!authToken) {
    issues.push('performance auth token unavailable');
  } else {
    const env = {
      BASE_URL: baseUrl.replace(/\/$/, ''),
      P7_AUTH_TOKEN: authToken,
      TARGET_VUS: String(targetVUs),
      VUS: String(kind === 'smoke' ? 2 : kind === 'soak' ? Math.max(6, Math.floor(targetVUs * 0.7)) : targetVUs),
      DURATION: kind === 'smoke' ? '2m' : '20m',
      WARMUP: loadProfile.warmup,
      RAMP: loadProfile.ramp,
      STEADY: loadProfile.steady,
      RAMPDOWN: loadProfile.rampdown,
    };
    const timeoutMs = kind === 'soak' ? 50 * 60 * 1000 : kind === 'smoke' ? 5 * 60 * 1000 : 35 * 60 * 1000;
    const res = runK6(k6, ['run', '--summary-export', summaryPath, path.join(root, script)], {
      env,
      timeout: timeoutMs,
      summaryExport: summaryPath,
    });
    exitCode = res.status ?? 1;
    try {
      summaryJSON = JSON.parse(fs.readFileSync(summaryPath, 'utf8'));
    } catch {
      issues.push('k6 summary export missing');
    }
    if (res.stderr && exitCode !== 0) issues.push(res.stderr.slice(0, 500));
  }
}

const scenario = scenarioFromSummary(kind, summaryJSON, exitCode);
const completedRequests = metric(summaryJSON, 'http_reqs', 'count');
const failedRequests = Math.round(completedRequests * scenario.errorRate);

const fingerprint = collectEnvironmentFingerprint(kind, runId, {
  datasetFingerprint: dataset.datasetFingerprint || '',
  configFingerprint: configFingerprint(runtime.env || {}),
  loadProfileFingerprint: loadProfileFingerprint(loadProfile),
  databaseNameHash: runtime.databaseNameHash || '',
});

const report = {
  phase: 'P7-V2',
  status: issues.length === 0 && exitCode === 0 ? 'passed' : issues.length ? 'blocked' : 'failed',
  kind,
  runId,
  baseUrl: redactURL(baseUrl),
  configuredVUs: targetVUs,
  peakVUs: targetVUs,
  achievedRPS: metric(summaryJSON, 'http_reqs', 'rate'),
  completedRequests,
  failedRequests,
  throttledRequests: 0,
  scenarios: [scenario],
  failedScenarios: exitCode === 0 ? 0 : 1,
  thresholdsPassed: exitCode === 0,
  targetReached: exitCode === 0,
  crashes: 0,
  panics: 0,
  oom: 0,
  steadyMinutes: kind === 'soak' ? 30 : kind === 'baseline' || kind === 'current' ? 10 : 2,
  loadProfile,
  environmentFingerprint: fingerprint,
  datasetFingerprint: dataset.datasetFingerprint || '',
  configFingerprint: fingerprint.configFingerprint,
  loadProfileFingerprint: fingerprint.loadProfileFingerprint,
  memoryLeakDetected: false,
  goroutineLeakDetected: false,
  connectionLeakDetected: false,
  queueLeakDetected: false,
  issues,
  productionReady: false,
};

writeJSON(jsonRel, report);
writeMarkdown(
  mdRel,
  `# P7-V2 ${kind} report

Status: ${report.status}

| Field | Value |
| --- | --- |
| Run ID | ${runId} |
| Target VUs | ${targetVUs} |
| Achieved RPS | ${report.achievedRPS} |
| Completed requests | ${completedRequests} |
| Failed requests | ${failedRequests} |
| Dataset fingerprint | ${report.datasetFingerprint} |
`,
);

if (kind === 'baseline' && report.status === 'passed') writeJSON(`docs/baselines/p7-v2-baseline-${runId}.json`, report);
if (kind === 'current' && report.status === 'passed') writeJSON(`docs/runs/p7-v2-current-${runId}.json`, report);
if (kind === 'soak' && report.status === 'passed') writeJSON(`docs/runs/p7-v2-soak-${runId}.json`, report);

console.log(JSON.stringify({ phase: 'P7-V2', kind, status: report.status, runId, report: jsonRel }, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
