#!/usr/bin/env node
import { spawnSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const check = process.argv.includes('--check');
const scripts = [
  'generate-command-reference.mjs',
  'generate-env-reference.mjs',
  'generate-api-routes.mjs',
  'generate-module-map.mjs',
];

let failed = false;
for (const script of scripts) {
  const result = spawnSync(process.execPath, [path.join(__dirname, script), ...(check ? ['--check'] : [])], {
    stdio: 'inherit',
  });
  if (result.status !== 0) failed = true;
}
process.exit(failed ? 1 : 0);
