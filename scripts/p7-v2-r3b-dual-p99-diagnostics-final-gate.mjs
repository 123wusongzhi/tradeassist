import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const reportPath = 'docs/p7-v2-r3b-dual-p99-low-cardinality-diagnostics.json';
const report = readJSON(reportPath) || {};

const checks = [
  ['formalRegistryWriteDisabled', report.formalRegistryWriteDisabled === true],
  ['validForClosure', report.validForClosure === false],
  ['diagnosticPairCompleted', report.diagnosticPairCompleted === true],
  ['diagnosticRunsIndependent', report.diagnosticRunsIndependent === true],
  ['datasetRows', report.datasetRows === 1900150],
  ['webhookStageCoveragePassed', report.webhookStageCoveragePassed === true],
  ['authStageCoveragePassed', report.authStageCoveragePassed === true],
  ['highCardinalityMetricLabelCount', report.highCardinalityMetricLabelCount === 0],
  ['diagnosticDropsReported', report.diagnosticDropsReported === true],
  ['dbPoolEvidenceCollected', report.dbPoolEvidenceCollected === true],
  ['runtimeEvidenceCollected', report.runtimeEvidenceCollected === true],
  ['tailCorrelationEvaluated', report.tailCorrelationEvaluated === true],
  ['rootCauseClassified', typeof report.primaryRootCause === 'string' && report.primaryRootCause.length > 0],
  ['repairPathSelected', typeof report.recommendedRepairPath === 'string' && report.recommendedRepairPath.length > 0],
  ['thresholdChanged', report.guardrails?.thresholdChanged === false],
  ['sloChanged', report.guardrails?.sloChanged === false],
  ['materialityChanged', report.guardrails?.materialityChanged === false],
  ['vusChanged', report.guardrails?.vusChanged === false],
  ['stagesChanged', report.guardrails?.stagesChanged === false],
  ['datasetChanged', report.guardrails?.datasetChanged === false],
  ['formalRerunStarted', report.formalRerunStarted === false],
];

const failed = checks.filter(([, passed]) => !passed).map(([id]) => id);
const gate = {
  phase: 'P7-V2-R3B-DUAL-P99-DIAGNOSTICS-FINAL-GATE',
  status: failed.length ? 'failed' : 'passed',
  failed,
  failedCount: failed.length,
  sourceReport: reportPath,
  formal: false,
  validForClosure: false,
  formalRerunStarted: false,
  diagnosticBaselineRunId: report.diagnosticBaselineRunId || '',
  diagnosticCurrentRunId: report.diagnosticCurrentRunId || '',
  primaryRootCause: report.primaryRootCause || '',
  repairPath: report.recommendedRepairPath || '',
  checks: checks.map(([id, passed]) => ({ id, status: passed ? 'passed' : 'failed' })),
};

writeJSON('docs/p7-v2-r3b-dual-p99-diagnostics-final-gate.json', gate);
writeMarkdown('docs/P7_V2_R3B_DUAL_P99_DIAGNOSTICS_FINAL_GATE.md', `# P7-V2-R3B Dual p99 Diagnostics Final Gate

Status: **${gate.status}**

- Formal: false
- Valid for closure: false
- Formal rerun started: false
- Diagnostic baseline run: \`${gate.diagnosticBaselineRunId || 'not_run'}\`
- Diagnostic current run: \`${gate.diagnosticCurrentRunId || 'not_run'}\`
- Failed checks: ${failed.length ? failed.join(', ') : 'none'}

Diagnostic gate passing does not mean P7 passed.
`);

console.log(JSON.stringify(gate, null, 2));
process.exit(failed.length ? 1 : 0);
