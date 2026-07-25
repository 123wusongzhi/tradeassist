#!/usr/bin/env node
import { existsSync, readFileSync, readdirSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..', '..');

export const coreSkills = [
  'frontend-design',
  'frontend-unit-testing',
  'admin-e2e-testing',
  'project-testing',
  'backend-testing',
  'api-contract-testing',
  'code-quality',
  'modular-architecture',
];

const requiredCursorRules = [
  '04-ui-style.mdc',
  'admin-e2e-testing.mdc',
  'project-testing.mdc',
  'code-quality.mdc',
  'modular-architecture.mdc',
];

const skillRules = new Map([
  ['frontend-design', ['04-ui-style.mdc', 'project-testing.mdc', 'code-quality.mdc', 'modular-architecture.mdc', 'admin-e2e-testing.mdc']],
  ['frontend-unit-testing', ['project-testing.mdc', 'code-quality.mdc']],
  ['admin-e2e-testing', ['admin-e2e-testing.mdc', 'project-testing.mdc', 'code-quality.mdc']],
  ['project-testing', ['project-testing.mdc', 'code-quality.mdc', 'modular-architecture.mdc']],
  ['backend-testing', ['project-testing.mdc', 'code-quality.mdc', 'modular-architecture.mdc']],
  ['api-contract-testing', ['project-testing.mdc', 'code-quality.mdc', 'modular-architecture.mdc']],
  ['code-quality', ['code-quality.mdc', 'modular-architecture.mdc']],
  ['modular-architecture', ['modular-architecture.mdc', 'code-quality.mdc']],
]);

const codeScenarioIds = new Set([
  'small-admin-ui',
  'admin-interaction-bug',
  'admin-api-envelope',
  'collector-pure-function',
  'backend-service',
  'backend-repository',
  'migration-change',
  'shared-type-change',
  'new-platform-adapter',
  'new-worker',
  'cross-module-feature',
  'test-only-change',
  'architecture-config-change',
]);

const deepScenarioIds = new Set([
  'backend-repository',
  'migration-change',
  'shared-type-change',
  'new-platform-adapter',
  'new-worker',
  'cross-module-feature',
  'architecture-config-change',
]);

const expectedScriptLikeCheck = /^[a-z][a-z0-9-]*(?::[a-z0-9-]+)+(?:\s|$)/;

function readText(relativePath) {
  return readFileSync(path.join(root, relativePath), 'utf8');
}

function normalize(relativePath) {
  return relativePath.replaceAll('\\', '/');
}

function skillPath(skill) {
  return `.agents/skills/${skill}/SKILL.md`;
}

function rulePath(rule) {
  return `.cursor/rules/${rule}`;
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
  return readdirSync(dir).filter((file) => file.endsWith('.mdc')).sort();
}

function readJson(relativePath) {
  return JSON.parse(readText(relativePath));
}

function existingPackageScripts() {
  return new Set(Object.keys(readJson('package.json').scripts || {}));
}

function referencedPaths(text) {
  const refs = new Set();
  const patterns = [
    /(?:^|[`\s(])((?:\.agents\/skills\/[^`\s)]+\/SKILL\.md)|(?:\.cursor\/rules\/[^`\s)]+)|(?:AGENTS\.md))/g,
    /(?:^|[`\s(])((?:\.agents\\skills\\[^`\s)]+\\SKILL\.md)|(?:\.cursor\\rules\\[^`\s)]+)|(?:AGENTS\.md))/g,
  ];
  for (const pattern of patterns) {
    for (const match of text.matchAll(pattern)) refs.add(normalize(match[1]));
  }
  return [...refs];
}

function scriptName(check) {
  const match = check.match(/^([a-z][a-z0-9-]*(?::[a-z0-9-]+)+)/);
  return match?.[1];
}

