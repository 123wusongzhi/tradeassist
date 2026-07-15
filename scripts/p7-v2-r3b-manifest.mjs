import fs from 'node:fs';
import path from 'node:path';
import { root, readJSON, writeJSON } from './p7-v2-lib.mjs';

export const MANIFEST_PATH = 'docs/p7-v2-r3b-run-manifest.json';

export function readR3BManifest() {
  return readJSON(MANIFEST_PATH) || {
    phase: 'P7-V2-R3B-FAST-CLOSE',
    baselineRunId: '',
    currentRunId: '',
    soakRunId: '',
    demoRun1Id: '',
    demoRun2Id: '',
    selectedHost: '127.0.0.1',
    selectedPort: 18080,
  };
}

export function updateR3BManifest(update) {
  const current = readR3BManifest();
  const next = {
    ...current,
    ...update,
    phase: current.phase || 'P7-V2-R3B-FAST-CLOSE',
    selectedHost: '127.0.0.1',
    selectedPort: 18080,
    updatedAt: new Date().toISOString(),
  };
  const target = path.join(root, MANIFEST_PATH);
  fs.mkdirSync(path.dirname(target), { recursive: true });
  const temporary = `${target}.${process.pid}.${Date.now()}.tmp`;
  fs.writeFileSync(temporary, `${JSON.stringify(next, null, 2)}\n`, 'utf8');
  fs.renameSync(temporary, target);
  return next;
}

if (import.meta.url === `file://${process.argv[1]?.replaceAll('\\', '/')}`) {
  const keys = ['baselineRunId', 'currentRunId', 'soakRunId', 'demoRun1Id', 'demoRun2Id'];
  const args = process.argv.slice(2);
  const update = Object.fromEntries(keys
    .filter((key) => args.includes(`--${key}`))
    .map((key) => [key, args[args.indexOf(`--${key}`) + 1] || '']));
  writeJSON(MANIFEST_PATH, updateR3BManifest(update));
}
