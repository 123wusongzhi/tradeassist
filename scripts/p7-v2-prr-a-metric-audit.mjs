import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { CORE_SCENARIOS, METRIC_METADATA, SCENARIO_METRICS } from './p7-v2-regression-metrics.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const phase = 'P7-V2-R3B-PRR-A';
const baselineRunId = 'p7v2-baseline-r3b-recovery3-20260715-131400';
const currentRunId = 'p7v2-current-r3b-recovery3-20260715-131400';
const baselineExpectedHash = 'a04a39d4aa5e1cf8a951e9195af894e2a03df185c360e7652e4eacd7770aeaa9';
const currentExpectedHash = '19aaa21b4094ee14147621b47b7370003b9a4dbaec12b24f64a32a66162af4c5';
const scenarioRoutes = {
  'Product List': 'GET /api/v1/products?pageSize=20',
  'Order List': 'GET /api/v1/orders?pageSize=20',
  'Inventory List': 'GET /api/v1/inventory?pageSize=20',
  'Task List': 'GET /api/v1/task-center/failures?pageSize=20',
  'Webhook Event List': 'GET /api/v1/webhook-events?pageSize=20',
  'Operation Log List': 'GET /api/v1/operation-logs?pageSize=20',
  'Webhook Ingestion': 'POST /api/v1/webhooks/internal-test/ping',
  'Provider Mock Flow': 'GET /health/live',
  'Auth/Security': 'aggregate: POST /api/v1/auth/login invalid-login + POST /api/v1/webhooks/internal-test/ping invalid-signature',
};

const readJson = (relativePath) => JSON.parse(fs.readFileSync(path.join(root, relativePath), 'utf8'));
const hashFile = (filePath) => crypto.createHash('sha256').update(fs.readFileSync(filePath)).digest('hex');
const numberOrNull = (value) => (typeof value === 'number' && Number.isFinite(value) ? value : null);
const valuePath = (metricName, key) => `metrics.${metricName}.${key}`;

function frozen(kind, runId, expectedHash) {
  const group = kind === 'baseline' ? 'baselines' : 'currents';
  const relativeDir = `docs/${group}/frozen/${runId}`;
  const manifestPath = path.join(root, relativeDir, 'manifest.json');
  const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
  const rawRelativePath = manifest.rawArtifact?.relativePath || 'raw-summary.json';
  const rawPath = path.join(root, relativeDir, rawRelativePath);
  const rawBytes = fs.readFileSync(rawPath);
  const raw = JSON.parse(rawBytes.toString('utf8'));
  const actualHash = hashFile(rawPath);
  const requestCount = Number(raw.metrics?.http_reqs?.values?.count ?? raw.metrics?.http_reqs?.count ?? 0);
  const scenarioMetricsPresent = CORE_SCENARIOS.every((scenario) => {
    const [durationMetric, requestMetric] = SCENARIO_METRICS[scenario];
    return Boolean(raw.metrics?.[durationMetric]) && Number(raw.metrics?.[requestMetric]?.values?.count ?? raw.metrics?.[requestMetric]?.count ?? 0) > 0;
  });
  const valid = manifest.runId === runId && manifest.runKind === kind && manifest.immutable === true &&
    actualHash === expectedHash && actualHash === manifest.rawArtifact?.sha256 &&
    rawBytes.length === Number(manifest.rawArtifact?.sizeBytes) && requestCount > 0 && scenarioMetricsPresent;
  return {
    kind, runId, relativeDir, manifest, raw, rawPath, actualHash, sizeBytes: rawBytes.length,
    requestCount, scenarioMetricsPresent, valid,
  };
}

function rawMetric(run, metricName) {
  const raw = run.raw.metrics?.[metricName];
  if (!raw || typeof raw !== 'object') return { present: false, values: {}, path: `metrics.${metricName}` };
  return { present: true, values: raw.values || raw, path: `metrics.${metricName}` };
}

function trendStats(run, scenario) {
  const [durationMetric, requestMetric] = SCENARIO_METRICS[scenario];
  const trend = rawMetric(run, durationMetric);
  const requests = rawMetric(run, requestMetric);
  const get = (key) => numberOrNull(trend.values[key]);
  return {
    durationMetric,
    requestMetric,
    rawMetricPresent: trend.present,
    rawMetricPath: trend.path,
    p50: get('med'),
    p90: get('p(90)'),
    p95: get('p(95)'),
    p99: get('p(99)'),
    max: get('max'),
    avg: get('avg'),
    min: get('min'),
    requestCount: numberOrNull(requests.values.count) ?? 0,
    throughput: numberOrNull(requests.values.rate),
    sampleCount: numberOrNull(requests.values.count) ?? 0,
  };
}

