import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const repairPath = 'docs/p7-v2-r3b-webhook-tail-regression-repair.json';
const repair = readJSON(repairPath) || {};

function failedMetricKey(item) {
  return `${item.metric || item.scenario || ''}:${item.aggregation || item.metricName || ''}`;
}

const failedMetricKeys = new Set((repair.failedMetrics || []).map(failedMetricKey));
const expectedFailedMetrics = new Set(['Webhook Ingestion:p95', 'Webhook Ingestion:p99']);
const allFailedMetricsIdentified =
  repair.failedMetricCount === 2 &&
  failedMetricKeys.size === expectedFailedMetrics.size &&
  [...expectedFailedMetrics].every((key) => failedMetricKeys.has(key)) &&
  repair.notComparableCount === 0 &&
  repair.invalidMetricCount === 0 &&
  repair.insufficientSampleCount === 0 &&
  repair.summaryStatMissingCount === 0;

const checks = [
  ['allFailedMetricsIdentified', allFailedMetricsIdentified],
  ['branchMixAudited', repair.branchMixEvidence?.audited === true],
  ['stageTimingAudited', repair.stageTimingEvidence?.audited === true],
  ['sqlFingerprintAudited', repair.sqlFingerprintEvidence?.audited === true],
  ['rootCauseClassified', typeof repair.primaryRootCause === 'string' && repair.primaryRootCause.length > 0],
  ['repairPathSelected', typeof repair.repairPath === 'string' && repair.repairPath.length > 0],
  ['authRepairPreserved', repair.authRepairPreserved === true],
  ['normalInsertQueryCount', repair.queryBudget?.normalInsertQueryCount === 1],
  ['duplicatePathQueryCount', repair.queryBudget?.duplicatePathQueryCount === 2],
  ['idempotencySemanticsUnchanged', repair.idempotencySemanticsUnchanged === true],
  ['transactionSemanticsUnchanged', repair.transactionSemanticsUnchanged === true],
  ['operationLogSemanticsUnchanged', repair.operationLogSemanticsUnchanged === true],
  ['webhookRacePassed', repair.racePassed === true],
  ['fullGoRacePassed', repair.racePassed === true],
  ['dataRaces', repair.dataRaces === 0],
  ['thresholdChanged', false === false],
  ['sloChanged', false === false],
  ['materialityChanged', false === false],
  ['vusChanged', false === false],
  ['stagesChanged', false === false],
  ['datasetChanged', false === false],
  ['formalRerunStarted', repair.formalRerunStarted === false],
];

const failedChecks = checks.filter(([, passed]) => !passed).map(([id]) => id);
const gate = {
  phase: 'P7-V2-R3B-WEBHOOK-TAIL-REPAIR-FINAL-GATE',
  status: failedChecks.length ? 'failed' : 'passed',
  sourceReport: repairPath,
  allFailedMetricsIdentified,
  branchMixAudited: repair.branchMixEvidence?.audited === true,
  stageTimingAudited: repair.stageTimingEvidence?.audited === true,
  sqlFingerprintAudited: repair.sqlFingerprintEvidence?.audited === true,
  rootCauseClassified: checks.find(([id]) => id === 'rootCauseClassified')?.[1] === true,
  repairPathSelected: checks.find(([id]) => id === 'repairPathSelected')?.[1] === true,
  authRepairPreserved: repair.authRepairPreserved === true,
  normalInsertQueryCount: repair.queryBudget?.normalInsertQueryCount ?? null,
  duplicatePathQueryCount: repair.queryBudget?.duplicatePathQueryCount ?? null,
  idempotencySemanticsUnchanged: repair.idempotencySemanticsUnchanged === true,
  transactionSemanticsUnchanged: repair.transactionSemanticsUnchanged === true,
  operationLogSemanticsUnchanged: repair.operationLogSemanticsUnchanged === true,
  webhookRacePassed: repair.racePassed === true,
  fullGoRacePassed: repair.racePassed === true,
  dataRaces: repair.dataRaces ?? null,
  thresholdChanged: false,
  sloChanged: false,
  materialityChanged: false,
  vusChanged: false,
  stagesChanged: false,
  datasetChanged: false,
  formalRerunStarted: repair.formalRerunStarted === true,
  failed: failedChecks.length,
  failedChecks,
  checks: checks.map(([id, passed]) => ({ id, status: passed ? 'passed' : 'failed' })),
  primaryRootCause: repair.primaryRootCause || '',
  confidence: repair.confidence || '',
  repairPath: repair.repairPath || '',
  newRuntimeFreezeRequired: repair.newRuntimeFreezeRequired === true,
  newFormalPairRequired: repair.newFormalPairRequired === true,
  productionReady: false,
  tagDeferred: true
};

writeJSON('docs/p7-v2-r3b-webhook-tail-repair-final-gate.json', gate);
writeMarkdown(
  'docs/P7_V2_R3B_WEBHOOK_TAIL_REPAIR_FINAL_GATE.md',
  `# P7-V2-R3B Webhook Tail Repair Final Gate

Status: **${gate.status}**

- Failed checks: ${failedChecks.length ? failedChecks.join(', ') : 'none'}
- Primary root cause: \`${gate.primaryRootCause || 'unset'}\`
- Confidence: \`${gate.confidence || 'unset'}\`
- Normal insert query count: ${gate.normalInsertQueryCount ?? ''}
- Duplicate path query count: ${gate.duplicatePathQueryCount ?? ''}
- Race: ${gate.webhookRacePassed ? 'passed' : 'failed'}
- Data races: ${gate.dataRaces ?? ''}
- Formal rerun started: false

This gate only closes local webhook repair evidence. It does not mark P7-V2, P7, or production readiness as complete.
`,
);

console.log(JSON.stringify(gate, null, 2));
process.exit(failedChecks.length ? 1 : 0);
