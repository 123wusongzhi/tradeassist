#!/usr/bin/env node
/**
 * Check relative markdown links in stable docs (not archive dumps).
 */
import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..', '..');

const ROOTS = [
  'AGENTS.md',
  'README.md',
  'README.en.md',
  'CONTRIBUTING.md',
  'docs/README.md',
  'docs/guides',
  'docs/reference',
  'docs/architecture',
  'docs/runbooks',
  'docs/status',
  'docs/archive/README.md',
  '.cursor/rules',
];

function normalize(p) {
  return p.replaceAll('\\', '/');
}

function walk(rel, out = []) {
  const abs = path.join(root, rel);
  if (!existsSync(abs)) return out;
  const st = statSync(abs);
  if (st.isFile()) {
    if (rel.endsWith('.md') || rel.endsWith('.mdc')) out.push(normalize(rel));
    return out;
  }
  for (const name of readdirSync(abs)) {
    if (name === 'node_modules' || name === 'assets') continue;
    walk(normalize(path.join(rel, name)), out);
  }
  return out;
}

function checkFile(rel) {
  const abs = path.join(root, rel);
  const text = readFileSync(abs, 'utf8');
  const failures = [];
  const re = /\[[^\]]*\]\(([^)]+)\)/g;
  for (const match of text.matchAll(re)) {
    let target = match[1].trim();
    if (!target || target.startsWith('http://') || target.startsWith('https://') || target.startsWith('mailto:')) continue;
    if (target.startsWith('#')) continue;
    target = target.split('#')[0];
    if (!target) continue;
    const resolved = normalize(path.posix.normalize(path.posix.join(path.posix.dirname(rel), target)));
    if (!existsSync(path.join(root, resolved))) {
      failures.push(`${rel} -> missing ${target} (resolved ${resolved})`);
    }
  }
  return failures;
}

export function checkLinks() {
  const files = ROOTS.flatMap((r) => walk(r));
  const failures = files.flatMap(checkFile);
  return { filesChecked: files.length, failures };
}

function main() {
  const result = checkLinks();
  console.log(`Link check: ${result.filesChecked} files`);
  if (result.failures.length) {
    for (const f of result.failures) console.log(`- ${f}`);
    process.exitCode = 1;
  } else {
    console.log('OK');
  }
}

if (process.argv[1]?.endsWith('check-links.mjs')) main();
