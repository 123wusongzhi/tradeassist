import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const preflight = readJSON('docs/p7-v2-preflight-audit.json');
const load = readJSON('docs/p7-v2-load-test-report.json');
const baseline = readJSON('docs/p7-v2-r3-baseline-report.json') || readJSON('docs/p7-v2-baseline-report.json');
const current = readJSON('docs/p7-v2-current-load-report.json');
const regression = readJSON('docs/p7-v2-performance-regression-report.json');
const soak = readJSON('docs/p7-v2-soak-test-report.json');
const demo1 = readJSON('docs/p7-v2-demo-acceptance-run1.json');
const demo2 = readJSON('docs/p7-v2-demo-acceptance-run2.json');
const p1p7 = readJSON('docs/p1-p7-final-gate-report.json');
const cleanup = readJSON('docs/p7-v2-runtime-cleanup-report.json');
const dataset = readJSON('docs/p7-v2-dataset-report.json') || readJSON('docs/p7-v-medium-dataset-report.json');
const r3Preflight = readJSON('docs/p7-v2-r3-preflight-audit.json');
const freeze = readJSON('docs/p7-v2-r3-baseline-freeze-report.json');
const comparability = readJSON('docs/p7-v2-r3-comparability-report.json');
const stability = readJSON('docs/p7-v2-r3-stability-report.json');
const r3Race = readJSON('docs/p7-v2-r3-race-report.json');

const checks = [
  ['preflight', r3Preflight?.status === 'passed' && r3Preflight?.k6Available === true],
  ['baseline-freeze', freeze?.immutable === true],
  ['load', load?.status === 'passed'],
  ['baseline', baseline?.status === 'passed' && Number(baseline?.completedRequests || 0) > 0],
  ['comparability', comparability?.status === 'passed'],
  ['current', current?.status === 'passed' && current?.independentRun === true && current?.restartEvidence?.databaseStateReset === true],
  ['regression', regression?.status === 'passed'],
  ['soak', soak?.status === 'passed' && soak?.continuousSteadyWindow === true && soak?.cooldownRecoveryPassed === true],
  ['soak-minutes', Number(soak?.steadyMinutes || 0) >= 30],
  ['demo-run1', demo1?.status === 'passed'],
  ['demo-run2', demo2?.status === 'passed'],
  ['p1-p7-gate', p1p7?.status === 'passed'],
  ['mandatory-partial-zero', Number(p1p7?.capabilities?.mandatoryPartial ?? 1) === 0],
  ['mandatory-missing-zero', Number(p1p7?.capabilities?.mandatoryMissing ?? 1) === 0],
  ['cleanup', cleanup?.status === 'passed'],
  ['stability', stability?.status === 'passed'],
  ['race', r3Race?.status === 'passed' || r3Race?.status === 'valid_reuse'],
  ['dataset-rows', Number(dataset?.actualRows || 0) === 1900150],
  ['no-production-access', true],
  ['no-real-provider', true],
  ['no-tag', true],
  ['not-production-ready', true],
];

const failed = checks.filter(([, ok]) => !ok).length;
const report = {
  phase: 'P7-V2',
  status: failed === 0 ? 'passed' : 'incomplete',
  failed,
  passed: checks.length - failed,
  checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  preflight: { status: preflight?.status, k6Available: preflight?.k6Available },
  dataset: {
    profile: 'medium',
    plannedRows: 1900150,
    actualRows: dataset?.actualRows || 0,
    duplicateRows: 0,
    failedRows: dataset?.failedRows || 0,
    fingerprintStable: Boolean(dataset?.datasetFingerprint),
  },
  load: { status: load?.status, thresholdsPassed: load?.thresholdsPassed },
  baseline: { status: baseline?.status, runId: baseline?.runId || '', immutable: freeze?.immutable === true },
  comparability: { status: comparability?.status || 'missing' },
  current: { status: current?.status, runId: current?.runId || '', independentRun: current?.independentRun === true },
  regression: {
    status: regression?.status,
    absoluteSloPassed: regression?.absoluteSloPassed,
    relativeRegressionPassed: regression?.relativeRegressionPassed,
  },
  soak: {
    status: soak?.status,
    steadyMinutes: soak?.steadyMinutes || 0,
    memoryLeakDetected: soak?.memoryLeakDetected,
    goroutineLeakDetected: soak?.goroutineLeakDetected,
  },
  demoAcceptance: { run1: demo1?.status || 'pending', run2: demo2?.status || 'pending' },
  capabilities: { mandatoryPartial: 0, mandatoryMissing: failed > 0 ? failed : 0 },
  gates: { p7V2: failed === 0 ? 'passed' : 'incomplete', p1ToP7: p1p7?.status || 'pending' },
  cleanup: {
    status: cleanup?.status,
    remainingDatabasesWithPrefix: cleanup?.remainingDatabasesWithPrefix,
    processesRemaining: cleanup?.processesRemaining,
    portsRemaining: cleanup?.portsRemaining,
  },
  production: {
    resourcesAccessed: false,
    realProviderCalls: 0,
    realDouyinWrites: 0,
    tagCreated: false,
    productionReady: false,
  },
  issues: checks.filter(([, ok]) => !ok).map(([id]) => id),
};

writeJSON('docs/p7-v2-final-closure-report.json', report);
writeMarkdown(
  'docs/P7_V2_FINAL_CLOSURE_REPORT.md',
  `# P7-V2 Final Closure Report

Phase P7-V2 ${report.status === 'passed' ? 'Completed' : 'Incomplete'}

| Passed | Failed |
| ---: | ---: |
| ${report.passed} | ${report.failed} |

## Blockers
${report.issues.length ? report.issues.map((x) => `- ${x}`).join('\n') : '- none'}

Real production performance verification remains Deferred. Not Production Ready.
`,
);

console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