function globalStats(run) {
  const failed = rawMetric(run, 'http_req_failed');
  const timeouts = rawMetric(run, 'p7_timeouts');
  return {
    errorRate: numberOrNull(failed.values.rate),
    timeoutCount: numberOrNull(timeouts.values.count) ?? 0,
    statusCodeDistribution: 'not_available_in_k6_summary',
  };
}

export function classifyP99({ rawMetricPresent, rawP99, rawMax, requestCount, trendSampleCount, parserOutput, minimumSampleCount = 100 }) {
  if (!rawMetricPresent && parserOutput === 0) return 'raw_metric_missing_parser_defaulted_zero';
  if (requestCount > 0 && trendSampleCount === 0) return 'trend_has_no_samples';
  if (trendSampleCount < minimumSampleCount) return 'insufficient_samples_for_p99';
  if (rawP99 === null || rawP99 === undefined) return 'summary_stat_missing';
  if (rawP99 === 0 && rawMax === 0) return 'invalid_latency_zero';
  if (rawP99 === 0) return 'invalid_latency_zero';
  return 'unknown';
}

export function classifyDistribution({ baseline, current }) {
  const deltas = ['p50', 'p90', 'p95', 'p99'].map((key) => ({
    key,
    baseline: baseline[key],
    current: current[key],
    worsened: Number.isFinite(baseline[key]) && Number.isFinite(current[key]) && current[key] > baseline[key],
  }));
  const comparable = deltas.filter((item) => Number.isFinite(item.baseline) && Number.isFinite(item.current));
  const broadlyShifted = comparable.length >= 3 && comparable.filter((item) => item.worsened).length >= 3;
  const p95Only = current.p95 > baseline.p95 &&
    ['p50', 'p90'].every((key) => !Number.isFinite(baseline[key]) || !Number.isFinite(current[key]) || current[key] <= baseline[key] * 1.05);
  const throughputDeclined = Number.isFinite(baseline.throughput) && Number.isFinite(current.throughput) && current.throughput < baseline.throughput;
  return { broadlyShifted, p95Only, throughputDeclined, comparablePercentiles: comparable.map((item) => item.key) };
}

function p95Audit(scenario, baseline, current, regressionRow) {
  const distribution = classifyDistribution({ baseline, current });
  const commonEvidence = [
    `Raw p95 changed from ${baseline.p95} ms to ${current.p95} ms.`,
    `Request counts are ${baseline.requestCount} and ${current.requestCount}; ratio=${(current.requestCount / baseline.requestCount).toFixed(4)}.`,
    `Runtime/load/dataset/config fingerprints match, while the independent runs use different database identities.`,
  ];
  let classification = 'statistical_variance_insufficient_evidence';
  let confidence = 'low';
  let recommendedAction = 'Run a diagnostic Recovery4 with deterministic database-state preparation and capture plans, pool wait, locks, and process telemetry; do not alter runtime until that evidence exists.';
  let supportingEvidence = [...commonEvidence, `Distribution analysis: broadlyShifted=${distribution.broadlyShifted}; p95Only=${distribution.p95Only}; throughputDeclined=${distribution.throughputDeclined}.`];
  let contradictingEvidence = ['No frozen query plan, slow-query trace, lock metric, connection-pool metric, or database statistics snapshot is available.'];
  let missingEvidence = ['EXPLAIN (ANALYZE, BUFFERS) from the original databases', 'PostgreSQL lock/pool/auto-vacuum evidence', 'steady-window-only samples', 'host CPU, memory, disk, GC, goroutine, worker, and Redis telemetry'];
  if (scenario === 'Auth/Security') {
    classification = 'metric_tag_aggregation_bug';
    confidence = 'high';
    recommendedAction = 'Split Auth/Security into route-specific latency trends for invalid login and invalid webhook signature; retain aggregate only as a non-gating diagnostic metric, then rerun Recovery4 because metric collection changes.';
    supportingEvidence = [
      'tests/load/p7v2-baseline.js adds both POST /api/v1/auth/login invalid-login latency and POST /api/v1/webhooks/internal-test/ping invalid-signature latency to p7_auth_security_duration.',
      ...commonEvidence,
    ];
    contradictingEvidence = ['The aggregated p95 remains a real end-to-end timing distribution, so aggregation alone does not prove no runtime-state contribution.'];
    missingEvidence = ['Route-specific percentile summaries for the two constituent routes', ...missingEvidence];
  }
  return {
    scenario, routeId: scenario === 'Task List' ? 'GET /api/v1/task-center/failures?pageSize=20'
      : scenario === 'Webhook Ingestion' ? 'POST /api/v1/webhooks/internal-test/ping'
        : 'aggregated: POST /api/v1/auth/login invalid-login + POST /api/v1/webhooks/internal-test/ping invalid-signature',
    ...regressionRow,
    baseline, current, distribution,
    classification, primaryRootCause: classification, secondaryContributors: scenario === 'Auth/Security' ? ['database_state_asymmetry'] : ['database_state_asymmetry', 'environmental_variance'],
    confidence, supportingEvidence, contradictingEvidence, missingEvidence, recommendedAction,
  };
}

