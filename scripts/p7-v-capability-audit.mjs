import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const jsonPath = path.join(docs, 'p7-v-capability-completeness-audit.json');
const mdPath = path.join(docs, 'P7_V_CAPABILITY_COMPLETENESS_AUDIT.md');

const implemented = [
  'core-list-max-limit',
  'deep-offset-upper-bound',
  'cursor-tamper-protection',
  'cursor-tenant-scope',
  'cursor-shop-scope',
  'db-max-open-idle-connections',
  'connection-lifetime-idle-time',
  'query-timeout-config',
  'transaction-timeout-config',
  'http-rate-limit',
  'auth-rate-limit',
  'webhook-burst-limit',
  '429-retry-after',
  'cache-ttl',
  'cache-entry-bound',
  'streaming-export',
  'export-maximum-rows',
  'export-maximum-bytes',
  'export-concurrency',
  'upload-body-limit',
  'bounded-readall',
  'pprof-internal-protection',
  'production-performance-test-guard',
  'load-test-public-host-guard',
];

const partial = [
  'product-cursor-keyset-pagination',
  'order-cursor-keyset-pagination',
  'inventory-cursor-keyset-pagination',
  'task-cursor-keyset-pagination',
  'webhook-cursor-keyset-pagination',
  'operation-log-cursor-keyset-pagination',
  'db-pool-wait-metrics',
  'rows-transaction-leak-protection',
  'worker-bounded-concurrency',
  'worker-bounded-queue',
  'worker-max-inflight',
  'worker-backpressure',
  'worker-priority',
  'worker-fairness',
  'worker-graceful-shutdown',
  'provider-rate-limit',
  'provider-concurrency-limit',
  'adaptive-slowdown',
  'redis-distributed-limiting-foundation',
  'redis-failure-safe-fallback',
  'tenant-quota',
  'shop-quota',
  'user-route-group-quota-boundary',
  'cache-invalidation',
  'singleflight',
  'negative-cache',
  'permission-cache-invalidation',
  'cache-failure-cross-tenant-protection',
  'streaming-upload',
  'temporary-file-cleanup',
  'memory-budget',
  'goroutine-lifecycle',
  'ticker-cleanup',
];

