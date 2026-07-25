#!/usr/bin/env node
import { execa } from 'execa';
import pc from 'picocolors';

const args = process.argv.slice(2);
const all = args.includes('--all');
const baseArg = args.find((arg) => arg.startsWith('--base='));
const base = baseArg?.slice('--base='.length) || process.env.TEST_AFFECTED_BASE || 'HEAD~1';

const commands = new Map([
  ['frontend', ['pnpm', ['test:frontend']]],
  ['collector', ['pnpm', ['test:collector']]],
  ['backend', ['pnpm', ['test:backend']]],
  ['contracts', ['pnpm', ['test:contracts']]],
  ['e2e-smoke', ['pnpm', ['test:e2e:smoke']]],
]);

function classify(path) {
  const selected = new Set();
  if (path.startsWith('admin/src/') || path === 'admin/vitest.config.ts') selected.add('frontend');
  if (path.startsWith('collector/src/') || path === 'collector/vitest.config.ts') selected.add('collector');
  if (path.startsWith('backend/') && path.endsWith('.go')) selected.add('backend');
  if (path.startsWith('backend/internal/testing/integration/') || path.includes('migrate')) selected.add('backend');
  if (path.startsWith('backend/internal/testing/redis/') || path.toLowerCase().includes('redis') || path.toLowerCase().includes('queue')) selected.add('backend');
  if (path.startsWith('tests/contracts/') || path.includes('/services/') || path.includes('router.go') || path.includes('handler.go')) selected.add('contracts');
  if (path.startsWith('admin/e2e/') || path === 'playwright.config.ts') selected.add('e2e-smoke');
  if (path === 'package.json' || path === 'pnpm-lock.yaml' || path.startsWith('.github/workflows/') || path.startsWith('scripts/testing/')) {
    selected.add('frontend');
    selected.add('collector');
    selected.add('backend');
    selected.add('contracts');
  }
  return selected;
}

async function changedFiles() {
  if (all) return ['package.json'];
  const { stdout } = await execa('git', ['diff', '--name-only', base, '--']);
  return stdout.split('\n').map((line) => line.trim()).filter(Boolean);
}

const files = await changedFiles();
const selected = new Set();
for (const file of files) {
  for (const name of classify(file)) selected.add(name);
}
if (!selected.size) selected.add('contracts');

console.log(pc.cyan('Affected files:'));
for (const file of files) console.log(`- ${file}`);
console.log(pc.cyan('Selected test suites:'), [...selected].join(', '));

for (const name of selected) {
  const command = commands.get(name);
  if (!command) continue;
  const [bin, commandArgs] = command;
  console.log(pc.bold(`\n> ${bin} ${commandArgs.join(' ')}`));
  await execa(bin, commandArgs, { stdio: 'inherit' });
}
