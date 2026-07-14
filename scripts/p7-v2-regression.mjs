import { readJSON, valueOf, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import { REGRESSION_THRESHOLDS } from './p7-v2-lib.mjs';

const args = process.argv.slice(2);
const baselinePath = valueOf(args, '--baseline') || 'docs/p7-v2-r3-baseline-report.json';
const currentPath = valueOf(args, '--current') || 'docs/p7-v2-current-load-report.json';
const baseline = readJSON(baselinePath);
const current = readJSON(currentPath);
const checks = [];
const comparisons = [];
const issues = [];
const metrics = [
  ['p95', REGRESSION_THRESHOLDS.p95DegradationPct, 'percent'],
  ['p99', REGRESSION_THRESHOLDS.p99DegradationPct, 'percent'],
  ['rps', REGRESSION_THRESHOLDS.throughputDegradationPct, 'inverse-percent'],
  ['errorRate', REGRESSION_THRESHOLDS.errorRateIncreasePts, 'points'],
  ['timeouts', REGRESSION_THRESHOLDS.timeoutIncreasePts, 'points'],
];
if (!baseline || baseline.status !== 'passed' || !current || current.status !== 'passed') issues.push('baseline or current report is not passed');
if (baseline?.runtimeSourceTreeHash !== current?.runtimeSourceTreeHash) issues.push('runtime source tree fingerprint mismatch');
for (const scenario of baseline?.scenarios || []) {
  const next = (current?.scenarios || []).find((item) => item.scenario === scenario.scenario);
  if (!next) {
    issues.push(`current scenario missing: ${scenario.scenario}`);
    continue;
  }
  for (const [metric, threshold, mode] of metrics) {
    const before = Number(scenario[metric] || 0);
    const after = Number(next[metric] || 0);
    const percentageDelta = before > 0 ? ((after - before) / before) * 100 : null;
    const absoluteDelta = after - before;
    const comparable = before > 0 && (mode !== 'inverse-percent' || after > 0);
    let status = 'not_comparable';
    if (comparable) {
      const pass =
        mode === 'points'
          ? absoluteDelta <= threshold
          : mode === 'inverse-percent'
            ? percentageDelta >= -threshold
            : percentageDelta <= threshold;
      status = pass ? 'passed' : 'failed';
    }
    comparisons.push({
      scenario: scenario.scenario,
      metric,
      baselineValue: before,
      currentValue: after,
      absoluteDelta,
      percentageDelta,
      threshold,
      absoluteSloStatus: current.absoluteSloPassed ? 'passed' : 'failed',
      relativeRegressionStatus: status,
      finalStatus: status === 'passed' && current.absoluteSloPassed ? 'passed' : status,
      reason: comparable ? '' : 'missing or zero source metric',
    });
  }
}
const failedMetricCount = comparisons.filter((item) => item.finalStatus === 'failed').length;
const notComparableCount = comparisons.filter((item) => item.finalStatus === 'not_comparable').length;
const report = {
  phase: 'P7-V2-R3',
  status: issues.length || failedMetricCount ? 'failed' : notComparableCount ? 'not_comparable' : 'passed',
  baseline: { path: baselinePath, runId: baseline?.runId || '' },
  current: { path: currentPath, runId: current?.runId || '', independentRun: current?.independentRun === true },
  absoluteSloPassed: current?.absoluteSloPassed === true,
  relativeRegressionPassed: failedMetricCount === 0 && notComparableCount === 0,
  notComparableCount,
  failedMetricCount,
  comparisons,
  checks,
  issues,
};
writeJSON('docs/p7-v2-performance-regression-report.json', report);
writeMarkdown(
  'docs/P7_V2_PERFORMANCE_REGRESSION_REPORT.md',
  `# P7-V2 Performance Regression Report\n\nStatus: **${report.status}**\n\n| Scenario | Metric | Baseline | Current | Delta % | Status |\n| --- | --- | ---: | ---: | ---: | --- |\n${comparisons.map((item) => `| ${item.scenario} | ${item.metric} | ${item.baselineValue} | ${item.currentValue} | ${item.percentageDelta?.toFixed(2) ?? 'n/a'} | ${item.finalStatus} |`).join('\n')}\n\n## Issues\n${issues.length ? issues.map((item) => `- ${item}`).join('\n') : '- none'}\n`,
);
console.log(JSON.stringify({ phase: report.phase, status: report.status, failedMetricCount, notComparableCount }, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