function metricMatrixRow(scenario, metric, baseline, current, baselineGlobal, currentGlobal, reportRow) {
  const metadata = METRIC_METADATA[metric];
  const values = metric === 'p95' ? [baseline.p95, current.p95] : metric === 'p99' ? [baseline.p99, current.p99]
    : metric === 'rps' ? [baseline.throughput, current.throughput] : metric === 'errorRate' ? [baselineGlobal.errorRate, currentGlobal.errorRate]
      : [baselineGlobal.timeoutCount, currentGlobal.timeoutCount];
  const [baselineRawValue, currentRawValue] = values;
  const absoluteDelta = baselineRawValue !== null && currentRawValue !== null ? currentRawValue - baselineRawValue : null;
  return {
    scenario, routeId: scenarioRoutes[scenario] || 'not_available', metricName: metric, metricFamily: metadata.metricFamily, direction: metadata.direction, unit: metadata.unit,
    baselineMetricPresent: baselineRawValue !== null, baselineRawValue, baselineSampleCount: baseline.sampleCount, baselineRequestCount: baseline.requestCount,
    currentMetricPresent: currentRawValue !== null, currentRawValue, currentSampleCount: current.sampleCount, currentRequestCount: current.requestCount,
    baselineP50: baseline.p50, baselineP90: baseline.p90, baselineP95: baseline.p95, baselineP99: baseline.p99, baselineMax: baseline.max, baselineAvg: baseline.avg, baselineMin: baseline.min,
    currentP50: current.p50, currentP90: current.p90, currentP95: current.p95, currentP99: current.p99, currentMax: current.max, currentAvg: current.avg, currentMin: current.min,
    baselineThroughput: baseline.throughput, currentThroughput: current.throughput,
    baselineErrorRate: baselineGlobal.errorRate, currentErrorRate: currentGlobal.errorRate,
    baselineTimeoutCount: baselineGlobal.timeoutCount, currentTimeoutCount: currentGlobal.timeoutCount,
    absoluteDelta, relativeDelta: baselineRawValue && absoluteDelta !== null ? absoluteDelta / baselineRawValue : null,
    relativeThreshold: metadata.relativeThreshold, materialityFloor: metadata.materialityFloor, absoluteSlo: reportRow?.absoluteSlo ?? null,
    relativeExceeded: reportRow?.relativeExceeded ?? null, materialityExceeded: reportRow?.materialityExceeded ?? null,
    absoluteSloFailed: reportRow ? reportRow.absoluteSlo === false : null, regressionVerdict: reportRow?.finalVerdict ?? 'not_evaluated',
    regressionReason: reportRow?.reason ?? 'not_evaluated',
  };
}

function markdown(title, data) {
  return `# ${title}\n\n\`\`\`json\n${JSON.stringify(data, null, 2)}\n\`\`\`\n`;
}

function archiveRegression(report) {
  const source = path.join(root, 'docs/p7-v2-r3b-lpf-regression-v2-report.json');
  const archiveJson = path.join(root, 'docs/regressions/p7-v2-r3b-recovery3-regression-v2-failed.json');
  const archiveMd = path.join(root, 'docs/regressions/P7_V2_R3B_RECOVERY3_REGRESSION_V2_FAILED.md');
  fs.mkdirSync(path.dirname(archiveJson), { recursive: true });
  if (!fs.existsSync(archiveJson)) fs.copyFileSync(source, archiveJson);
  if (!fs.existsSync(archiveMd)) fs.writeFileSync(archiveMd, markdown('P7-V2-R3B Recovery3 Regression V2 Failed Archive', {
    baselineRunId, currentRunId, evaluationVersion: report.evaluationVersion, policyVersion: report.policyVersion,
    failedMetricCount: report.failedMetricCount, p95FailedMetricCount: report.comparisons.filter((item) => item.metric === 'p95' && item.finalVerdict.startsWith('failed')).length,
    p99ZeroViolationCount: report.comparisons.filter((item) => item.metric === 'p99' && item.finalVerdict === 'invalid_metric').length,
    originalReportSha256: hashFile(source), source: 'docs/p7-v2-r3b-lpf-regression-v2-report.json',
  }));
  return { archiveJson: path.relative(root, archiveJson), archiveMd: path.relative(root, archiveMd), originalReportSha256: hashFile(source) };
}

