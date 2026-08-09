#!/usr/bin/env node
/**
 * Generate documentation inventory for Agent/context migration.
 * Output: artifacts/docs-inventory/<date>.json and .md
 */
import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readdirSync, readFileSync, statSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..', '..');

const SKIP_DIRS = new Set([
  'node_modules',
  '.git',
  'dist',
  'build',
  'coverage',
  'test-results',
  'playwright-report',
  'midscene_run',
  'data',
  'admin/dist',
  'collector/dist',
  'collector/data',
  'backend/migrations',
]);

const DOC_EXTENSIONS = new Set(['.md', '.mdc']);
const PHASE_PATTERN = /\bP[4-9](?:[-_][A-Z0-9]+)?\b|Stage update|Release Candidate|Tag deferred|closure report|acceptance report/i;
const EVIDENCE_PATTERN = /\.(json|png|jpg|jpeg|webm|trace|sha256)$/i;
const HISTORY_DIRS = ['docs/archive', 'docs/baselines', 'docs/currents', 'docs/fingerprints', 'docs/runs', 'docs/regressions'];

function normalize(p) {
  return p.replaceAll('\\', '/');
}

function shouldSkip(rel) {
  const parts = normalize(rel).split('/');
  return parts.some((part) => SKIP_DIRS.has(part) || part === 'node_modules');
}

function walk(dir, out = []) {
  let entries;
  try {
    entries = readdirSync(dir, { withFileTypes: true });
  } catch {
    return out;
  }
  for (const entry of entries) {
    const abs = path.join(dir, entry.name);
    const rel = normalize(path.relative(root, abs));
    if (shouldSkip(rel)) continue;
    if (entry.isDirectory()) {
      walk(abs, out);
      continue;
    }
    const ext = path.extname(entry.name).toLowerCase();
    const isSkill = entry.name === 'SKILL.md' || rel.endsWith('/SKILL.md');
    const isAgentJson = rel.startsWith('config/agent/') && ext === '.json';
    if (DOC_EXTENSIONS.has(ext) || isSkill || isAgentJson) {
      out.push(rel);
    }
  }
  return out;
}

