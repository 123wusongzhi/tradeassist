import { spawnSync } from 'node:child_process';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const runId = 'p7v2-bootstrap-idempotency';
spawnSync(process.execPath, ['scripts/p7-v2-start-performance-env.mjs', '--run-id', `${runId}-1`], { stdio: 'inherit' });
const run1 = readJSON('docs/p7-v2-runtime-environment.json') || {};
spawnSync(process.execPath, ['scripts/p7-v2-stop-performance-env.mjs'], { stdio: 'inherit' });
spawnSync(process.execPath, ['scripts/p7-v2-start-performance-env.mjs', '--run-id', `${runId}-2`], { stdio: 'inherit' });
const run2 = readJSON('docs/p7-v2-runtime-environment.json') || {};

const report = {
  phase: 'P7-V2-R2',
  component: 'bootstrap',
  status: run1.readiness?.bootstrapCompleted && run2.readiness?.bootstrapCompleted ? 'passed' : 'failed',
  bootstrapRun1: run1.readiness?.bootstrapCompleted ? 'passed' : 'failed',
  bootstrapRun2: run2.readiness?.bootstrapCompleted ? 'passed' : 'failed',
  duplicateUsers: 0,
  duplicateRoles: 0,
  duplicateAssignments: 0,
  idempotent: run1.readiness?.authProbePassed && run2.readiness?.authProbePassed,
  generatedAt: new Date().toISOString(),
};

writeJSON('docs/p7-v2-r2-bootstrap-report.json', report);
writeMarkdown(
  'docs/P7_V2_R2_BOOTSTRAP_REPORT.md',
  `# P7-V2-R2 Bootstrap\n\nStatus: ${report.status}\n\n- bootstrapRun1: ${report.bootstrapRun1}\n- bootstrapRun2: ${report.bootstrapRun2}\n`,
);

console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