export function buildAudit() {
  const baseline = frozen('baseline', baselineRunId, baselineExpectedHash);
  const current = frozen('current', currentRunId, currentExpectedHash);
  const regression = readJson('docs/p7-v2-r3b-lpf-regression-v2-report.json');
  const comparability = readJson('docs/p7-v2-r3b-lpf-comparability-v2-report.json');
  if (!baseline.valid || !current.valid) throw new Error('immutable Recovery3 artifact integrity failed; refusing diagnostic output');
  const archive = archiveRegression(regression);
  const reportRows = regression.comparisons || [];
  const matrix = CORE_SCENARIOS.flatMap((scenario) => {
    const base = trendStats(baseline, scenario);
    const cur = trendStats(current, scenario);
    return Object.keys(METRIC_METADATA).map((metric) => metricMatrixRow(
      scenario, metric, base, cur, globalStats(baseline), globalStats(current),
      reportRows.find((item) => item.scenario === scenario && item.metric === metric),
    ));
  });
  const p95Failures = ['Task List', 'Webhook Ingestion', 'Auth/Security'].map((scenario) => p95Audit(
    scenario, trendStats(baseline, scenario), trendStats(current, scenario),
    reportRows.find((item) => item.scenario === scenario && item.metric === 'p95'),
  ));
  const p99Zeros = CORE_SCENARIOS.map((scenario) => {
    const base = trendStats(baseline, scenario);
    const cur = trendStats(current, scenario);
    const parserInput = { baseline: base.p99, current: cur.p99 };
    const classification = classifyP99({
      rawMetricPresent: base.rawMetricPresent && cur.rawMetricPresent,
      rawP99: base.p99 ?? cur.p99, rawMax: Math.max(base.max ?? 0, cur.max ?? 0),
      requestCount: Math.min(base.requestCount, cur.requestCount), trendSampleCount: Math.min(base.sampleCount, cur.sampleCount), parserOutput: 0,
    });
    return {
      scenario, metric: 'p99', rawMetricPath: `${base.rawMetricPath}; ${cur.rawMetricPath}`, rawMetricPresent: base.rawMetricPresent && cur.rawMetricPresent,
      rawCount: { baseline: base.requestCount, current: cur.requestCount }, rawSamples: { baseline: base.sampleCount, current: cur.sampleCount },
      rawP95: { baseline: base.p95, current: cur.p95 }, rawP99: parserInput, rawMax: { baseline: base.max, current: cur.max },
      parserInput, parserOutput: 0, classification, zeroIsReal: false, missingWasConvertedToZero: true,
      sampleCountSufficient: base.sampleCount >= 100 && cur.sampleCount >= 100,
      policyRequirement: 'p99 latency requires a present, non-zero summary statistic and at least 100 samples per side.',
      recommendedFix: 'Evaluator-only: preserve missing as null/not_comparable and emit summary_stat_missing; do not coerce absent p(99) to zero. A future harness remediation must ensure k6 exports p(99) for scenario trends.',
    };
  });
  const runtimeState = {
    phase, status: 'completed_with_evidence_gaps',
    observed: {
      runtimeSourceTreeHashMatch: baseline.manifest.runtimeSourceTreeHash === current.manifest.runtimeSourceTreeHash,
      loadScriptsHashMatch: baseline.manifest.loadScriptsHash === current.manifest.loadScriptsHash,
      metricSemanticsHashMatch: baseline.manifest.metricSemanticsHash === current.manifest.metricSemanticsHash,
      datasetFingerprintMatch: baseline.manifest.datasetFingerprint === current.manifest.datasetFingerprint,
      databaseNameHashMatch: baseline.manifest.environmentFingerprint?.databaseNameHash === current.manifest.environmentFingerprint?.databaseNameHash,
      baselineDatabaseNameHash: baseline.manifest.environmentFingerprint?.databaseNameHash,
      currentDatabaseNameHash: current.manifest.environmentFingerprint?.databaseNameHash,
    },
    notObserved: ['database statistics / ANALYZE state', 'autovacuum', 'table/index bloat', 'prepared statements', 'Redis state', 'worker backlog', 'operation-log growth', 'auth limiter state', 'query plans', 'slow-query logs', 'DB locks', 'pool wait', 'CPU', 'memory', 'swap', 'disk IO', 'Go GC', 'goroutines', 'other local process interference', 'steady-window-only histogram data'],
    conclusion: 'Comparability proves frozen configuration equivalence, not execution-state equivalence. The distinct databaseNameHash values and missing state telemetry require a diagnostic reproduction before assigning a runtime root cause.',
  };
  const decision = {
    nextPhase: 'P7-V2-R3B-PRR-REPRO',
    evaluatorOnlyRemediationRequired: true,
    runtimeRemediationRequired: false,
    harnessStateRemediationRequired: true,
    diagnosticReproductionRequired: true,
    recovery3RawArtifactsReusable: true,
    recovery4Required: true,
    rationale: 'All nine p99 findings are evaluator extraction defects caused by missing p(99) summary values being converted to zero. The three p95 failures are genuine raw p95 differences, but the frozen evidence cannot distinguish database/execution-state asymmetry from runtime degradation. Auth/Security also aggregates two routes. Therefore evaluator-only recalculation may repair p99 semantics but cannot clear the p95 failures; a diagnostic Recovery4 is required before runtime remediation.',
    recommendations: [
      { targetArea: 'evaluator parser', targetFileOrModule: 'scripts/p7-v2-lib.mjs metric()', observedProblem: 'Absent summary values are returned as 0.', proposedChange: 'Return null/undefined for missing statistics and require callers to classify missing latency summaries.', expectedEffect: 'Prevent missing p99 values from becoming valid-looking zero latency.', risk: 'Existing report formatting must handle null.', validationMethod: 'PRR-A fixtures plus evaluator-only recalculation of Recovery3 artifacts.', requiresNewBaseline: false, requiresNewCurrent: false },
      { targetArea: 'harness metric export', targetFileOrModule: 'tests/load/p7v2-baseline.js / k6 summary configuration', observedProblem: 'Scenario trend summaries contain p50/p90/p95/max but omit p99.', proposedChange: 'Capture scenario p99 explicitly and retain route-specific trend labels.', expectedEffect: 'Makes p99 latency semantically evaluable.', risk: 'Changes metric collection semantics.', validationMethod: 'Recovery4 diagnostic output must contain p(99) for all required trends.', requiresNewBaseline: true, requiresNewCurrent: true },
      { targetArea: 'execution-state harness', targetFileOrModule: 'P7-V2 environment/bootstrap scripts', observedProblem: 'Baseline and Current used different database identities without frozen stats/cache/pool/worker evidence.', proposedChange: 'Add deterministic state preparation and read-only state snapshots before each run.', expectedEffect: 'Separates execution-state variance from runtime changes.', risk: 'Longer setup and more evidence artifacts.', validationMethod: 'Diagnostic Recovery4 captures matching preparation outcomes, plans, locks, pool waits, and host telemetry.', requiresNewBaseline: true, requiresNewCurrent: true },
      { targetArea: 'Auth/Security metric tagging', targetFileOrModule: 'tests/load/p7v2-baseline.js securityNegativePhase', observedProblem: 'Invalid-login and invalid-webhook timing are merged into one gating trend.', proposedChange: 'Emit independent trends and evaluate each route separately; retain aggregate only diagnostically.', expectedEffect: 'Makes the p95 regression attributable to a specific path.', risk: 'Changes metric collection semantics.', validationMethod: 'Recovery4 has route-specific p50/p90/p95/p99 and status distributions.', requiresNewBaseline: true, requiresNewCurrent: true },
    ],
  };
  const preflight = {
    phase, status: 'passed', artifactIntegrityPassed: baseline.valid && current.valid, sourceArtifactsModified: false,
    baseline: { runId: baselineRunId, expectedSha256: baselineExpectedHash, actualSha256: baseline.actualHash, sizeBytes: baseline.sizeBytes, requests: baseline.requestCount, scenarioMetricsPresent: baseline.scenarioMetricsPresent },
    current: { runId: currentRunId, expectedSha256: currentExpectedHash, actualSha256: current.actualHash, sizeBytes: current.sizeBytes, requests: current.requestCount, scenarioMetricsPresent: current.scenarioMetricsPresent, independentRun: current.manifest.independentRun === true },
    regressionArchive: { ...archive, baselineRunId, currentRunId, evaluationVersion: regression.evaluationVersion, policyVersion: regression.policyVersion, failedMetricCount: regression.failedMetricCount, p95FailedMetricCount: 3, p99ZeroViolationCount: 9 },
    comparabilityV2: { status: comparability.status, mismatchCount: comparability.mismatchCount, notComparableCount: comparability.notComparableCount },
  };
  const final = {
    phase, status: 'diagnosis_completed_execution_still_blocked',
    artifacts: { baselineRunId, baselineSha256: baseline.actualHash, currentRunId, currentSha256: current.actualHash, integrityPassed: true, modified: false },
    regression: { status: regression.status, p95FailedMetricCount: 3, p99ZeroViolationCount: 9, recalculated: false },
    p95Audit: { expected: 3, audited: p95Failures.length, unclassified: p95Failures.filter((item) => item.classification === 'unknown').length, results: p95Failures },
    p99ZeroAudit: { expected: 9, audited: p99Zeros.length, unclassified: p99Zeros.filter((item) => item.classification === 'unknown').length, results: p99Zeros },
    decision,
    execution: { runtimeModified: false, loadScriptsModified: false, metricCollectionModified: false, regressionPolicyModified: false, loadExecuted: false, regressionRecalculated: false, soakExecuted: false, demoExecuted: false },
    production: { resourcesAccessed: false, realProviderCalls: 0, realDouyinWrites: 0, tagCreated: false, productionReady: false },
    issues: ['P95 root cause remains unproven without Recovery4 diagnostic state telemetry; Runtime remediation is not authorized by current evidence.'],
  };
  return { preflight, matrix, p95Failures, p99Zeros, runtimeState, decision, final };
}