function detectAffectedCycles() {
  const files = new Map([
    ['test:affected', 'scripts/testing/test-affected.mjs'],
    ['quality:affected', 'scripts/quality/check-affected.mjs'],
    ['architecture:affected', 'scripts/architecture/check-affected.mjs'],
    ['workflow:check', 'scripts/workflow/check-skill-triggers.mjs'],
  ]);
  const graph = new Map([...files.keys()].map((name) => [name, new Set()]));

  for (const [name, file] of files) {
    const text = readText(file);
    for (const target of files.keys()) {
      if (target !== name && text.includes(target)) graph.get(name).add(target);
    }
  }

  const cycles = [];
  function visit(node, stack = []) {
    if (stack.includes(node)) {
      cycles.push([...stack.slice(stack.indexOf(node)), node]);
      return;
    }
    for (const next of graph.get(node) || []) visit(next, [...stack, node]);
  }
  for (const node of graph.keys()) visit(node);

  return { graph, cycles: cycles.map((cycle) => cycle.join(' -> ')) };
}

function detectReferenceCycles() {
  const files = ['AGENTS.md', ...cursorRules().map(rulePath)];
  const graph = new Map(files.map((file) => [file, new Set()]));
  for (const file of files) {
    const refs = referencedPaths(readText(file));
    for (const ref of refs) {
      if (ref !== file && graph.has(ref)) graph.get(file).add(ref);
    }
  }

  const cycles = [];
  function visit(node, stack = []) {
    if (stack.includes(node)) {
      cycles.push([...stack.slice(stack.indexOf(node)), node]);
      return;
    }
    for (const next of graph.get(node) || []) visit(next, [...stack, node]);
  }
  for (const file of graph.keys()) visit(file);
  return cycles.map((cycle) => cycle.join(' -> '));
}

function assertScenarioPolicies(scenario, scripts) {
  const failures = [];
  const expectedSkills = new Set(scenario.expectedSkills || []);
  const forbiddenSkills = new Set(scenario.forbiddenSkills || []);
  const expectedChecks = new Set(scenario.expectedChecks || []);
  const forbiddenChecks = new Set(scenario.forbiddenChecks || []);

  for (const skill of expectedSkills) {
    if (!coreSkills.includes(skill)) failures.push(`unknown expected skill: ${skill}`);
    if (forbiddenSkills.has(skill)) failures.push(`skill ${skill} is both expected and forbidden`);
  }
  for (const skill of forbiddenSkills) {
    if (!coreSkills.includes(skill)) failures.push(`unknown forbidden skill: ${skill}`);
  }
  for (const check of expectedChecks) {
    if (forbiddenChecks.has(check)) failures.push(`check ${check} is both expected and forbidden`);
    const name = scriptName(check);
    if (name && !scripts.has(name)) failures.push(`expected check ${name} is not a package script`);
    if (!name && expectedScriptLikeCheck.test(check)) failures.push(`expected check ${check} is not parseable as a package script`);
  }

  if (scenario.depth !== 'light' && scenario.depth !== 'deep') failures.push('depth must be light or deep');
  if (!scenario.description) failures.push('description is required');
  if (!scenario.reason) failures.push('reason is required');
  if (!Array.isArray(scenario.files) || !scenario.files.length) failures.push('files must include example affected files');

  if (codeScenarioIds.has(scenario.id)) {
    for (const skill of ['code-quality', 'project-testing']) {
      if (!expectedSkills.has(skill)) failures.push(`code scenario must include ${skill}`);
    }
  }

  if (scenario.id.includes('admin') || scenario.files.some((file) => file.startsWith('admin/src/'))) {
    if (!expectedSkills.has('frontend-design')) failures.push('UI scenario must include frontend-design');
  }
  if (scenario.id.includes('backend') || scenario.files.some((file) => file.startsWith('backend/'))) {
    if (!expectedSkills.has('backend-testing')) failures.push('backend scenario must include backend-testing');
  }
  if (scenario.id.includes('api') || scenario.description.toLowerCase().includes('api') || scenario.description.includes('DTO') || scenario.description.includes('envelope')) {
    if (!expectedSkills.has('api-contract-testing')) failures.push('API scenario must include api-contract-testing');
  }
  if (scenario.id === 'cross-module-feature' && !expectedSkills.has('modular-architecture')) failures.push('cross-module scenario must include modular-architecture');
  if (deepScenarioIds.has(scenario.id) && scenario.depth !== 'deep') failures.push('repository/migration/shared/adapter/worker/cross-module/config scenarios must be deep');
  if (scenario.id === 'small-admin-ui') {
    if (scenario.depth !== 'light') failures.push('small-admin-ui must stay light');
    if (expectedSkills.has('modular-architecture')) failures.push('small-admin-ui must not require modular-architecture deep review');
    if ([...expectedChecks].some((check) => check.startsWith('architecture:'))) failures.push('small-admin-ui must not require architecture deep checks');
  }
  if (scenario.id === 'documentation-only') {
    if ([...expectedChecks].some((check) => check.includes('e2e'))) failures.push('documentation-only must not require E2E');
  }

  return failures;
}

