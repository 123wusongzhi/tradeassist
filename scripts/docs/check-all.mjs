#!/usr/bin/env node
import { spawnSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..', '..');

const steps = [
  ['node', ['scripts/workflow/check-agent-context.mjs']],
  ['node', ['scripts/docs/check-links.mjs']],
  ['node', ['scripts/docs/check-stale-claims.mjs']],
  ['node', ['scripts/docs/generate-all.mjs', '--check']],
];

let failed = false;
for (const [cmd, args] of steps) {
  console.log(`\n>> ${cmd} ${args.join(' ')}`);
  const result = spawnSync(cmd, args, { cwd: root, stdio: 'inherit', shell: process.platform === 'win32' });
  if (result.status !== 0) failed = true;
}
process.exit(failed ? 1 : 0);
