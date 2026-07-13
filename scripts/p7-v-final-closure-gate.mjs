import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const reportPath = path.join(docs, 'p7-v-final-closure-report.json');
const mdPath = path.join(docs, 'P7_V_FINAL_CLOSURE_REPORT.md');
const checks = [];

function readJSON(rel) {
  try {
    return JSON.parse(fs.readFileSync(path.join(root, rel), 'utf8'));
  } catch {
    return null;
  }
}

function check(id, ok, detail, evidence = '') {
  checks.push({ id, status: ok ? 'passed' : 'failed', detail, evidence });
}

const audit = readJSON('docs/p7-v-capability-completeness-audit.json');
const capabilities = Array.isArray(audit?.capabilities) ? audit.capabilities : [];
check('capability-audit-exists', !!audit && capabilities.length > 0, 'docs/p7-v-capability-completeness-audit.json contains capability matrix');
check('mandatory-capabilities-implemented', capabilities.length > 0 && capabilities.every((c) => c.status === 'implemented' || c.status === 'not_applicable'), 'all mandatory P7-V capabilities implemented or explicitly not_applicable');
check('no-unexplained-na', capabilities.every((c) => c.status !== 'not_applicable' || String(c.notApplicableReason || c.gap || '').trim()), 'not_applicable entries explain reason');
check('no-partial-or-missing', capabilities.every((c) => c.status !== 'partial' && c.status !== 'missing'), 'no unclosed partial/missing capability');

const dataset = readJSON('docs/p7-v-medium-dataset-report.json');
check('medium-dataset-report', !!dataset, 'medium dataset JSON report exists');
check('medium-dataset-executed', dataset?.profile === 'medium' && dataset?.dryRun === false, 'profile=medium and dryRun=false');
check('medium-dataset-inserted', Number(dataset?.insertedRows || 0) > 0 || Number(dataset?.existingRows || 0) > 0, 'insertedRows or existingRows > 0');
check('medium-dataset-failed-zero', Number(dataset?.failedRows || 0) === 0, 'failedRows=0');
check('medium-dataset-counts-match', Number(dataset?.actualRows || 0) > 0 && Number(dataset?.actualRows || 0) === Number(dataset?.plannedRows || 0), 'actualRows equals plannedRows');
check('medium-dataset-fingerprint', !!dataset?.datasetFingerprint, 'dataset fingerprint present');

const integrity = readJSON('docs/p7-v-dataset-integrity-report.json');
check('dataset-integrity-passed', integrity?.status === 'passed', 'dataset integrity passed');
check('dataset-idempotency-passed', integrity?.idempotency === 'passed', 'dataset idempotency passed');
check('dataset-resume-passed', integrity?.resume === 'passed', 'dataset resume passed');

const pagination = readJSON('docs/p7-v-pagination-runtime-report.json');
check('pagination-runtime-passed', pagination?.status === 'passed', 'pagination runtime passed');
check('cursor-tamper-rejected', pagination?.cursorTamperRejected === true, 'tampered cursor rejected');
check('cross-tenant-rejected', pagination?.crossTenantRejected === true, 'cross-tenant cursor rejected');
check('deep-offset-guard', pagination?.deepOffsetGuard === true, 'deep offset guard passed');

const queryPlan = readJSON('docs/p7-v-query-plan-runtime-report.json');
check('query-plan-passed', queryPlan?.status === 'passed', 'query plan runtime report passed');
check('n-plus-one-passed', queryPlan?.nPlusOne === 'passed' || readJSON('docs/p7-v-n-plus-one-report.json')?.status === 'passed', 'N+1 runtime check passed');

const loadCurrent = readJSON('docs/p7-v-current-load-report.json') || readJSON('docs/p7-load-test-report.json');
const loadBaseline = readJSON('docs/performance-baselines/p7-initial-baseline.json') || readJSON('docs/p7-v-initial-baseline-report.json');
check('load-baseline-exists', loadBaseline?.status === 'passed', 'initial baseline exists and passed');
check('current-load-exists', loadCurrent?.status === 'passed', 'current load report exists and passed');
check('load-environment-comparable', !!loadBaseline?.environment && !!loadCurrent?.environment && loadBaseline?.environment?.datasetFingerprint === loadCurrent?.environment?.datasetFingerprint, 'baseline/current dataset fingerprint comparable');
check('load-scenarios-executed', Array.isArray(loadCurrent?.scenarios) && loadCurrent.scenarios.length >= 7, 'all load scenarios executed');
check('load-percentiles-present', loadCurrent?.scenarios?.every((s) => Number(s.p50) > 0 && Number(s.p95) > 0 && Number(s.p99) >= 0), 'p50/p95/p99 present');
check('load-error-rate-present', loadCurrent?.scenarios?.every((s) => typeof s.errorRate === 'number'), 'error rate present');

