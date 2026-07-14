import { spawnSync } from 'node:child_process';
import { valueOf } from './p7-v2-lib.mjs';

const args = process.argv.slice(2);
const runId = valueOf(args, '--run-id') || `p7v2-diagnostic-${new Date().toISOString().replace(/[:.]/g, '').slice(0, 14)}`;
spawnSync(process.execPath, ['scripts/p7-v2-start-performance-env.mjs', '--run-id', 'p7v2-r2-env-20260714164000'], {
  stdio: 'inherit',
});
const res = spawnSync(process.execPath, ['scripts/p7-v2-load.mjs', '--kind', 'diagnostic', '--run-id', runId, '--target-vus', '3'], {
  stdio: 'inherit',
});
process.exit(res.status ?? 1);
