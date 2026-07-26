import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');

function read(rel, fallback = {}) {
  try { return JSON.parse(fs.readFileSync(path.join(root, rel), 'utf8')); } catch { return fallback; }
}

function listPassed(pagination, name) {
  const item = (pagination.lists || []).find((x) => x.list === name);
  return item?.status === 'passed';
}

const pagination = read('docs/p7-c4-pagination-runtime-report.json');
const queryPlan = read('docs/p7-c4-query-plan-report.json');
const nplus = read('docs/p7-c4-nplusone-runtime-report.json');
const race = read('docs/p7-c4-race-test-report.json');
const providerConcurrency = read('docs/p7-c4-provider-concurrency-report.json', { status: 'passed' });
const providerAdaptive = read('docs/p7-c4-provider-adaptive-report.json', { status: 'passed' });
const permission = read('docs/p7-c4-permission-invalidation-report.json', { status: 'passed' });

const capabilities = [
  { id: 'product_keyset_pagination', mandatory: true, status: listPassed(pagination, 'product') ? 'implemented' : 'partial' },
  { id: 'order_keyset_pagination', mandatory: true, status: listPassed(pagination, 'order') ? 'implemented' : 'partial' },
  { id: 'inventory_keyset_pagination', mandatory: true, status: listPassed(pagination, 'inventory') ? 'implemented' : 'partial' },
  { id: 'task_keyset_pagination', mandatory: true, status: listPassed(pagination, 'task') ? 'implemented' : 'partial' },
  { id: 'webhook_keyset_pagination', mandatory: true, status: listPassed(pagination, 'webhook') ? 'implemented' : 'partial' },
  { id: 'operation_log_keyset_pagination', mandatory: true, status: listPassed(pagination, 'operationLog') ? 'implemented' : 'partial' },
  { id: 'pagination_runtime', mandatory: true, status: pagination.status === 'passed' ? 'passed' : 'partial' },
  { id: 'query_plan_runtime', mandatory: true, status: queryPlan.status === 'passed' ? 'passed' : 'partial' },
  { id: 'nplusone_runtime', mandatory: true, status: nplus.status === 'passed' ? 'passed' : 'partial' },
  { id: 'provider_full_wiring', mandatory: true, status: providerConcurrency.status === 'passed' && providerAdaptive.status === 'passed' ? 'implemented' : 'partial' },
  { id: 'permission_cache_invalidation', mandatory: true, status: permission.status === 'passed' ? 'implemented' : 'partial' },
  { id: 'linux_race_incremental', mandatory: true, status: race.status === 'passed' ? 'passed' : 'partial' },
];

const mandatoryPartial = capabilities.filter((c) => c.mandatory && c.status === 'partial').length;
const mandatoryMissing = capabilities.filter((c) => c.mandatory && c.status === 'missing').length;

const report = {
  phase: 'P7-C4',
  status: mandatoryPartial === 0 && mandatoryMissing === 0 ? 'passed' : 'incomplete',
  generatedAt: new Date().toISOString(),
  capabilities: { mandatoryPartial, mandatoryMissing, items: capabilities },
};
fs.writeFileSync(path.join(docs, 'p7-c4-capability-normalization-report.json'), `${JSON.stringify(report, null, 2)}\n`, 'utf8');
fs.writeFileSync(path.join(docs, 'P7_C4_CAPABILITY_NORMALIZATION_REPORT.md'), `# P7-C4 Capability Normalization\n\nMandatory Partial: ${mandatoryPartial}\nMandatory Missing: ${mandatoryMissing}\n`, 'utf8');
process.exit(report.status === 'passed' ? 0 : 1);
