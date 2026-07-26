import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const evidencePath = 'docs/p7-v2-r3b-auth-operation-log-tail-repair.json';
const evidence = readJSON(evidencePath) || {};

const checks = [
  ['rootCause', evidence.primaryRootCause === 'B_auth_audit_or_operation_log_db_tail'],
  ['repairPath', evidence.repairPath === 'auth_operation_log_hash_chain_or_commit_path_minimal_fix'],
  ['queryBudgetEvidencePassed', evidence.queryBudgetEvidencePassed === true],
  ['explainEvidencePassed', evidence.explainEvidencePassed === true],
  ['benchmarkEvidencePassed', evidence.benchmarkEvidencePassed === true],
  ['hashChainSemanticsUnchanged', evidence.hashChainSemanticsUnchanged === true],
  ['securityAuditSemanticsUnchanged', evidence.securityAuditSemanticsUnchanged === true],
  ['failedLoginSemanticsUnchanged', evidence.failedLoginSemanticsUnchanged === true],
  ['transactionSemanticsUnchanged', evidence.transactionSemanticsUnchanged === true],
  ['hashChainConcurrencyPassed', evidence.hashChainConcurrencyPassed === true],
  ['rollbackTestsPassed', evidence.rollbackTestsPassed === true],
  ['fullGoRacePassed', evidence.fullGoRacePassed === true],
  ['dataRaces', evidence.dataRaces === 0],
  ['formalDiagnosticWriterDisabled', evidence.formalDiagnosticWriterDisabled === true],
  ['thresholdChanged', evidence.guardrails?.thresholdChanged === false],
  ['sloChanged', evidence.guardrails?.sloChanged === false],
  ['materialityChanged', evidence.guardrails?.materialityChanged === false],
  ['vusChanged', evidence.guardrails?.vusChanged === false],
  ['stagesChanged', evidence.guardrails?.stagesChanged === false],
  ['datasetChanged', evidence.guardrails?.datasetChanged === false],
  ['formalRerunStarted', evidence.formalRerunStarted === false],
];

const failedChecks = checks.filter(([, passed]) => !passed).map(([id]) => id);
const report = {
  phase: 'P7-V2-R3B-AUTH-OPERATION-LOG-TAIL-MINIMAL-REPAIR-FINAL-GATE',
  component: 'auth-operation-log-tail-repair-final-gate',
  status: failedChecks.length ? 'failed' : 'passed',
  failed: failedChecks.length,
  failedChecks,
  sourceReport: evidencePath,
  rootCause: evidence.primaryRootCause || '',
  repairPath: evidence.repairPath || '',
  queryBudgetEvidencePassed: evidence.queryBudgetEvidencePassed === true,
  explainEvidencePassed: evidence.explainEvidencePassed === true,
  benchmarkEvidencePassed: evidence.benchmarkEvidencePassed === true,
  hashChainSemanticsUnchanged: evidence.hashChainSemanticsUnchanged === true,
  securityAuditSemanticsUnchanged: evidence.securityAuditSemanticsUnchanged === true,
  failedLoginSemanticsUnchanged: evidence.failedLoginSemanticsUnchanged === true,
  transactionSemanticsUnchanged: evidence.transactionSemanticsUnchanged === true,
  hashChainConcurrencyPassed: evidence.hashChainConcurrencyPassed === true,
  rollbackTestsPassed: evidence.rollbackTestsPassed === true,
  fullGoRacePassed: evidence.fullGoRacePassed === true,
  dataRaces: evidence.dataRaces,
  formalDiagnosticWriterDisabled: evidence.formalDiagnosticWriterDisabled === true,
  thresholdChanged: evidence.guardrails?.thresholdChanged !== false,
  sloChanged: evidence.guardrails?.sloChanged !== false,
  materialityChanged: evidence.guardrails?.materialityChanged !== false,
  vusChanged: evidence.guardrails?.vusChanged !== false,
  stagesChanged: evidence.guardrails?.stagesChanged !== false,
  datasetChanged: evidence.guardrails?.datasetChanged !== false,
  formalRerunStarted: evidence.formalRerunStarted === true,
  checks: checks.map(([id, passed]) => ({ id, status: passed ? 'passed' : 'failed' })),
};

writeJSON('docs/p7-v2-r3b-auth-operation-log-tail-repair-final-gate.json', report);
writeMarkdown(
  'docs/P7_V2_R3B_AUTH_OPERATION_LOG_TAIL_REPAIR_FINAL_GATE.md',
  `# P7-V2-R3B Auth Operation Log Tail Repair Final Gate

Status: **${report.status}**

- Failed checks: ${report.failed}
- Root cause: \`${report.rootCause || 'unset'}\`
- Repair path: \`${report.repairPath || 'unset'}\`
- Full Go race passed: ${report.fullGoRacePassed}
- Data races: ${report.dataRaces}
- Formal rerun started: ${report.formalRerunStarted}

This gate covers only the local repair evidence. It does not pass P7-V2, the formal pair, soak, demo, stability, cleanup, final gates, or P7 Development Closure.

## Failed Checks

${failedChecks.length ? failedChecks.map((item) => `- ${item}`).join('\n') : '- none'}
`,
);

console.log(JSON.stringify(report, null, 2));
process.exit(failedChecks.length ? 1 : 0);
