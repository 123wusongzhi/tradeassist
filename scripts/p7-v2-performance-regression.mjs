import fs from 'node:fs';
import path from 'node:path';
import {
  DEFAULT_SLO,
  REGRESSION_THRESHOLDS,
  docsDir,
  readJSON,
  root,
  writeJSON,
  writeMarkdown,
} from './p7-v2-lib.mjs';

function latestFile(prefix, dir = path.join(docsDir, 'baselines')) {
  if (!fs.existsSync(dir)) return null;
  const files = fs.readdirSync(dir).filter((f) => f.startsWith(prefix)).sort();
  return files.length ? readJSON(path.join('docs/baselines', files[files.length - 1])) : null;
}

const baseline = readJSON('docs/p7-v2-baseline-report.json') || latestFile('p7-v2-baseline-');
const current = readJSON('docs/p7-v2-current-load-report.json');
const comparisons = [];
const issues = [];

function compareScenario(name, baselineValue, currentValue, threshold, metricName) {
  const absoluteDelta = currentValue - baselineValue;
  const percentageDelta = baselineValue > 0 ? (absoluteDelta / baselineValue) * 100 : 0;
  const status =
    baselineValue <= 0 || currentValue <= 0
      ? 'not_comparable'
      : percentageDelta <= threshold
        ? 'passed'
        : 'failed';
  comparisons.push({
    scenario: name,
    metric: metricName,
    baselineValue,
    currentValue,
    absoluteDelta,
    percentageDelta,
    threshold,
    status,
    reason: status === 'not_comparable' ? 'missing metrics' : '',
  });
  return status;
}

if (!baseline || baseline.status !== 'passed') issues.push('baseline missing or not passed');
if (!current || current.status !== 'passed') issues.push('current load missing or not passed');

if (baseline && current) {
  const comparable =
    baseline.datasetFingerprint &&
    current.datasetFingerprint &&
    baseline.datasetFingerprint === current.datasetFingerprint &&
    baseline.loadProfileFingerprint === current.loadProfileFingerprint;
  if (!comparable) issues.push('baseline/current not comparable (fingerprint mismatch)');

  const b = baseline.scenarios?.[0] || {};
  const c = current.scenarios?.[0] || {};
  compareScenario('aggregate', b.p95 || 0, c.p95 || 0, REGRESSION_THRESHOLDS.p95DegradationPct, 'p95');
  compareScenario('aggregate', b.p99 || 0, c.p99 || 0, REGRESSION_THRESHOLDS.p99DegradationPct, 'p99');
  compareScenario('aggregate', b.rps || 0, c.rps || 0, REGRESSION_THRESHOLDS.throughputDegradationPct, 'throughput');
  compareScenario('aggregate', b.errorRate || 0, c.errorRate || 0, REGRESSION_THRESHOLDS.errorRateIncreasePts, 'errorRate');

  const absSloFailed = (c.p95 || 0) > DEFAULT_SLO.readListP95Ms || (c.errorRate || 0) > DEFAULT_SLO.httpReqFailedMax;
  if (absSloFailed) issues.push('absolute SLO failed on current run');
}

const failed = comparisons.filter((c) => c.status === 'failed').length;
const notComparable = comparisons.filter((c) => c.status === 'not_comparable').length;
const report = {
  phase: 'P7-V2',
  status: issues.length === 0 && failed === 0 && notComparable === 0 ? 'passed' : failed > 0 || issues.length ? 'failed' : 'not_comparable',
  absoluteSloPassed: !issues.some((x) => x.includes('absolute SLO')),
  relativeRegressionPassed: failed === 0,
  failed,
  notComparable,
  comparisons,
  thresholds: REGRESSION_THRESHOLDS,
  issues,
};

writeJSON('docs/p7-v2-performance-regression-report.json', report);
writeMarkdown(
  'docs/P7_V2_PERFORMANCE_REGRESSION_REPORT.md',
  `# P7-V2 Performance Regression Report

Status: ${report.status}

| Metric | Baseline | Current | Delta % | Status |
| --- | ---: | ---: | ---: | --- |
${comparisons.map((c) => `| ${c.metric} | ${c.baselineValue} | ${c.currentValue} | ${c.percentageDelta.toFixed(2)} | ${c.status} |`).join('\n')}
`,
);

console.log(JSON.stringify({ phase: 'P7-V2', status: report.status, failed, notComparable }, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
