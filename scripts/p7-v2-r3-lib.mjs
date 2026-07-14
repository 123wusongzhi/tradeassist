import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { root, run, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const RUNTIME_INCLUDE = [
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
const EXCLUDED_PARTS = new Set(['.git', 'node_modules', 'dist', 'artifacts', 'docs', 'data']);

function sha256(value) {
  return crypto.createHash('sha256').update(value).digest('hex');
}

function walk(abs, rel, output) {
  const stat = fs.statSync(abs);
  if (stat.isFile()) {
    if (
      (rel.startsWith('backend/') && (rel.endsWith('.go') || rel === 'backend/go.mod' || rel === 'backend/go.sum')) ||
      rel.startsWith('tests/load/') ||
      (rel.startsWith('scripts/') && /^scripts\/p7-v2-.*\.mjs$/.test(rel)) ||
      ['package.json', 'pnpm-lock.yaml', '.env.example', '.env.docker.example', 'docker-compose.yml', 'docker-compose.full.yml'].includes(rel)
    ) {
      output.push({ path: rel.replaceAll('\\', '/'), hash: sha256(fs.readFileSync(abs)) });
    }
    return;
  }
  for (const child of fs.readdirSync(abs).sort()) {
    if (EXCLUDED_PARTS.has(child)) continue;
    walk(path.join(abs, child), path.posix.join(rel.replaceAll('\\', '/'), child), output);
  }
}

export function runtimeSourceFingerprint() {
  const files = [];
  for (const rel of RUNTIME_INCLUDE) {
    const abs = path.join(root, rel);
    if (fs.existsSync(abs)) walk(abs, rel, files);
  }
  files.sort((a, b) => a.path.localeCompare(b.path));
  return { algorithm: 'sha256', fileCount: files.length, hash: sha256(JSON.stringify(files)), files };
}

export function trackedDiffHash() {
  const diff = run('git', ['diff', '--binary', 'HEAD']);
  return { hash: sha256(diff.stdout || ''), clean: !(diff.stdout || '').trim(), commandStatus: diff.status };
}

export function untrackedRuntimeManifest() {
  const status = run('git', ['status', '--porcelain=v1', '--untracked-files=all']);
  const paths = (status.stdout || '')
    .split(/\r?\n/)
    .filter(Boolean)
    .map((line) => line.slice(3).replaceAll('\\', '/'))
    .filter((file) => file.startsWith('backend/') || file.startsWith('tests/load/') || /^scripts\/p7-v2-.*\.mjs$/.test(file) || file === 'package.json' || file === 'pnpm-lock.yaml');
  return { hash: sha256(JSON.stringify(paths.sort())), paths: paths.sort() };
}

export function jsonHash(value) {
  return sha256(JSON.stringify(value));
}

export function summaryHasTraffic(summary) {
  const requests = summary?.metrics?.http_reqs?.values || summary?.metrics?.http_reqs || {};
  const duration = summary?.metrics?.http_req_duration?.values || summary?.metrics?.http_req_duration || {};
  const count = Number(requests.count || 0);
  const rate = Number(requests.rate || requests.value || 0);
  const p95 = Number(duration['p(95)'] || 0);
  return count > 0 && rate > 0 && p95 > 0;
}

export function writeR3Report(jsonPath, markdownPath, title, report, rows = []) {
  writeJSON(jsonPath, report);
  const table = rows.length
    ? `\n| Field | Value |\n| --- | --- |\n${rows.map(([key, value]) => `| ${key} | ${String(value)} |`).join('\n')}\n`
    : '';
  writeMarkdown(markdownPath, `# ${title}\n\nStatus: **${report.status}**\n${table}\n## Issues\n${report.issues?.length ? report.issues.map((x) => `- ${x}`).join('\n') : '- none'}\n`);
}
