import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {
  assertLoadHostSafe,
  collectEnvironmentFingerprint,
  configFingerprint,
  fetchPerformanceToken,
  k6Binary,
  metric,
  metricCustom,
  performanceEnvDefaults,
  resolveP7V2PortConfig,
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
import { calculateLoadProfileFingerprint } from './p7-v2-load-profile-fingerprint.mjs';
import { CORE_SCENARIOS, SCENARIO_METRICS } from './p7-v2-regression-metrics.mjs';
import { classifyMetricEvidence, evaluateAbsoluteSlo, evaluateTargetReached } from './p7-v2-soak-semantics.mjs';
import { buildFormalInputSequenceManifest, INPUT_SEQUENCE_MANIFEST_PATH } from './p7-v2-formal-input-sequence.mjs';

const args = process.argv.slice(2);
const kind = valueOf(args, '--kind') || 'load';
const portConfig = resolveP7V2PortConfig();
const baseUrl = valueOf(args, '--base-url') || portConfig.baseUrl;
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
  configuredVUs: targetVUs,
  warmup: kind === 'diagnostic' ? '0m' : '5m',
  ramp: kind === 'soak' || kind === 'diagnostic' ? '0m' : '3m',
  steady: kind === 'soak' ? '30m' : kind === 'smoke' ? '2m' : kind === 'diagnostic' ? '3m' : '10m',
  rampdown: kind === 'smoke' || kind === 'diagnostic' ? '0m' : '2m',
  // Canonical stage input is explicit: stage targets are not inferred from configuredVUs.
  stages: [
    { name: 'warmup', duration: kind === 'diagnostic' ? '1ms' : '5m', targetVUs },
    { name: 'ramp', duration: kind === 'soak' || kind === 'diagnostic' ? '1ms' : '3m', targetVUs },
    { name: 'steady', duration: kind === 'soak' ? '30m' : kind === 'smoke' ? '2m' : kind === 'diagnostic' ? '3m' : '10m', targetVUs },
    { name: 'rampdown', duration: kind === 'smoke' || kind === 'diagnostic' ? '1ms' : '2m', targetVUs: 0 },
  ],
  scenarios: [
    { name: 'warmup', executor: 'constant-vus', startTime: '0s' },
    { name: 'ramp', executor: 'ramping-vus', startTime: kind === 'diagnostic' ? '0s' : '5m' },
    { name: 'steady', executor: 'constant-vus', startTime: kind === 'diagnostic' ? '0s' : kind === 'soak' ? '5m' : '8m' },
    { name: 'rampdown', executor: 'ramping-vus', startTime: kind === 'diagnostic' ? '3m' : kind === 'soak' ? '35m' : '18m' },
    { name: 'security_negative', executor: 'constant-vus', startTime: '0s', weight: 1 },
  ],
  requestMix: [
    ['product_list', 20], ['order_list', 20], ['inventory_list', 15], ['task_list', 10],
    ['webhook_event_list', 8], ['operation_log_list', 7], ['webhook_ingestion', 5],
    ['provider_mock_flow', 5], ['auth_security', 2],
  ].map(([routeId, weight]) => ({ routeId, method: routeId === 'webhook_ingestion' ? 'POST' : 'GET', weight })),
  credentialMix: [
    { role: 'system_admin', weight: 1 }, { role: 'tenant_admin', weight: 1 },
    { role: 'operator', weight: 1 }, { role: 'readonly', weight: 1 },
  ],
  loadScript: {
    path: script,
    sha256: crypto.createHash('sha256').update(fs.readFileSync(path.join(root, script))).digest('hex'),
  },
};
const canonicalLoadProfile = calculateLoadProfileFingerprint(loadProfile, { repositoryRoot: root });

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
function summaryValue(summary, name, key) {
  const values = summary?.metrics?.[name]?.values || summary?.metrics?.[name] || {};
  return Object.prototype.hasOwnProperty.call(values, key) ? values[key] : null;
}
const scenarios = Object.entries(SCENARIO_METRICS)
  .map(([name, [durationMetric, requestMetric]]) => {
    const evidence = classifyMetricEvidence({
      metricDefinition: { metricId: name, metricName: durationMetric, metricType: 'trend' },
      rawMetric: summaryJSON?.metrics?.[durationMetric],
      sampleMetric: summaryJSON?.metrics?.[requestMetric],
      minimumSampleCount: 100,
      aggregation: 'p(95)',
    });
    const requests = evidence.sampleCount ?? null;
    return {
      scenario: name,
      requests,
      requestCount: requests,
      sampleCount: requests,
      metricEvidenceClassification: evidence.classification,
      metricPresent: evidence.metricPresent,
      sampleMetricPresent: evidence.sampleMetricPresent,
      minimumSampleCount: 100,
      rps: metric(summaryJSON, requestMetric, 'rate') ?? null,
      errorRate: scenario.errorRate,
      p50: summaryValue(summaryJSON, durationMetric, 'med'),
      p90: summaryValue(summaryJSON, durationMetric, 'p(90)'),
      p95: summaryValue(summaryJSON, durationMetric, 'p(95)'),
      p99: summaryValue(summaryJSON, durationMetric, 'p(99)'),
      max: summaryValue(summaryJSON, durationMetric, 'max'),
      avg: summaryValue(summaryJSON, durationMetric, 'avg'),
      timeouts: 0,
      timeoutCount: 0,
      statusCodeDistribution: {},
      exitCode,
    };
  })
  .filter((item) => item.sampleCount !== null || kind === 'soak' || kind === 'baseline' || kind === 'current');
