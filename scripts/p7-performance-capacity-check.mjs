import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const results = [];

function exists(rel) {
  return fs.existsSync(path.join(root, rel));
}

function text(rel) {
  try { return fs.readFileSync(path.join(root, rel), 'utf8'); } catch { return ''; }
}

function check(id, ok, detail) {
  results.push({ id, status: ok ? 'passed' : 'failed', detail });
}

function requireFile(id, rel) {
  check(id, exists(rel), rel);
}

function requireText(id, rel, needles) {
  const body = text(rel);
  const missing = needles.filter((n) => !body.includes(n));
  check(id, missing.length === 0, `${rel}${missing.length ? ` missing ${missing.join(', ')}` : ''}`);
}

function readJSON(rel) {
  try { return JSON.parse(text(rel)); } catch { return null; }
}

requireFile('p7-config', 'backend/internal/config/p7_config.go');
requireText('p7-production-guards', 'backend/internal/config/p7_config.go', [
  'PERFORMANCE_TEST_MODE must be false in production',
  'ALLOW_PERFORMANCE_DATASET must be false in production',
  'PPROF_INTERNAL_ONLY must be true',
  'RATE_LIMIT_ENABLED=false in production requires',
]);
requireText('db-pool-configured', 'backend/internal/database/database.go', [
  'DBMaxOpenConnections',
  'SetConnMaxIdleTime',
]);
requireFile('pagination-package', 'backend/internal/pkg/pagination/pagination.go');
requireText('pagination-guards', 'backend/internal/pkg/pagination/pagination.go', [
  'MaxOffset',
  'cursor tenant scope mismatch',
  'cursor shop scope mismatch',
]);
requireFile('ratelimit-package', 'backend/internal/pkg/ratelimit/limiter.go');
requireText('http-rate-limit-middleware', 'backend/internal/middleware/ratelimit.go', [
  'Retry-After',
  'RATE_LIMITED',
  'rateLimitKey',
]);
requireText('server-ratelimit-wired', 'backend/cmd/server/main.go', ['middleware.RateLimit(cfg)']);
requireFile('p7-migration', 'backend/internal/database/migrate_p7.go');
requireText('p7-models', 'backend/internal/modules/performance/model.go', [
  'performance_test_runs',
  'performance_regressions',
  'capacity_snapshots',
  'rate_limit_policies',
  'quota_policies',
]);
requireFile('p7-load-command', 'backend/cmd/p7load/main.go');
requireText('p7-load-guards', 'backend/cmd/p7load/main.go', [
  'APP_ENV must be performance',
  'DB_NAME must start with trademind_p7_',
  'EXTERNAL_PROVIDER_MODE must be mock',
]);
requireFile('dataset-script', 'scripts/p7-generate-dataset.mjs');

for (const rel of [
  'docs/P7_PERFORMANCE_CAPACITY_AUDIT.md',
  'docs/P7_PERFORMANCE_ARCHITECTURE.md',
  'docs/P7_PERFORMANCE_TARGETS.md',
  'docs/P7_LARGE_DATASET_MATRIX.md',
  'docs/P7_DATABASE_INDEX_AUDIT.md',
  'docs/P7_QUERY_PLAN_REPORT.md',
  'docs/P7_PAGINATION_DESIGN.md',
  'docs/P7_DATABASE_POOL_AND_TRANSACTION.md',
  'docs/P7_WORKER_CAPACITY.md',
  'docs/P7_BACKPRESSURE_DESIGN.md',
  'docs/P7_RATE_LIMIT_DESIGN.md',
  'docs/P7_PROVIDER_LIMITING.md',
  'docs/P7_QUOTA_DESIGN.md',
  'docs/P7_CACHE_CONSISTENCY_POLICY.md',
  'docs/P7_EXPORT_AND_UPLOAD_STREAMING.md',
  'docs/P7_MEMORY_BUDGET.md',
  'docs/P7_PROFILING_SECURITY.md',
  'docs/P7_CAPACITY_MODEL.md',
  'docs/P7_LOAD_TEST_REPORT.md',
  'docs/P7_SOAK_TEST_REPORT.md',
  'docs/P7_RACE_TEST_REPORT.md',
  'docs/P7_DEVELOPMENT_ACCEPTANCE_RUN_1.md',
  'docs/P7_DEVELOPMENT_ACCEPTANCE_RUN_2.md',
]) {
  requireFile(`doc:${path.basename(rel)}`, rel);
}

