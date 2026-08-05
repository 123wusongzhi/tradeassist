#!/usr/bin/env node
/**
 * Generate command index from package.json scripts.
 */
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..', '..');
const outRel = 'docs/reference/generated/commands.generated.md';

const STABLE_PREFIXES = [
  'dev',
  'build',
  'test:',
  'quality:',
  'architecture:',
  'agent:',
  'docs:',
  'workflow:',
  'check:dev',
  'check:ui-copy',
  'seed:demo',
  'verify:demo',
];

const LEGACY_RE = /^(check:p|drill:p|p[4-9]|demo:auto)/i;

function loadScripts(rel) {
  try {
    const pkg = JSON.parse(readFileSync(path.join(root, rel), 'utf8'));
    return { name: pkg.name || rel, scripts: pkg.scripts || {} };
  } catch {
    return { name: rel, scripts: {} };
  }
}

function categorize(name) {
  if (LEGACY_RE.test(name) || name.includes('p7-v') || name.includes('p4-') || name.includes('p6-')) {
    return 'legacy-evidence';
  }
  if (STABLE_PREFIXES.some((p) => name === p || name.startsWith(p))) return 'stable';
  return 'other';
}

function render(checkOnly = false) {
  const packages = [
    loadScripts('package.json'),
    loadScripts('admin/package.json'),
    loadScripts('collector/package.json'),
  ];
  const lines = [
    '---',
    'doc_type: generated',
    'status: generated',
    'owner: maintainers',
    'generator: pnpm docs:generate:commands',
    '---',
    '',
    '# Command reference (generated)',
    '',
    '> Do not edit by hand. Run `pnpm docs:generate:commands`.',
    '',
    '## Stable commands (root)',
    '',
    '| Script | Package |',
    '| --- | --- |',
  ];
  const legacy = [];
  const other = [];
  for (const pkg of packages) {
    for (const [name] of Object.entries(pkg.scripts).sort(([a], [b]) => a.localeCompare(b))) {
      const cat = categorize(name);
      const row = `| \`${name}\` | \`${pkg.name}\` |`;
      if (pkg.name === 'trademind-monorepo' || pkg.name.includes('monorepo') || packages[0] === pkg) {
        if (cat === 'stable') lines.push(row);
        else if (cat === 'legacy-evidence') legacy.push(row);
        else other.push(row);
      } else if (cat === 'stable') {
        lines.push(row);
      }
    }
  }
  lines.push('', '## Workspace package scripts (selected)', '');
  lines.push('See each package.json for package-local scripts. Prefer root scripts when available.', '');
  lines.push('## Legacy / evidence commands', '');
  lines.push('These remain for historical gates and evidence regeneration. Prefer stable scripts for daily development.', '');
  lines.push('| Script | Package |', '| --- | --- |', ...legacy.slice(0, 80));
  if (legacy.length > 80) lines.push(`| … | ${legacy.length - 80} more omitted |`);
  lines.push('');
  const content = lines.join('\n');
  const abs = path.join(root, outRel);
  mkdirSync(path.dirname(abs), { recursive: true });
  if (checkOnly) {
    let existing = '';
    try {
      existing = readFileSync(abs, 'utf8');
    } catch {
      console.error(`missing generated file: ${outRel}`);
      process.exitCode = 1;
      return;
    }
    if (existing !== content) {
      console.error(`generated drift: ${outRel}; run pnpm docs:generate:commands`);
      process.exitCode = 1;
      return;
    }
    console.log(`OK ${outRel}`);
    return;
  }
  writeFileSync(abs, content);
  console.log(`wrote ${outRel}`);
}

const checkOnly = process.argv.includes('--check');
render(checkOnly);
