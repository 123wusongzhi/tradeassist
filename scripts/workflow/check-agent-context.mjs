#!/usr/bin/env node
/**
 * v2 agent context checker.
 * Replaces skill-trigger cascade rules with context-map budgets and routing matrix.
 */
import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { inferRiskFlags, select } from './select-context.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..', '..');

function normalize(p) {
  return String(p || '').replaceAll('\\', '/');
}

function readText(rel) {
  return readFileSync(path.join(root, rel), 'utf8');
}

function readJson(rel) {
  return JSON.parse(readText(rel));
}

function packageScripts() {
  return new Set(Object.keys(readJson('package.json').scripts || {}));
}

function extractFrontmatter(text) {
  const match = text.match(/^---\r?\n([\s\S]*?)\r?\n---/);
  if (!match) return null;
  const fields = new Map();
  for (const line of match[1].trim().split(/\r?\n/)) {
    const field = line.match(/^([A-Za-z][A-Za-z0-9_-]*):\s*(.*)$/);
    if (field) fields.set(field[1], field[2]);
  }
  return fields;
}

function cursorRules() {
  const dir = path.join(root, '.cursor', 'rules');
  if (!existsSync(dir)) return [];
  return readdirSync(dir).filter((f) => f.endsWith('.mdc')).sort();
}

function skillPathsInText(text) {
  const refs = new Set();
  const re = /\.agents\/skills\/[A-Za-z0-9_-]+\/SKILL\.md/g;
  for (const m of text.matchAll(re)) refs.add(m[0]);
  return [...refs];
}

function loadMatrix() {
  const v2 = 'tests/workflow/context-routing-matrix.json';
  const v1 = 'tests/workflow/skill-trigger-matrix.json';
  if (existsSync(path.join(root, v2))) return { path: v2, matrix: readJson(v2) };
  if (existsSync(path.join(root, v1))) return { path: v1, matrix: readJson(v1), legacy: true };
  throw new Error('missing tests/workflow/context-routing-matrix.json');
}

function validateContextMap(map, failures) {
  if (map.version !== 2) failures.push('context-map version must be 2');
  if (!map.budgets) failures.push('context-map missing budgets');
  if (!map.contexts || typeof map.contexts !== 'object') failures.push('context-map missing contexts');
  if (!Array.isArray(map.tasks) || map.tasks.length < 5) failures.push('context-map must include tasks');

  const ids = new Set();
  for (const [id, ctx] of Object.entries(map.contexts || {})) {
    if (ids.has(id)) failures.push(`duplicate context id: ${id}`);
    ids.add(id);
    if (!ctx.path) failures.push(`context ${id} missing path`);
    else if (!existsSync(path.join(root, ctx.path))) failures.push(`context path missing: ${ctx.path}`);
    if (['docs/archive/', 'docs/status/history/', 'artifacts/'].some((p) => normalize(ctx.path).startsWith(p))) {
      failures.push(`context ${id} points to forbidden area: ${ctx.path}`);
    }
  }

  const taskIds = new Set();
  for (const task of map.tasks || []) {
    if (taskIds.has(task.id)) failures.push(`duplicate task id: ${task.id}`);
    taskIds.add(task.id);
    const req = task.requiredContexts || [];
    const opt = task.optionalContexts || [];
    for (const c of req) {
      if (!map.contexts[c]) failures.push(`task ${task.id} unknown required context ${c}`);
    }
    for (const c of opt) {
      if (!map.contexts[c]) failures.push(`task ${task.id} unknown optional context ${c}`);
      if (req.includes(c)) failures.push(`task ${task.id} context ${c} is both required and optional`);
    }
    if (task.id === 'documentation-only') {
      if (req.length !== 1 || req[0] !== 'docs-maintenance') {
        failures.push('documentation-only requiredContexts must be only docs-maintenance');
      }
    }
    if (task.id === 'small-admin-ui' && req.length > 1) {
      failures.push('small-admin-ui requiredContexts must be at most 1');
    }
    if (task.id === 'cross-module-feature' && req.length > 4) {
      failures.push('cross-module-feature requiredContexts must be at most 4');
    }
    if ((task.depth === 'light' || task.depth === 'deep') === false) {
      failures.push(`task ${task.id} depth must be light|deep`);
    }
  }
}

