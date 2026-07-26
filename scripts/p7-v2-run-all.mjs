import { spawnSync } from 'node:child_process';
import { safeRunId, valueOf } from './p7-v2-lib.mjs';

const args = process.argv.slice(2);
if (process.env.R3B_MODE === 'true' || args.includes('--r3b')) {
  console.error('R3B is a manually sequenced validation chain. Use the p7-v2:r3b:* commands; run-all is legacy.');
  process.exit(1);
}
const runId = safeRunId(valueOf(args, '--run-id') || `p7v2-${new Date().toISOString().replace(/[:.]/g, '-')}`);
const skipLoad = args.includes('--skip-load');
const steps = [
  ['host-guard', ['scripts/p7-v2-load-host-guard.mjs']],
  ['install-k6', ['scripts/install-k6-local.mjs']],
  ['preflight', ['scripts/p7-v2-preflight-audit.mjs']],
  ['env-start', ['scripts/p7-v2-start-performance-env.mjs', '--run-id', runId]],
  ['dataset', ['scripts/p7-v2-dataset.mjs', '--run-id', runId, '--execute']],
  ['smoke', ['scripts/p7-v2-load.mjs', '--kind', 'smoke', '--run-id', `${runId}-smoke`]],
  ['baseline', ['scripts/p7-v2-baseline.mjs', '--run-id', `${runId}-baseline`]],
  ['current', ['scripts/p7-v2-current.mjs', '--run-id', `${runId}-current`]],
  ['regression', ['scripts/p7-v2-performance-regression.mjs']],
  ['soak', ['scripts/p7-v2-soak.mjs', '--run-id', `${runId}-soak`]],
  ['race', ['scripts/p7-v2-race.mjs']],
  ['demo1', ['scripts/p7-v2-demo-acceptance.mjs', '--run', '1', '--run-id', `${runId}-demo1`]],
  ['demo2', ['scripts/p7-v2-demo-acceptance.mjs', '--run', '2', '--run-id', `${runId}-demo2`]],
  ['cleanup', ['scripts/p7-v2-stop-performance-env.mjs']],
  ['p1-p7-gate', ['scripts/p1-p7-final-gate.mjs']],
  ['final-gate', ['scripts/p7-v2-final-closure-gate.mjs']],
];

const skip = new Set(args.filter((a) => a.startsWith('--skip=')).map((a) => a.slice(7)));
const results = [];
for (const [name, cmd] of steps) {
  if (skip.has(name) || (skipLoad && ['smoke', 'baseline', 'current', 'regression', 'soak'].includes(name))) {
    results.push({ name, status: 'skipped' });
    continue;
  }
  const res = spawnSync(process.execPath, cmd, { stdio: 'inherit' });
  results.push({ name, status: res.status === 0 ? 'passed' : 'failed', exitCode: res.status ?? 1 });
  if (res.status !== 0 && !['install-k6', 'preflight'].includes(name)) break;
}

const failed = results.filter((r) => r.status === 'failed').length;
console.log(JSON.stringify({ phase: 'P7-V2', runId, failed, results }, null, 2));
process.exit(failed === 0 ? 0 : 1);
