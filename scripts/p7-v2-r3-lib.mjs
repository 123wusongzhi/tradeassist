import crypto from 'node:crypto';
import { run, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import {
  buildRuntimeFreezeSourceManifest,
  immutableTrackedDiffHash,
  isEvidenceToolingPath,
  isRuntimeSourcePath,
  RUNTIME_FREEZE_SCOPE_VERSION,
} from './p7-v2-runtime-freeze-scope.mjs';

function sha256(value) {
  return crypto.createHash('sha256').update(value).digest('hex');
}

export function runtimeSourceFingerprint() {
  const manifest = buildRuntimeFreezeSourceManifest();
  const files = manifest.includedFiles.map((file) => ({ path: file.path, hash: file.sha256, sha256: file.sha256 }));
  return {
    algorithm: 'sha256',
    scopeVersion: RUNTIME_FREEZE_SCOPE_VERSION,
    fileCount: files.length,
    hash: manifest.manifestSha256,
    files,
    sourceManifest: manifest,
  };
}

export function trackedDiffHash() {
  return immutableTrackedDiffHash();
}

export function untrackedRuntimeManifest() {
  const status = run('git', ['status', '--porcelain=v1', '--untracked-files=all']);
  const paths = (status.stdout || '')
    .split(/\r?\n/)
    .filter(Boolean)
    .map((line) => line.slice(3).replaceAll('\\', '/'))
    .filter((file) => isRuntimeSourcePath(file) || isEvidenceToolingPath(file));
  return { scopeVersion: RUNTIME_FREEZE_SCOPE_VERSION, hash: sha256(JSON.stringify(paths.sort())), paths: paths.sort() };
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
