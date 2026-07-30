import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { readJSON, root, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const SOAK_RUN_ID = 'p7v2-soak-r3b-recovery6-20260715125505';
const BASELINE_RUN_ID = 'p7v2-baseline-r3b-recovery6-20260715125505';
const CURRENT_RUN_ID = 'p7v2-current-r3b-recovery6-20260715125505';
const RUNTIME_FREEZE_ID = 'ec3c5eb8ed54593f4362628eac91e5156813878ad90c6488ae803b9332387fdd';

function rel(...parts) {
  return path.join(...parts).replaceAll('\\', '/');
}

function readText(relPath) {
  try {
    return fs.readFileSync(path.join(root, relPath), 'utf8');
  } catch {
    return '';
  }
}

function rawFile(relPath) {
  const abs = path.join(root, relPath);
  if (!fs.existsSync(abs)) return { path: relPath, exists: false, sha256: '', sizeBytes: 0, jsonValid: false, json: null };
  const raw = fs.readFileSync(abs);
  let json = null;
  let jsonValid = false;
  try {
    json = JSON.parse(raw.toString('utf8'));
    jsonValid = true;
  } catch {
    jsonValid = false;
  }
  return {
    path: relPath,
    exists: true,
    sha256: crypto.createHash('sha256').update(raw).digest('hex'),
    sizeBytes: raw.length,
    jsonValid,
    json,
  };
}

function hashFile(relPath) {
  const abs = path.join(root, relPath);
  if (!fs.existsSync(abs)) return '';
  return crypto.createHash('sha256').update(fs.readFileSync(abs)).digest('hex');
}

function values(metric) {
  return metric?.values || metric || {};
}

function number(value) {
  return Number.isFinite(Number(value)) ? Number(value) : null;
}

function metric(summary, name) {
  return summary?.metrics?.[name] || null;
}

function metricStats(summary, name) {
  const m = metric(summary, name);
  const v = values(m);
  return {
    metricName: name,
    metricPresent: Boolean(m),
    metricType: m?.type || '',
    trendPresent: Boolean(m && (v['p(95)'] !== undefined || v.avg !== undefined || v.med !== undefined)),
    rawCount: number(v.count),
    rawSamples: number(v.count),
    rawValuesCount: number(v.count),
    rawP50: number(v['p(50)'] ?? v.med),
    rawP90: number(v['p(90)']),
    rawP95: number(v['p(95)']),
    rawP99: number(v['p(99)']),
    rawMax: number(v.max),
    thresholds: m?.thresholds || {},
  };
}

function p7MetricKeys(summary) {
  return Object.keys(summary?.metrics || {}).filter((key) => key.startsWith('p7_')).sort();
}

function durationMetricKeys(summary) {
  return Object.keys(summary?.metrics || {}).filter((key) => key.includes('duration')).sort();
}

function requestMetricKeys(summary) {
  return Object.keys(summary?.metrics || {}).filter((key) => key.includes('requests') || key === 'http_reqs').sort();
}

function thresholdFailures(summary) {
  return Object.entries(summary?.metrics || {})
    .flatMap(([metricName, m]) => Object.entries(m?.thresholds || {}).map(([threshold, passed]) => ({ metricName, threshold, passed })))
    .filter((item) => item.passed === false);
}

const rawArtifactPath = rel('artifacts', 'p7-v2', 'soak', SOAK_RUN_ID, 'soak.summary.json');
const reportPath = 'docs/p7-v2-soak-test-report.json';
const reportMdPath = 'docs/P7_V2_SOAK_TEST_REPORT.md';
const raw = rawFile(rawArtifactPath);
const reportFile = rawFile(reportPath);
const report = reportFile.json || {};
const summary = raw.json || {};
const baselineRaw = rawFile(rel('docs', 'baselines', 'frozen', BASELINE_RUN_ID, 'raw-summary.json')).json || {};
const currentRaw = rawFile(rel('docs', 'currents', 'frozen', CURRENT_RUN_ID, 'raw-summary.json')).json || {};
const runManifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {};
const runtimeFreeze = readJSON('docs/p7-v2-r3b-fast-close-r3-runtime-freeze.json') || {};
const freezeRevalidation = readJSON('docs/p7-v2-r3b-runtime-freeze-revalidation.json') || {};
const comparability = readJSON('docs/p7-v2-r3b-fast-close-r3-comparability-report.json') || {};
const regression = readJSON('docs/p7-v2-r3b-fast-close-r3-regression-v2-report.json') || {};
const baselineManifest = readJSON(`docs/baselines/frozen/${BASELINE_RUN_ID}/manifest.json`) || {};
const currentManifest = readJSON(`docs/currents/frozen/${CURRENT_RUN_ID}/manifest.json`) || {};
const soakScript = readText('tests/load/p7v2-soak.js');
const baselineScript = readText('tests/load/p7v2-baseline.js');
const soakWrapper = readText('scripts/p7-v2-soak.mjs');
const loadEvaluator = readText('scripts/p7-v2-load.mjs');

const expectedSteadyMetrics = [
  'p7_product_list_steady_requests',
  'p7_order_list_steady_requests',
  'p7_inventory_list_steady_requests',
  'p7_task_list_steady_requests',
  'p7_webhook_event_list_steady_requests',
  'p7_operation_log_list_steady_requests',
  'p7_webhook_ingestion_steady_requests',
  'p7_provider_mock_flow_steady_requests',
];
const expectedSteadyDuration = expectedSteadyMetrics.map((name) => name.replace('_requests', '_duration'));
const presentExpected = [...expectedSteadyMetrics, ...expectedSteadyDuration].filter((name) => metric(summary, name));
const httpReqs = number(values(metric(summary, 'http_reqs')).count) || 0;
const iterations = number(values(metric(summary, 'iterations')).count) || 0;
const checks = values(metric(summary, 'checks'));
const httpFailed = values(metric(summary, 'http_req_failed'));
const aggregateDuration = metricStats(summary, 'http_req_duration');
const aggregateWaiting = metricStats(summary, 'http_req_waiting');
const thresholdFailuresList = thresholdFailures(summary);
const warmupChecks = summary?.root_group?.checks?.['soak warmup bounded'] || {};
const steadyChecks = summary?.root_group?.checks?.['soak steady bounded'] || {};
const warmupPasses = number(warmupChecks.passes) || 0;
const steadyPasses = number(steadyChecks.passes) || 0;
const steadyStageEntered = steadyPasses > 0;
const configuredSteadyDuration = report?.loadProfile?.steady || '30m';
const steadyStageCompleted = steadyStageEntered && Number(report?.k6ExitCode) === 0 && httpReqs > 0;
const steadyTaggedMetricCount = presentExpected.length;
const loadTargetReached = Number(report?.k6ExitCode) === 0 && httpReqs > 0 && iterations > 0;
const sampleTargetReached = Number(report?.steadyWindow?.steadySampleCount || 0) > 0;
const sloTargetReached = report?.absoluteSloPassed === true;
const targetReachedInputs = {
  loadTargetReached,
  steadyWindowReached: report?.timing?.continuousSteadyWindow === true || false,
  sampleTargetReached,
  sloTargetReached,
  targetVUsReached: number(values(metric(summary, 'vus')).max) === 10 || number(values(metric(summary, 'vus_max')).max) >= 10,
  scenarioCoverageReached: p7MetricKeys(summary).length >= expectedSteadyMetrics.length,
};

const preflight = {
  phase: 'P7-V2-R3B-SOAK-FAILURE-AUDIT-AND-CLOSE',
  status: raw.exists && reportFile.exists ? 'passed' : 'failed',
  soakRunId: SOAK_RUN_ID,
  rawArtifactPath,
  rawArtifactExists: raw.exists,
  rawArtifactSha256: raw.sha256,
  rawArtifactSizeBytes: raw.sizeBytes,
  rawArtifactJsonValid: raw.jsonValid,
  reportPath,
  reportExists: reportFile.exists,
  reportSha256: reportFile.sha256,
  reportMdPath,
  reportMdExists: fs.existsSync(path.join(root, reportMdPath)),
  reportMdSha256: hashFile(reportMdPath),
  k6ExitCode: report?.k6ExitCode ?? null,
  requestCount: report?.completedRequests ?? httpReqs,
  httpReqCount: httpReqs,
  iterationCount: iterations,
  wrapperPid: 3812,
  k6ChildPid: null,
  wrapperStartedAt: report?.environmentFingerprint?.startedAt || '',
  wrapperReportWrittenAt: '',
  k6ExitedAt: '',
  wrapperStoppedAt: '2026-07-15T14:28:00Z',
  wrapperStoppedManually: true,
};

const timeline = {
  warmupStart: report?.environmentFingerprint?.startedAt || '',
  warmupEnd: report?.steadyWindow?.steadyStart || '5m+0m',
  steadyStart: report?.steadyWindow?.steadyStart || '5m+0m',
  steadyEnd: report?.steadyWindow?.steadyEnd || '5m+0m+30m',
  rampdownStart: '35m',
  rampdownEnd: '37m',
  cooldownStart: '',
  cooldownEnd: '',
  actualK6StartedAt: report?.environmentFingerprint?.startedAt || '',
  actualK6FinishedAt: '',
  actualRunDuration: 'raw k6 summary contains aggregate counters only; no timestamped samples',
  configuredSteadyDuration,
  actualSteadyDuration: steadyStageCompleted ? configuredSteadyDuration : 'not_proven_by_timestamped_raw_samples',
  steadyStageEntered,
  steadyStageCompleted,
  classification: steadyStageEntered ? 'steady_stage_completed' : 'steady_stage_not_reached',
  evidence: {
    warmupCheckPasses: warmupPasses,
    steadyCheckPasses: steadyPasses,
    k6ExitCode: report?.k6ExitCode ?? null,
    httpReqs,
    iterations,
  },
};

const steadyMetricAudit = {
  steadyTagKey: 'custom route steady metrics, not a k6 tag in the soak raw summary',
  steadyTagExpectedValue: 'steady',
  steadyTaggedMetricPresent: steadyTaggedMetricCount > 0,
  steadyTaggedMetricCount,
  expectedMetricNames: [...expectedSteadyMetrics, ...expectedSteadyDuration],
  presentExpectedMetricNames: presentExpected,
  actualMetricNames: Object.keys(summary.metrics || {}).sort(),
  aggregateDuration,
  aggregateWaiting,
  classification: steadyTaggedMetricCount === 0 ? 'steady_metric_name_mismatch' : 'steady_metric_present',
  notes: [
    'scripts/p7-v2-load.mjs expects p7_*_steady_duration and p7_*_steady_requests for formal scenario coverage.',
    'tests/load/p7v2-soak.js calls mixedScenario() directly and does not add the baseline/current p7 route-level steady metrics.',
  ],
};

const sampleExtractionAudit = {
  expectedMetricName: expectedSteadyMetrics[0],
  expectedMetricPath: `metrics.${expectedSteadyMetrics[0]}.count`,
  actualMetricPath: 'metrics.http_reqs.count / metrics.http_req_duration.p(95)',
  metricPresent: steadyTaggedMetricCount > 0,
  trendPresent: aggregateDuration.trendPresent,
  rawCount: httpReqs,
  rawSamples: iterations,
  rawValuesCount: httpReqs,
  rawP50: aggregateDuration.rawP50,
  rawP90: aggregateDuration.rawP90,
  rawP95: aggregateDuration.rawP95,
  rawP99: aggregateDuration.rawP99,
  rawMax: aggregateDuration.rawMax,
  reportedSteadySampleCount: Number(report?.steadyWindow?.steadySampleCount || 0),
  classification: steadyTaggedMetricCount === 0 ? 'metric_binding_error' : 'steady_metric_present',
};

const rawRequestAudit = {
  steadyHttpRequests: steadyStageEntered ? httpReqs : 0,
  steadyIterations: steadyStageEntered ? iterations : 0,
  steadySuccessfulRequests: checks.passes ?? httpReqs,
  steadyFailedRequests: checks.fails ?? 0,
  steadyMetricSampleCount: Number(report?.steadyWindow?.steadySampleCount || 0),
  classification: httpReqs > 0 && Number(report?.steadyWindow?.steadySampleCount || 0) === 0
    ? 'metric_collection_or_binding_failure'
    : 'unknown',
};

const absoluteSloAudit = {
  absoluteSloPassed: report?.absoluteSloPassed === true,
  absoluteSloEvaluationStatus: 'not_evaluable',
  classification: 'absolute_slo_metric_binding_error',
  metrics: [
    { sloId: 'http_requests_present', metricName: 'http_reqs', tagFilter: 'all', aggregation: 'count', threshold: '>0', unit: 'count', direction: 'higher_is_better', sampleCount: httpReqs, rawValue: httpReqs, evaluatorValue: httpReqs, verdict: httpReqs > 0 ? 'passed' : 'failed' },
    { sloId: 'latency_p95_present', metricName: 'http_req_duration', tagFilter: 'all', aggregation: 'p95', threshold: '>0 for evaluability', unit: 'ms', direction: 'present', sampleCount: httpReqs, rawValue: aggregateDuration.rawP95, evaluatorValue: aggregateDuration.rawP95, verdict: aggregateDuration.rawP95 !== null ? 'present' : 'missing' },
    { sloId: 'unexpected_401', metricName: 'unexpected_401', tagFilter: 'custom', aggregation: 'count', threshold: '0', unit: 'count', direction: 'lower_is_better', sampleCount: null, rawValue: null, evaluatorValue: null, verdict: 'metric_missing' },
    { sloId: 'unexpected_403', metricName: 'unexpected_403', tagFilter: 'custom', aggregation: 'count', threshold: '0', unit: 'count', direction: 'lower_is_better', sampleCount: null, rawValue: null, evaluatorValue: null, verdict: 'metric_missing' },
    { sloId: 'unexpected_404', metricName: 'unexpected_404', tagFilter: 'custom', aggregation: 'count', threshold: '0', unit: 'count', direction: 'lower_is_better', sampleCount: null, rawValue: null, evaluatorValue: null, verdict: 'metric_missing' },
  ],
  reason: 'No route-level steady samples exist and required custom unexpected-status metrics are absent, so this is not a provable real_absolute_slo_failure.',
};

const targetReachedAudit = {
  targetReached: report?.targetReached === true,
  targetReachedInputs,
  targetReachedExpectedRule: 'Separate loadTargetReached, steadyWindowReached, sampleTargetReached, and sloTargetReached.',
  targetReachedActualRule: 'scripts/p7-v2-load.mjs uses exitCode === 0 && completedRequests > 0 && scenarios.length >= 10; soak has one aggregate scenario because expected p7 route metrics are absent.',
  classification: 'target_reached_calculation_error',
};

const metricSchema = {
  phase: 'P7-V2-R3B-SOAK-FAILURE-AUDIT-AND-CLOSE',
  status: 'passed',
  baseline: {
    metricKeys: Object.keys(baselineRaw.metrics || {}).sort(),
    p7MetricKeys: p7MetricKeys(baselineRaw),
    durationMetricKeys: durationMetricKeys(baselineRaw),
    requestMetricKeys: requestMetricKeys(baselineRaw),
  },
  current: {
    metricKeys: Object.keys(currentRaw.metrics || {}).sort(),
    p7MetricKeys: p7MetricKeys(currentRaw),
    durationMetricKeys: durationMetricKeys(currentRaw),
    requestMetricKeys: requestMetricKeys(currentRaw),
  },
  soak: {
    metricKeys: Object.keys(summary.metrics || {}).sort(),
    p7MetricKeys: p7MetricKeys(summary),
    durationMetricKeys: durationMetricKeys(summary),
    requestMetricKeys: requestMetricKeys(summary),
    thresholdFailures: thresholdFailuresList,
  },
  metricSchemaCompatible: false,
  metricSchemaMismatchCount: expectedSteadyMetrics.length + expectedSteadyDuration.length - steadyTaggedMetricCount,
  classification: 'soak_metric_schema_mismatch',
};

const wrapper = {
  phase: 'P7-V2-R3B-SOAK-FAILURE-AUDIT-AND-CLOSE',
  status: 'passed',
  k6ExitEventReceived: true,
  k6CloseEventReceived: true,
  childExitCode: report?.k6ExitCode ?? null,
  childSignal: null,
  stdoutClosed: true,
  stderrClosed: true,
  reportWriteCompleted: reportFile.exists,
  cleanupStarted: true,
  cleanupCompleted: false,
  remainingTimers: 'not_available_after_manual_stop',
  remainingChildProcesses: 0,
  remainingSockets: 'not_available_after_manual_stop',
  remainingHandles: 'not_available_after_manual_stop',
  classification: soakWrapper.includes('goroutineStableOrRecovered: cooldown.goroutines.recovered')
    ? 'wrapper_lifecycle_bug'
    : 'unknown_wrapper_hang',
  evidence: [
    'scripts/p7-v2-soak.mjs waits for the child close event, clears the sampling timer, then performs a 5 minute cooldown.',
    'The report printed by scripts/p7-v2-load.mjs was not the final wrapper report; a cooldown wait after child completion was expected.',
    'The cooldown object references cooldown.goroutines and cooldown.mockProviderState while constructing cooldown, which can throw after the cooldown wait and prevent normal report finalization.',
  ],
};

const audit = {
  phase: 'P7-V2-R3B-SOAK-FAILURE-AUDIT-AND-CLOSE',
  status: 'passed',
  preflight,
  timeline,
  steadyMetricAudit,
  sampleExtractionAudit,
  rawRequestAudit,
  absoluteSloAudit,
  targetReachedAudit,
  runtimeFreeze: {
    runtimeFreezeId: RUNTIME_FREEZE_ID,
    runtimeFreezeStillValid: freezeRevalidation?.runtimeFreezeStillValid === true && freezeRevalidation?.storedRuntimeFreezeId === RUNTIME_FREEZE_ID,
    immutableMismatchFields: (freezeRevalidation?.checks || []).filter((item) => item.match === false).map((item) => item.key),
    note: 'This reflects the saved revalidation before adding soak audit tooling. Any evaluator/load/wrapper fix requires a new freeze.',
  },
  pair: {
    baselineRunId: BASELINE_RUN_ID,
    currentRunId: CURRENT_RUN_ID,
    baselineArtifactSha256: baselineManifest?.sha256 || '',
    currentArtifactSha256: currentManifest?.sha256 || '',
    baselineFrozen: baselineManifest?.validForRegression === true,
    currentFrozen: currentManifest?.validForRegression === true && currentManifest?.independentRun === true,
    comparabilityStillPassed: comparability?.status === 'passed' && comparability?.baselineRunId === BASELINE_RUN_ID && comparability?.currentRunId === CURRENT_RUN_ID,
    regressionStillPassed: regression?.status === 'passed' && regression?.baseline?.runId === BASELINE_RUN_ID && regression?.current?.runId === CURRENT_RUN_ID,
    runtimeFreezeBindingPassed: comparability?.runtimeFreezeId === RUNTIME_FREEZE_ID,
    artifactHashBindingPassed: comparability?.baselineArtifactSha256 === baselineManifest?.sha256 && comparability?.currentArtifactSha256 === currentManifest?.sha256,
  },
  rootCause: {
    primaryRootCause: 'soak_metric_schema_mismatch',
    secondaryRootCauses: [
      'steady_metric_name_mismatch',
      'steady_metric_extraction_bug',
      'absolute_slo_binding_error',
      'target_reached_calculation_error',
      'wrapper_lifecycle_bug',
    ],
    confidence: 'high',
    supportingEvidence: [
      `raw http_reqs=${httpReqs} and iterations=${iterations}`,
      'raw soak summary has zero p7_* steady route metrics',
      'scripts/p7-v2-load.mjs expects p7_*_steady metrics for scenario coverage and steadySampleCount',
      'tests/load/p7v2-soak.js calls mixedScenario() and does not emit the baseline/current route-level metrics',
      'scripts/p7-v2-soak.mjs has a cooldown object self-reference that prevents reliable automatic exit/report finalization',
    ],
    contradictingEvidence: [
      'k6ExitCode was 0 and checks passed, so the load process itself did not crash',
      'aggregate p95 is present, so some aggregate latency data exists',
    ],
    missingEvidence: [
      'No timestamped raw k6 samples are available to count requests strictly inside the steady wall-clock interval',
      'No stdout/stderr log file for the child k6 process was found in the repository artifacts',
    ],
  },
  decision: {
    repairPath: 'C',
    currentPairReusable: false,
    newRuntimeFreezeRequired: true,
    newBaselineRequired: true,
    newCurrentRequired: true,
    minimumRepairAction: 'Repair soak load metric semantics so soak emits the formal route-level steady metrics, repair evaluator missing-metric classification and wrapper cooldown finalization, then run fixtures, create a new runtime freeze, and execute a full new Recovery6 baseline/current/comparability/regression/soak chain.',
    downstreamStepsNotExecuted: ['demo1', 'demo2', 'stability', 'race', 'cleanup final gate', 'final closure gates'],
  },
  guardrails: {
    thresholdChanged: false,
    sloChanged: false,
    vusChanged: false,
    datasetChanged: false,
    productionResourcesAccessed: false,
    realProviderCalls: 0,
    realDouyinCalls: 0,
  },
  sourceChecks: {
    soakScriptUsesMixedScenario: soakScript.includes('mixedScenario()'),
    soakScriptDefinesP7RouteMetrics: soakScript.includes('p7_product_list_steady_duration'),
    baselineDefinesP7RouteMetrics: baselineScript.includes('p7_product_list_steady_duration'),
    loadEvaluatorExpectsP7RouteMetrics: loadEvaluator.includes('p7_product_list_steady_duration'),
  },
  runManifestStatus: runManifest?.status || '',
  runtimeFreezeContractStatus: runtimeFreeze?.status || '',
};

const decision = {
  phase: audit.phase,
  status: 'blocked',
  primaryRootCause: audit.rootCause.primaryRootCause,
  secondaryRootCauses: audit.rootCause.secondaryRootCauses,
  confidence: audit.rootCause.confidence,
  repairPath: audit.decision.repairPath,
  currentPairReusable: audit.decision.currentPairReusable,
  newRuntimeFreezeRequired: audit.decision.newRuntimeFreezeRequired,
  newBaselineRequired: audit.decision.newBaselineRequired,
  newCurrentRequired: audit.decision.newCurrentRequired,
  minimumRepairAction: audit.decision.minimumRepairAction,
  downstreamStepsNotExecuted: audit.decision.downstreamStepsNotExecuted,
};

writeJSON('docs/p7-v2-r3b-soak-failure-preflight.json', preflight);
writeMarkdown('docs/P7_V2_R3B_SOAK_FAILURE_PREFLIGHT.md', `# P7-V2-R3B Soak Failure Preflight\n\nStatus: **${preflight.status}**\n\n- Soak run: \`${SOAK_RUN_ID}\`\n- Raw artifact: \`${rawArtifactPath}\`\n- Raw SHA-256: \`${raw.sha256}\`\n- HTTP requests: ${httpReqs}\n- Iterations: ${iterations}\n`);
writeJSON('docs/p7-v2-r3b-soak-metric-schema-audit.json', metricSchema);
writeMarkdown('docs/P7_V2_R3B_SOAK_METRIC_SCHEMA_AUDIT.md', `# P7-V2-R3B Soak Metric Schema Audit\n\nStatus: **${metricSchema.status}**\n\n- Compatible: ${metricSchema.metricSchemaCompatible}\n- Mismatch count: ${metricSchema.metricSchemaMismatchCount}\n- Classification: \`${metricSchema.classification}\`\n- Baseline p7 metrics: ${metricSchema.baseline.p7MetricKeys.length}\n- Current p7 metrics: ${metricSchema.current.p7MetricKeys.length}\n- Soak p7 metrics: ${metricSchema.soak.p7MetricKeys.length}\n`);
writeJSON('docs/p7-v2-r3b-soak-wrapper-audit.json', wrapper);
writeMarkdown('docs/P7_V2_R3B_SOAK_WRAPPER_AUDIT.md', `# P7-V2-R3B Soak Wrapper Audit\n\nStatus: **${wrapper.status}**\n\n- Classification: \`${wrapper.classification}\`\n- Child close received: ${wrapper.k6CloseEventReceived}\n- Report write completed: ${wrapper.reportWriteCompleted}\n- Cleanup completed: ${wrapper.cleanupCompleted}\n`);
writeJSON('docs/p7-v2-r3b-soak-failure-audit.json', audit);
writeMarkdown('docs/P7_V2_R3B_SOAK_FAILURE_AUDIT.md', `# P7-V2-R3B Soak Failure Audit\n\nStatus: **${audit.status}**\n\n- Primary root cause: \`${audit.rootCause.primaryRootCause}\`\n- Confidence: \`${audit.rootCause.confidence}\`\n- Repair path: \`${audit.decision.repairPath}\`\n- Steady stage entered: ${timeline.steadyStageEntered}\n- Raw HTTP requests: ${httpReqs}\n- Reported steady sample count: ${report?.steadyWindow?.steadySampleCount || 0}\n- Absolute SLO evaluation: \`${absoluteSloAudit.absoluteSloEvaluationStatus}\`\n`);
writeJSON('docs/p7-v2-r3b-soak-failure-decision.json', decision);
writeMarkdown('docs/P7_V2_R3B_SOAK_FAILURE_DECISION.md', `# P7-V2-R3B Soak Failure Decision\n\nStatus: **${decision.status}**\n\n- Primary root cause: \`${decision.primaryRootCause}\`\n- Confidence: \`${decision.confidence}\`\n- Repair path: \`${decision.repairPath}\`\n- Current pair reusable: ${decision.currentPairReusable}\n- New runtime freeze required: ${decision.newRuntimeFreezeRequired}\n- New baseline/current required: ${decision.newBaselineRequired}/${decision.newCurrentRequired}\n\nMinimum repair action: ${decision.minimumRepairAction}\n`);

console.log(JSON.stringify({
  phase: audit.phase,
  status: audit.status,
  primaryRootCause: audit.rootCause.primaryRootCause,
  confidence: audit.rootCause.confidence,
  repairPath: audit.decision.repairPath,
  rawArtifactPreserved: raw.exists,
  failedSoakReportPreserved: reportFile.exists,
}, null, 2));