export function validateSkillTriggers({ matrix = readJson('tests/workflow/skill-trigger-matrix.json') } = {}) {
  const failures = [];
  const missingRules = [];
  const conflictRules = [];
  const overTriggered = [];
  const underTriggered = [];
  const scenarioResults = [];
  const scripts = existingPackageScripts();
  const agents = readText('AGENTS.md');
  const rules = cursorRules();
  const ruleTexts = new Map(rules.map((rule) => [rule, readText(rulePath(rule))]));

  if (matrix.version !== 1) failures.push('trigger matrix version must be 1');
  if (!Array.isArray(matrix.scenarios) || matrix.scenarios.length < 14) failures.push('trigger matrix must include at least 14 scenarios');

  for (const skill of coreSkills) {
    const relativePath = skillPath(skill);
    if (!existsSync(path.join(root, relativePath))) failures.push(`missing skill file: ${relativePath}`);
    if (!agents.includes(relativePath)) failures.push(`AGENTS.md does not reference ${relativePath}`);
  }

  for (const rule of requiredCursorRules) {
    if (!rules.includes(rule)) missingRules.push(rule);
  }

  for (const [rule, text] of ruleTexts) {
    const frontmatter = extractFrontmatter(text);
    if (!frontmatter) conflictRules.push(`${rule}: missing frontmatter`);
    else {
      if (!frontmatter.has('description')) conflictRules.push(`${rule}: missing description`);
      if (requiredCursorRules.includes(rule) && !frontmatter.has('globs')) conflictRules.push(`${rule}: missing globs`);
      if (!frontmatter.has('alwaysApply')) conflictRules.push(`${rule}: missing alwaysApply`);
      if (frontmatter.get('alwaysApply') === 'true' && requiredCursorRules.includes(rule)) {
        conflictRules.push(`${rule}: core workflow rule must not alwaysApply`);
      }
    }
    for (const ref of referencedPaths(text)) {
      if (ref !== rulePath(rule) && !existsSync(path.join(root, ref))) conflictRules.push(`${rule}: referenced path does not exist: ${ref}`);
    }
  }

  for (const ref of referencedPaths(agents)) {
    if (!existsSync(path.join(root, ref))) conflictRules.push(`AGENTS.md referenced path does not exist: ${ref}`);
  }

  for (const skill of coreSkills) {
    const candidates = skillRules.get(skill) || [];
    const hasRule = candidates.some((rule) => ruleTexts.get(rule)?.includes(skillPath(skill)));
    const hasAgents = agents.includes(skillPath(skill));
    if (!hasRule && !hasAgents) underTriggered.push(`${skill}: no Cursor rule or AGENTS entry references the skill`);
  }

  for (const scenario of matrix.scenarios || []) {
    const scenarioFailures = assertScenarioPolicies(scenario, scripts);
    for (const failure of scenarioFailures) failures.push(`${scenario.id}: ${failure}`);
    scenarioResults.push({
      id: scenario.id,
      skills: scenario.expectedSkills || [],
      checks: scenario.expectedChecks || [],
      depth: scenario.depth,
      failures: scenarioFailures,
    });
  }

  const { graph, cycles: affectedCycles } = detectAffectedCycles();
  const forbiddenAffectedCycles = affectedCycles.filter((cycle) => {
    return cycle.includes('test:affected') || cycle.includes('quality:affected') || cycle.includes('architecture:affected');
  });
  for (const cycle of forbiddenAffectedCycles) failures.push(`affected orchestration cycle: ${cycle}`);

  const referenceCycles = detectReferenceCycles();
  for (const cycle of referenceCycles) failures.push(`AGENTS/Cursor reference cycle: ${cycle}`);

  if (ruleTexts.get('modular-architecture.mdc')?.includes('按钮间距') && ruleTexts.get('modular-architecture.mdc')?.includes('通常不触发完整深度架构审查')) {
    // expected lightweight guard is present
  } else {
    overTriggered.push('modular-architecture.mdc lacks explicit small-change deep-review exclusion');
  }

  failures.push(...missingRules.map((rule) => `missing required Cursor rule: ${rule}`));
  failures.push(...conflictRules.map((rule) => `conflict rule: ${rule}`));
  failures.push(...overTriggered.map((item) => `over-trigger risk: ${item}`));
  failures.push(...underTriggered.map((item) => `under-trigger risk: ${item}`));

  return {
    scenarioTotal: matrix.scenarios?.length || 0,
    passedScenarios: scenarioResults.filter((item) => !item.failures.length).length,
    failedScenarios: scenarioResults.filter((item) => item.failures.length).length,
    scenarioResults,
    missingRules,
    conflictRules,
    overTriggered,
    underTriggered,
    affectedGraph: Object.fromEntries([...graph].map(([name, targets]) => [name, [...targets]])),
    affectedCycles: forbiddenAffectedCycles,
    referenceCycles,
    failures,
  };
}

