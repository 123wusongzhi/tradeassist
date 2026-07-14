import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import { resolveActiveBaseline } from './p7-v2-evidence-resolver.mjs';
import { validateCleanup, validateCurrent, validateRegression, validateSoak } from './p7-v2-r3b-gate-lib.mjs';

const baseline = resolveActiveBaseline();
const current = validateCurrent(readJSON('docs/p7-v2-current-load-report.json') || {});
const regression = validateRegression(readJSON('docs/p7-v2-performance-regression-report.json') || {});
const soak = validateSoak(readJSON('docs/p7-v2-soak-test-report.json') || {});
const cleanup = validateCleanup(readJSON('docs/p7-v2-runtime-cleanup-report.json') || {});
const checks = [
  ['Frozen-Baseline-Registry', baseline.valid, baseline.issues],
  ['Current-Independent', current.valid, current.issues],
  ['Regression', regression.valid, regression.issues],
  ['Soak', soak.valid, soak.issues],
  ['Cleanup', cleanup.valid, cleanup.issues],
];
const failed = checks.filter(([, ok]) => !ok).length;
const report = {
  phase: 'P1-P7',
  status: failed === 0 ? 'passed' : 'incomplete',
  failed,
  passed: checks.length - failed,
  checks: checks.map(([id, ok, issues]) => ({ id, status: ok ? 'passed' : 'failed', issues })),
  baseline: { runId: baseline.baseline?.runId || '', source: baseline.source, registryValidated: baseline.valid },
  productionReady: false,
  tag: 'deferred',
};
writeJSON('docs/p1-p7-final-gate-report.json', report);
writeMarkdown('docs/P1_P7_FINAL_GATE_REPORT.md', `# P1-P7 Final Gate Report\n\nStatus: ${report.status}\n\n## Blockers\n${report.checks.filter((check) => check.status === 'failed').map((check) => `- ${check.id}: ${check.issues.join('; ')}`).join('\n') || '- none'}\n`);
console.log(JSON.stringify({ phase: report.phase, status: report.status, failed }, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
