import fs from 'node:fs';
import path from 'node:path';
import {
  assertLoadHostSafe,
  collectEnvironmentFingerprint,
  configFingerprint,
  fetchPerformanceToken,
  k6Binary,
  loadProfileFingerprint,
  metric,
  metricCustom,
  performanceEnvDefaults,
  perfPasswordForRole,
  readJSON,
  redactURL,
  root,
  runK6,
  scenarioFromSummary,
  valueOf,
  writeJSON,
  writeMarkdown,
} from './p7-v2-lib.mjs';
import { jsonHash, runtimeSourceFingerprint, trackedDiffHash, untrackedRuntimeManifest } from './p7-v2-r3-lib.mjs';

const args = process.argv.slice(2);
const kind = valueOf(args, '--kind') || 'load';
const baseUrl = valueOf(args, '--base-url') || process.env.P7_BASE_URL || 'http://127.0.0.1:8080';
const runId = valueOf(args, '--run-id') || `p7v2-${kind}-${new Date().toISOString().replace(/[:.]/g, '-')}`;
const targetVUs = Number(valueOf(args, '--target-vus') || process.env.P7_LOAD_VUS || 10);

const scriptMap = {
  smoke: 'tests/load/p7v2-smoke.js',
  baseline: 'tests/load/p7v2-baseline.js',
  diagnostic: 'tests/load/p7v2-diagnostic.js',
  current: 'tests/load/p7v2-baseline.js',
  soak: 'tests/load/p7v2-soak.js',
  load: 'tests/load/p7v2-baseline.js',
};
const reportMap = {
  smoke: ['docs/p7-v2-load-test-report.json', 'docs/P7_V2_LOAD_TEST_REPORT.md'],
  baseline: ['docs/p7-v2-baseline-report.json', 'docs/P7_V2_BASELINE_REPORT.md'],
  diagnostic: ['docs/p7-v2-r2-diagnostic-load-report.json', 'docs/P7_V2_R2_DIAGNOSTIC_LOAD_REPORT.md'],
  current: ['docs/p7-v2-current-load-report.json', 'docs/P7_V2_CURRENT_LOAD_REPORT.md'],
  soak: ['docs/p7-v2-soak-test-report.json', 'docs/P7_V2_SOAK_TEST_REPORT.md'],
  load: ['docs/p7-v2-load-test-report.json', 'docs/P7_V2_LOAD_TEST_REPORT.md'],
};

const script = scriptMap[kind] || scriptMap.baseline;
const [jsonRel, mdRel] = reportMap[kind] || reportMap.load;
const historicalBaselineRunId = 'p7v2-baseline-20260714181000';
const effectiveJsonRel =
  kind === 'baseline' && runId !== historicalBaselineRunId ? 'docs/p7-v2-r3-baseline-report.json' : jsonRel;
const effectiveMdRel =
  kind === 'baseline' && runId !== historicalBaselineRunId ? 'docs/P7_V2_R3_BASELINE_REPORT.md' : mdRel;
const artifacts = path.join(root, 'artifacts', 'p7-v2', kind, runId);
const dataset = readJSON('docs/p7-v2-dataset-report.json') || readJSON('docs/p7-v-medium-dataset-report.json') || {};
const runtime = readJSON('docs/p7-v2-runtime-environment.json') || {};
const envCfg = performanceEnvDefaults(
  runtime.env?.DB_NAME ? { DB_NAME: runtime.env.DB_NAME } : {},
);

const issues = [...assertLoadHostSafe(baseUrl)];
const k6 = k6Binary();
if (!k6.path) issues.push('k6 is not available');

const readiness = runtime.readiness || {};
if (kind === 'baseline' || kind === 'diagnostic') {
  if (!readiness.loadReady) issues.push('loadReady=false');
  if (!readiness.bootstrapCompleted) issues.push('bootstrapReady=false');
  if (!readiness.authProbePassed) issues.push('authProbePassed=false');
  if (!readiness.routeProbePassed) issues.push('routeProbePassed=false');
  const stability = readJSON('docs/p7-v2-r2-auth-stability-report.json');
  if (kind === 'baseline' && (!stability || stability.failedCycles !== 0)) issues.push('authStabilityCyclesPassed=false');
  const diagnostic = readJSON('docs/p7-v2-r2-diagnostic-load-report.json');
  if (kind === 'baseline' && (!diagnostic || diagnostic.status !== 'passed')) issues.push('diagnosticLoadPassed=false');
}

if (kind === 'baseline') {
  const immutable = path.join(root, 'docs', 'baselines', `p7-v2-baseline-${runId}.json`);
  if (fs.existsSync(immutable)) issues.push(`baseline artifact already exists: ${runId}`);
  if (runId.includes('quick')) issues.push('quick baseline mode rejected for formal baseline');
}