export function printReport(result) {
  console.log('AI Workflow Trigger Verification');
  console.log(`Scenarios: ${result.scenarioTotal}`);
  console.log(`Passed scenarios: ${result.passedScenarios}`);
  console.log(`Failed scenarios: ${result.failedScenarios}`);
  console.log('\nScenario skills:');
  for (const scenario of result.scenarioResults) {
    console.log(`- ${scenario.id} [${scenario.depth}]: ${scenario.skills.join(', ')}`);
  }
  console.log('\nMissing rules:');
  console.log(result.missingRules.length ? result.missingRules.map((item) => `- ${item}`).join('\n') : '- none');
  console.log('\nConflict rules:');
  console.log(result.conflictRules.length ? result.conflictRules.map((item) => `- ${item}`).join('\n') : '- none');
  console.log('\nOver-trigger:');
  console.log(result.overTriggered.length ? result.overTriggered.map((item) => `- ${item}`).join('\n') : '- none');
  console.log('\nUnder-trigger:');
  console.log(result.underTriggered.length ? result.underTriggered.map((item) => `- ${item}`).join('\n') : '- none');
  console.log('\nAffected orchestration graph:');
  for (const [name, targets] of Object.entries(result.affectedGraph)) console.log(`- ${name} -> ${targets.join(', ') || '<none>'}`);
  console.log('\nOrchestration cycles:');
  console.log(result.affectedCycles.length ? result.affectedCycles.map((item) => `- ${item}`).join('\n') : '- none');

  if (result.failures.length) {
    console.error('\nFailures:');
    for (const failure of result.failures) console.error(`- ${failure}`);
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const result = validateSkillTriggers();
  printReport(result);
  if (result.failures.length) process.exit(1);
}