const names = [
  ['core-list-max-limit', '核心列表最大 Limit'],
  ['deep-offset-upper-bound', '深 Offset 上限'],
  ['product-cursor-keyset-pagination', '商品 Cursor/Keyset Pagination'],
  ['order-cursor-keyset-pagination', '订单 Cursor/Keyset Pagination'],
  ['inventory-cursor-keyset-pagination', '库存 Cursor/Keyset Pagination'],
  ['task-cursor-keyset-pagination', '任务 Cursor/Keyset Pagination'],
  ['webhook-cursor-keyset-pagination', 'Webhook Cursor/Keyset Pagination'],
  ['operation-log-cursor-keyset-pagination', '审计日志 Cursor/Keyset Pagination'],
  ['cursor-tamper-protection', 'Cursor 防篡改'],
  ['cursor-tenant-scope', 'Cursor Tenant Scope'],
  ['cursor-shop-scope', 'Cursor Shop Scope'],
  ['db-max-open-idle-connections', 'DB Max Open/Idle Connections'],
  ['connection-lifetime-idle-time', 'Connection Lifetime/Idle Time'],
  ['query-timeout-config', 'Query Timeout'],
  ['transaction-timeout-config', 'Transaction Timeout'],
  ['db-pool-wait-metrics', 'DB Pool Wait Metrics'],
  ['rows-transaction-leak-protection', 'Rows/Transaction Leak Protection'],
  ['worker-bounded-concurrency', 'Worker Bounded Concurrency'],
  ['worker-bounded-queue', 'Worker Bounded Queue'],
  ['worker-max-inflight', 'Worker Max Inflight'],
  ['worker-backpressure', 'Worker Backpressure'],
  ['worker-priority', 'Worker Priority'],
  ['worker-fairness', 'Worker Fairness'],
  ['worker-graceful-shutdown', 'Worker Graceful Shutdown'],
  ['http-rate-limit', 'HTTP Rate Limit'],
  ['auth-rate-limit', 'Auth Rate Limit'],
  ['webhook-burst-limit', 'Webhook Burst Limit'],
  ['provider-rate-limit', 'Provider Rate Limit'],
  ['provider-concurrency-limit', 'Provider Concurrency Limit'],
  ['429-retry-after', '429 Retry-After'],
  ['adaptive-slowdown', 'Adaptive Slowdown'],
  ['redis-distributed-limiting-foundation', 'Redis Distributed Limiting Foundation'],
  ['redis-failure-safe-fallback', 'Redis Failure Safe Fallback'],
  ['tenant-quota', 'Tenant Quota'],
  ['shop-quota', 'Shop Quota'],
  ['user-route-group-quota-boundary', 'User/Route Group Quota Boundary'],
  ['cache-ttl', 'Cache TTL'],
  ['cache-invalidation', 'Cache Invalidation'],
  ['cache-entry-bound', 'Cache Entry Bound'],
  ['singleflight', 'Singleflight'],
  ['negative-cache', 'Negative Cache'],
  ['permission-cache-invalidation', 'Permission Cache Invalidation'],
  ['cache-failure-cross-tenant-protection', 'Cache Failure Cannot Cause Cross-Tenant Access'],
  ['streaming-export', 'Streaming Export'],
  ['export-maximum-rows', 'Export Maximum Rows'],
  ['export-maximum-bytes', 'Export Maximum Bytes'],
  ['export-concurrency', 'Export Concurrency'],
  ['streaming-upload', 'Streaming Upload'],
  ['upload-body-limit', 'Upload Body Limit'],
  ['temporary-file-cleanup', 'Temporary File Cleanup'],
  ['memory-budget', 'Memory Budget'],
  ['bounded-readall', 'Bounded ReadAll'],
  ['goroutine-lifecycle', 'Goroutine Lifecycle'],
  ['ticker-cleanup', 'Ticker Cleanup'],
  ['pprof-internal-protection', 'pprof Internal Protection'],
  ['production-performance-test-guard', 'Production Performance Test Guard'],
  ['load-test-public-host-guard', 'Load Test Public Host Guard'],
];

const capabilities = names.map(([id, capability]) => {
  const status = implemented.includes(id) ? 'implemented' : partial.includes(id) ? 'partial' : 'missing';
  return {
    id,
    capability,
    requirement: 'P7-V mandatory capability audit item',
    codeLocation: codeLocation(id),
    configLocation: configLocation(id),
    unitTestLocation: testLocation(id),
    staticGate: 'scripts/p7-performance-capacity-check.mjs / scripts/p7-v-final-closure-gate.mjs',
    runtimeVerification: runtimeVerification(id),
    status,
    gap: gap(status, id),
    resultThisRound: result(status, id),
  };
});

const report = {
  phase: 'P7-V',
  status: capabilities.some((c) => c.status === 'missing' || c.status === 'partial') ? 'incomplete' : 'implemented',
  generatedAt: new Date().toISOString(),
  summary: {
    implemented: capabilities.filter((c) => c.status === 'implemented').length,
    partial: capabilities.filter((c) => c.status === 'partial').length,
    missing: capabilities.filter((c) => c.status === 'missing').length,
    notApplicable: capabilities.filter((c) => c.status === 'not_applicable').length,
  },
  capabilities,
  productionReady: false,
};

fs.mkdirSync(docs, { recursive: true });
fs.writeFileSync(jsonPath, `${JSON.stringify(report, null, 2)}\n`);
fs.writeFileSync(mdPath, markdown(report));
console.log(JSON.stringify({ phase: 'P7-V', status: report.status, summary: report.summary }, null, 2));
process.exit(report.status === 'implemented' ? 0 : 1);

