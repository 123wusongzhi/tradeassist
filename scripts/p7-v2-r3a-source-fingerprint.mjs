import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { root, run, writeJSON } from './p7-v2-lib.mjs';

const INCLUDE_ROOTS = [
  'backend',
  'tests/load',
  'scripts',
  'package.json',
  'pnpm-lock.yaml',
  '.env.example',
  '.env.docker.example',
  'docker-compose.yml',
  'docker-compose.full.yml',
];
const CONFIG_TEMPLATES = new Set([
  '.env.example',
  '.env.docker.example',
  'docker-compose.yml',
  'docker-compose.full.yml',
]);
const SKIP_DIRS = new Set(['.git', 'node_modules', 'dist', 'build', 'tmp', 'artifacts', 'docs', 'data', '.agents']);

function sha256(value) {
  return crypto.createHash('sha256').update(value).digest('hex');
}

function includeFile(rel) {
  const normalized = rel.replaceAll('\\', '/');
  return (
    (normalized.startsWith('backend/') && (normalized.endsWith('.go') || normalized === 'backend/go.mod' || normalized === 'backend/go.sum')) ||
    normalized.startsWith('tests/load/') ||
    /^scripts\/p7-v2-.*\.mjs$/.test(normalized) ||
    CONFIG_TEMPLATES.has(normalized) ||
    normalized === 'package.json' ||
    normalized === 'pnpm-lock.yaml'
  );
}

function walk(abs, rel, files) {
  const stat = fs.statSync(abs);
  if (stat.isFile()) {
    if (includeFile(rel)) files.push({ path: rel.replaceAll('\\', '/'), sha256: sha256(fs.readFileSync(abs)) });
    return;
  }
  for (const entry of fs.readdirSync(abs).sort()) {
    if (SKIP_DIRS.has(entry)) continue;
    walk(path.join(abs, entry), path.posix.join(rel.replaceAll('\\', '/'), entry), files);
  }
}

function filesHash(files) {
  return sha256(JSON.stringify(files));
}

function gitStatusSnapshot() {
  const result = run('git', ['status', '--porcelain=v1', '--untracked-files=all']);
  return (result.stdout || '').split(/\r?\n/).filter(Boolean);
}

const files = [];
for (const rel of INCLUDE_ROOTS) {
  const abs = path.join(root, rel);
  if (fs.existsSync(abs)) walk(abs, rel, files);
}
files.sort((a, b) => a.path.localeCompare(b.path));

const status = gitStatusSnapshot();
const trackedDiff = run('git', ['diff', '--binary', 'HEAD']).stdout || '';
const untrackedRuntimeFiles = status
  .filter((line) => line.startsWith('?? '))
  .map((line) => line.slice(3).replaceAll('\\', '/'))
  .filter(includeFile)
  .sort();
const runtimeFiles = files.filter((file) => file.path.startsWith('backend/'));
const loadFiles = files.filter((file) => file.path.startsWith('tests/load/') || /^scripts\/p7-v2-.*\.mjs$/.test(file.path));
const migrations = files.filter((file) => file.path.includes('/migrate') || file.path.includes('/migration'));
const manifests = files.filter((file) => ['package.json', 'pnpm-lock.yaml'].includes(file.path));

const report = {
  algorithm: 'sha256',
  generatedAt: new Date().toISOString(),
  gitCommit: (run('git', ['rev-parse', 'HEAD']).stdout || '').trim(),
  gitDirty: status.length > 0,
  gitStatusSnapshot: status,
  trackedDiffHash: sha256(trackedDiff),
  runtimeSourceTreeHash: filesHash(files),
  loadScriptsHash: filesHash(loadFiles),
  migrationHash: filesHash(migrations),
  packageManifestHash: filesHash(manifests),
  untrackedRuntimeFileManifest: untrackedRuntimeFiles,
  untrackedRuntimeFileManifestHash: sha256(JSON.stringify(untrackedRuntimeFiles)),
  files,
};

const output = process.argv.includes('--output');
const outputPath = output ? process.argv[process.argv.indexOf('--output') + 1] : '';
if (outputPath) writeJSON(outputPath, report);
console.log(JSON.stringify(report, null, 2));
