#!/usr/bin/env node
/**
 * Generate module impact map from change-impact.json and architecture boundaries when present.
 */
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..', '..');
const outRel = 'docs/reference/generated/module-map.generated.md';

function readJson(rel) {
  return JSON.parse(readFileSync(path.join(root, rel), 'utf8'));
}

function render(checkOnly = false) {
  const impact = readJson('config/agent/change-impact.json');
  let boundaries = null;
  const boundaryPath = 'tests/architecture/module-boundaries.json';
  if (existsSync(path.join(root, boundaryPath))) {
    try {
      boundaries = readJson(boundaryPath);
    } catch {
      boundaries = null;
    }
  }

  const lines = [
    '---',
    'doc_type: generated',
    'status: generated',
    'owner: maintainers',
    'generator: pnpm docs:generate:module-map',
    '---',
    '',
    '# Module / change impact map (generated)',
    '',
    '> Do not edit by hand. Run `pnpm docs:generate:module-map`.',
    '',
    '## Change → documentation impact',
    '',
    '| Rule | Paths | Required docs | Generator / check |',
    '| --- | --- | --- | --- |',
  ];
  for (const rule of impact.rules || []) {
    lines.push(
      `| \`${rule.id}\` | ${(rule.paths || []).map((p) => `\`${p}\``).join('<br>')} | ${(rule.requiredDocs || []).map((p) => `\`${p}\``).join('<br>')} | ${rule.generator || rule.check || ''} |`,
    );
  }
  lines.push('', '## Architecture boundaries', '');
  if (boundaries) {
    lines.push(`Source: \`${boundaryPath}\``);
    lines.push('');
    lines.push('```json');
    lines.push(JSON.stringify(boundaries, null, 2).slice(0, 12000));
    lines.push('```');
  } else {
    lines.push('_No module-boundaries.json found or unreadable._');
  }
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
      console.error(`generated drift: ${outRel}; run pnpm docs:generate:module-map`);
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
