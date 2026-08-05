#!/usr/bin/env node
/**
 * Select minimal agent contexts for a task or changed files.
 *
 * Usage:
 *   node scripts/workflow/select-context.mjs --intent documentation-only
 *   node scripts/workflow/select-context.mjs --files "admin/src/pages/X/index.tsx"
 *   node scripts/workflow/select-context.mjs --files-from-git
 *   node scripts/workflow/select-context.mjs --files-from-git --json
 */
import { execSync } from 'node:child_process';
import { existsSync, readFileSync, statSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..', '..');

const FORBIDDEN_CONTEXT_PREFIXES = ['docs/archive/', 'docs/status/history/', 'artifacts/'];

function normalize(p) {
  return String(p || '').replaceAll('\\', '/').replace(/^\.\//, '');
}

function readJson(rel) {
  return JSON.parse(readFileSync(path.join(root, rel), 'utf8'));
}

function parseArgs(argv) {
  const args = {
    intent: null,
    files: [],
    filesFromGit: false,
    json: false,
    riskFlags: [],
  };
  for (let i = 0; i < argv.length; i += 1) {
    const a = argv[i];
    if (a === '--intent') args.intent = argv[++i];
    else if (a === '--files') {
      const value = argv[++i] || '';
      args.files.push(...value.split(',').map((s) => s.trim()).filter(Boolean));
    } else if (a === '--files-from-git') args.filesFromGit = true;
    else if (a === '--json') args.json = true;
    else if (a === '--risk') {
      const value = argv[++i] || '';
      args.riskFlags.push(...value.split(',').map((s) => s.trim()).filter(Boolean));
    } else if (a === '--help' || a === '-h') args.help = true;
  }
  return args;
}

function gitFiles() {
  const cmds = [
    'git diff --name-only',
    'git diff --name-only --cached',
    'git ls-files --others --exclude-standard',
  ];
  const set = new Set();
  for (const cmd of cmds) {
    try {
      const out = execSync(cmd, { cwd: root, encoding: 'utf8' });
      for (const line of out.split(/\r?\n/)) {
        const n = normalize(line.trim());
        if (n) set.add(n);
      }
    } catch {
      // ignore git failures in non-git environments
    }
  }
  return [...set];
}

function matchGlob(file, pattern) {
  const f = normalize(file);
  const p = normalize(pattern);
  if (p.startsWith('**/')) {
    const rest = p.slice(3);
    if (rest.includes('**')) return simpleGlob(f, p);
    // **/foo or **/*.md
    if (rest.startsWith('*.')) {
      const ext = rest.slice(1); // .md
      return f.endsWith(ext) || f.includes(ext);
    }
    return f === rest || f.endsWith('/' + rest) || f.includes('/' + rest.replace(/^\*\//, ''));
  }
  return simpleGlob(f, p);
}

function simpleGlob(file, pattern) {
  // Convert limited glob to regex: ** * ?
  let re = '';
  for (let i = 0; i < pattern.length; i += 1) {
    const ch = pattern[i];
    if (ch === '*' && pattern[i + 1] === '*') {
      re += '.*';
      i += 1;
      if (pattern[i + 1] === '/') {
        // already covered by .*
      }
    } else if (ch === '*') {
      re += '[^/]*';
    } else if (ch === '?') {
      re += '[^/]';
    } else if (/[.+^${}()|[\]\\]/.test(ch)) {
      re += `\\${ch}`;
    } else {
      re += ch;
    }
  }
  return new RegExp(`^${re}$`).test(file);
}

function inferRiskFlags(files) {
  const flags = new Set();
  for (const file of files) {
    const f = normalize(file);
    if (f.includes('migration') || f.endsWith('.sql') || f.includes('/migrations/')) flags.add('migration').add('db-schema');
    if (f.includes('handler') || f.includes('router') || f.includes('api-contracts') || f.includes('/api/')) flags.add('api-contract');
    if (f.includes('types/') || f.includes('dto') || f.includes('envelope')) flags.add('dto').add('api-contract');
    if (f.includes('providers/platform') || f.includes('/adapter')) flags.add('platform-adapter');
    if (f.includes('/queue/') || f.includes('worker') || f.includes('scheduler')) flags.add('queue-worker');
    if (f.includes('shared') || f.includes('module-boundaries') || f.includes('common/')) flags.add('shared-boundary');
    if (f.includes('repository') || f.includes('migrate')) flags.add('db-schema');
    if (f.includes('auth') || f.includes('tenant') || f.includes('permission') || f.includes('rbac')) {
      flags.add('auth').add('tenant-boundary');
    }
    if (f.includes('e2e/') || /Modal|Drawer|Form|Submit|publish|sync/i.test(f)) {
      if (f.startsWith('admin/')) flags.add('write-flow');
    }
    if (f.includes('routes') || f.includes('config/routes')) flags.add('route-change');
    if (f.includes('components/') && f.startsWith('admin/src/')) flags.add('responsive-layout');
  }
  return [...flags];
}

function taskMatches(task, files, riskFlags) {
  const match = task.match || {};
  const include = match.include || [];
  const exclude = match.exclude || [];
  const riskAny = match.riskFlagsAny || [];
  const riskAbsent = match.riskFlagsAbsent || [];

  if (riskAny.length) {
    if (!riskAny.some((r) => riskFlags.includes(r))) return false;
  }
  if (riskAbsent.length) {
    if (riskAbsent.some((r) => riskFlags.includes(r))) return false;
  }
  if (include.length) {
    const hit = files.some((file) => include.some((pat) => matchGlob(file, pat)));
    if (!hit) return false;
  }
  if (exclude.length) {
    const blocked = files.some((file) => exclude.some((pat) => matchGlob(file, pat)));
    if (blocked && !riskAny.length) {
      // exclude only applies when matching purely by path
      const onlyExcluded = files.every((file) => exclude.some((pat) => matchGlob(file, pat)));
      if (onlyExcluded) return false;
    }
  }
  return include.length > 0 || riskAny.length > 0;
}

function select(map, { intent, files, riskFlags }) {
  const reasons = [];
  let selectedTasks = [];

  if (intent) {
    const byId = map.tasks.find((t) => t.id === intent);
    if (!byId) {
      return { error: `unknown intent/task id: ${intent}` };
    }
    selectedTasks = [byId];
    reasons.push(`explicit intent ${intent}`);
  } else {
    const matches = map.tasks
      .filter((task) => taskMatches(task, files, riskFlags))
      .sort((a, b) => b.priority - a.priority);
    if (!matches.length) {
      // fallback: if only markdown-like, docs; else light backend/admin by path
      if (files.every((f) => /\.(md|mdc)$/i.test(f) || f.startsWith('docs/') || f.startsWith('config/agent/'))) {
        selectedTasks = [map.tasks.find((t) => t.id === 'documentation-only')].filter(Boolean);
        reasons.push('fallback documentation-only for markdown/docs paths');
      } else if (files.some((f) => f.startsWith('admin/'))) {
        selectedTasks = [map.tasks.find((t) => t.id === 'small-admin-ui')].filter(Boolean);
        reasons.push('fallback small-admin-ui for admin paths');
      } else if (files.some((f) => f.startsWith('backend/'))) {
        selectedTasks = [map.tasks.find((t) => t.id === 'backend-service')].filter(Boolean);
        reasons.push('fallback backend-service for backend paths');
      } else if (files.some((f) => f.startsWith('collector/') || f.startsWith('browser-extension/'))) {
        selectedTasks = [map.tasks.find((t) => t.id === 'collector-change')].filter(Boolean);
        reasons.push('fallback collector-change');
      } else {
        return { error: 'no matching task for provided files; pass --intent or --risk' };
      }
    } else {
      // Keep highest priority task plus any equal-priority deep risks that add new contexts, but cap required
      const top = matches[0];
      selectedTasks = [top];
      reasons.push(`matched ${top.id} (priority ${top.priority})`);
      for (const extra of matches.slice(1)) {
        if (extra.priority >= top.priority - 10 && extra.depth === 'deep') {
          selectedTasks.push(extra);
          reasons.push(`also matched ${extra.id}`);
        }
      }
    }
  }

  const requiredSet = new Set();
  const optionalMap = new Map();
  const checks = new Set();
  let depth = 'light';
  const taskIds = [];

  for (const task of selectedTasks) {
    taskIds.push(task.id);
    for (const c of task.requiredContexts || []) requiredSet.add(c);
    for (const c of task.optionalContexts || []) {
      if (!optionalMap.has(c)) optionalMap.set(c, `optional for ${task.id}; load only when risk condition hits`);
    }
    for (const c of task.checks || []) checks.add(c);
    if (task.depth === 'deep') depth = 'deep';
  }

  // Optional must never be auto-required
  for (const c of requiredSet) optionalMap.delete(c);

  // Cap required contexts: cross-module merges stay reviewable (max 4)
  const MAX_REQUIRED = 4;
  let requiredContexts = [...requiredSet];
  if (requiredContexts.length > MAX_REQUIRED) {
    const overflow = requiredContexts.slice(MAX_REQUIRED);
    requiredContexts = requiredContexts.slice(0, MAX_REQUIRED);
    for (const id of overflow) {
      optionalMap.set(id, 'demoted from required due to max 4 required contexts budget');
    }
  }
  const optionalContexts = [...optionalMap.entries()].map(([id, when]) => ({ id, when }));

  const requiredPaths = [];
  for (const id of requiredContexts) {
    const ctx = map.contexts[id];
    if (!ctx) return { error: `unknown context id in requiredContexts: ${id}` };
    const p = normalize(ctx.path);
    if (FORBIDDEN_CONTEXT_PREFIXES.some((prefix) => p.startsWith(prefix))) {
      return { error: `forbidden context path selected: ${p}` };
    }
    const abs = path.join(root, p);
    if (!existsSync(abs)) return { error: `context path does not exist: ${p}` };
    requiredPaths.push(p);
  }

  for (const opt of optionalContexts) {
    const ctx = map.contexts[opt.id];
    if (!ctx) return { error: `unknown context id in optionalContexts: ${opt.id}` };
    const p = normalize(ctx.path);
    if (!existsSync(path.join(root, p))) return { error: `optional context path does not exist: ${p}` };
  }

  let requiredBytes = 0;
  for (const p of requiredPaths) {
    requiredBytes += statSync(path.join(root, p)).size;
  }

  const budgets = map.budgets;
  const maxFiles = depth === 'deep' ? budgets.deepRequiredFilesMax : budgets.normalRequiredFilesMax;
  const maxBytes = depth === 'deep' ? budgets.deepRequiredBytesMax : budgets.normalRequiredBytesMax;
  if (requiredPaths.length > maxFiles) {
    return {
      error: `required context files ${requiredPaths.length} exceed budget ${maxFiles} for depth=${depth}`,
      taskIds,
      requiredContexts,
    };
  }
  if (requiredBytes > maxBytes) {
    return {
      error: `required context bytes ${requiredBytes} exceed budget ${maxBytes} for depth=${depth}`,
      taskIds,
      requiredContexts,
      requiredPaths,
      requiredBytes,
    };
  }

  return {
    taskIds,
    primaryTask: taskIds[0],
    files: files.map(normalize),
    riskFlags,
    requiredContexts,
    requiredPaths,
    optionalContexts,
    checks: [...checks],
    depth,
    requiredFileCount: requiredPaths.length,
    requiredBytes,
    budgets: { maxFiles, maxBytes },
    reasons,
  };
}

function printHuman(result) {
  if (result.error) {
    console.error(`ERROR: ${result.error}`);
    process.exitCode = 1;
    return;
  }
  console.log('Agent Context Selection');
  console.log(`task: ${result.primaryTask}${result.taskIds.length > 1 ? ` (+${result.taskIds.slice(1).join(',')})` : ''}`);
  console.log(`depth: ${result.depth}`);
  console.log(`files: ${result.files.length ? result.files.join(', ') : '(none)'}`);
  console.log(`riskFlags: ${result.riskFlags.length ? result.riskFlags.join(', ') : '(none)'}`);
  console.log('requiredContexts:');
  for (const id of result.requiredContexts) {
    const p = result.requiredPaths[result.requiredContexts.indexOf(id)];
    console.log(`  - ${id} -> ${p}`);
  }
  console.log('optionalContexts (do not auto-load):');
  if (!result.optionalContexts.length) console.log('  - none');
  for (const item of result.optionalContexts) console.log(`  - ${item.id}: ${item.when}`);
  console.log('checks:');
  for (const c of result.checks) console.log(`  - ${c}`);
  console.log(`budget: ${result.requiredFileCount} files / ${result.requiredBytes} bytes (max ${result.budgets.maxFiles} / ${result.budgets.maxBytes})`);
  console.log('reasons:');
  for (const r of result.reasons) console.log(`  - ${r}`);
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  if (args.help) {
    console.log(`Usage:
  pnpm agent:context -- --intent documentation-only
  pnpm agent:context -- --files "admin/src/pages/X/index.tsx"
  pnpm agent:context -- --files-from-git
  pnpm agent:context -- --files-from-git --json
  pnpm agent:context -- --files "backend/x.go" --risk db-schema,api-contract`);
    return;
  }

  const map = readJson('config/agent/context-map.json');
  if (map.version !== 2) {
    console.error('context-map.json version must be 2');
    process.exit(1);
  }

  let files = args.files.map(normalize);
  if (args.filesFromGit) files = [...new Set([...files, ...gitFiles()])];
  const inferred = inferRiskFlags(files);
  const riskFlags = [...new Set([...args.riskFlags, ...inferred])];

  const result = select(map, { intent: args.intent, files, riskFlags });
  if (args.json) {
    console.log(JSON.stringify(result, null, 2));
    if (result.error) process.exit(1);
    return;
  }
  printHuman(result);
  if (result.error) process.exit(1);
}

export { select, matchGlob, inferRiskFlags, normalize };

if (import.meta.url === `file://${process.argv[1].replaceAll('\\', '/')}` || process.argv[1]?.endsWith('select-context.mjs')) {
  main();
}
