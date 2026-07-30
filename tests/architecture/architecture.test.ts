import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execa } from 'execa';
import { describe, expect, it } from 'vitest';
import { classifyAffected } from '../../scripts/architecture/check-affected.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const checker = path.join(root, 'scripts/architecture/check-boundaries.mjs');
const fixtureRoot = 'tests/architecture/fixtures';

function createTempJson(name: string, data: unknown) {
  const dir = mkdtempSync(path.join(tmpdir(), 'tm-arch-'));
  const file = path.join(dir, name);
  writeFileSync(file, `${JSON.stringify(data, null, 2)}\n`);
  return { dir, file };
}

function cleanup(dir: string) {
  rmSync(dir, { recursive: true, force: true });
}

function baseConfig(rootPath: string) {
  return {
    version: 1,
    applications: {
      fixture: { root: rootPath, type: 'frontend-app', notes: 'fixture' },
    },
    modules: [
      {
        id: 'feature-a',
        root: `${rootPath}/a`,
        layer: 'feature',
        type: 'frontend-runtime',
        allowedDependencies: ['feature-b'],
        forbiddenDependencies: [],
        publicEntrypoints: [`${rootPath}/a/index.ts`],
        notes: '',
      },
      {
        id: 'feature-b',
        root: `${rootPath}/b`,
        layer: 'feature',
        type: 'frontend-runtime',
        allowedDependencies: [],
        forbiddenDependencies: ['feature-a'],
        publicEntrypoints: [`${rootPath}/b/index.ts`],
        notes: '',
      },
      {
        id: 'shared',
        root: `${rootPath}/shared`,
        layer: 'shared',
        type: 'frontend-runtime',
        allowedDependencies: [],
        forbiddenDependencies: ['feature'],
        publicEntrypoints: [],
        notes: '',
      },
      {
        id: 'feature',
        root: `${rootPath}/feature`,
        layer: 'feature',
        type: 'frontend-runtime',
        allowedDependencies: ['shared'],
        forbiddenDependencies: [],
        publicEntrypoints: [],
        notes: '',
      },
      {
        id: 'admin-app',
        root: `${rootPath}/admin`,
        layer: 'app',
        type: 'frontend-runtime',
        allowedDependencies: [],
        forbiddenDependencies: ['collector-app'],
        publicEntrypoints: [],
        notes: '',
      },
      {
        id: 'collector-app',
        root: `${rootPath}/collector`,
        layer: 'app',
        type: 'node-runtime',
        allowedDependencies: [],
        forbiddenDependencies: ['admin-app'],
        publicEntrypoints: [],
        notes: '',
      },
    ],
    rules: {
      productionForbiddenPathPatterns: ['**/__tests__/**', '**/*.test.ts', '**/fixtures/**'],
      runtimeCycleSeverity: 'high',
      typeOnlyCycleSeverity: 'medium',
      deepImportSeverity: 'medium',
      crossAppSeverity: 'high',
      goLayerSeverity: 'high',
    },
    ignoredPaths: ['node_modules/**', 'dist/**', 'build/**', 'coverage/**'],
    notes: [],
    skipGo: true,
    skipDefaultTsRoots: true,
    includeTestFixturesInGraph: true,
  };
}

async function runChecker(config: unknown, baseline = { version: 1, violations: [] }) {
  const configTemp = createTempJson('module-boundaries.json', config);
  const baselineTemp = createTempJson('baseline.json', baseline);
  try {
    const result = await execa('node', [checker, '--json', '--config', configTemp.file, '--baseline', baselineTemp.file], { cwd: root, reject: false });
    const report = JSON.parse(result.stdout);
    return { result, report, baselineFile: baselineTemp.file };
  } finally {
    cleanup(configTemp.dir);
    cleanup(baselineTemp.dir);
  }
}