function codeLocation(id) {
  if (id.includes('cursor') || id.includes('offset') || id.includes('limit')) return 'backend/internal/pkg/pagination; selected list services';
  if (id.startsWith('db') || id.includes('connection') || id.includes('transaction')) return 'backend/internal/database; backend/internal/config/p7_config.go';
  if (id.startsWith('worker')) return 'backend/internal/modules/*/worker*; backend/internal/pkg/tasklease; backend/internal/pkg/taskretry';
  if (id.includes('rate') || id.includes('429') || id.includes('redis')) return 'backend/internal/pkg/ratelimit; backend/internal/middleware/ratelimit.go; providers';
  if (id.includes('cache') || id.includes('singleflight')) return 'backend/internal/pkg/cache; backend/internal/config/p7_config.go';
  if (id.includes('export') || id.includes('upload') || id.includes('readall') || id.includes('temporary')) return 'backend/internal/modules/exportmod; backend/internal/modules/files; backend/internal/pkg/security/upload.go';
  if (id.includes('pprof')) return 'backend/internal/config/p7_config.go; backend/cmd/server';
  return 'backend/internal';
}

function configLocation(id) {
  if (id.includes('pagination') || id.includes('offset') || id.includes('limit')) return 'PAGINATION_*';
  if (id.startsWith('db') || id.includes('connection') || id.includes('transaction')) return 'DB_*';
  if (id.startsWith('worker')) return 'WORKER_*';
  if (id.includes('rate') || id.includes('429') || id.includes('redis')) return 'RATE_LIMIT_*';
  if (id.includes('cache') || id.includes('singleflight')) return 'CACHE_*';
  if (id.includes('export')) return 'EXPORT_*';
  if (id.includes('upload')) return 'UPLOAD_MAX_MB / WEBHOOK_MAX_BODY_KB';
  if (id.includes('pprof')) return 'PPROF_*';
  return 'PERFORMANCE_*';
}

function testLocation(id) {
  if (id.includes('pagination') || id.includes('cursor') || id.includes('offset')) return 'backend/internal/pkg/pagination/pagination_test.go';
  if (id.includes('rate')) return 'backend/internal/pkg/ratelimit/limiter_test.go';
  if (id.includes('cache')) return 'backend/internal/pkg/cache/...';
  if (id.startsWith('worker')) return 'worker/task module tests and P7-V race matrix';
  return 'module tests plus P7-V runtime reports';
}

function runtimeVerification(id) {
  if (id.includes('pagination') || id.includes('cursor') || id.includes('offset')) return 'docs/P7_V_PAGINATION_RUNTIME_REPORT.md';
  if (id.startsWith('db')) return 'docs/P7_V_QUERY_PLAN_RUNTIME_REPORT.md and load reports';
  if (id.startsWith('worker')) return 'tests/load/task-backlog.js and soak/race reports';
  if (id.includes('rate') || id.includes('429')) return 'tests/load/auth-rate-limit.js / provider-429.js / webhook-burst.js';
  return 'P7-V load/soak/final gate';
}

function gap(status, id) {
  if (status === 'implemented') return 'No code-level gap found in this audit; runtime closure evidence may still be required.';
  if (status === 'partial') return `Code foundation exists for ${id}, but P7-V requires runtime evidence or broader module adoption before closure.`;
  return `Mandatory P7-V capability ${id} has no sufficient implementation evidence.`;
}

function result(status, id) {
  if (status === 'implemented') return 'Audited as implemented at code-foundation level.';
  if (status === 'partial') return `Left open for targeted implementation/runtime verification: ${id}.`;
  return `Left open as missing: ${id}.`;
}

function markdown(report) {
  const rows = report.capabilities.map((c) => `| ${c.capability} | ${c.status} | ${c.codeLocation} | ${c.gap} |`);
  return `# P7-V Capability Completeness Audit

Status: ${report.status}

| Result | Count |
| --- | ---: |
| Implemented | ${report.summary.implemented} |
| Partial | ${report.summary.partial} |
| Missing | ${report.summary.missing} |
| Not applicable | ${report.summary.notApplicable} |

| Capability | Status | Code location | Gap |
| --- | --- | --- | --- |
${rows.join('\n')}
`;
}
