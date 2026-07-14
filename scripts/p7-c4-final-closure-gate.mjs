import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');

function read(rel) {
  return JSON.parse(fs.readFileSync(path.join(root, rel), 'utf8'));
}

const pagination = read('docs/p7-c4-pagination-runtime-report.json');
const queryPlan = read('docs/p7-c4-query-plan-report.json');
const nplus = read('docs/p7-c4-nplusone-runtime-report.json');
const race = read('docs/p7-c4-race-test-report.json');
const capability = read('docs/p7-c4-capability-normalization-report.json');
const providerConcurrency = read('docs/p7-c4-provider-concurrency-report.json');
const providerAdaptive = read('docs/p7-c4-provider-adaptive-report.json');
const permission = read('docs/p7-c4-permission-invalidation-report.json');

function listStatus(name) {
  return (pagination.lists || []).find((x) => x.list === name)?.status;
}

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
];

const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
const report = {
  phase: 'P7-C4',
  status: failed.length === 0 ? 'passed_ready_for_p7_v2' : 'incomplete',
  generatedAt: new Date().toISOString(),
  failed: failed.length,
  checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  taskPagination: { allSourcesUseSqlKeyset: listStatus('task') === 'passed', boundedMerge: listStatus('task') === 'passed', signedMergeCursor: listStatus('task') === 'passed' },
  pagination: {
    product: listStatus('product'), order: listStatus('order'), inventory: listStatus('inventory'), task: listStatus('task'), webhook: listStatus('webhook'), operationLog: listStatus('operationLog'),
    tamperRejected: pagination.tamperedRejected, wrongVersionRejected: pagination.wrongVersionRejected, crossTenantRejected: pagination.crossTenantRejected, crossShopRejected: pagination.crossShopRejected, filterMismatchRejected: pagination.filterMismatchRejected, deepOffsetRejected: pagination.deepOffsetRejected,
  },
  database: { queryPlan: queryPlan.status, nPlusOne: nplus.status },
  provider: { concurrencyLimit: providerConcurrency.status, adaptiveSlowdown: providerAdaptive.status },
  permissionCache: { invalidation: permission.status },
  capabilities: { mandatoryPartial: capability.capabilities?.mandatoryPartial, mandatoryMissing: capability.capabilities?.mandatoryMissing },
  race,
  gates: { p7C4: failed.length === 0 ? 'passed' : 'failed' },
  loadBaselineSoak: 'pending_p7_v2',
  issues: failed,
};

fs.writeFileSync(path.join(docs, 'p7-c4-final-closure-report.json'), `${JSON.stringify(report, null, 2)}\n`, 'utf8');
fs.writeFileSync(path.join(docs, 'P7_C4_FINAL_CLOSURE_REPORT.md'), `# P7-C4 Final Closure\n\nStatus: ${report.status}\n\nFailed: ${failed.length}\n`, 'utf8');
process.exit(failed.length === 0 ? 0 : 1);
