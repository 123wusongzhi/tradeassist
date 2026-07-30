import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const preflight = readJSON('docs/p7-v2-r3-preflight-audit.json') || {};
const freeze = readJSON('docs/p7-v2-r3-baseline-freeze-report.json') || {};
const comparability = readJSON('docs/p7-v2-r3-comparability-report.json') || {};
const baseline = readJSON('docs/p7-v2-r3-baseline-report.json') || {};
const current = readJSON('docs/p7-v2-current-load-report.json') || {};
const regression = readJSON('docs/p7-v2-performance-regression-report.json') || {};
const soak = readJSON('docs/p7-v2-soak-test-report.json') || {};
const demo1 = readJSON('docs/p7-v2-demo-acceptance-run1.json') || {};
const demo2 = readJSON('docs/p7-v2-demo-acceptance-run2.json') || {};
const cleanup = readJSON('docs/p7-v2-runtime-cleanup-report.json') || {};
const p1p7 = readJSON('docs/p1-p7-final-gate-report.json') || {};
const finalGate = readJSON('docs/p7-v2-final-closure-report.json') || {};
const blockers = [];
if (comparability.status !== 'passed') blockers.push('baseline/current comparability is not passed');
if (current.status !== 'passed') blockers.push('independent current load was not executed');
if (regression.status !== 'passed') blockers.push('performance regression was not executed');
if (soak.status !== 'passed') blockers.push('30-minute soak was not executed');
if (demo1.status !== 'passed' || demo2.status !== 'passed') blockers.push('two independent demo runs were not executed');
if (cleanup.status !== 'passed') blockers.push('runtime cleanup has remaining isolated resources');
if (p1p7.status !== 'passed' || finalGate.status !== 'passed') blockers.push('final gates are incomplete');
const report = {
  phase: 'P7-V2-R3',
  status: blockers.length ? 'incomplete' : 'passed',
  baseline: { runId: baseline.runId || '', status: baseline.status || 'missing', historicalFreeze: freeze.status || 'missing' },
  comparability: { status: comparability.status || 'missing' },
  current: { runId: current.runId || '', status: current.status || 'pending' },
  regression: { status: regression.status || 'pending' },
  soak: { runId: soak.runId || '', status: soak.status || 'pending', steadyMinutes: soak.steadyMinutes || 0 },
  demoAcceptance: { run1: { runId: demo1.runId || '', status: demo1.status || 'pending' }, run2: { runId: demo2.runId || '', status: demo2.status || 'pending' } },
  gates: { p1ToP7: p1p7.status || 'pending', p7V2: finalGate.status || 'pending' },
  cleanup: { status: cleanup.status || 'pending', remainingDatabasesWithPrefix: cleanup.remainingDatabasesWithPrefix ?? null },
  production: { resourcesAccessed: false, realProviderCalls: 0, realDouyinWrites: 0, autoListingTriggered: false, tagCreated: false, productionReady: false },
  issues: blockers,
};
writeJSON('docs/p7-v2-r3-final-closure-report.json', report);
writeMarkdown(
  'docs/P7_V2_R3_FINAL_CLOSURE_REPORT.md',
  `# P7-V2-R3 Final Closure Report\n\nPhase P7-V2-R3 ${report.status === 'passed' ? 'Completed' : 'Incomplete'}\n\n## Blockers\n${blockers.length ? blockers.map((item) => `- ${item}`).join('\n') : '- none'}\n\nProduction verification remains Deferred. Non-production resources only.\n`,
);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
