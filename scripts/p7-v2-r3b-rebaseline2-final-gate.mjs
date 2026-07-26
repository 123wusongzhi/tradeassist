import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const runtimeFreeze = readJSON('docs/p7-v2-r3b-rebaseline2-runtime-freeze-report.json') || {};
const baseline = readJSON('docs/p7-v2-r3b-rebaseline2-baseline-report.json') || {};
const current = readJSON('docs/p7-v2-r3b-rebaseline2-current-report.json') || {};
const comparability = readJSON('docs/p7-v2-r3b-rebaseline2-comparability-report.json') || {};
const regression = readJSON('docs/p7-v2-r3b-rebaseline2-regression-v2-report.json') || {};
const checks = [
  ['runtimeFreeze', runtimeFreeze.status === 'passed'],
  ['baseline', baseline.status === 'passed' && baseline.immutable === true && baseline.validForRegression === true && Number(baseline.requests) > 0 && baseline.hashVerified === true],
  ['current', current.status === 'passed' && current.independentRun === true && current.immutable === true && current.validForRegression === true && Number(current.requests) > 0 && current.hashVerified === true],
  ['comparability', comparability.status === 'passed' && Number(comparability.mismatchCount) === 0 && Number(comparability.notComparableCount) === 0],
  ['regression', regression.status === 'passed' && regression.evaluationVersion === 2 && Number(regression.failedMetricCount) === 0 && Number(regression.notComparableCount) === 0 && Number(regression.invalidMetricCount) === 0 && Number(regression.insufficientSampleCount) === 0 && Number(regression.zeroSemanticErrors) === 0],
];
const failed = checks.filter(([, passed]) => !passed).map(([id]) => id);
const report = {
  phase: 'P7-V2-R3B-REBASELINE2',
  status: failed.length === 0 ? 'passed' : 'blocked',
  failed: failed.length,
  checks: checks.map(([id, passed]) => ({ id, status: passed ? 'passed' : 'failed' })),
  next: { soak: 'pending', demoRun1: 'pending', demoRun2: 'pending', finalGates: 'pending' },
  production: { resourcesAccessed: false, realProviderCalls: 0, realDouyinWrites: 0, autoListingTriggered: false, tagCreated: false, productionReady: false },
};
writeJSON('docs/p7-v2-r3b-rebaseline2-final-gate-report.json', report);
writeMarkdown('docs/P7_V2_R3B_REBASELINE2_FINAL_GATE.md', `# P7-V2-R3B-REBASELINE2 Scoped Final Gate\n\nStatus: **${report.status}**\n\nFailed: ${report.failed}\n`);
console.log(JSON.stringify(report, null, 2));
process.exit(failed.length === 0 ? 0 : 1);
