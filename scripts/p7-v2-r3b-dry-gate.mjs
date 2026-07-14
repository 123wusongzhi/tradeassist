import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import { resolveActiveBaseline } from './p7-v2-evidence-resolver.mjs';

const baseline = resolveActiveBaseline();
const steps = {
  baseline: baseline.valid ? 'passed' : 'failed',
  current: 'pending',
  regression: 'pending',
  soak: 'pending',
  demo: 'pending',
  stability: 'pending',
};
const report = {
  phase: 'P7-V2-R3B-FIX',
  status: 'failed_as_expected_due_to_pending_execution',
  steps,
  finalClosure: 'failed_as_expected',
  productionReady: false,
  issues: baseline.valid ? ['execution evidence is pending'] : [...baseline.issues, 'execution evidence is pending'],
};
writeJSON('docs/p7-v2-r3b-fix-dry-gate-report.json', report);
writeMarkdown('docs/P7_V2_R3B_FIX_DRY_GATE_REPORT.md', `# P7-V2-R3B-FIX Dry Gate\n\nStatus: ${report.status}\n\n- Baseline: ${steps.baseline}\n- Current: ${steps.current}\n- Regression: ${steps.regression}\n- Soak: ${steps.soak}\n- Demo: ${steps.demo}\n`);
console.log(JSON.stringify(report, null, 2));
process.exit(0);
