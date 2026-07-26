import fs from 'node:fs';
import { classifyMetricEvidence, evaluateAbsoluteSlo, evaluateTargetReached } from './p7-v2-soak-semantics.mjs';
import { SCENARIO_METRICS } from './p7-v2-regression-metrics.mjs';
import { root, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import { jsonHash } from './p7-v2-r3-lib.mjs';

const read = (rel) => fs.readFileSync(`${root}/${rel}`, 'utf8');
const baselineSource = read('tests/load/p7v2-baseline.js');
const soakSource = read('tests/load/p7v2-soak.js');
const formalSource = read('tests/load/lib/formal-metrics.js');
const wrapperSource = read('scripts/p7-v2-soak.mjs');
const loadSource = read('scripts/p7-v2-load.mjs');
const semanticsSource = read('scripts/p7-v2-soak-semantics.mjs');

const trend = (p95 = 10) => ({ values: { avg: 5, med: 5, 'p(95)': p95, 'p(99)': p95 + 1, max: p95 + 2 } });
const counter = (count) => ({ values: { count, rate: count / 60 } });
const cases = {
  missingMetricNotZero: classifyMetricEvidence({ rawMetric: undefined, sampleMetric: counter(200) }).classification === 'metric_missing',
  zeroSampleNotSuccess: classifyMetricEvidence({ rawMetric: trend(), sampleMetric: counter(0) }).classification === 'insufficient_samples',
  insufficientSampleNotSuccess: classifyMetricEvidence({ rawMetric: trend(), sampleMetric: counter(99) }).classification === 'insufficient_samples',
  absoluteSloMissingNotFalse: evaluateAbsoluteSlo({ rawMetric: undefined, sampleMetric: counter(200), threshold: 800 }).evaluationStatus === 'not_evaluable_metric_missing',
  absoluteSloRealFailureSeparated: evaluateAbsoluteSlo({ rawMetric: trend(901), sampleMetric: counter(200), threshold: 800 }).realAbsoluteSloFailure === true,
  targetReachedSplit: evaluateTargetReached({
    loadTargetReached: true,
    steadyStageEntered: true,
    steadyStageCompleted: true,
    steadyDurationReached: true,
    scenarioCoverageReached: true,
    sampleTargetReached: true,
    sloEvaluationCompleted: true,
  }).targetReached === true,
};

const formalMetricSetMatchesRegressionMatrix = Object.values(SCENARIO_METRICS)
  .every(([durationMetric, requestMetric]) => {
    const scenarioId = durationMetric.replace(/^p7_/, '').replace(/_steady_duration$/, '');
    return requestMetric === `p7_${scenarioId}_steady_requests` && formalSource.includes(`scenarioId: '${scenarioId}'`);
  }) &&
  formalSource.includes('steadyDurationMetricName: `p7_${definition.scenarioId}_steady_duration`') &&
  formalSource.includes('steadyRequestMetricName: `p7_${definition.scenarioId}_steady_requests`');
const checks = {
  sharedFormalMetricRegistryIntroduced: formalSource.includes('formalRouteMetricDefinitions') && formalSource.includes('createFormalRouteMetrics'),
  baselineUsesSharedRegistry: baselineSource.includes("from './lib/formal-metrics.js'") && !/new\s+(Trend|Counter)\(\s*['"]p7_/.test(baselineSource),
  soakUsesSharedRegistry: soakSource.includes("from './lib/formal-metrics.js'") && !/new\s+(Trend|Counter)\(\s*['"]p7_/.test(soakSource),
  formalMetricSetMatchesRegressionMatrix,
  soakRecordsAllFormalMetrics: [
    'productList',
    'orderList',
    'inventoryList',
    'taskList',
    'webhookEventList',
    'operationLogList',
    'webhookIngestion',
    'providerMockFlow',
    'authInvalidLogin',
    'webhookInvalidSignature',
  ].every((metricId) => soakSource.includes(metricId)),
  missingMetricNotZero: cases.missingMetricNotZero,
  zeroSampleNotSuccess: cases.zeroSampleNotSuccess,
  insufficientSampleNotSuccess: cases.insufficientSampleNotSuccess,
  absoluteSloMissingNotFalse: cases.absoluteSloMissingNotFalse,
  absoluteSloRealFailureSeparated: cases.absoluteSloRealFailureSeparated,
  absoluteSloEvaluationStatusExported: loadSource.includes('absoluteSloEvaluationStatus') && loadSource.includes('sloEvaluations'),
  targetReachedSplit: cases.targetReachedSplit && loadSource.includes('targetReachedComponents'),
  wrapperSelfReferenceFixed: !/cooldown\.(goroutines|mockProviderState|circuitState)\.recovered/.test(wrapperSource),
  wrapperAutoExitEvidenceContract: wrapperSource.includes('wrapperExitedAutomatically: true') && wrapperSource.includes('manualStopRequired: false'),
  processExitAvoidedInWrapper: !/process\.exit\(/.test(wrapperSource),
};
const failedChecks = Object.entries(checks).filter(([, passed]) => passed !== true).map(([name]) => name);
const report = {
  phase: 'P7-V2-R3B-SOAK-SEMANTICS-FIX',
  status: failedChecks.length ? 'blocked' : 'passed',
  stage: 'A',
  diagnosticOnly: true,
  formalExecutionStarted: false,
  newRuntimeFreezeCreated: false,
  oldRecovery6PairReusableForFinalClosure: false,
  requiredNewFreezeAfterFix: true,
  formalMetricRegistryHash: jsonHash(formalSource),
  soakSemanticsHash: jsonHash(semanticsSource),
  checks,
  failedChecks,
  scenarioMetricCount: Object.keys(SCENARIO_METRICS).length,
};

const probeReport = {
  phase: 'P7-V2-R3B-SOAK-SEMANTICS-PROBE',
  status: report.status,
  diagnosticOnly: true,
  runtimeProbeExecuted: false,
  shortProbePassed: report.status === 'passed',
  probeType: 'deterministic_semantics_fixture_probe',
  note: 'This Stage A probe validates metric binding and evaluator semantics without starting formal runtime execution.',
  checks: cases,
  formalMetricRegistryHash: report.formalMetricRegistryHash,
};

writeJSON('docs/p7-v2-r3b-soak-semantics-fix-report.json', report);
writeJSON('docs/p7-v2-r3b-soak-semantics-probe-report.json', probeReport);
writeMarkdown('docs/P7_V2_R3B_SOAK_SEMANTICS_FIX_REPORT.md', `# P7-V2 R3B Soak Semantics Fix Report

Status: ${report.status}

- Stage: A
- Formal execution started: false
- New runtime freeze created: false
- Old Recovery6 pair reusable for final closure: false
- Required new freeze after fix: true
- Failed checks: ${failedChecks.length ? failedChecks.join(', ') : 'none'}
`);
writeMarkdown('docs/P7_V2_R3B_SOAK_SEMANTICS_PROBE_REPORT.md', `# P7-V2 R3B Soak Semantics Probe Report

Status: ${probeReport.status}

- Diagnostic only: true
- Runtime probe executed: false
- Short probe passed: ${probeReport.shortProbePassed}
- Probe type: ${probeReport.probeType}
`);

console.log(JSON.stringify({ status: report.status, report: 'docs/p7-v2-r3b-soak-semantics-fix-report.json', probe: 'docs/p7-v2-r3b-soak-semantics-probe-report.json' }, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
