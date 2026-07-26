#!/usr/bin/env node
import { existsSync, readFileSync, readdirSync, statSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execa } from 'execa';
import pc from 'picocolors';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..', '..');
const args = process.argv.slice(2);
const updateBaseline = args.includes('--update');
const json = args.includes('--json');
const configPathArg = valueArg('--config');
const baselinePathArg = valueArg('--baseline');
const configPath = path.resolve(root, configPathArg || 'tests/architecture/module-boundaries.json');
const baselinePath = path.resolve(root, baselinePathArg || 'tests/architecture/baselines/module-boundaries.json');

const EXTENSIONS = ['.ts', '.tsx', '.js', '.jsx'];
const INDEX_FILES = EXTENSIONS.map((ext) => `index${ext}`);
const IMPORT_RE = /(?:^|\n)\s*(import\s+type\s+[^'";]+\s+from\s+['"]([^'"]+)['"]|import\s+[^'";]+\s+from\s+['"]([^'"]+)['"]|export\s+type\s+[^'";]+\s+from\s+['"]([^'"]+)['"]|export\s+[^'";]+\s+from\s+['"]([^'"]+)['"]|import\s*\(\s*['"]([^'"]+)['"]\s*\)|require\s*\(\s*['"]([^'"]+)['"]\s*\))/g;

function valueArg(name) {
  const index = args.indexOf(name);
  if (index >= 0) return args[index + 1];
  const prefixed = args.find((arg) => arg.startsWith(`${name}=`));
  return prefixed?.slice(name.length + 1);
}

function readJson(file) {
  return JSON.parse(readFileSync(file, 'utf8'));
}

function toPosix(file) {
  return file.split(path.sep).join('/');
}

function rel(file) {
  return toPosix(path.relative(root, file));
}

function isIgnored(file, config) {
  const normalized = toPosix(file);
  return config.ignoredPaths.some((pattern) => matchGlob(normalized, pattern));
}

function matchGlob(value, pattern) {
  const normalized = pattern.replace(/\\/g, '/');
  const escaped = normalized
    .replace(/[.+^${}()|[\]\\]/g, '\\$&')
    .replace(/\*\*/g, '::DOUBLE_STAR::')
    .replace(/\*/g, '[^/]*')
    .replace(/::DOUBLE_STAR::/g, '.*');
  return new RegExp(`^${escaped}$`).test(value);
}

function walk(dir, config, files = []) {
  if (!existsSync(dir)) return files;
  for (const entry of readdirSync(dir)) {
    const full = path.join(dir, entry);
    const relative = rel(full);
    if (isIgnored(relative, config)) continue;
    const stat = statSync(full);
    if (stat.isDirectory()) walk(full, config, files);
    else if (EXTENSIONS.includes(path.extname(entry))) files.push(full);
  }
  return files;
}

function productionTsRoots(config) {
  const roots = new Set();
  for (const app of Object.values(config.applications)) {
    if (app.type === 'frontend-app' || app.type === 'node-app' || app.type === 'tooling') roots.add(app.root);
  }
  if (!config.skipDefaultTsRoots) {
    roots.add('admin/src');
    roots.add('collector/src');
    roots.add('scripts');
  }
  return [...roots].filter((dir) => existsSync(path.join(root, dir)));
}

function isTestPath(file, config = {}) {
  if (config.includeTestFixturesInGraph && file.startsWith('tests/architecture/fixtures/')) return false;
  return /(^|\/)(__tests__|test|tests|e2e|mock|mocks|fixture|fixtures)(\/|$)/.test(file) || /\.(test|spec)\.[tj]sx?$/.test(file);
}

function parseImports(file) {
  const source = readFileSync(file, 'utf8');
  const imports = [];
  let match;
  while ((match = IMPORT_RE.exec(source))) {
    const specifier = match.slice(2).find(Boolean);
    if (!specifier) continue;
    const statement = match[1];
    imports.push({ specifier, typeOnly: /^import\s+type\b/.test(statement) || /^export\s+type\b/.test(statement) });
  }
  return imports;
}

function resolveImport(fromFile, specifier) {
  if (specifier.startsWith('@/')) return resolveCandidate(path.join(root, 'admin/src', specifier.slice(2)));
  if (specifier.startsWith('~/')) return resolveCandidate(path.join(root, 'admin/src', specifier.slice(2)));
  if (specifier.startsWith('.')) return resolveCandidate(path.resolve(path.dirname(fromFile), specifier));
  if (specifier.startsWith('@trademind/admin/')) return resolveCandidate(path.join(root, 'admin', specifier.slice('@trademind/admin/'.length)));
  if (specifier.startsWith('@trademind/collector/')) return resolveCandidate(path.join(root, 'collector', specifier.slice('@trademind/collector/'.length)));
  return null;
}

function resolveCandidate(candidate) {
  if (existsSync(candidate) && statSync(candidate).isFile()) return candidate;
  for (const ext of EXTENSIONS) {
    if (existsSync(`${candidate}${ext}`)) return `${candidate}${ext}`;
  }
  if (existsSync(candidate) && statSync(candidate).isDirectory()) {
    for (const indexFile of INDEX_FILES) {
      const full = path.join(candidate, indexFile);
      if (existsSync(full)) return full;
    }
  }
  return null;
}

function moduleFor(file, modules) {
  const normalized = toPosix(file);
  const candidates = modules
    .filter((module) => normalized === module.root || normalized.startsWith(`${module.root}/`))
    .sort((a, b) => b.root.length - a.root.length);
  return candidates[0] || null;
}

function buildTsGraph(config) {
  const files = productionTsRoots(config).flatMap((dir) => walk(path.join(root, dir), config)).filter((file) => !isTestPath(rel(file), config));
  const modules = config.modules.map((module) => ({ ...module, root: module.root.replace(/\\/g, '/') }));
  const graph = new Map();
  const typeGraph = new Map();
  const violations = [];

  for (const file of files) {
    const sourcePath = rel(file);
    const sourceModule = moduleFor(sourcePath, modules);
    graph.set(sourcePath, new Set());
    typeGraph.set(sourcePath, new Set());
    for (const item of parseImports(file)) {
      const target = resolveImport(file, item.specifier);
      if (!target) continue;
      const targetPath = rel(target);
      if (isIgnored(targetPath, config)) continue;
      const targetModule = moduleFor(targetPath, modules);
      if (!item.typeOnly) graph.get(sourcePath).add(targetPath);
      else typeGraph.get(sourcePath).add(targetPath);

      if (!isTestPath(sourcePath, config) && isTestPath(targetPath, config)) {
        violations.push(makeViolation('production-imports-test', 'critical', sourcePath, sourceModule, targetPath, targetModule, item.specifier));
      }

      if (sourceModule && targetModule && sourceModule.id !== targetModule.id) {
        checkForbidden(violations, sourcePath, sourceModule, targetPath, targetModule, item.specifier, item.typeOnly);
        checkCrossApp(violations, sourcePath, sourceModule, targetPath, targetModule, item.specifier);
      }
    }
  }

  addCycles(violations, graph, 'runtime-cycle', config.rules.runtimeCycleSeverity || 'high');
  addCycles(violations, typeGraph, 'type-only-cycle', config.rules.typeOnlyCycleSeverity || 'medium', graph);

  return { files, graph, typeGraph, violations };
}

function checkForbidden(violations, sourcePath, sourceModule, targetPath, targetModule, specifier, typeOnly) {
  if (!sourceModule.forbiddenDependencies?.includes(targetModule.id)) return;
  violations.push(makeViolation('forbidden-dependency', typeOnly ? 'medium' : 'high', sourcePath, sourceModule, targetPath, targetModule, specifier));
}

function checkCrossApp(violations, sourcePath, sourceModule, targetPath, targetModule, specifier) {
  const sourceApp = appOf(sourceModule);
  const targetApp = appOf(targetModule);
  if (!sourceApp || !targetApp || sourceApp === targetApp) return;
  if (sourceApp === 'admin' && targetApp === 'collector') violations.push(makeViolation('cross-app-dependency', 'high', sourcePath, sourceModule, targetPath, targetModule, specifier));
  if (sourceApp === 'collector' && targetApp === 'admin') violations.push(makeViolation('cross-app-dependency', 'high', sourcePath, sourceModule, targetPath, targetModule, specifier));
}

function appOf(module) {
  if (module.root.startsWith('admin/')) return 'admin';
  if (module.root.startsWith('collector/')) return 'collector';
  if (module.root.startsWith('backend/')) return 'backend';
  if (module.root.startsWith('scripts/')) return 'scripts';
  if (module.root.startsWith('tests/')) return 'tests';
  return null;
}

function addCycles(violations, graph, ruleId, severity, runtimeGraph = null) {
  const visited = new Set();
  const stack = [];
  const inStack = new Set();
  const seenCycles = new Set();

  function visit(node) {
    visited.add(node);
    stack.push(node);
    inStack.add(node);
    for (const next of graph.get(node) || []) {
      if (runtimeGraph?.get(node)?.has(next)) continue;
      if (!graph.has(next)) continue;
      if (!visited.has(next)) visit(next);
      else if (inStack.has(next)) {
        const cycle = stack.slice(stack.indexOf(next)).concat(next);
        const key = canonicalCycle(cycle);
        if (!seenCycles.has(key)) {
          seenCycles.add(key);
          violations.push({
            ruleId,
            severity,
            source: cycle[0],
            target: cycle[cycle.length - 2],
            sourceModule: moduleSummary(cycle[0]),
            targetModule: moduleSummary(cycle[cycle.length - 2]),
            importPath: cycle.join(' -> '),
            fingerprint: `${ruleId}|${key}`,
            message: `${ruleId}: ${cycle.join(' -> ')}`,
            count: 1,
          });
        }
      }
    }
    stack.pop();
    inStack.delete(node);
  }

  for (const node of graph.keys()) {
    if (!visited.has(node)) visit(node);
  }
}

function canonicalCycle(cycle) {
  const body = cycle.slice(0, -1);
  const rotations = body.map((_, index) => body.slice(index).concat(body.slice(0, index)).join(' -> '));
  rotations.sort();
  return rotations[0];
}

function moduleSummary(pathValue) {
  if (pathValue.startsWith('admin/')) return 'admin';
  if (pathValue.startsWith('collector/')) return 'collector';
  if (pathValue.startsWith('backend/')) return 'backend';
  if (pathValue.startsWith('scripts/')) return 'scripts';
  return 'unknown';
}

function makeViolation(ruleId, severity, sourcePath, sourceModule, targetPath, targetModule, importPath) {
  const source = sourcePath.replace(/\\/g, '/');
  const target = targetPath.replace(/\\/g, '/');
  const sourceId = sourceModule?.id || moduleSummary(source);
  const targetId = targetModule?.id || moduleSummary(target);
  return {
    ruleId,
    severity,
    source,
    target,
    sourceModule: sourceId,
    targetModule: targetId,
    importPath,
    fingerprint: `${ruleId}|${sourceId}|${targetId}|${source}|${importPath}`,
    message: `${ruleId}: ${source} imports ${importPath} (${target})`,
    count: 1,
  };
}

async function checkGo(config) {
  const violations = [];
  if (config.skipGo) return violations;
  const backendDir = path.join(root, 'backend');
  if (!existsSync(backendDir)) return violations;

  const result = await execa('go', ['list', '-json', './...'], { cwd: backendDir, reject: false });
  if (result.exitCode !== 0) {
    violations.push({
      ruleId: 'go-list-failed',
      severity: 'high',
      source: 'backend',
      target: 'go list ./...',
      sourceModule: 'backend',
      targetModule: 'go-toolchain',
      importPath: 'go list ./...',
      fingerprint: 'go-list-failed|backend|go list ./...',
      message: result.stderr || 'go list ./... failed',
      count: 1,
    });
    return violations;
  }

  const packages = parseGoListJson(result.stdout);
  for (const pkg of packages) {
    const relDir = rel(pkg.Dir);
    if (!relDir.startsWith('backend/internal/')) continue;
    const sourceModule = goModuleFor(relDir);
    for (const imported of pkg.Imports || []) {
      if (!imported.startsWith('github.com/trademind-ai/trademind/backend/internal/')) continue;
      const targetRel = `backend/internal/${imported.split('/backend/internal/')[1]}`;
      const targetModule = goModuleFor(targetRel);
      if (!sourceModule || !targetModule || sourceModule === targetModule) continue;
      const ruleId = goForbiddenRule(sourceModule, targetModule);
      if (ruleId) {
        violations.push({
          ruleId,
          severity: 'high',
          source: relDir,
          target: targetRel,
          sourceModule,
          targetModule,
          importPath: imported,
          fingerprint: `${ruleId}|${sourceModule}|${targetModule}|${relDir}|${imported}`,
          message: `${ruleId}: ${relDir} imports ${imported}`,
          count: 1,
        });
      }
    }
  }
  return violations;
}

function parseGoListJson(stdout) {
  const packages = [];
  let depth = 0;
  let start = -1;
  for (let index = 0; index < stdout.length; index += 1) {
    const char = stdout[index];
    if (char === '{') {
      if (depth === 0) start = index;
      depth += 1;
    } else if (char === '}') {
      depth -= 1;
      if (depth === 0 && start >= 0) packages.push(JSON.parse(stdout.slice(start, index + 1)));
    }
  }
  return packages;
}

function goModuleFor(relDir) {
  if (relDir.startsWith('backend/internal/api')) return 'backend-api';
  if (relDir.startsWith('backend/internal/modules')) return 'backend-modules';
  if (relDir.startsWith('backend/internal/providers')) return 'backend-providers';
  if (relDir.startsWith('backend/internal/rdb')) return 'backend-rdb';
  if (relDir.startsWith('backend/internal/queue')) return 'backend-queue';
  if (relDir.startsWith('backend/internal/pkg')) return 'backend-pkg';
  return 'backend-internal';
}

function goForbiddenRule(sourceModule, targetModule) {
  if (targetModule === 'backend-api' && sourceModule !== 'backend-api') return 'go-layer-forbidden-api-dependency';
  if (sourceModule === 'backend-providers' && targetModule === 'backend-modules') return 'go-provider-depends-on-module';
  if ((sourceModule === 'backend-pkg' || sourceModule === 'backend-rdb') && ['backend-api', 'backend-modules', 'backend-providers'].includes(targetModule)) return 'go-shared-reverse-dependency';
  return null;
}

function aggregate(violations) {
  const map = new Map();
  for (const violation of violations) {
    const existing = map.get(violation.fingerprint);
    if (existing) existing.count += 1;
    else map.set(violation.fingerprint, { ...violation, count: violation.count || 1 });
  }
  return [...map.values()].sort((a, b) => a.fingerprint.localeCompare(b.fingerprint));
}

function compareBaseline(current, baseline) {
  const baselineMap = new Map((baseline.violations || []).map((item) => [item.fingerprint, item]));
  const currentMap = new Map(current.map((item) => [item.fingerprint, item]));
  const newViolations = current.filter((item) => !baselineMap.has(item.fingerprint));
  const increased = current.filter((item) => baselineMap.has(item.fingerprint) && item.count > baselineMap.get(item.fingerprint).count);
  const reduced = (baseline.violations || []).filter((item) => !currentMap.has(item.fingerprint) || currentMap.get(item.fingerprint).count < item.count);
  const unchanged = current.filter((item) => baselineMap.has(item.fingerprint) && item.count <= baselineMap.get(item.fingerprint).count);
  return { newViolations, increased, reduced, unchanged };
}

function printableViolation(violation) {
  return `${violation.severity.toUpperCase()} ${violation.ruleId}: ${violation.source} -> ${violation.importPath}`;
}

const config = readJson(configPath);
const tsResult = buildTsGraph(config);
const goViolations = await checkGo(config);
const violations = aggregate([...tsResult.violations, ...goViolations]);
const baseline = existsSync(baselinePath) ? readJson(baselinePath) : { version: 1, violations: [] };
const comparison = compareBaseline(violations, baseline);

if (updateBaseline) {
  const nextBaseline = {
    version: 1,
    violations: violations.map(({ ruleId, severity, sourceModule, targetModule, importPath, fingerprint, count }) => ({ ruleId, severity, sourceModule, targetModule, importPath, fingerprint, count })),
    notes: ['Updated explicitly by pnpm architecture:baseline -- --update. Review the diff before committing.'],
  };
  writeFileSync(baselinePath, `${JSON.stringify(nextBaseline, null, 2)}\n`);
}

const report = {
  tsFiles: tsResult.files.length,
  violations: violations.length,
  newViolations: comparison.newViolations.length,
  increased: comparison.increased.length,
  reduced: comparison.reduced.length,
  unchanged: comparison.unchanged.length,
  baselineUpdated: updateBaseline,
  current: violations,
};

if (json) console.log(JSON.stringify(report, null, 2));
else {
  console.log(pc.cyan('Architecture boundary report'));
  console.log(`TypeScript files scanned: ${report.tsFiles}`);
  console.log(`Current violations: ${report.violations}`);
  console.log(`New violations: ${report.newViolations}`);
  console.log(`Increased baseline violations: ${report.increased}`);
  console.log(`Reduced baseline violations: ${report.reduced}`);
  if (comparison.newViolations.length) {
    console.log(pc.red('\nNew architecture violations:'));
    for (const violation of comparison.newViolations) console.log(`- ${printableViolation(violation)}`);
  }
  if (comparison.increased.length) {
    console.log(pc.red('\nIncreased baseline violations:'));
    for (const violation of comparison.increased) console.log(`- ${printableViolation(violation)}`);
  }
  if (comparison.reduced.length) {
    console.log(pc.yellow('\nReduced historical violations; consider tightening baseline:'));
    for (const violation of comparison.reduced) console.log(`- ${violation.ruleId}: ${violation.fingerprint}`);
  }
  if (updateBaseline) console.log(pc.yellow(`Baseline updated: ${rel(baselinePath)}`));
}

if (!updateBaseline && (comparison.newViolations.length || comparison.increased.length)) process.exit(1);
