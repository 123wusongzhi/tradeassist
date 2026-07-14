import { spawnSync } from 'node:child_process';
import { valueOf } from './p7-v2-lib.mjs';

const args = process.argv.slice(2);
const runId = valueOf(args, '--run-id') || `p7v2-soak-${new Date().toISOString().replace(/[:.]/g, '-')}`;
const res = spawnSync(process.execPath, ['scripts/p7-v2-load.mjs', '--kind', 'soak', '--run-id', runId, ...args.filter((a) => !a.startsWith('--run-id'))], {
  stdio: 'inherit',
});
process.exit(res.status ?? 1);
