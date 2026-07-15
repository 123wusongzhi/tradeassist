import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import { resolveActiveBaseline, resolveActiveCurrent } from './p7-v2-evidence-resolver.mjs';

const baseline = resolveActiveBaseline();
const current = resolveActiveCurrent();
const restart = readJSON('docs/p7-v2-r3b-ci-restart-stability-report.json') || {};
const reuse = readJSON('docs/p7-v2-r3b-ci-baseline-reuse-decision.json') || {};
const comparability = readJSON('docs/p7-v2-r3b-ci-comparability-report.json') || {};
const regression = readJSON('docs/p7-v2-performance-regression-report.json') || {};
const final = readJSON('docs/p7-v2-r3b-ci-rg-final-report.json') || {};
const checks = [
  ['recoveryBaseline', baseline.valid],
  ['baselineReuse', reuse.status === 'reusable'],
  ['restartStability', Number(restart.passedCycles) === 3 && Number(restart.failedCycles) === 0],
  ['current', current.valid],
  ['comparability', comparability.status === 'passed' && Number(comparability.mismatchCount) === 0 && Number(comparability.notComparableCount) === 0],
  ['regression', regression.status === 'passed' && regression.evaluationVersion === 2 && Number(regression.failedMetricCount) === 0 && Number(regression.notComparableCount) === 0 && Number(regression.invalidMetricCount) === 0 && Number(regression.insufficientSampleCount) === 0],
  ['soakPending', final.next?.soak === 'pending'],
  ['demoPending', final.next?.demoRun1 === 'pending' && final.next?.demoRun2 === 'pending'],
  ['productionGuard', final.production?.resourcesAccessed === false && final.production?.realProviderCalls === 0 && final.production?.realDouyinWrites === 0 && final.production?.tagCreated === false && final.production?.productionReady === false],
];
const failed = checks.filter(([, passed]) => !passed).map(([id]) => id);
const report = { phase: 'P7-V2-R3B-CI-RG', status: failed.length === 0 ? 'passed' : 'blocked', failed: failed.length, checks: checks.map(([id, passed]) => ({ id, status: passed ? 'passed' : 'failed' })) };
writeJSON('docs/p7-v2-r3b-ci-rg-final-gate-report.json', report);
writeMarkdown('docs/P7_V2_R3B_CI_RG_FINAL_GATE.md', `# P7-V2-R3B-CI-RG Final Scoped Gate\n\nStatus: **${report.status}**\n\nFailed: ${report.failed}\n`);
console.log(JSON.stringify(report, null, 2));
process.exit(failed.length === 0 ? 0 : 1);