function validateAlwaysApplyBudget(failures) {
  const rules = cursorRules();
  let bytes = 0;
  let lines = 0;
  const always = [];
  for (const rule of rules) {
    const rel = `.cursor/rules/${rule}`;
    const text = readText(rel);
    const fm = extractFrontmatter(text);
    if (fm?.get('alwaysApply') === 'true') {
      always.push(rel);
      bytes += Buffer.byteLength(text, 'utf8');
      lines += text.split(/\r?\n/).length;
    }
  }
  if (bytes > 12288) failures.push(`alwaysApply Cursor rules total ${bytes} bytes exceeds 12288`);
  if (lines > 120) failures.push(`alwaysApply Cursor rules total ${lines} lines exceeds 120`);
  return { always, bytes, lines };
}

function validateSkillsNoCascade(failures, map) {
  const skillsDir = path.join(root, '.agents', 'skills');
  if (!existsSync(skillsDir)) return;
  const optionalPaths = new Set(
    Object.values(map.contexts || {})
      .filter((ctx) => ctx.kind === 'optional-reference')
      .map((ctx) => normalize(ctx.path)),
  );
  for (const name of readdirSync(skillsDir)) {
    const rel = `.agents/skills/${name}/SKILL.md`;
    const abs = path.join(root, rel);
    if (!existsSync(abs)) continue;
    const text = readText(rel);
    // optional-reference skills (e.g. upstream frontend-design) may keep historical cross-links
    if (!optionalPaths.has(rel)) {
      const refs = skillPathsInText(text).filter((r) => r !== rel);
      for (const ref of refs) {
        const surrounding =
          text.includes('optional') ||
          text.includes('按需') ||
          text.includes('optional-reference') ||
          text.includes('深度设计');
        if (!surrounding) {
          failures.push(`${rel} references other skill path without optional marker: ${ref}`);
        }
      }
    }
    const size = statSync(abs).size;
    const lineCount = text.split(/\r?\n/).length;
    if (!optionalPaths.has(rel)) {
      if (lineCount > 160) failures.push(`${rel} has ${lineCount} lines (>160)`);
      if (size > 16 * 1024) failures.push(`${rel} is ${size} bytes (>16KiB)`);
    }
  }
}

function validateCursorAdapters(failures) {
  for (const rule of cursorRules()) {
    const rel = `.cursor/rules/${rule}`;
    const text = readText(rel);
    const refs = skillPathsInText(text);
    if (refs.length > 1) failures.push(`${rel} links more than one skill path`);
    const fm = extractFrontmatter(text);
    if (!fm) failures.push(`${rel}: missing frontmatter`);
    else {
      if (!fm.has('description')) failures.push(`${rel}: missing description`);
      if (!fm.has('alwaysApply')) failures.push(`${rel}: missing alwaysApply`);
    }
  }
}

function validateMatrix(map, matrixInfo, scripts, failures) {
  const { matrix, path: matrixPath, legacy } = matrixInfo;
  if (legacy) {
    console.warn(`DEPRECATION: ${matrixPath} is v1; migrate to tests/workflow/context-routing-matrix.json`);
  }
  if (!matrix.scenarios || matrix.scenarios.length < 10) {
    failures.push('routing matrix must include at least 10 scenarios');
  }

  for (const scenario of matrix.scenarios || []) {
    const expected = scenario.expectedRequiredContexts || scenario.expectedSkills || [];
    const forbidden = scenario.forbiddenRequiredContexts || scenario.forbiddenSkills || [];
    const checks = scenario.expectedChecks || [];
    const files = scenario.files || [];
    const riskFlags = scenario.riskFlags || inferRiskFlags(files);
    const intent = scenario.intent || scenario.taskId || null;

    if (!scenario.id) failures.push('scenario missing id');
    if (!scenario.depth || !['light', 'deep'].includes(scenario.depth)) {
      failures.push(`${scenario.id}: depth must be light|deep`);
    }
    for (const check of checks) {
      if (/^[a-z][a-z0-9-]*(?::[a-z0-9-]+)+/.test(check) && !scripts.has(check.split(/\s/)[0])) {
        // allow non-script labels like "生产数据库"
        const name = check.match(/^([a-z][a-z0-9-]*(?::[a-z0-9-]+)+)/)?.[1];
        if (name && !scripts.has(name)) failures.push(`${scenario.id}: check not in package scripts: ${name}`);
      }
    }

    const result = select(map, {
      intent: intent || undefined,
      files,
      riskFlags,
    });
    if (result.error) {
      failures.push(`${scenario.id}: selector error: ${result.error}`);
      continue;
    }

    const got = new Set(result.requiredContexts);
    for (const need of expected) {
      // v1 skill names may map to v2 context ids; allow either exact or mapped
      const mapped = mapSkillToContext(need);
      if (!got.has(need) && !got.has(mapped)) {
        // For v2 matrix, expected are context ids
        if (!legacy) failures.push(`${scenario.id}: missing required context ${need}; got [${[...got].join(', ')}]`);
      }
    }
    for (const bad of forbidden) {
      const mapped = mapSkillToContext(bad);
      if (got.has(bad) || got.has(mapped)) {
        failures.push(`${scenario.id}: forbidden required context present: ${bad}`);
      }
    }

    if (scenario.id === 'documentation-only' || scenario.intent === 'documentation-only') {
      if (result.requiredContexts.length !== 1 || result.requiredContexts[0] !== 'docs-maintenance') {
        failures.push(`${scenario.id}: docs-only must require only docs-maintenance`);
      }
    }
    if (scenario.id === 'small-admin-ui') {
      if (result.requiredContexts.length > 1) {
        failures.push(`${scenario.id}: small-admin-ui required contexts must be <= 1`);
      }
    }
    if (scenario.id === 'cross-module-feature' && result.requiredContexts.length > 4) {
      failures.push(`${scenario.id}: cross-module required contexts must be <= 4`);
    }

    scenario._selector = {
      requiredContexts: result.requiredContexts,
      checks: result.checks,
      depth: result.depth,
    };
  }
}

