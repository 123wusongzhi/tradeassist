import { spawnSync } from 'node:child_process';

console.error('Deprecated entry point: delegating to the sole P7-V2 regression engine.');
const result = spawnSync(process.execPath, ['scripts/p7-v2-regression.mjs', ...process.argv.slice(2)], { stdio: 'inherit' });
process.exit(result.status ?? 1);
