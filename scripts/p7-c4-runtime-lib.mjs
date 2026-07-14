import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');

export function readRuntimeEnv() {
  return JSON.parse(fs.readFileSync(path.join(docs, 'p7-c4-runtime-environment.json'), 'utf8'));
}

export function shellExports(vars) {
  return Object.entries(vars)
    .map(([k, v]) => `export ${k}=${JSON.stringify(String(v))}`)
    .join(' && ');
}

export function runWSLWithEnv(envObj, bashBody) {
  const vars = {
    ...envObj.env,
    PAGINATION_CURSOR_SIGNING_KEY: process.env.PAGINATION_CURSOR_SIGNING_KEY || 'trademind-p7-development-cursor-signing-key',
  };
  const cmd = `${shellExports(vars)} && ${bashBody}`;
  return spawnSync('wsl', ['-d', 'Ubuntu-22.04', '--', 'bash', '-lc', cmd], {
    cwd: root,
    encoding: 'utf8',
    maxBuffer: 1024 * 1024 * 20,
    timeout: 2 * 60 * 60 * 1000,
  });
}

export function parseJSONReport(stdout) {
  const match = (stdout || '').match(/\{[\s\S]*\}/);
  return match ? JSON.parse(match[0]) : null;
}