function extractFrontmatter(text) {
  const match = text.match(/^---\r?\n([\s\S]*?)\r?\n---/);
  if (!match) return {};
  const fields = {};
  for (const line of match[1].split(/\r?\n/)) {
    const m = line.match(/^([A-Za-z][A-Za-z0-9_-]*):\s*(.*)$/);
    if (m) fields[m[1]] = m[2].replace(/^["']|["']$/g, '');
  }
  return fields;
}

function countLinks(text) {
  const out = [...text.matchAll(/\[[^\]]*\]\(([^)]+)\)/g)].map((m) => m[1]);
  const code = [...text.matchAll(/(?:`|\s)((?:docs|\.agents|\.cursor)\/[A-Za-z0-9_./-]+\.(?:md|mdc))/g)].map((m) => m[1]);
  return [...new Set([...out, ...code])];
}

function isDefaultAgentContext(rel) {
  if (rel === 'AGENTS.md') return true;
  if (rel.startsWith('.agents/skills/') && rel.endsWith('/SKILL.md')) return true;
  if (rel.startsWith('.cursor/rules/') && rel.endsWith('.mdc')) return true;
  if (rel === 'docs/ai-workflow.md' || rel === 'docs/ai-coding-rules.md') return true;
  return false;
}

function classify(rel, text) {
  if (rel.startsWith('docs/archive/')) return 'history';
  if (HISTORY_DIRS.some((d) => rel.startsWith(d + '/') || rel === d)) return 'history';
  if (EVIDENCE_PATTERN.test(rel) && rel.startsWith('docs/')) return 'evidence';
  if (PHASE_PATTERN.test(path.basename(rel)) || PHASE_PATTERN.test(text.slice(0, 2000))) {
    if (rel.startsWith('docs/') && /P[4-9]_/.test(path.basename(rel))) return 'history';
  }
  if (rel.startsWith('.agents/skills/')) return 'domain-procedure';
  if (rel.startsWith('.cursor/rules/')) return 'domain-procedure';
  if (rel === 'AGENTS.md') return 'global-agent-invariant';
  if (rel.startsWith('docs/status/')) return 'current-status';
  if (rel.includes('.generated.')) return 'generated-reference';
  if (rel.startsWith('docs/runbooks/')) return 'runbook';
  if (rel.startsWith('docs/architecture/adr/')) return 'architecture-decision';
  if (rel.startsWith('docs/guides/')) return 'human-guide';
  if (rel.startsWith('docs/reference/')) return 'current-reference';
  if (rel.startsWith('docs/')) return 'current-reference';
  return 'current-reference';
}

function main() {
  const date = new Date().toISOString().slice(0, 10);
  const files = walk(root).sort();
  const agentsText = existsSync(path.join(root, 'AGENTS.md')) ? readFileSync(path.join(root, 'AGENTS.md'), 'utf8') : '';
  const packageJson = existsSync(path.join(root, 'package.json'))
    ? readFileSync(path.join(root, 'package.json'), 'utf8')
    : '';

  const items = [];
  const inbound = new Map();

  for (const rel of files) {
    const abs = path.join(root, rel);
    let text = '';
    try {
      text = readFileSync(abs, 'utf8');
    } catch {
      continue;
    }
    const st = statSync(abs);
    const hash = createHash('sha256').update(text).digest('hex');
    const lines = text.split(/\r?\n/).length;
    const fm = extractFrontmatter(text);
    const links = countLinks(text);
    items.push({
      path: rel,
      bytes: st.size,
      lines,
      sha256: hash,
      doc_type: fm.doc_type || null,
      status: fm.status || null,
      owner: fm.owner || null,
      outLinks: links,
      outLinkCount: links.length,
      phaseOrHistory: PHASE_PATTERN.test(path.basename(rel)) || PHASE_PATTERN.test(text.slice(0, 1500)),
      defaultAgentContext: isDefaultAgentContext(rel),
      referencedByAgents: agentsText.includes(rel),
      referencedByPackage: packageJson.includes(rel),
      classification: classify(rel, text),
    });
    for (const link of links) {
      const cleaned = link.split('#')[0].replace(/^\.\//, '');
      if (!cleaned || cleaned.startsWith('http')) continue;
      const key = cleaned.startsWith('/') ? cleaned.slice(1) : cleaned;
      inbound.set(key, (inbound.get(key) || 0) + 1);
    }
  }

  for (const item of items) {
    item.inLinkCount = inbound.get(item.path) || 0;
  }

  const summary = {
    generatedAt: new Date().toISOString(),
    rootCommit: process.env.GIT_COMMIT || 'unverified',
    fileCount: items.length,
    defaultAgentContextCount: items.filter((i) => i.defaultAgentContext).length,
    phaseOrHistoryCount: items.filter((i) => i.phaseOrHistory).length,
    totalBytes: items.reduce((sum, i) => sum + i.bytes, 0),
    totalLines: items.reduce((sum, i) => sum + i.lines, 0),
    byClassification: {},
  };
  for (const item of items) {
    summary.byClassification[item.classification] = (summary.byClassification[item.classification] || 0) + 1;
  }

  const payload = { summary, files: items };
  const outDir = path.join(root, 'artifacts', 'docs-inventory');
  mkdirSync(outDir, { recursive: true });
  const jsonPath = path.join(outDir, `${date}.json`);
  const mdPath = path.join(outDir, `${date}.md`);
  writeFileSync(jsonPath, JSON.stringify(payload, null, 2));

  const topAgent = items
    .filter((i) => i.defaultAgentContext)
    .sort((a, b) => b.bytes - a.bytes)
    .slice(0, 30)
    .map((i) => `- \`${i.path}\` ${i.lines} lines / ${i.bytes} bytes / ${i.classification}`)
    .join('\n');

  const md = `# Docs inventory ${date}

- Files: ${summary.fileCount}
- Total lines: ${summary.totalLines}
- Total bytes: ${summary.totalBytes}
- Default agent context files: ${summary.defaultAgentContextCount}
- Phase/history-like: ${summary.phaseOrHistoryCount}

## By classification

${Object.entries(summary.byClassification)
  .sort((a, b) => b[1] - a[1])
  .map(([k, v]) => `- ${k}: ${v}`)
  .join('\n')}

## Largest default agent-context files

${topAgent}
`;
  writeFileSync(mdPath, md);
  console.log(`Wrote ${normalize(path.relative(root, jsonPath))}`);
  console.log(`Wrote ${normalize(path.relative(root, mdPath))}`);
  console.log(JSON.stringify(summary, null, 2));
}

main();
