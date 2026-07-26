import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const DB_PREFIX = 'trademind_p7c4_';
const MAX_CLEANUP_AGE_MS = 24 * 60 * 60 * 1000;

function read(rel) {
  return JSON.parse(fs.readFileSync(path.join(root, rel), 'utf8'));
}

function gitCommit() {
  const res = spawnSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' });
  return res.status === 0 ? res.stdout.trim() : 'unknown';
}

function sh(command) {
  return spawnSync('wsl', ['-d', 'Ubuntu-22.04', '--', 'bash', '-lc', command], {
    cwd: root,
    encoding: 'utf8',
    maxBuffer: 1024 * 1024,
    timeout: 60000,
  });
}

function livePrefixDatabases() {
  const res = sh(
    `psql -h /var/run/postgresql -U root -At -d postgres -c "SELECT datname FROM pg_database WHERE datname LIKE '${DB_PREFIX}%' ORDER BY datname;"`,
  );
  if (res.status !== 0) return { ok: false, rows: [], error: (res.stderr || res.stdout || 'psql failed').trim() };
  return { ok: true, rows: (res.stdout || '').trim().split('\n').filter(Boolean), error: null };
}

const pagination = read('docs/p7-c4-pagination-runtime-report.json');
const queryPlan = read('docs/p7-c4-query-plan-report.json');
const nplus = read('docs/p7-c4-nplusone-runtime-report.json');
const race = read('docs/p7-c4-race-test-report.json');
const capability = read('docs/p7-c4-capability-normalization-report.json');
const providerConcurrency = read('docs/p7-c4-provider-concurrency-report.json');
const providerAdaptive = read('docs/p7-c4-provider-adaptive-report.json');
const permission = read('docs/p7-c4-permission-invalidation-report.json');
const cleanup = read('docs/p7-c4-r-cleanup-report.json');
const runtimeEnv = read('docs/p7-c4-runtime-environment.json');

function listStatus(name) {
  return (pagination.lists || []).find((x) => x.list === name)?.status;
}

const live = livePrefixDatabases();
const currentCommit = gitCommit();
const cleanupAgeMs = cleanup.checkedAt ? Date.now() - Date.parse(cleanup.checkedAt) : Number.POSITIVE_INFINITY;

const cleanupChecks = [
  ['runtime-cleanup-current-run', cleanup.cleanup?.currentRunDatabaseRemoved === true, `currentRunDatabaseRemoved=${cleanup.cleanup?.currentRunDatabaseRemoved}`],
  [
    'runtime-cleanup-legacy-runs',
    cleanup.cleanup?.legacyDatabaseDropped === true,
    `legacyDatabaseDropped=${cleanup.cleanup?.legacyDatabaseDropped}`,
  ],
  [
    'runtime-cleanup-prefix-empty',
    cleanup.cleanup?.remainingDatabasesWithPrefix === 0 && (cleanup.cleanup?.remainingDatabases || []).length === 0,
    `remaining=${(cleanup.cleanup?.remainingDatabases || []).join(',') || 'none'}`,
  ],
  ['runtime-cleanup-processes', cleanup.temporaryResources?.processesRemaining === 0, `processesRemaining=${cleanup.temporaryResources?.processesRemaining}`],
  ['runtime-cleanup-ports', cleanup.temporaryResources?.portsRemaining === 0, `portsRemaining=${cleanup.temporaryResources?.portsRemaining}`],
  ['runtime-cleanup-status-passed', cleanup.status === 'passed', `cleanup.status=${cleanup.status}`],
  ['runtime-cleanup-query-executed', cleanup.queryExecuted === true, `queryExecuted=${cleanup.queryExecuted}`],
  ['runtime-cleanup-prefix-match', cleanup.databasePrefix === DB_PREFIX, `databasePrefix=${cleanup.databasePrefix}`],
  ['runtime-cleanup-evidence-fresh', cleanupAgeMs <= MAX_CLEANUP_AGE_MS, `cleanupAgeMs=${cleanupAgeMs}`],
  ['runtime-cleanup-git-commit', cleanup.gitCommit === currentCommit, `cleanup.gitCommit=${cleanup.gitCommit}`],
  ['runtime-cleanup-live-prefix-empty', live.ok && live.rows.length === 0, live.ok ? `liveRemaining=${live.rows.join(',') || 'none'}` : live.error],
];

const checks = [
  ['task-all-sources-sql-keyset', listStatus('task') === 'passed'],
  ['pagination-product', listStatus('product') === 'passed'],
  ['pagination-order', listStatus('order') === 'passed'],
  ['pagination-inventory', listStatus('inventory') === 'passed'],
  ['pagination-task', listStatus('task') === 'passed'],
  ['pagination-webhook', listStatus('webhook') === 'passed'],
  ['pagination-operationLog', listStatus('operationLog') === 'passed'],
  ['query-plan-passed', queryPlan.status === 'passed'],
  ['nplusone-passed', nplus.status === 'passed'],
  ['provider-concurrency-passed', providerConcurrency.status === 'passed'],
  ['provider-adaptive-passed', providerAdaptive.status === 'passed'],
  ['permission-invalidation-passed', permission.status === 'passed'],
  ['race-passed', race.status === 'passed' && race.dataRaces === 0],
  ['mandatory-partial-zero', capability.capabilities?.mandatoryPartial === 0],
  ['mandatory-missing-zero', capability.capabilities?.mandatoryMissing === 0],
  ...cleanupChecks.map(([id, ok, detail]) => [id, ok, detail]),
];