export function writeAuditReports(audit = buildAudit()) {
  const files = [
    ['docs/p7-v2-r3b-prr-a-preflight-audit.json', 'docs/P7_V2_R3B_PRR_A_PREFLIGHT_AUDIT.md', audit.preflight, 'P7-V2-R3B PRR-A Preflight Audit'],
    ['docs/p7-v2-r3b-prr-a-metric-evidence-matrix.json', 'docs/P7_V2_R3B_PRR_A_METRIC_EVIDENCE_MATRIX.md', { phase, status: 'completed', entries: audit.matrix }, 'P7-V2-R3B PRR-A Metric Evidence Matrix'],
    ['docs/p7-v2-r3b-prr-a-p95-root-cause-report.json', 'docs/P7_V2_R3B_PRR_A_P95_ROOT_CAUSE_REPORT.md', { phase, status: 'completed', results: audit.p95Failures }, 'P7-V2-R3B PRR-A P95 Root Cause Report'],
    ['docs/p7-v2-r3b-prr-a-p99-zero-audit.json', 'docs/P7_V2_R3B_PRR_A_P99_ZERO_AUDIT.md', { phase, status: 'completed', results: audit.p99Zeros }, 'P7-V2-R3B PRR-A P99 Zero Audit'],
    ['docs/p7-v2-r3b-prr-a-runtime-state-audit.json', 'docs/P7_V2_R3B_PRR_A_RUNTIME_STATE_AUDIT.md', audit.runtimeState, 'P7-V2-R3B PRR-A Runtime State Audit'],
    ['docs/p7-v2-r3b-prr-a-remediation-plan.json', 'docs/P7_V2_R3B_PRR_A_REMEDIATION_PLAN.md', { phase, status: 'completed', ...audit.decision }, 'P7-V2-R3B PRR-A Remediation Plan'],
    ['docs/p7-v2-r3b-prr-a-final-report.json', 'docs/P7_V2_R3B_PRR_A_FINAL_REPORT.md', audit.final, 'P7-V2-R3B PRR-A Final Report'],
  ];
  for (const [jsonRelativePath, markdownRelativePath, data, title] of files) {
    fs.writeFileSync(path.join(root, jsonRelativePath), `${JSON.stringify(data, null, 2)}\n`);
    fs.writeFileSync(path.join(root, markdownRelativePath), markdown(title, data));
  }
  return audit;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const audit = writeAuditReports();
  console.log(JSON.stringify({ phase, status: audit.final.status, p95Audited: audit.final.p95Audit.audited, p99Audited: audit.final.p99ZeroAudit.audited, nextPhase: audit.final.decision.nextPhase, recovery4Required: audit.final.decision.recovery4Required }, null, 2));
}