function mapSkillToContext(skillOrContext) {
  const map = {
    'frontend-design': 'admin-ui',
    'frontend-unit-testing': 'frontend-unit',
    'admin-e2e-testing': 'admin-e2e',
    'backend-testing': 'backend-testing',
    'api-contract-testing': 'api-contract',
    'modular-architecture': 'architecture-change',
    'code-quality': 'deep-code-review',
    'project-testing': 'project-testing',
  };
  return map[skillOrContext] || skillOrContext;
}

function validateAgentsBudget(failures) {
  if (!existsSync(path.join(root, 'AGENTS.md'))) {
    failures.push('AGENTS.md missing');
    return;
  }
  const text = readText('AGENTS.md');
  const lines = text.split(/\r?\n/).length;
  const bytes = Buffer.byteLength(text, 'utf8');
  if (lines > 100) failures.push(`AGENTS.md has ${lines} lines (>100)`);
  if (bytes > 10 * 1024) failures.push(`AGENTS.md is ${bytes} bytes (>10KiB)`);
  // Should not list all skills as must-read table
  const skillRefs = skillPathsInText(text);
  if (skillRefs.length > 0) {
    failures.push(`AGENTS.md should not hardcode skill paths; found ${skillRefs.length}`);
  }
}

export function validateAgentContext() {
  const failures = [];
  const map = readJson('config/agent/context-map.json');
  const scripts = packageScripts();
  const matrixInfo = loadMatrix();

  validateContextMap(map, failures);
  const always = validateAlwaysApplyBudget(failures);
  validateSkillsNoCascade(failures, map);
  validateCursorAdapters(failures);
  validateMatrix(map, matrixInfo, scripts, failures);
  validateAgentsBudget(failures);

  // commands referenced by tasks must exist
  for (const task of map.tasks || []) {
    for (const check of task.checks || []) {
      const name = check.split(/\s/)[0];
      if (/^[a-z][a-z0-9-]*:/.test(name) && !scripts.has(name)) {
        failures.push(`task ${task.id} check not in package scripts: ${name}`);
      }
    }
  }

  // required scripts
  for (const name of ['agent:context', 'agent:check', 'docs:check']) {
    if (!scripts.has(name)) failures.push(`missing package script: ${name}`);
  }

  return {
    matrixPath: matrixInfo.path,
    scenarioTotal: matrixInfo.matrix.scenarios?.length || 0,
    alwaysApply: always,
    failures,
    ok: failures.length === 0,
  };
}

function main() {
  const result = validateAgentContext();
  console.log('Agent Context Verification (v2)');
  console.log(`matrix: ${result.matrixPath}`);
  console.log(`scenarios: ${result.scenarioTotal}`);
  console.log(`alwaysApply rules: ${result.alwaysApply.always.length} (${result.alwaysApply.lines} lines / ${result.alwaysApply.bytes} bytes)`);
  if (result.failures.length) {
    console.log('\nFailures:');
    for (const f of result.failures) console.log(`- ${f}`);
    process.exitCode = 1;
  } else {
    console.log('\nOK: all agent context checks passed');
  }
}

if (process.argv[1] && process.argv[1].endsWith('check-agent-context.mjs')) {
  main();
}