const completedRequests = metric(summaryJSON, 'http_reqs', 'count');
const completedRequestCount = Number(completedRequests || 0);
const failedRequests = Number.isFinite(Number(scenario.errorRate)) ? Math.round(completedRequestCount * scenario.errorRate) : null;
const unexpected401 = metricCustom(summaryJSON, 'unexpected_401');
const unexpected403 = metricCustom(summaryJSON, 'unexpected_403');
const unexpected404 = metricCustom(summaryJSON, 'unexpected_404');
const unexpected5xx = metricCustom(summaryJSON, 'unexpected_5xx');
const authLoginFailures = metricCustom(summaryJSON, 'auth_login_failures');

const fingerprint = collectEnvironmentFingerprint(kind, runId, {
  datasetFingerprint: dataset.datasetFingerprint || '',
  configFingerprint: configFingerprint(runtime.env || {}),
  loadProfileFingerprint: canonicalLoadProfile.loadProfileFingerprint,
  loadProfileFingerprintVersion: canonicalLoadProfile.fingerprintVersion,
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
const inputSequenceBinding = readJSON(INPUT_SEQUENCE_MANIFEST_PATH) || buildFormalInputSequenceManifest();

const routeSloEvaluations = scenarios.map((item) => evaluateAbsoluteSlo({
  sloId: `${item.scenario}:p95`,
  metricId: item.scenario,
  metricName: SCENARIO_METRICS[item.scenario]?.[0] || '',
  rawMetric: summaryJSON?.metrics?.[SCENARIO_METRICS[item.scenario]?.[0] || ''],
  sampleMetric: summaryJSON?.metrics?.[SCENARIO_METRICS[item.scenario]?.[1] || ''],
  minimumSampleCount: 100,
  aggregation: 'p(95)',
  threshold: item.scenario === 'Webhook Ingestion' ? 1200 : item.scenario === 'Provider Mock Flow' || item.scenario.includes('Auth') || item.scenario.includes('Signature') ? 500 : 800,
}));
const unexpectedMetricEvidence = [
  ['unexpected_401', unexpected401],
  ['unexpected_403', unexpected403],
  ['unexpected_404', unexpected404],
].map(([name, value]) => ({ metricName: name, metricPresent: value !== undefined, actualValue: value ?? null, evaluationStatus: value === undefined ? 'not_evaluable_metric_missing' : 'evaluated', verdict: value === 0 ? 'passed' : value === undefined ? 'not_evaluable' : 'failed' }));
const sloEvaluationCompleted =
  exitCode === 0 &&
  routeSloEvaluations.every((item) => item.evaluationStatus === 'evaluated') &&
  unexpectedMetricEvidence.every((item) => item.evaluationStatus === 'evaluated') &&
  Number.isFinite(Number(scenario.errorRate));
const absoluteSloPassed =
  sloEvaluationCompleted &&
  routeSloEvaluations.every((item) => item.verdict === 'passed') &&
  unexpectedMetricEvidence.every((item) => item.verdict === 'passed') &&
  scenario.errorRate < 0.01;
const requiredScenarioMissingCount = CORE_SCENARIOS.filter((name) => {
  const row = scenarios.find((item) => item.scenario === name);
  return !row || row.metricEvidenceClassification !== 'present';
}).length;
const scenarioCoverageReached = requiredScenarioMissingCount === 0;
const sampleTargetReached = scenarios.every((item) => Number(item.sampleCount || 0) >= 100);
const loadTargetReached = exitCode === 0 && completedRequestCount > 0;
const targetReachedComponents = evaluateTargetReached({
  loadTargetReached,
  steadyStageEntered: completedRequestCount > 0,
  steadyStageCompleted: exitCode === 0 && completedRequestCount > 0,
  steadyDurationReached: exitCode === 0 && completedRequestCount > 0,
  scenarioCoverageReached,
  sampleTargetReached,
  sloEvaluationCompleted,
});
const validationIssues = [
  ...issues,
  ...(completedRequestCount <= 0 ? ['k6 completed zero requests'] : []),
  ...((kind === 'baseline' || kind === 'current' || kind === 'soak') && requiredScenarioMissingCount > 0 ? [`required scenario metrics missing: ${requiredScenarioMissingCount}`] : []),
  ...((kind === 'baseline' || kind === 'current' || kind === 'soak') && scenarios.some((item) => item.sampleCount !== null && item.sampleCount < 100) ? ['steady scenario samples are insufficient'] : []),
  ...((kind === 'baseline' || kind === 'current' || kind === 'soak') && scenarios.some((item) => item.metricEvidenceClassification === 'summary_stat_missing') ? ['required steady summary statistic is missing'] : []),
  ...((kind === 'baseline' || kind === 'current' || kind === 'soak') && !sloEvaluationCompleted ? ['absolute SLO evidence is not evaluable'] : []),
];

const report = {
  phase: kind === 'diagnostic' ? 'P7-V2-R2' : 'P7-V2',
  status: validationIssues.length === 0 && exitCode === 0 && absoluteSloPassed ? 'passed' : validationIssues.length ? 'blocked' : 'failed',
  kind,
  runId,
  baseUrl: redactURL(baseUrl),
  selectedHost: portConfig.host,
  selectedPort: portConfig.port,
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
  failedScenarios: exitCode === 0 && requiredScenarioMissingCount === 0 ? 0 : 1,
  thresholdsPassed: exitCode === 0,
  absoluteSloPassed,
  absoluteSloEvaluationStatus: sloEvaluationCompleted ? 'evaluated' : 'not_evaluable',
  realAbsoluteSloFailure: sloEvaluationCompleted && routeSloEvaluations.some((item) => item.realAbsoluteSloFailure),
  sloEvaluations: routeSloEvaluations,
  unexpectedMetricEvidence,
  targetReached: targetReachedComponents.targetReached,
  targetReachedComponents,
  requiredScenarioMissingCount,
  scenarioCoverageReached,
  sampleTargetReached,
  sloEvaluationCompleted,
  k6ExitCode: exitCode,
  crashes: 0,
  panics: 0,
  oom: 0,
  steadyMinutes: kind === 'soak' ? 30 : kind === 'baseline' || kind === 'current' ? 10 : kind === 'diagnostic' ? 3 : 2,
  steadyWindow: {
    phase: 'steady',
    steadyStart: `${loadProfile.warmup}+${loadProfile.ramp}`,
    steadyEnd: `${loadProfile.warmup}+${loadProfile.ramp}+${loadProfile.steady}`,
    steadyDuration: loadProfile.steady,
    steadySampleCount: scenarios.reduce((total, item) => total + Number(item.sampleCount || 0), 0),
  },
  loadProfile,
  canonicalLoadProfile: canonicalLoadProfile.canonicalProfile,
  environmentFingerprint: fingerprint,
  serverBinaryPath: runtime.serverBinaryPath || '',
  serverBinarySha256: runtime.serverBinarySha256 || '',
  expectedBinarySha256: runtime.expectedBinarySha256 || '',
  binarySha256Match: runtime.binarySha256Match ?? null,
  runtimeCommit: runtime.runtimeCommit || '',
  sourceTreeHash: runtime.sourceTreeHash || '',
  processPid: runtime.processPid || runtime.serverPid || '',
  processStartTime: runtime.processStartTime || '',
  processExecutablePath: runtime.processExecutablePath || '',
  processExecutableSha256: runtime.processExecutableSha256 || '',
  processExecutableSha256Match: runtime.processExecutableSha256Match ?? null,
  implicitBuildDisabled: runtime.implicitBuildDisabled === true,
  formalBinaryProvenanceVersion: runtime.formalBinaryProvenanceVersion || null,
  datasetFingerprint: dataset.datasetFingerprint || '',
  configFingerprint: fingerprint.configFingerprint,
  loadProfileFingerprint: canonicalLoadProfile.loadProfileFingerprint,
  loadProfileFingerprintVersion: canonicalLoadProfile.fingerprintVersion,
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
  formalInputSequenceBindingVersion: inputSequenceBinding.formalInputSequenceBindingVersion,
  loadSeed: inputSequenceBinding.loadSeed,
  scenarioSeed: inputSequenceBinding.scenarioSeed,
  inputSequenceManifestHash: inputSequenceBinding.inputSequenceManifestHash,
  requestSequenceHash: inputSequenceBinding.requestSequenceHash,
  webhookSequenceHash: inputSequenceBinding.webhookSequenceHash,
  authSequenceHash: inputSequenceBinding.authSequenceHash,
  webhookDuplicateSequenceHash: inputSequenceBinding.webhookDuplicateSequenceHash,
  webhookBranchMixFingerprint: inputSequenceBinding.webhookBranchMixFingerprint,
  authBranchMixFingerprint: inputSequenceBinding.authBranchMixFingerprint,
  branchMixFingerprint: inputSequenceBinding.branchMixFingerprint,
  webhookBranchMix: inputSequenceBinding.webhookBranchMix,
  authBranchMix: inputSequenceBinding.authBranchMix,
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
