import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const envExists = fs.existsSync(path.join(docs, 'p7-c2-runtime-environment.json'));
const pagination = (() => {
  try {
    return JSON.parse(fs.readFileSync(path.join(docs, 'p7-c2-pagination-runtime-report.json'), 'utf8'));
  } catch {
    return {};
  }
})();
const scenarios = ['Product Cursor List', 'Order Cursor List', 'Inventory Cursor List', 'Task Claim', 'Webhook Dedup', 'Operation Log Cursor List', 'Role/Permission Batch Read'].map((name) => ({
  name,
  status: 'blocked',
  planningTimeMs: null,
  executionTimeMs: null,
  actualRows: null,
  rowsRemovedByFilter: null,
  sharedHitBlocks: null,
  sharedReadBlocks: null,
  scanType: '',
  indexName: '',
  sortMethod: '',
  sortSpaceType: '',
  diskSpill: null,
  issue: envExists ? 'EXPLAIN not executed because pagination/service keyset runtime is not yet passed.' : 'Missing P7-C2 runtime environment.',
}));
const report = {
  phase: 'P7-C2',
  status: 'failed',
  generatedAt: new Date().toISOString(),
  runtimeEnvironment: envExists ? 'docs/p7-c2-runtime-environment.json' : 'missing',
  paginationPrerequisite: pagination.status || 'missing',
  unintendedLargeTableSeqScan: null,
  unresolvedDiskSpill: null,
  checks: scenarios,
  dryRun: false,
  issues: scenarios.map((item) => `${item.name}: ${item.issue}`),
};
fs.writeFileSync(path.join(docs, 'p7-c2-query-plan-report.json'), `${JSON.stringify(report, null, 2)}\n`, 'utf8');
fs.writeFileSync(path.join(docs, 'P7_C2_QUERY_PLAN_REPORT.md'), `# P7-C2 Query Plan Report\n\nStatus: ${report.status}\n\n${report.issues.map((item) => `- ${item}`).join('\n')}\n`, 'utf8');
console.log(JSON.stringify(report, null, 2));
process.exit(1);
