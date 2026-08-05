#!/usr/bin/env node
/**
 * Fail if stable docs contain internal stage/execution-log claims.
 */
import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..', '..');

const TARGETS = [
  'AGENTS.md',
  'README.md',
  'README.en.md',
  'CONTRIBUTING.md',
  'docs/README.md',
  'docs/guides',
  'docs/reference',
  'docs/architecture',
  'docs/status/current.md',
  '.cursor/rules',
  '.agents/skills',
];

// Intentionally avoid matching product words; focus on internal stage markers.
const PATTERNS = [
  { id: 'stage-update', re: /Stage update/i },
  { id: 'phase-p', re: /\bP(?:[4-9]|[1-9][0-9])(?:[-_][A-Z0-9]+)+\b/ },
  { id: 'tag-deferred', re: /Tag deferred/i },
  { id: 'release-candidate', re: /Release Candidate/i },
  { id: 'production-ready-label', re: /Production Ready:/i },
  { id: 'codex-branch', re: /\bcodex\// },
  { id: 'origin-main-log', re: /\borigin\/main\b/ },
  { id: 'working-tree-log', re: /working tree/i },
  { id: 'uncommitted-log', re: /未 commit|未 push|not committed|not pushed/i },
];

function walk(rel, out = []) {
  const abs = path.join(root, rel);
  if (!existsSync(abs)) return out;
  const st = statSync(abs);
  if (st.isFile()) {
    if (/\.(md|mdc)$/i.test(rel)) out.push(rel.replaceAll('\\', '/'));
    return out;
  }
  for (const name of readdirSync(abs)) {
    if (name === 'frontend-design') continue; // optional upstream may contain examples
    walk(path.join(rel, name).replaceAll('\\', '/'), out);
  }
  return out;
}

export function checkStaleClaims() {
  const files = TARGETS.flatMap((t) => walk(t));
  const failures = [];
  for (const file of files) {
    // generated files may mention scripts with p7 names; still scan current hand-written
    if (file.includes('.generated.')) continue;
    if (file.includes('docs/archive/')) continue;
    const text = readFileSync(path.join(root, file), 'utf8');
    const lines = text.split(/\r?\n/);
    lines.forEach((line, idx) => {
      for (const pattern of PATTERNS) {
        if (pattern.re.test(line)) {
          failures.push(`${file}:${idx + 1} [${pattern.id}] ${line.trim().slice(0, 160)}`);
        }
      }
    });
  }
  return { filesChecked: files.length, failures };
}

function main() {
  const result = checkStaleClaims();
  console.log(`Stale claim check: ${result.filesChecked} files`);
  if (result.failures.length) {
    for (const f of result.failures.slice(0, 50)) console.log(`- ${f}`);
    if (result.failures.length > 50) console.log(`... and ${result.failures.length - 50} more`);
    process.exitCode = 1;
  } else {
    console.log('OK');
  }
}

if (process.argv[1]?.endsWith('check-stale-claims.mjs')) main();
