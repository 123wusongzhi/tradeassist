#!/usr/bin/env node
/**
 * Generate env variable reference from .env.example files.
 */
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..', '..');
const outRel = 'docs/reference/configuration/environment.generated.md';

function parseEnvExample(rel) {
  const abs = path.join(root, rel);
  if (!existsSync(abs)) return [];
  const lines = readFileSync(abs, 'utf8').split(/\r?\n/);
  const vars = [];
  let comment = [];
  for (const line of lines) {
    if (!line.trim() || line.trim().startsWith('#')) {
      if (line.trim().startsWith('#')) comment.push(line.replace(/^#\s?/, ''));
      continue;
    }
    const m = line.match(/^([A-Za-z_][A-Za-z0-9_]*)=(.*)$/);
    if (!m) continue;
    vars.push({
      name: m[1],
      example: m[2],
      comment: comment.join(' ').slice(0, 200),
      source: rel,
    });
    comment = [];
  }
  return vars;
}

function render(checkOnly = false) {
  const vars = [
    ...parseEnvExample('.env.example'),
    ...parseEnvExample('.env.docker.example'),
  ];
  const seen = new Set();
  const rows = [];
  for (const v of vars) {
    const key = `${v.source}:${v.name}`;
    if (seen.has(key)) continue;
    seen.add(key);
    const secret = /KEY|SECRET|TOKEN|PASSWORD|COOKIE|PRIVATE/i.test(v.name);
    rows.push(`| \`${v.name}\` | \`${v.source}\` | ${secret ? 'yes' : 'no'} | ${v.comment || ''} |`);
  }
  const content = [
    '---',
    'doc_type: generated',
    'status: generated',
    'owner: maintainers',
    'generator: pnpm docs:generate:env',
    '---',
    '',
    '# Environment variables (generated)',
    '',
    '> Do not edit by hand. Run `pnpm docs:generate:env`.',
    '',
    'Narrative guidance: `docs/reference/configuration/environment.md`.',
    '',
    '| Variable | Source | Secret-like | Notes |',
    '| --- | --- | --- | --- |',
    ...rows,
    '',
  ].join('\n');

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
      console.error(`generated drift: ${outRel}; run pnpm docs:generate:env`);
      process.exitCode = 1;
      return;
    }
    console.log(`OK ${outRel}`);
    return;
  }
  writeFileSync(abs, content);
  console.log(`wrote ${outRel}`);
}

render(process.argv.includes('--check'));
