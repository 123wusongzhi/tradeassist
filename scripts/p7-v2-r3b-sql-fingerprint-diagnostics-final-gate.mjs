import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const reportPath = 'docs/p7-v2-r3b-sql-fingerprint-pg-wait-diagnostics.json';
const report = readJSON(reportPath) || {};

const checks = [
  ['diagnosticPairIndependent', report.diagnosticRunsIndependent === true],
  ['datasetRows', report.datasetRows === 1900150],
  ['authStageCoveragePassed', report.authStageCoveragePassed === true],
  ['webhookStageCoveragePassed', report.webhookStageCoveragePassed === true],
  ['sqlFingerprintCoveragePassed', report.sqlFingerprintCoveragePassed === true],
  ['pgWaitEvidenceCollected', report.pgWaitEvidenceCollected === true],
  ['dbPoolEvidenceCollected', report.dbPoolEvidenceCollected === true],
  ['rawSqlParameterLeakCount', report.rawSqlParameterLeakCount === 0],
  ['credentialLeakCount', report.credentialLeakCount === 0],
  ['highCardinalityLabelCount', report.highCardinalityLabelCount === 0],
  ['rootCauseClassified', report.rootCauseClassified === true && typeof report.primaryRootCause === 'string' && report.primaryRootCause.length > 0],
  ['repairPathSelected', report.repairPathSelected === true && typeof report.repairPath === 'string' && report.repairPath.length > 0],
  ['formal', report.formal === false],
  ['validForClosure', report.validForClosure === false],
  ['formalRerunStarted', report.formalRerunStarted === false],
  ['thresholdChanged', report.guardrails?.thresholdChanged === false],
  ['sloChanged', report.guardrails?.sloChanged === false],
  ['vusChanged', report.guardrails?.vusChanged === false],
  ['stagesChanged', report.guardrails?.stagesChanged === false],
  ['datasetChanged', report.guardrails?.datasetChanged === false],
];

const failed = checks.filter(([, passed]) => !passed).map(([id]) => id);
const gate = {
  phase: 'P7-V2-R3B-SQL-FINGERPRINT-DIAGNOSTICS-FINAL-GATE',
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
  repairPath: report.repairPath || report.recommendedRepairPath || '',
  checks: checks.map(([id, passed]) => ({ id, status: passed ? 'passed' : 'failed' })),
};

writeJSON('docs/p7-v2-r3b-sql-fingerprint-diagnostics-final-gate.json', gate);
writeMarkdown(
  'docs/P7_V2_R3B_SQL_FINGERPRINT_DIAGNOSTICS_FINAL_GATE.md',
  `# P7-V2-R3B SQL Fingerprint Diagnostics Final Gate

Status: **${gate.status}**

- Formal: false
- Valid for closure: false
- Formal rerun started: false
- Diagnostic baseline run: \`${gate.diagnosticBaselineRunId || 'not_run'}\`
- Diagnostic current run: \`${gate.diagnosticCurrentRunId || 'not_run'}\`
- Primary root cause: \`${gate.primaryRootCause || 'unset'}\`
- Failed checks: ${failed.length ? failed.join(', ') : 'none'}

Diagnostic gate passing does not mean P7 passed.
`,
);

console.log(JSON.stringify(gate, null, 2));
process.exit(failed.length ? 1 : 0);