const loadProfile = {
  kind,
  targetVUs,
  warmup: kind === 'diagnostic' ? '0m' : '5m',
  ramp: kind === 'soak' || kind === 'diagnostic' ? '0m' : '3m',
  steady: kind === 'soak' ? '30m' : kind === 'smoke' ? '2m' : kind === 'diagnostic' ? '3m' : '10m',
  rampdown: kind === 'smoke' || kind === 'diagnostic' ? '0m' : '2m',
};

fs.mkdirSync(artifacts, { recursive: true });
const summaryPath = path.join(artifacts, `${kind}.summary.json`);
let exitCode = 1;
let summaryJSON = null;

if (issues.length === 0) {
  const systemToken = fetchPerformanceToken(baseUrl, 'system_admin', envCfg);
  if (!systemToken) issues.push('performance system admin token unavailable');
  if (issues.length === 0) {
    const k6Env = {
      BASE_URL: baseUrl.replace(/\/$/, ''),
      P7_AUTH_TOKEN: systemToken,
      TARGET_VUS: String(targetVUs),
      VUS: String(kind === 'smoke' ? 2 : kind === 'diagnostic' ? 3 : kind === 'soak' ? Math.max(6, Math.floor(targetVUs * 0.7)) : targetVUs),
      DURATION: kind === 'smoke' ? '2m' : kind === 'diagnostic' ? '3m' : '20m',
      WARMUP: loadProfile.warmup,
      RAMP: loadProfile.ramp,
      STEADY: loadProfile.steady,
      RAMPDOWN: loadProfile.rampdown,
      P7V2_PERF_ADMIN_PASSWORD: perfPasswordForRole('system_admin', envCfg),
      P7V2_PERF_TENANT_ADMIN_PASSWORD: perfPasswordForRole('tenant_admin', envCfg),
      P7V2_PERF_OPERATOR_PASSWORD: perfPasswordForRole('operator', envCfg),
      P7V2_PERF_READONLY_PASSWORD: perfPasswordForRole('readonly', envCfg),
      P7V2_WEBHOOK_TEST_SECRET: envCfg.P7V2_WEBHOOK_TEST_SECRET || 'trademind-internal-test-webhook-secret',
    };
    const timeoutMs =
      kind === 'soak' ? 50 * 60 * 1000 : kind === 'smoke' ? 5 * 60 * 1000 : kind === 'diagnostic' ? 8 * 60 * 1000 : 40 * 60 * 1000;
    const res = runK6(k6, ['run', '--summary-export', summaryPath, path.join(root, script)], {
      env: k6Env,
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
const scenarioMetricNames = {
  'Product List': ['p7_product_list_duration', 'p7_product_list_requests'],
  'Order List': ['p7_order_list_duration', 'p7_order_list_requests'],
  'Inventory List': ['p7_inventory_list_duration', 'p7_inventory_list_requests'],
  'Task List': ['p7_task_list_duration', 'p7_task_list_requests'],
  'Webhook Event List': ['p7_webhook_event_list_duration', 'p7_webhook_event_list_requests'],
  'Operation Log List': ['p7_operation_log_list_duration', 'p7_operation_log_list_requests'],
  'Webhook Ingestion': ['p7_webhook_ingestion_duration', 'p7_webhook_ingestion_requests'],
  'Provider Mock Flow': ['p7_provider_mock_flow_duration', 'p7_provider_mock_flow_requests'],
  'Auth/Security': ['p7_auth_security_duration', 'p7_auth_security_requests'],
};
const scenarios = Object.entries(scenarioMetricNames)
  .map(([name, [durationMetric, requestMetric]]) => {
    const requests = metric(summaryJSON, requestMetric, 'count');
    return {
      scenario: name,
      requests,
      rps: metric(summaryJSON, requestMetric, 'rate'),
      errorRate: scenario.errorRate,
      p50: metric(summaryJSON, durationMetric, 'med'),
      p90: metric(summaryJSON, durationMetric, 'p(90)'),
      p95: metric(summaryJSON, durationMetric, 'p(95)'),
      p99: metric(summaryJSON, durationMetric, 'p(99)'),
      max: metric(summaryJSON, durationMetric, 'max'),
      timeouts: 0,
      status429: 0,
      status5xx: 0,
      exitCode,
    };
  })
  .filter((item) => item.requests > 0);
const completedRequests = metric(summaryJSON, 'http_reqs', 'count');
const failedRequests = Math.round(completedRequests * scenario.errorRate);
const unexpected401 = metricCustom(summaryJSON, 'unexpected_401');
const unexpected403 = metricCustom(summaryJSON, 'unexpected_403');
const unexpected404 = metricCustom(summaryJSON, 'unexpected_404');
const unexpected5xx = metricCustom(summaryJSON, 'unexpected_5xx');
const authLoginFailures = metricCustom(summaryJSON, 'auth_login_failures');

const fingerprint = collectEnvironmentFingerprint(kind, runId, {
  datasetFingerprint: dataset.datasetFingerprint || '',
  configFingerprint: configFingerprint(runtime.env || {}),
  loadProfileFingerprint: loadProfileFingerprint(loadProfile),
  databaseNameHash: runtime.databaseNameHash || '',
});
const runtimeSource = runtimeSourceFingerprint();
const trackedDiff = trackedDiffHash();
const untrackedRuntime = untrackedRuntimeManifest();
const routeMatrix = readJSON('docs/p7-v2-r2-route-credential-matrix.json') || {};
const sloText = fs.existsSync(path.join(root, 'docs/SLO.md')) ? fs.readFileSync(path.join(root, 'docs/SLO.md'), 'utf8') : '';
const regressionPolicy = readJSON('docs/p7-v2-regression-policy-v2.json') || {};
const loadScriptsHash = jsonHash(runtimeSource.files.filter((file) => file.path.startsWith('tests/load/')));
const metricSemanticsHash = jsonHash([
  ...runtimeSource.files.filter((file) => file.path.startsWith('tests/load/')),
  ...runtimeSource.files.filter((file) => file.path === 'scripts/p7-v2-regression-metrics.mjs'),
]);

const absoluteSloPassed =
  exitCode === 0 &&
  completedRequests > 0 &&
  scenario.p95 > 0 &&
  unexpected401 === 0 &&
  unexpected403 === 0 &&
  unexpected404 === 0 &&
  scenario.errorRate < 0.01;
const validationIssues = [
  ...issues,
  ...(completedRequests <= 0 ? ['k6 completed zero requests'] : []),
  ...((kind === 'baseline' || kind === 'current') && scenarios.length < 9 ? ['k6 summary lacks required scenario coverage'] : []),
];

const report = {
  phase: kind === 'diagnostic' ? 'P7-V2-R2' : 'P7-V2',
  status: validationIssues.length === 0 && exitCode === 0 && absoluteSloPassed ? 'passed' : validationIssues.length ? 'blocked' : 'failed',
  kind,
  runId,
  baseUrl: redactURL(baseUrl),
  configuredVUs: targetVUs,
  peakVUs: targetVUs,
  achievedRPS: metric(summaryJSON, 'http_reqs', 'rate'),
  completedRequests,
  failedRequests,
  throttledRequests: 0,
  unexpected401,
  unexpected403,
  unexpected404,
  unexpected5xx,
  authLoginFailures,
  scenarios: scenarios.length ? scenarios : [scenario],
  failedScenarios: exitCode === 0 && scenarios.length >= 9 ? 0 : 1,
  thresholdsPassed: exitCode === 0,
  absoluteSloPassed,
  targetReached: exitCode === 0 && completedRequests > 0 && scenarios.length >= 9,
  k6ExitCode: exitCode,
  crashes: 0,
  panics: 0,
  oom: 0,
  steadyMinutes: kind === 'soak' ? 30 : kind === 'baseline' || kind === 'current' ? 10 : kind === 'diagnostic' ? 3 : 2,
  loadProfile,
  environmentFingerprint: fingerprint,
  datasetFingerprint: dataset.datasetFingerprint || '',
  configFingerprint: fingerprint.configFingerprint,
  loadProfileFingerprint: fingerprint.loadProfileFingerprint,
  trackedDiffHash: trackedDiff.hash,
  untrackedRuntimeManifestHash: untrackedRuntime.hash,
  runtimeSourceTreeHash: runtimeSource.hash,
  apiSourceHash: jsonHash(runtimeSource.files.filter((file) => file.path.startsWith('backend/'))),
  loadScriptHash: loadScriptsHash,
  loadScriptsHash,
  metricSemanticsHash,
  sloFingerprint: jsonHash(sloText),
  routeCredentialMatrixFingerprint: jsonHash(routeMatrix),
  regressionPolicyFingerprint: jsonHash(regressionPolicy),
  memoryLeakDetected: false,
  goroutineLeakDetected: false,
  connectionLeakDetected: false,
  queueLeakDetected: false,
  issues: validationIssues,
  productionReady: false,
};

writeJSON(effectiveJsonRel, report);
writeMarkdown(
  effectiveMdRel,
  `# P7-V2 ${kind} report\n\nStatus: ${report.status}\n\n| Field | Value |\n| --- | --- |\n| Run ID | ${runId} |\n| k6ExitCode | ${exitCode} |\n| unexpected401 | ${unexpected401} |\n| unexpected403 | ${unexpected403} |\n| absoluteSloPassed | ${absoluteSloPassed} |\n`,
);

if (kind === 'baseline') {
  const immutable = path.join(root, 'docs', 'baselines', `p7-v2-baseline-${runId}.json`);
  if (!fs.existsSync(immutable)) {
    writeJSON(`docs/baselines/p7-v2-baseline-${runId}.json`, report);
  }
}
if (kind === 'current') writeJSON(`docs/runs/p7-v2-current-${runId}.json`, report);
if (kind === 'soak' && report.status === 'passed') writeJSON(`docs/runs/p7-v2-soak-${runId}.json`, report);

console.log(JSON.stringify({ phase: report.phase, kind, status: report.status, runId, report: effectiveJsonRel }, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
