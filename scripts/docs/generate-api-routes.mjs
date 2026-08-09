#!/usr/bin/env node
/**
 * Best-effort API route index from Go route registrations.
 * Not a full OpenAPI generator; CI can detect drift of the snapshot text.
 */
import { mkdirSync, readdirSync, readFileSync, statSync, writeFileSync, existsSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..', '..');
const outRel = 'docs/reference/api/routes.generated.md';
const backendRoot = path.join(root, 'backend');

const METHOD_RE = /\.(GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\(\s*"([^"]+)"/g;

function walk(dir, out = []) {
  if (!existsSync(dir)) return out;
  for (const name of readdirSync(dir)) {
    const abs = path.join(dir, name);
    const st = statSync(abs);
    if (st.isDirectory()) {
      if (name === 'vendor' || name === 'testdata') continue;
      walk(abs, out);
    } else if (name.endsWith('.go') && !name.endsWith('_test.go')) {
      out.push(abs);
    }
  }
  return out;
}

function collect() {
  const files = walk(backendRoot);
  const routes = [];
  for (const abs of files) {
    const text = readFileSync(abs, 'utf8');
    const rel = path.relative(root, abs).replaceAll('\\', '/');
    for (const m of text.matchAll(METHOD_RE)) {
      routes.push({ method: m[1], path: m[2], file: rel });
    }
  }
  routes.sort((a, b) => (a.path + a.method).localeCompare(b.path + b.method));
  return routes;
}

function render(checkOnly = false) {
  const routes = collect();
  const lines = [
    '---',
    'doc_type: generated',
    'status: generated',
    'owner: maintainers',
    'generator: pnpm docs:generate:api',
    '---',
    '',
    '# API routes (generated snapshot)',
    '',
    '> Do not edit by hand. Run `pnpm docs:generate:api`.',
    '>',
    '> This is a static scan of Go `.METHOD("path")` registrations. Prefer OpenAPI if introduced later.',
    '',
    `Total matched registrations: ${routes.length}`,
    '',
    '| Method | Path | Source file |',
    '| --- | --- | --- |',
    ...routes.map((r) => `| ${r.method} | \`${r.path}\` | \`${r.file}\` |`),
    '',
  ];
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
      console.error(`generated drift: ${outRel}; run pnpm docs:generate:api`);
      process.exitCode = 1;
      return;
    }
    console.log(`OK ${outRel}`);
    return;
  }
  writeFileSync(abs, content);
  console.log(`wrote ${outRel} (${routes.length} routes)`);
}

render(process.argv.includes('--check'));
