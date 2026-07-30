import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');

function has(rel, patterns) {
  const p = path.join(root, rel);
  if (!fs.existsSync(p)) return false;
  const text = fs.readFileSync(p, 'utf8');
  return patterns.every((pattern) => pattern.test(text));
}

const lists = [
  { list: 'product', service: 'backend/internal/modules/product/service.go', keyset: [/DecodeCursor|EncodeCursor/, /created_at DESC, id DESC/, /created_at <|updated_at </] },
  { list: 'order', service: 'backend/internal/modules/order/service.go', keyset: [/DecodeCursor|EncodeCursor/, /created_at DESC, id DESC/, /created_at </] },
  { list: 'inventory', service: 'backend/internal/modules/inventory/center_list.go', keyset: [/DecodeCursor|EncodeCursor/, /id DESC/, /updated_at </] },
  { list: 'task', service: 'backend/internal/modules/taskcenter/service_list.go', keyset: [/DecodeCursor|EncodeCursor/, /updated_at DESC, id DESC/, /updated_at </] },
  { list: 'webhook', service: 'backend/internal/modules/webhook/service.go', keyset: [/DecodeCursor|EncodeCursor/, /created_at DESC, id DESC/, /created_at </] },
  { list: 'operationLog', service: 'backend/internal/modules/operationlog/service.go', keyset: [/DecodeCursor|EncodeCursor/, /created_at DESC, id DESC/, /created_at </] },
].map((item) => {
  const keysetWired = has(item.service, item.keyset);
  return {
    list: item.list,
    status: keysetWired ? 'pending_runtime' : 'failed',
    service: item.service,
    repositoryServiceWired: keysetWired,
    pagesRead: 0,
    rowsRead: 0,
    duplicates: 0,
    unexpectedMissingRows: 0,
    maxPageDurationMs: 0,
    tamperedRejected: false,
    wrongVersionRejected: false,
    crossTenantRejected: false,
    crossShopRejected: false,
    deepOffsetRejected: false,
    limitGuardPassed: false,
    issue: keysetWired ? 'Runtime harness not yet executed.' : 'Business list service is still offset/limit backed or lacks cursor keyset wiring.',
  };
});

const failed = lists.filter((item) => item.status !== 'passed');
const report = {
  phase: 'P7-C2',
  status: failed.length === 0 ? 'passed' : 'failed',
  generatedAt: new Date().toISOString(),
  runtimeEnvironment: fs.existsSync(path.join(docs, 'p7-c2-runtime-environment.json')) ? 'docs/p7-c2-runtime-environment.json' : 'missing',
  lists,
  tamperedRejected: false,
  wrongVersionRejected: false,
  crossTenantRejected: false,
  crossShopRejected: false,
  deepOffsetRejected: false,
  limitGuardPassed: false,
  dryRun: false,
  issues: failed.map((item) => `${item.list}: ${item.issue}`),
};

fs.writeFileSync(path.join(docs, 'p7-c2-pagination-runtime-report.json'), `${JSON.stringify(report, null, 2)}\n`, 'utf8');
fs.writeFileSync(
  path.join(docs, 'P7_C2_PAGINATION_RUNTIME_REPORT.md'),
  `# P7-C2 Pagination Runtime Report\n\nStatus: ${report.status}\n\n${lists.map((item) => `- ${item.list}: ${item.status} - ${item.issue}`).join('\n')}\n`,
  'utf8',
);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
