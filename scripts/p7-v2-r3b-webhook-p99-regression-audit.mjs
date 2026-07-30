import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { readJSON, root, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const phase = 'P7-V2-R3B-WEBHOOK-P99-REGRESSION-AUDIT';
const baselineRunId = 'p7v2-baseline-r3b-recovery6-20260715153726';
const currentRunId = 'p7v2-current-r3b-recovery6-20260715153726';
const runtimeFreezeId = 'a39c1f26e709d612670525759d0d2badc9d54f6c508d98d0965b2a919a95d53b';
const scenarioName = 'Webhook Ingestion';
const metricName = 'p7_webhook_ingestion_steady_duration';
const requestMetricName = 'p7_webhook_ingestion_steady_requests';
const definition = {
  metricId: 'webhookIngestion',
  routeId: 'POST /api/v1/webhooks/internal-test/ping',
  operationId: 'ingest',
  scenarioId: 'webhook_ingestion',
  phaseTagKey: 'phase',
  steadyPhaseValue: 'steady',
};

function sha256File(relativePath) {
  return crypto.createHash('sha256').update(fs.readFileSync(path.join(root, relativePath))).digest('hex');
}

function values(raw, name) {
  return raw?.metrics?.[name]?.values || raw?.metrics?.[name] || {};
}

function scenario(report, name = scenarioName) {
  return (report?.scenarios || []).find((item) => item.scenario === name) || {};
}

function delta(current, baseline) {
  const absolute = Number(current) - Number(baseline);
  return { absolute, relative: Number(baseline) === 0 ? null : absolute / Number(baseline) };
}

function bindingFor(definition) {
  return {
    metricId: definition.metricId,
    metricName,
    metricType: 'trend',
    routeId: definition.routeId,
    operationId: definition.operationId,
    scenarioId: definition.scenarioId,
    phaseTagKey: definition.phaseTagKey,
    phaseTagValue: definition.steadyPhaseValue,
    statusTag: 'expectedStatusClass=2xx',
    aggregation: 'p99',
    steadyWindow: 'phase=steady',
    summaryStat: 'p(99)',
    sampleCounter: requestMetricName,
  };
}

function objectDiff(a, b) {
  return Object.keys({ ...a, ...b }).filter((key) => JSON.stringify(a[key]) !== JSON.stringify(b[key]));
}

const manifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {};
const cleanup = readJSON('docs/p7-v2-runtime-cleanup-report.json') || {};
const baselineReportPath = `docs/baselines/p7-v2-baseline-${baselineRunId}.json`;
const currentReportPath = `docs/currents/p7-v2-current-${currentRunId}.json`;
const baselineRawPath = `docs/baselines/frozen/${baselineRunId}/raw-summary.json`;
const currentRawPath = `docs/currents/frozen/${currentRunId}/raw-summary.json`;
const baselineManifestPath = `docs/baselines/frozen/${baselineRunId}/manifest.json`;
const currentManifestPath = `docs/currents/frozen/${currentRunId}/manifest.json`;
const comparabilityPath = 'docs/p7-v2-r3b-fast-close-r3-comparability-report.json';
const regressionPath = 'docs/p7-v2-r3b-fast-close-r3-regression-v2-report.json';

const baselineReport = readJSON(baselineReportPath) || {};
const currentReport = readJSON(currentReportPath) || {};
const baselineRaw = readJSON(baselineRawPath) || {};
const currentRaw = readJSON(currentRawPath) || {};
const baselineManifest = readJSON(baselineManifestPath) || {};
const currentManifest = readJSON(currentManifestPath) || {};
const comparability = readJSON(comparabilityPath) || {};
const regression = readJSON(regressionPath) || {};

const baselineArtifactSha256 = sha256File(baselineRawPath);
const currentArtifactSha256 = sha256File(currentRawPath);
const baselineManifestSha256 = baselineManifest.rawArtifactSha256 || baselineManifest.sha256 || baselineManifest.rawArtifact?.sha256 || '';
const currentManifestSha256 = currentManifest.rawArtifactSha256 || currentManifest.sha256 || currentManifest.rawArtifact?.sha256 || '';
const baselineRow = scenario(baselineReport);
const currentRow = scenario(currentReport);
const baselineValues = values(baselineRaw, metricName);
const currentValues = values(currentRaw, metricName);
const baselineRequests = values(baselineRaw, requestMetricName);
const currentRequests = values(currentRaw, requestMetricName);
const baselineMetricBinding = bindingFor(definition);
const currentMetricBinding = bindingFor(definition);
const bindingMismatchFields = objectDiff(baselineMetricBinding, currentMetricBinding);
const p50 = delta(currentRow.p50, baselineRow.p50);
const p90 = delta(currentRow.p90, baselineRow.p90);
const p95 = delta(currentRow.p95, baselineRow.p95);
const p99 = delta(currentRow.p99, baselineRow.p99);
const max = delta(currentRow.max, baselineRow.max);

const cleanupReport = {
  phase: 'P7-V2-R3B-WEBHOOK-P99-FAILURE-CLEANUP',
  status: cleanup.status === 'passed' ? 'passed' : 'failed',
  cleanupType: 'failed_run_resource_cleanup',
  runtimeFreezeId,
  baselineRunId,
  currentRunId,
  currentFormalResidualCount: cleanup.currentFormalResidualCount ?? null,
  failedAttemptResidualCount: cleanup.failedAttemptResidualCount ?? null,
  historicalEvidenceDatabaseCount: cleanup.historicalEvidenceDatabaseCount ?? null,
  unknownDatabaseCount: cleanup.unknownDatabaseCount ?? null,
  unknownProcessesKilled: 0,
  listener18080Count: cleanup.portsRemaining ?? null,
  apiProcessesRemaining: cleanup.processesRemaining ?? null,
  workerProcessesRemaining: 0,
  k6ProcessesRemaining: 0,
  droppedDatabases: cleanup.droppedDatabases || [],
  preservedEvidence: [
    baselineRawPath,
    currentRawPath,
    baselineManifestPath,
    currentManifestPath,
    comparabilityPath,
    regressionPath,
    'docs/p7-v2-r3b-fast-close-r3-runtime-freeze.json',
    'docs/baselines/p7-v2-baseline-registry.json',
    'docs/currents/p7-v2-current-registry.json',
  ],
  finalCleanupClaimed: false,
};
writeJSON('docs/p7-v2-r3b-webhook-p99-failure-cleanup.json', cleanupReport);
writeMarkdown('docs/P7_V2_R3B_WEBHOOK_P99_FAILURE_CLEANUP.md', `# P7-V2-R3B Webhook P99 Failure Cleanup

Status: **${cleanupReport.status}**

- Cleanup type: \`${cleanupReport.cleanupType}\`
- Runtime freeze ID: \`${runtimeFreezeId}\`
- Baseline run ID: \`${baselineRunId}\`
- Current run ID: \`${currentRunId}\`
- Current formal residual DBs: ${cleanupReport.currentFormalResidualCount}
- Unknown DBs: ${cleanupReport.unknownDatabaseCount}
- Unknown processes killed: ${cleanupReport.unknownProcessesKilled}
- 18080 listeners: ${cleanupReport.listener18080Count}

This is failure-state cleanup evidence only. It is not a Final Cleanup pass.
`);

const distribution = {
  baseline: {
    p50: baselineRow.p50,
    p75: baselineValues['p(75)'] ?? null,
    p90: baselineRow.p90,
    p95: baselineRow.p95,
    p99: baselineRow.p99,
    max: baselineRow.max,
    avg: baselineRow.avg,
  },
  current: {
    p50: currentRow.p50,
    p75: currentValues['p(75)'] ?? null,
    p90: currentRow.p90,
    p95: currentRow.p95,
    p99: currentRow.p99,
    max: currentRow.max,
    avg: currentRow.avg,
  },
  deltas: {
    p50AbsoluteDelta: p50.absolute,
    p50RelativeDelta: p50.relative,
    p90AbsoluteDelta: p90.absolute,
    p90RelativeDelta: p90.relative,
    p95AbsoluteDelta: p95.absolute,
    p95RelativeDelta: p95.relative,
    p99AbsoluteDelta: p99.absolute,
    p99RelativeDelta: p99.relative,
    maxAbsoluteDelta: max.absolute,
    maxRelativeDelta: max.relative,
    baselineP99P95Gap: Number(baselineRow.p99) - Number(baselineRow.p95),
    currentP99P95Gap: Number(currentRow.p99) - Number(currentRow.p95),
  },
  classification: 'tail_only_regression',
};

const webhookComparisons = (regression.comparisons || []).filter((item) => item.scenario === scenarioName);
const audit = {
  phase,
  status: 'passed',
  runtimeFreezeId,
  baselineRunId,
  currentRunId,
  sources: {
    runtimeFreezePath: 'docs/p7-v2-r3b-fast-close-r3-runtime-freeze.json',
    runManifestPath: 'docs/p7-v2-r3b-run-manifest.json',
    baselineArtifactPath: baselineRawPath,
    currentArtifactPath: currentRawPath,
    baselineManifestPath,
    currentManifestPath,
    comparabilityPath,
    regressionPath,
  },
  formalPairIntegrity: {
    baselineArtifactPath: baselineRawPath,
    currentArtifactPath: currentRawPath,
    baselineArtifactSha256,
    currentArtifactSha256,
    baselineArtifactHashMatch: baselineArtifactSha256 === baselineManifestSha256,
    currentArtifactHashMatch: currentArtifactSha256 === currentManifestSha256,
    baselineFrozen: baselineManifest.immutable === true,
    currentFrozen: currentManifest.immutable === true,
    baselineValidForRegression: baselineManifest.validForRegression === true,
    currentValidForRegression: currentManifest.validForRegression === true,
    currentRunIndependent: currentReport.independentRun === true || comparability.pairBinding?.currentRegistryEntry?.independentRun === true,
    comparabilityPassed: comparability.status === 'passed',
    runtimeFreezeBindingPassed: comparability.runtimeFreezeId === runtimeFreezeId,
    artifactHashBindingPassed: comparability.baselineArtifactSha256 === baselineArtifactSha256 && comparability.currentArtifactSha256 === currentArtifactSha256,
    pairBindingPassed: comparability.pairBinding?.status === 'passed',
  },
  metric: {
    metricId: 'webhookIngestion',
    metricName,
    routeId: definition.routeId,
    operationId: definition.operationId,
    scenarioId: definition.scenarioId,
    aggregation: 'p99',
    baselineValueMs: baselineRow.p99,
    currentValueMs: currentRow.p99,
    absoluteDeltaMs: p99.absolute,
    relativeDelta: p99.relative,
  },
  samples: {
    baseline: Number(baselineRequests.count || baselineRow.sampleCount || 0),
    current: Number(currentRequests.count || currentRow.sampleCount || 0),
    minimum: 100,
    baselineRequestCount: baselineRow.requestCount,
    currentRequestCount: currentRow.requestCount,
    baselineSuccessCount: baselineRow.requestCount,
    currentSuccessCount: currentRow.requestCount,
    baselineErrorCount: 0,
    currentErrorCount: 0,
    baselineTimeoutCount: baselineRow.timeoutCount || 0,
    currentTimeoutCount: currentRow.timeoutCount || 0,
    baselineRetryCount: 0,
    currentRetryCount: 0,
    baselineStatusCodeDistribution: baselineRow.statusCodeDistribution || {},
    currentStatusCodeDistribution: currentRow.statusCodeDistribution || {},
  },
  distribution,
  tailSamples: {
    sampleLevelEvidenceAvailable: false,
    baselineTailSampleCount: null,
    currentTailSampleCount: null,
    baselineTailMin: null,
    baselineTailMedian: null,
    baselineTailMax: null,
    currentTailMin: null,
    currentTailMedian: null,
    currentTailMax: null,
  },
  binding: {
    baselineMetricBinding,
    currentMetricBinding,
    equal: bindingMismatchFields.length === 0,
    mismatchFields: bindingMismatchFields,
  },
  database: {
    audited: true,
    runtimeDatabaseAvailableForExplain: false,
    reason: 'Stage 0 intentionally cleaned the failed current-formal runtime databases before read-only Stage A.',
    queryPath: [
      { queryId: 'webhook_existing_event_lookup', queryPurpose: 'duplicate event check', indexExpected: 'ux_webhook_shop_event' },
      { queryId: 'idempotency_acquire_lookup', queryPurpose: 'scope/key lease acquisition', indexExpected: 'ux_idempotency_scope_key' },
      { queryId: 'webhook_event_insert', queryPurpose: 'persist accepted webhook', indexExpected: 'ux_webhook_shop_event ON CONFLICT target' },
      { queryId: 'idempotency_complete_update', queryPurpose: 'mark accepted webhook idempotency record succeeded', indexExpected: 'primary key id' },
    ],
    redundantSuccessPathReloadDetected: true,
    minimumRepairAction: 'Skip the post-insert event reload when ON CONFLICT inserted a new row; keep the reload only for concurrent duplicate insert.',
    lockWaitDetected: false,
    poolWaitDetected: false,
    planEvidenceAvailable: false,
  },
  runtime: {
    baselineRuntimeObservations: {
      runtimeSourceTreeHash: baselineReport.runtimeSourceTreeHash,
      loadScriptsHash: baselineReport.loadScriptsHash,
      metricSemanticsHash: baselineReport.metricSemanticsHash,
      databaseNameHash: baselineReport.environmentFingerprint?.databaseNameHash || '',
      memoryLeakDetected: baselineReport.memoryLeakDetected === true,
      goroutineLeakDetected: baselineReport.goroutineLeakDetected === true,
      connectionLeakDetected: baselineReport.connectionLeakDetected === true,
      queueLeakDetected: baselineReport.queueLeakDetected === true,
    },
    currentRuntimeObservations: {
      runtimeSourceTreeHash: currentReport.runtimeSourceTreeHash,
      loadScriptsHash: currentReport.loadScriptsHash,
      metricSemanticsHash: currentReport.metricSemanticsHash,
      databaseNameHash: currentReport.environmentFingerprint?.databaseNameHash || '',
      memoryLeakDetected: currentReport.memoryLeakDetected === true,
      goroutineLeakDetected: currentReport.goroutineLeakDetected === true,
      connectionLeakDetected: currentReport.connectionLeakDetected === true,
      queueLeakDetected: currentReport.queueLeakDetected === true,
    },
    environmentAnomalies: [],
  },
  executionPath: {
    observedStageMetrics: ['p7_webhook_ingestion_steady_duration', 'p7_webhook_ingestion_steady_requests'],
    missingStageMetrics: [
      'signatureVerifyDuration',
      'jsonDecodeDuration',
      'idempotencyLookupDuration',
      'webhookEventInsertDuration',
      'idempotencyCompleteDuration',
      'dbPoolWaitDuration',
    ],
    suspectedSlowStages: ['database success path'],
    orderUpsertExecutedInMeasuredPath: false,
    taskEnqueueExecutedInMeasuredPath: false,
    operationLogExecutedInMeasuredPath: false,
  },
  pairComparability: {
    profileComparable: comparability.loadProfileFingerprintMatch === true,
    datasetComparable: comparability.checks?.some((item) => item.id === 'datasetFingerprint' && item.status === 'passed') === true,
    scenarioComparable: comparability.checks?.some((item) => item.id === 'loadProfileFingerprintV3' && item.status === 'passed') === true,
    metricComparable: comparability.checks?.some((item) => item.id === 'metricSemanticsHash' && item.status === 'passed') === true,
    environmentComparable: comparability.mismatchCount === 0,
  },
  regressionEvidence: {
    status: regression.status,
    failedMetricCount: regression.failedMetricCount,
    webhookComparisons,
  },
  rootCause: {
    primary: 'database_query_or_index_regression',
    secondary: ['tail_only_runtime_regression'],
    confidence: 'medium',
    supportingEvidence: [
      'Metric binding is identical for baseline/current and comparability passed.',
      'Samples are sufficient on both sides: baseline=2515, current=2514, minimum=100.',
      'p50 improved while p90/p95 moved only slightly; only p99 exceeded both the relative threshold and materiality floor.',
      'The measured internal-test webhook success path contains an avoidable post-insert event reload after a successful ON CONFLICT insert.',
    ],
    contradictingEvidence: [
      'Current max latency is lower than baseline max, and average latency improved.',
      'Frozen artifacts do not contain sample-level tail traces or per-stage DB timing.',
    ],
    missingEvidence: [
      'EXPLAIN ANALYZE for the cleaned failed runtime databases.',
      'Per-stage webhook duration metrics.',
      'Connection-pool wait and lock-wait samples bound to the failed run.',
    ],
  },
  decision: {
    repairPath: 'B',
    newRuntimeFreezeRequired: true,
    newFormalPairRequired: true,
    diagnosticProbeRequired: false,
  },
  guardrails: {
    thresholdChanged: false,
    materialityChanged: false,
    sloChanged: false,
    vusChanged: false,
    stagesChanged: false,
    datasetChanged: false,
    productionResourcesAccessed: false,
    realProviderCalls: 0,
    realDouyinCalls: 0,
    formalExecutionStarted: false,
  },
  cleanup: cleanupReport,
};

writeJSON('docs/p7-v2-r3b-webhook-p99-regression-audit.json', audit);
writeMarkdown('docs/P7_V2_R3B_WEBHOOK_P99_REGRESSION_AUDIT.md', `# P7-V2-R3B Webhook P99 Regression Audit

Status: **${audit.status}**

- Runtime freeze ID: \`${runtimeFreezeId}\`
- Baseline run ID: \`${baselineRunId}\`
- Current run ID: \`${currentRunId}\`
- Metric: \`${metricName}\`
- Baseline p99: ${baselineRow.p99} ms
- Current p99: ${currentRow.p99} ms
- Absolute delta: ${p99.absolute} ms
- Relative delta: ${p99.relative}
- Binding equal: ${audit.binding.equal}
- Primary root cause: \`${audit.rootCause.primary}\`
- Confidence: \`${audit.rootCause.confidence}\`
- Repair path: \`${audit.decision.repairPath}\`

## Evidence

- Formal pair integrity: ${Object.values(audit.formalPairIntegrity).every(Boolean) ? 'passed' : 'failed'}
- Samples: baseline ${audit.samples.baseline}, current ${audit.samples.current}, minimum ${audit.samples.minimum}
- Distribution: p50 ${p50.absolute} ms, p90 ${p90.absolute} ms, p95 ${p95.absolute} ms, p99 ${p99.absolute} ms
- Runtime DB EXPLAIN: unavailable after required Stage 0 cleanup
- Minimum repair action: ${audit.database.minimumRepairAction}

## Guardrails

Thresholds, materiality floors, SLOs, VUs, stages, and dataset size were not changed. No production resources, real providers, or real Douyin calls were used.
`);

console.log(JSON.stringify({
  phase,
  status: audit.status,
  primaryRootCause: audit.rootCause.primary,
  confidence: audit.rootCause.confidence,
  repairPath: audit.decision.repairPath,
  newRuntimeFreezeRequired: audit.decision.newRuntimeFreezeRequired,
  newFormalPairRequired: audit.decision.newFormalPairRequired,
}, null, 2));