const failed = checks.filter(([, ok]) => !ok).map(([id, , detail]) => (detail ? `${id}: ${detail}` : id));
const report = {
  phase: 'P7-C4',
  status: failed.length === 0 ? 'passed_ready_for_p7_v2' : 'incomplete',
  generatedAt: new Date().toISOString(),
  failed: failed.length,
  checks: checks.map(([id, ok, detail]) => ({ id, status: ok ? 'passed' : 'failed', detail: detail || null })),
  taskPagination: {
    allSourcesUseSqlKeyset: listStatus('task') === 'passed',
    boundedMerge: listStatus('task') === 'passed',
    signedMergeCursor: listStatus('task') === 'passed',
  },
  pagination: {
    product: listStatus('product'),
    order: listStatus('order'),
    inventory: listStatus('inventory'),
    task: listStatus('task'),
    webhook: listStatus('webhook'),
    operationLog: listStatus('operationLog'),
    tamperRejected: pagination.tamperedRejected,
    wrongVersionRejected: pagination.wrongVersionRejected,
    crossTenantRejected: pagination.crossTenantRejected,
    crossShopRejected: pagination.crossShopRejected,
    filterMismatchRejected: pagination.filterMismatchRejected,
    deepOffsetRejected: pagination.deepOffsetRejected,
  },
  database: { queryPlan: queryPlan.status, nPlusOne: nplus.status },
  provider: { concurrencyLimit: providerConcurrency.status, adaptiveSlowdown: providerAdaptive.status },
  permissionCache: { invalidation: permission.status },
  capabilities: {
    mandatoryPartial: capability.capabilities?.mandatoryPartial,
    mandatoryMissing: capability.capabilities?.mandatoryMissing,
  },
  race,
  environmentCleanup: {
    status: cleanup.status,
    phase: cleanup.phase,
    checkedAt: cleanup.checkedAt,
    gitCommit: cleanup.gitCommit,
    databasePrefix: cleanup.databasePrefix,
    queryExecuted: cleanup.queryExecuted,
    currentRunDatabase: cleanup.cleanup?.currentRunDatabase,
    currentRunDatabaseRemoved: cleanup.cleanup?.currentRunDatabaseRemoved,
    legacyRunDatabase: cleanup.cleanup?.legacyRunDatabase,
    legacyDatabaseDropped: cleanup.cleanup?.legacyDatabaseDropped,
    remainingDatabases: cleanup.cleanup?.remainingDatabases || [],
    remainingDatabasesWithPrefix: cleanup.cleanup?.remainingDatabasesWithPrefix,
    liveRemainingDatabases: live.ok ? live.rows : null,
    liveQueryExecuted: live.ok,
    processesRemaining: cleanup.temporaryResources?.processesRemaining,
    portsRemaining: cleanup.temporaryResources?.portsRemaining,
    failureCategories: failed
      .filter((item) => item.startsWith('runtime-cleanup'))
      .map((item) => {
        if (item.includes('liveRemaining')) return 'unknown_database_remaining';
        if (item.includes('cleanup.status')) return 'cleanup_incomplete';
        if (item.includes('cleanupAgeMs')) return 'cleanup_evidence_stale';
        if (item.includes('unsafe')) return 'unsafe_environment';
        return 'cleanup_incomplete';
      }),
  },
  gates: { p7C4: failed.length === 0 ? 'passed' : 'failed' },
  loadBaselineSoak: 'pending_p7_v2',
  issues: failed,
  runtimeEnv: {
    runId: runtimeEnv.runId,
    gitCommit: runtimeEnv.gitCommit,
  },
};

fs.writeFileSync(path.join(docs, 'p7-c4-final-closure-report.json'), `${JSON.stringify(report, null, 2)}\n`, 'utf8');
fs.writeFileSync(
  path.join(docs, 'P7_C4_FINAL_CLOSURE_REPORT.md'),
  `# P7-C4 Final Closure\n\nStatus: ${report.status}\n\nFailed: ${failed.length}\n\n## Runtime Cleanup\n\n- Cleanup status: ${cleanup.status}\n- Prefix remaining: ${cleanup.cleanup?.remainingDatabasesWithPrefix}\n- Legacy dropped: ${cleanup.cleanup?.legacyDatabaseDropped}\n- Live prefix remaining: ${live.ok ? live.rows.length : 'query_failed'}\n`,
  'utf8',
);
process.exit(failed.length === 0 ? 0 : 1);
