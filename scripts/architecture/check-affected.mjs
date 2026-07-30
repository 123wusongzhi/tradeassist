#!/usr/bin/env node
import { existsSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execa } from 'execa';
import pc from 'picocolors';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..', '..');
const args = process.argv.slice(2);
const all = args.includes('--all');
const ci = args.includes('--ci');
const skipQuality = args.includes('--skip-quality');
const skipTests = args.includes('--skip-tests');
const baseArg = valueArg('--base');
const headArg = valueArg('--head');
const base = baseArg || process.env.ARCHITECTURE_BASE_SHA || process.env.QUALITY_BASE_SHA || process.env.TEST_AFFECTED_BASE;
const head = headArg || process.env.ARCHITECTURE_HEAD_SHA || process.env.QUALITY_HEAD_SHA || process.env.GITHUB_SHA || 'HEAD';

const commands = new Map([
  ['architecture-test', { command: ['pnpm', ['architecture:test']], reason: '架构脚本、配置或 fixture 变化' }],
  ['architecture-check', { command: ['pnpm', ['architecture:check']], reason: '完整模块边界、循环依赖、baseline ratchet 检查' }],
  ['quality-sensitive', { command: ['pnpm', ['quality:sensitive']], reason: '与 code-quality 联动检查 changed diff 敏感信息' }],
  ['test-frontend', { command: ['pnpm', ['test:frontend']], reason: 'Admin 模块边界影响前端单元测试' }],
  ['test-collector', { command: ['pnpm', ['test:collector']], reason: 'Collector 模块边界影响采集单元测试' }],
  ['test-backend', { command: ['pnpm', ['test:backend']], reason: 'Go backend 模块边界影响后端测试' }],
  ['test-contracts', { command: ['pnpm', ['test:contracts']], reason: 'API contract、DTO 或跨模块 API 变化' }],
]);

function valueArg(name) {
  const index = args.indexOf(name);
  if (index >= 0) return args[index + 1];
  const prefixed = args.find((arg) => arg.startsWith(`${name}=`));
  return prefixed?.slice(name.length + 1);
}

async function git(commandArgs) {
  const result = await execa('git', commandArgs, { cwd: root, reject: false });
  if (result.exitCode !== 0) throw new Error(result.stderr || `git ${commandArgs.join(' ')} failed`);
  return result.stdout;
}

export async function changedFiles({ forcedAll = all, baseRef = base, headRef = head } = {}) {
  if (forcedAll) return ['tests/architecture/module-boundaries.json'];

  if (baseRef) {
    const commandArgs = headRef && headRef !== 'HEAD' ? ['diff', '--name-only', baseRef, headRef, '--'] : ['diff', '--name-only', baseRef, '--'];
    try {
      const stdout = await git(commandArgs);
      const files = stdout.split('\n').map((line) => line.trim()).filter(Boolean);
      return files.length ? files : [];
    } catch {
      return ['package.json'];
    }
  }

  const unstaged = await git(['diff', '--name-only', '--']);
  const staged = await git(['diff', '--cached', '--name-only', '--']);
  const untracked = await git(['ls-files', '--others', '--exclude-standard']);
  return [...new Set([...unstaged.split('\n'), ...staged.split('\n'), ...untracked.split('\n')].map((line) => line.trim()).filter(Boolean))];
}

