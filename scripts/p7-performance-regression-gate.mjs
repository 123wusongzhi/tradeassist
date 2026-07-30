import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const baselineDir = path.join(docs, 'performance-baselines');
const reportPath = path.join(docs, 'p7-performance-regression-report.json');
const mdPath = path.join(docs, 'P7_PERFORMANCE_REGRESSION_REPORT.md');

function readJSON(rel) {
  try { return JSON.parse(fs.readFileSync(path.join(root, rel), 'utf8')); } catch { return null; }
}

const checks = [];
function check(id, ok, detail, extra = {}) {
  checks.push({ id, status: ok ? 'passed' : 'failed', detail, ...extra });
}

const baselines = [
  'go-benchmark.json',
  'api-load-baseline.json',
  'database-query-baseline.json',
  'worker-baseline.json',
  'webhook-baseline.json',
];
for (const name of baselines) {
  check(`baseline:${name}`, fs.existsSync(path.join(baselineDir, name)), `docs/performance-baselines/${name}`);
}

const current = readJSON('docs/p7-load-test-report.json');
check('current-load-report', !!current, 'docs/p7-load-test-report.json');

const regressionThresholdPct = 20;
const errorRateThreshold = 0.01;
if (current) {
  for (const [name, item] of Object.entries(current.metrics || {})) {
    const baselineP95 = Number(item.baselineP95Ms || 0);
    const currentP95 = Number(item.currentP95Ms || item.p95Ms || 0);
    const errorRate = Number(item.errorRate || 0);
    const regressionPct = baselineP95 > 0 ? ((currentP95 - baselineP95) / baselineP95) * 100 : 0;
    check(
      `metric:${name}`,
      baselineP95 > 0 && currentP95 > 0 && regressionPct <= regressionThresholdPct && errorRate <= errorRateThreshold,
      `baseline=${baselineP95} current=${currentP95} regression=${regressionPct.toFixed(2)} errorRate=${errorRate}`,
      { baselineP95, currentP95, regressionPct, errorRate },
    );
  }
  if (!current.metrics || Object.keys(current.metrics).length === 0) {
    check('current-metrics-present', false, 'load report must include metrics with baseline/current p95 and error rate');
  }
}

const failed = checks.filter((c) => c.status !== 'passed').length;
const report = {
  phase: 'P7',
  status: failed === 0 ? 'passed' : 'incomplete',
  failed,
  passed: checks.length - failed,
  regressionThresholdPct,
  errorRateThreshold,
  checks,
};

fs.mkdirSync(docs, { recursive: true });
fs.writeFileSync(reportPath, `${JSON.stringify(report, null, 2)}\n`);
fs.writeFileSync(mdPath, `# P7 Performance Regression Report

Status: ${report.status}

| Result | Count |
| --- | ---: |
| Passed | ${report.passed} |
| Failed | ${report.failed} |

The gate compares real current load metrics with stored baselines. It fails when baselines or measured p95/error-rate evidence are missing.
`);

console.log(JSON.stringify({ phase: 'P7', failed, passed: report.passed }, null, 2));
process.exit(failed === 0 ? 0 : 1);