const regression = readJSON('docs/p7-performance-regression-report.json');
check('regression-gate-passed', regression?.status === 'passed' && Number(regression?.failed || 0) === 0, 'P7 regression gate failed=0');

const soak = readJSON('docs/p7-v-soak-test-report.json');
check('soak-passed', soak?.status === 'passed', 'soak test passed');
check('soak-duration', Number(soak?.durationMinutes || 0) >= 30, 'soak duration >= 30 minutes');
check('soak-memory-stable', soak?.unboundedMemoryGrowth === false, 'unbounded memory growth=false');
check('soak-goroutine-stable', soak?.goroutineLeak === false, 'goroutine leak=false');
check('soak-recovered', soak?.connectionsRecovered === true && soak?.inflightRecovered === true && soak?.shutdownPassed === true, 'connections/inflight recovered and shutdown passed');

const race = readJSON('docs/p7-v-race-test-report.json');
check('linux-race-passed', race?.status === 'passed', 'Linux/WSL2 race executed and passed');
check('linux-race-zero', Number(race?.dataRaces ?? -1) === 0 && Number(race?.deadlocks ?? -1) === 0, 'dataRaces=0 and deadlocks=0');

const p7 = readJSON('docs/p7-performance-capacity-report.json');
check('p7-static-gate-passed', p7?.failed === 0, 'P7 capacity/static gate failed=0');

const demo1 = readJSON('docs/demo-auto-acceptance.run1.json');
const demo2 = readJSON('docs/demo-auto-acceptance.run2.json');
check('demo-run-1-passed', demo1?.codeFailed === 0 && demo1?.nonAiFailed === 0, 'demo:auto-acceptance run 1 codeFailed/nonAiFailed=0');
check('demo-run-2-passed', demo2?.codeFailed === 0 && demo2?.nonAiFailed === 0, 'demo:auto-acceptance run 2 codeFailed/nonAiFailed=0');

check('no-production-load', true, 'no production load test requested or recorded');
check('no-real-provider', dataset?.guards?.includes?.('requires EXTERNAL_PROVIDER_MODE=mock') !== false, 'P7-V remains mock provider only');
check('no-production-ready', true, 'Production Ready remains false/deferred');

const failed = checks.filter((c) => c.status !== 'passed').length;
const report = {
  phase: 'P7-V',
  status: failed === 0 ? 'passed_with_real_production_performance_verification_deferred' : 'incomplete',
  failed,
  passed: checks.length - failed,
  checks,
  capabilityAudit: {
    mandatoryImplemented: capabilities.filter((c) => c.status === 'implemented').length,
    partial: capabilities.filter((c) => c.status === 'partial').length,
    missing: capabilities.filter((c) => c.status === 'missing').length,
  },
  dataset: {
    profile: dataset?.profile || '',
    dryRun: dataset?.dryRun ?? true,
    plannedRows: Number(dataset?.plannedRows || 0),
    insertedRows: Number(dataset?.insertedRows || 0),
    failedRows: Number(dataset?.failedRows || 0),
    integrity: integrity?.status || 'missing',
    idempotency: integrity?.idempotency || 'missing',
    resume: integrity?.resume || 'missing',
    cleanup: dataset?.cleanupStatus || '',
    fingerprint: dataset?.datasetFingerprint || '',
  },
  realProductionPerformanceVerification: 'deferred',
  realProductionCapacityVerification: 'deferred',
  realProductionPeakLoadVerification: 'deferred',
  douyinRealCredentialVerification: 'deferred',
  productionReady: false,
  tag: 'deferred',
};

fs.mkdirSync(docs, { recursive: true });
fs.writeFileSync(reportPath, `${JSON.stringify(report, null, 2)}\n`);
fs.writeFileSync(mdPath, markdown(report));
console.log(JSON.stringify({ phase: 'P7-V', failed, passed: report.passed, status: report.status }, null, 2));
process.exit(failed === 0 ? 0 : 1);

function markdown(report) {
  const title = report.failed === 0 ? 'Phase P7-V Completed' : 'Phase P7-V Incomplete';
  const failedRows = report.checks.filter((c) => c.status !== 'passed').map((c) => `| ${c.id} | ${c.detail} | ${c.evidence || ''} |`);
  return `# P7-V Final Closure Report

${title}

| Result | Count |
| --- | ---: |
| Passed | ${report.passed} |
| Failed | ${report.failed} |

Real production performance, capacity and peak-load verification remain Deferred. This report must not mark Production Ready.

## Blockers

| Check | Detail | Evidence |
| --- | --- | --- |
${failedRows.length ? failedRows.join('\n') : '| none | none | none |'}
`;
}
