import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import { resolveActiveBaseline } from './p7-v2-evidence-resolver.mjs';
import { validateCleanup, validateCurrent, validateDemo, validateRegression, validateSoak } from './p7-v2-r3b-gate-lib.mjs';

const baseline = resolveActiveBaseline();
const current = validateCurrent(readJSON('docs/p7-v2-current-load-report.json') || {});
const regression = validateRegression(readJSON('docs/p7-v2-performance-regression-report.json') || {});
const soak = validateSoak(readJSON('docs/p7-v2-soak-test-report.json') || {});
const demo = validateDemo(readJSON('docs/p7-v2-demo-acceptance-run1.json') || {}, readJSON('docs/p7-v2-demo-acceptance-run2.json') || {});
const cleanup = validateCleanup(readJSON('docs/p7-v2-runtime-cleanup-report.json') || {});
const stability = readJSON('docs/p7-v2-r3-stability-report.json') || {};
const race = readJSON('docs/p7-v2-r3-race-report.json') || {};
const p1p7 = readJSON('docs/p1-p7-final-gate-report.json') || {};
const checks = [
  ['baseline', baseline.valid, baseline.issues],
  ['current', current.valid, current.issues],
  ['regression', regression.valid, regression.issues],
  ['soak', soak.valid, soak.issues],
  ['demo', demo.valid, demo.issues],
  ['stability', stability.status === 'passed', ['stability status is not passed']],
  ['race', ['passed', 'valid_reuse'].includes(race.status), ['race status is not passed or valid_reuse']],
  ['cleanup', cleanup.valid, cleanup.issues],
  ['p1-p7', p1p7.status === 'passed', ['P1-P7 final gate is not passed']],
];
const failed = checks.filter(([, ok]) => !ok).length;
const report = {
  phase: 'P7-V2',
  status: failed === 0 ? 'passed' : 'incomplete',
  failed,
  passed: checks.length - failed,
  checks: checks.map(([id, ok, issues]) => ({ id, status: ok ? 'passed' : 'failed', issues: ok ? [] : issues })),
  baseline: { runId: baseline.baseline?.runId || '', registryValidated: baseline.valid, source: baseline.source },
  production: { resourcesAccessed: false, realProviderCalls: 0, realDouyinWrites: 0, tagCreated: false, productionReady: false },
};
writeJSON('docs/p7-v2-final-closure-report.json', report);
writeMarkdown('docs/P7_V2_FINAL_CLOSURE_REPORT.md', `# P7-V2 Final Closure Report\n\nPhase P7-V2 ${report.status === 'passed' ? 'Completed' : 'Incomplete'}\n\n## Blockers\n${report.checks.filter((check) => check.status === 'failed').map((check) => `- ${check.id}: ${check.issues.join('; ')}`).join('\n') || '- none'}\n\nReal production performance verification remains deferred. Not Production Ready.\n`);
console.log(JSON.stringify({ phase: report.phase, status: report.status, failed }, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
