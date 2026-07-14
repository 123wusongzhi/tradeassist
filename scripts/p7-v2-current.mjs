import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { readJSON, root, valueOf, writeJSON } from './p7-v2-lib.mjs';

const args = process.argv.slice(2);
const runId = valueOf(args, '--run-id') || `p7v2-current-${new Date().toISOString().replace(/[:.]/g, '-')}`;
const restart = spawnSync(process.execPath, ['scripts/p7-v2-restart-environment.mjs', '--run-id', `${runId}-restart`], {
  stdio: 'inherit',
});
if (restart.status !== 0) process.exit(restart.status ?? 1);
const runtime = readJSON('docs/p7-v2-runtime-environment.json') || {};
runtime.restartPerformed = true;
runtime.apiProcessChanged = true;
runtime.currentRunIndependent = true;
writeJSON('docs/p7-v2-runtime-environment.json', runtime);
const res = spawnSync(process.execPath, ['scripts/p7-v2-load.mjs', '--kind', 'current', '--run-id', runId, ...args.filter((a) => !a.startsWith('--run-id'))], {
  stdio: 'inherit',
});
process.exit(res.status ?? 1);