export function classifyAffected(files) {
  const selected = new Map();
  const reasons = [];
  const apps = new Set();
  const modules = new Set();

  function add(name, reason) {
    if (!selected.has(name)) selected.set(name, new Set());
    selected.get(name).add(reason || commands.get(name)?.reason || 'matched');
    reasons.push(reason || name);
  }

  for (const file of files) {
    if (file.startsWith('admin/src/')) {
      apps.add('admin');
      modules.add(moduleSegment(file, 2));
      add('architecture-check', 'Admin runtime changed; run lightweight boundary and cycle check');
      if (!skipTests) add('test-frontend', 'Admin affected module changed');
    }
    if (file.startsWith('collector/src/')) {
      apps.add('collector');
      modules.add(moduleSegment(file, 2));
      add('architecture-check', 'Collector runtime changed; run lightweight boundary and cycle check');
      if (!skipTests) add('test-collector', 'Collector affected module changed');
    }
    if (file.startsWith('backend/')) {
      apps.add('backend');
      modules.add(moduleSegment(file, 2));
      add('architecture-check', 'Go backend changed; run Go layer boundary check');
      if (!skipTests && file.endsWith('.go')) add('test-backend', 'Go source changed');
    }
    if (isArchitectureFile(file)) {
      add('architecture-test', 'Architecture scripts/config/baseline changed');
      add('architecture-check', 'Architecture boundary config changed');
    }
    if (isSharedOrCommon(file)) {
      add('architecture-test', 'Shared/common or public type changed');
      add('architecture-check', 'Shared/common changes require full architecture check');
      if (!skipTests) add('test-contracts', 'Shared/common or public type may affect contracts');
    }
    if (isDeepTrigger(file)) {
      add('architecture-check', 'Repository/migration/worker/queue/scheduler/adapter/API boundary changed');
      if (!skipTests) add('test-contracts', 'Boundary-sensitive backend/API change');
    }
    if (file === 'package.json' || file === 'pnpm-lock.yaml' || file === 'pnpm-workspace.yaml' || file.startsWith('.github/workflows/')) {
      add('architecture-test', 'Workspace/package/CI change affects gate wiring');
      add('architecture-check', 'Workspace/package/CI change requires boundary check');
    }
  }

  if (apps.size >= 2 || modules.size >= 3) {
    add('architecture-test', 'Cross-application or three-plus module change');
    add('architecture-check', 'Cross-application or three-plus module change requires full check');
    if (!skipTests) add('test-contracts', 'Cross-module change may affect API contracts');
  }

  if (!skipQuality && files.length) add('quality-sensitive', 'Architecture affected gate links to code-quality sensitive scan');

  if (!selected.size) add('architecture-check', 'No changed files or safe fallback; run lightweight boundary check');

  return { selected, apps: [...apps], modules: [...modules], reasons };
}

function moduleSegment(file, offset) {
  const parts = file.split('/');
  return parts.slice(0, offset + 1).join('/');
}

function isArchitectureFile(file) {
  return file.startsWith('scripts/architecture/') || file.startsWith('tests/architecture/') || file === '.agents/skills/modular-architecture/SKILL.md' || file === 'docs/architecture/module-boundaries.md';
}

function isSharedOrCommon(file) {
  return /(^|\/)(shared|common|types|contracts|constants)(\/|$)/.test(file) || file.includes('dto') || file.includes('DTO') || file.includes('enum') || file.includes('Enum');
}

function isDeepTrigger(file) {
  const lower = file.toLowerCase();
  return lower.includes('migration') || lower.includes('repository') || lower.includes('worker') || lower.includes('queue') || lower.includes('scheduler') || lower.includes('cron') || lower.includes('adapter') || lower.includes('provider') || lower.includes('/handler.go') || lower.includes('/router.go') || lower.includes('/service.go');
}

async function runCheck(name) {
  const item = commands.get(name);
  if (!item) return;
  const [bin, commandArgs] = item.command;
  console.log(pc.bold(`\n> ${bin} ${commandArgs.join(' ')}`));
  await execa(bin, commandArgs, { cwd: root, stdio: 'inherit' });
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const files = await changedFiles();
  const { selected, apps, modules } = classifyAffected(files);

  console.log(pc.cyan('Architecture affected files:'));
  if (files.length) for (const file of files) console.log(`- ${file}`);
  else console.log('- <none>');

  console.log(pc.cyan('\nAffected applications:'), apps.length ? apps.join(', ') : '<none>');
  console.log(pc.cyan('Affected modules:'), modules.length ? modules.join(', ') : '<none>');
  console.log(pc.cyan('\nSelected architecture checks:'));
  for (const [name, reasonSet] of selected) console.log(`- ${name}: ${[...reasonSet].join('; ')}`);

  const order = ['architecture-test', 'architecture-check', 'quality-sensitive', 'test-contracts', 'test-frontend', 'test-collector', 'test-backend'];
  const failures = [];
  for (const name of order.filter((item) => selected.has(item))) {
    try {
      await runCheck(name);
    } catch (error) {
      failures.push({ name, error });
      console.error(pc.red(`Architecture affected check failed: ${name}`));
      break;
    }
  }

  if (process.env.GITHUB_STEP_SUMMARY && ci) {
    const summary = [
      '## Architecture Gate',
      `- Changed files: ${files.length}`,
      `- Affected apps: ${apps.join(', ') || '<none>'}`,
      `- Selected checks: ${[...selected.keys()].join(', ')}`,
      '- Baseline update: disabled in CI',
      failures.length ? '- Result: failed' : '- Result: passed',
      '',
    ].join('\n');
    await import('node:fs').then((fs) => fs.appendFileSync(process.env.GITHUB_STEP_SUMMARY, summary));
  }

  console.log(pc.cyan('\nArchitecture affected report'));
  console.log(`Changed files: ${files.length}`);
  console.log(`Triggered checks: ${[...selected.keys()].join(', ')}`);
  console.log(`Failures: ${failures.length}`);

  if (failures.length) process.exit(1);
  console.log(pc.green('Affected architecture checks passed.'));
}
