#!/usr/bin/env node
/**
 * Report required documentation updates for changed files.
 */
import { execSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..', '..');

function normalize(p) {
  return String(p || '').replaceAll('\\', '/').replace(/^\.\//, '');
}

function readJson(rel) {
  return JSON.parse(readFileSync(path.join(root, rel), 'utf8'));
}

function parseArgs(argv) {
  const args = { files: [], filesFromGit: false, json: false };
  for (let i = 0; i < argv.length; i += 1) {
    if (argv[i] === '--files') {
      args.files.push(...String(argv[++i] || '').split(',').map((s) => s.trim()).filter(Boolean));
    } else if (argv[i] === '--files-from-git') args.filesFromGit = true;
    else if (argv[i] === '--json') args.json = true;
  }
  return args;
}

function gitFiles() {
  const set = new Set();
  for (const cmd of ['git diff --name-only', 'git diff --name-only --cached', 'git ls-files --others --exclude-standard']) {
    try {
      for (const line of execSync(cmd, { cwd: root, encoding: 'utf8' }).split(/\r?\n/)) {
        const n = normalize(line.trim());
        if (n) set.add(n);
      }
    } catch {
      // ignore
    }
  }
  return [...set];
}

function matchPath(file, pattern) {
  const f = normalize(file);
  const p = normalize(pattern);
  if (p.endsWith('/**')) {
    const prefix = p.slice(0, -3);
    return f === prefix || f.startsWith(prefix + '/');
  }
  if (p.includes('*')) {
    const re = new RegExp('^' + p.replace(/[.+^${}()|[\]\\]/g, '\\$&').replace(/\*\*/g, '.*').replace(/\*/g, '[^/]*') + '$');
    return re.test(f);
  }
  return f === p || f.startsWith(p + '/');
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  let files = args.files.map(normalize);
  if (args.filesFromGit) files = [...new Set([...files, ...gitFiles()])];
  const impact = readJson('config/agent/change-impact.json');
  const hits = [];

  for (const rule of impact.rules || []) {
    const matched = files.filter((f) => (rule.paths || []).some((p) => matchPath(f, p)));
    if (!matched.length) continue;
    hits.push({
      id: rule.id,
      matchedFiles: matched,
      requiredDocs: rule.requiredDocs || [],
      generator: rule.generator || null,
      check: rule.check || null,
    });
  }

  const result = { files, hits };
  if (args.json) {
    console.log(JSON.stringify(result, null, 2));
    return;
  }
  console.log('Documentation impact');
  console.log(`files: ${files.length}`);
  if (!hits.length) {
    console.log('No documentation impact rules matched.');
    return;
  }
  for (const hit of hits) {
    console.log(`\n- ${hit.id}`);
    console.log(`  matched: ${hit.matchedFiles.join(', ')}`);
    if (hit.requiredDocs.length) console.log(`  requiredDocs: ${hit.requiredDocs.join(', ')}`);
    if (hit.generator) console.log(`  generator: pnpm ${hit.generator}`);
    if (hit.check) console.log(`  check: pnpm ${hit.check}`);
  }
}

main();