for (const rel of [
  'deploy/observability/dashboards/performance-overview.json',
  'deploy/observability/dashboards/database-capacity.json',
  'deploy/observability/dashboards/rate-limit-and-backpressure.json',
  'deploy/observability/dashboards/worker-capacity.json',
  'deploy/observability/dashboards/cache-performance.json',
]) {
  requireFile(`dashboard:${path.basename(rel)}`, rel);
}

for (const rel of [
  'docs/runbooks/API_LATENCY_HIGH.md',
  'docs/runbooks/DATABASE_POOL_SATURATED.md',
  'docs/runbooks/SLOW_QUERY_SPIKE.md',
  'docs/runbooks/WEBHOOK_BURST.md',
  'docs/runbooks/TASK_BACKLOG_HIGH.md',
  'docs/runbooks/PROVIDER_RATE_LIMITED.md',
  'docs/runbooks/RATE_LIMIT_REJECTION_SPIKE.md',
  'docs/runbooks/CACHE_STAMPEDE.md',
  'docs/runbooks/MEMORY_GROWTH.md',
  'docs/runbooks/GOROUTINE_LEAK.md',
  'docs/runbooks/EXPORT_BACKLOG.md',
  'docs/runbooks/CAPACITY_THRESHOLD_REACHED.md',
]) {
  requireFile(`runbook:${path.basename(rel)}`, rel);
}

const dataset = readJSON('docs/p7-v-medium-dataset-report.json') || readJSON('docs/p7-dataset-generation-report.json');
check('medium-dataset-validation', dataset?.status === 'dataset_generated' && dataset?.profile === 'medium' && Number(dataset?.insertedRows ?? dataset?.rowsWritten ?? dataset?.actualRows ?? 0) > 0 && Number(dataset?.failedRows || 0) === 0, 'requires real medium dataset generation evidence');
const load = readJSON('docs/p7-v-current-load-report.json') || readJSON('docs/p7-load-test-report.json');
const loadHasP95 = Number(load?.api?.p95 || 0) > 0 || (Array.isArray(load?.scenarios) && load.scenarios.some((s) => Number(s.p95 || 0) > 0));
check('load-test-passed', load?.status === 'passed' && loadHasP95, 'requires real load test p95 evidence');
const soak = readJSON('docs/p7-v-soak-test-report.json') || readJSON('docs/p7-soak-test-report.json');
check('soak-test-passed', soak?.status === 'passed' && (soak?.memoryLeak === false || soak?.unboundedMemoryGrowth === false) && soak?.goroutineLeak === false, 'requires real soak test evidence');
const race = readJSON('docs/p7-v-race-test-report.json') || readJSON('docs/p7-race-test-report.json');
check('linux-race-passed', race?.status === 'passed' && Number(race?.dataRaces || -1) === 0, 'requires Linux/WSL2 race evidence');
const acceptance = readJSON('docs/p7-development-acceptance-report.json');
check('development-acceptance', acceptance?.run1CodeFailed === 0 && acceptance?.run2CodeFailed === 0 && acceptance?.run1NonAiFailed === 0 && acceptance?.run2NonAiFailed === 0, 'requires two demo:auto-acceptance runs with codeFailed/nonAiFailed=0');

const failed = results.filter((r) => r.status !== 'passed').length;
const report = {
  phase: 'P7',
  status: failed === 0 ? 'passed_with_real_production_capacity_verification_deferred' : 'incomplete',
  failed,
  passed: results.length - failed,
  results,
  realProductionPerformanceVerification: 'deferred',
  realProductionCapacityVerification: 'deferred',
  productionReady: false,
};

fs.mkdirSync(docs, { recursive: true });
fs.writeFileSync(path.join(docs, 'p7-performance-capacity-report.json'), `${JSON.stringify(report, null, 2)}\n`);
fs.writeFileSync(path.join(docs, 'P7_PERFORMANCE_CAPACITY_REPORT.md'), `# P7 Performance Capacity Report

${failed === 0 ? 'Phase P7 Static and Evidence Gate Passed' : 'Phase P7 Incomplete'}

| Result | Count |
| --- | ---: |
| Passed | ${report.passed} |
| Failed | ${report.failed} |

Real production performance and capacity verification remain Deferred. This report must not be used to mark Production Ready.
`);

console.log(JSON.stringify({ phase: 'P7', failed, passed: report.passed }, null, 2));
process.exit(failed === 0 ? 0 : 1);
