import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const fix = readJSON('docs/p7-v2-r3b-soak-semantics-fix-report.json') || {};
const probe = readJSON('docs/p7-v2-r3b-soak-semantics-probe-report.json') || {};
const requiredTrue = [
  ['fix.status.passed', fix.status === 'passed'],
  ['fix.sharedFormalMetricRegistryIntroduced', fix.checks?.sharedFormalMetricRegistryIntroduced === true],
  ['fix.baselineUsesSharedRegistry', fix.checks?.baselineUsesSharedRegistry === true],
  ['fix.soakUsesSharedRegistry', fix.checks?.soakUsesSharedRegistry === true],
  ['fix.formalMetricSetMatchesRegressionMatrix', fix.checks?.formalMetricSetMatchesRegressionMatrix === true],
  ['fix.missingMetricNotZero', fix.checks?.missingMetricNotZero === true],
  ['fix.zeroSampleNotSuccess', fix.checks?.zeroSampleNotSuccess === true],
  ['fix.insufficientSampleNotSuccess', fix.checks?.insufficientSampleNotSuccess === true],
  ['fix.absoluteSloMissingNotFalse', fix.checks?.absoluteSloMissingNotFalse === true],
  ['fix.absoluteSloRealFailureSeparated', fix.checks?.absoluteSloRealFailureSeparated === true],
  ['fix.absoluteSloEvaluationStatusExported', fix.checks?.absoluteSloEvaluationStatusExported === true],
  ['fix.targetReachedSplit', fix.checks?.targetReachedSplit === true],
  ['fix.wrapperSelfReferenceFixed', fix.checks?.wrapperSelfReferenceFixed === true],
  ['fix.wrapperAutoExitEvidenceContract', fix.checks?.wrapperAutoExitEvidenceContract === true],
  ['fix.processExitAvoidedInWrapper', fix.checks?.processExitAvoidedInWrapper === true],
  ['probe.shortProbePassed', probe.shortProbePassed === true],
  ['noFormalExecutionStarted', fix.formalExecutionStarted === false],
  ['noNewRuntimeFreezeCreated', fix.newRuntimeFreezeCreated === false],
];
const failed = requiredTrue.filter(([, passed]) => !passed).map(([name]) => name);
const report = {
  phase: 'P7-V2-R3B-SOAK-SEMANTICS-FINAL-GATE',
  status: failed.length ? 'blocked' : 'passed',
  stageAComplete: failed.length === 0,
  failed,
  checkedReports: [
    'docs/p7-v2-r3b-soak-semantics-fix-report.json',
    'docs/p7-v2-r3b-soak-semantics-probe-report.json',
  ],
  formalExecutionStarted: false,
  newRuntimeFreezeCreated: false,
};

writeJSON('docs/p7-v2-r3b-soak-semantics-final-gate.json', report);
writeMarkdown('docs/P7_V2_R3B_SOAK_SEMANTICS_FINAL_GATE.md', `# P7-V2 R3B Soak Semantics Final Gate

Status: ${report.status}

- Stage A complete: ${report.stageAComplete}
- Formal execution started: false
- New runtime freeze created: false
- Failed checks: ${failed.length ? failed.join(', ') : 'none'}
`);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