describe('architecture boundary checker', () => {
  it('allows declared dependencies', async () => {
    const { result, report } = await runChecker(baseConfig(`${fixtureRoot}/allowed`));
    expect(result.exitCode).toBe(0);
    expect(report.newViolations).toBe(0);
  });

  it('blocks forbidden shared reverse dependencies', async () => {
    const { result, report } = await runChecker(baseConfig(`${fixtureRoot}/forbidden`));
    expect(result.exitCode).toBe(1);
    expect(report.current.some((item: { ruleId: string }) => item.ruleId === 'forbidden-dependency')).toBe(true);
  });

  it('blocks runtime cycles', async () => {
    const { result, report } = await runChecker(baseConfig(`${fixtureRoot}/cycle`));
    expect(result.exitCode).toBe(1);
    expect(report.current.some((item: { ruleId: string }) => item.ruleId === 'runtime-cycle')).toBe(true);
  });

  it('blocks cross-app dependencies', async () => {
    const { result, report } = await runChecker(baseConfig(`${fixtureRoot}/cross-app`));
    expect(result.exitCode).toBe(1);
    expect(report.current.some((item: { ruleId: string }) => item.ruleId === 'cross-app-dependency' || item.ruleId === 'forbidden-dependency')).toBe(true);
  });

  it('passes when current violations are already baselined', async () => {
    const first = await runChecker(baseConfig(`${fixtureRoot}/cycle`));
    const baseline = { version: 1, violations: first.report.current.map(({ ruleId, severity, sourceModule, targetModule, importPath, fingerprint, count }: Record<string, unknown>) => ({ ruleId, severity, sourceModule, targetModule, importPath, fingerprint, count })) };
    const second = await runChecker(baseConfig(`${fixtureRoot}/cycle`), baseline);
    expect(second.result.exitCode).toBe(0);
    expect(second.report.unchanged).toBeGreaterThan(0);
  });

  it('allows reduced baseline violations and reports ratchet opportunity', async () => {
    const first = await runChecker(baseConfig(`${fixtureRoot}/cycle`));
    const baseline = { version: 1, violations: first.report.current.map(({ ruleId, severity, sourceModule, targetModule, importPath, fingerprint, count }: Record<string, unknown>) => ({ ruleId, severity, sourceModule, targetModule, importPath, fingerprint, count })) };
    const second = await runChecker(baseConfig(`${fixtureRoot}/allowed`), baseline);
    expect(second.result.exitCode).toBe(0);
    expect(second.report.reduced).toBeGreaterThan(0);
  });

  it('updates baseline only with explicit update flag', async () => {
    const configTemp = createTempJson('module-boundaries.json', baseConfig(`${fixtureRoot}/cycle`));
    const baselineTemp = createTempJson('baseline.json', { version: 1, violations: [] });
    try {
      const result = await execa('node', [checker, '--update', '--config', configTemp.file, '--baseline', baselineTemp.file], { cwd: root, reject: false });
      expect(result.exitCode).toBe(0);
      const updated = JSON.parse(readFileSync(baselineTemp.file, 'utf8'));
      expect(updated.violations.length).toBeGreaterThan(0);
    } finally {
      cleanup(configTemp.dir);
      cleanup(baselineTemp.dir);
    }
  });
});

describe('architecture affected classifier', () => {
  it('runs full architecture checks for shared public types', () => {
    const result = classifyAffected(['admin/src/types/product.ts']);
    expect(result.selected.has('architecture-test')).toBe(true);
    expect(result.selected.has('architecture-check')).toBe(true);
    expect(result.selected.has('test-contracts')).toBe(true);
  });

  it('runs architecture tests when module boundary config changes', () => {
    const result = classifyAffected(['tests/architecture/module-boundaries.json']);
    expect(result.selected.has('architecture-test')).toBe(true);
    expect(result.selected.has('architecture-check')).toBe(true);
  });

  it('escalates cross-module changes without recursive quality/test affected calls', () => {
    const result = classifyAffected(['admin/src/pages/A/index.tsx', 'collector/src/providers/x.ts', 'backend/internal/modules/product/service.go']);
    expect(result.selected.has('architecture-test')).toBe(true);
    expect(result.selected.has('architecture-check')).toBe(true);
    expect(result.selected.has('quality-sensitive')).toBe(true);
    expect(result.selected.has('test-frontend')).toBe(true);
    expect(result.selected.has('test-collector')).toBe(true);
    expect(result.selected.has('test-backend')).toBe(true);
  });

  it('falls back safely on empty changes', () => {
    const result = classifyAffected([]);
    expect(result.selected.has('architecture-check')).toBe(true);
  });
});
