import assert from 'node:assert/strict';
import fs from 'node:fs';
import { classifyMetricEvidence, evaluateAbsoluteSlo, evaluateTargetReached } from '../../../../scripts/p7-v2-soak-semantics.mjs';
import { SCENARIO_METRICS } from '../../../../scripts/p7-v2-regression-metrics.mjs';

const baselineSource = fs.readFileSync('tests/load/p7v2-baseline.js', 'utf8');
const soakSource = fs.readFileSync('tests/load/p7v2-soak.js', 'utf8');
const formalSource = fs.readFileSync('tests/load/lib/formal-metrics.js', 'utf8');
const wrapperSource = fs.readFileSync('scripts/p7-v2-soak.mjs', 'utf8');

for (const [scenario, [durationMetric, requestMetric]] of Object.entries(SCENARIO_METRICS)) {
  const scenarioId = durationMetric.replace(/^p7_/, '').replace(/_steady_duration$/, '');
  assert.match(formalSource, new RegExp(`displayName: '${scenario.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}'`));
  assert.match(formalSource, new RegExp(`scenarioId: '${scenarioId.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}'`));
  assert.equal(requestMetric, `p7_${scenarioId}_steady_requests`);
}
assert.match(formalSource, /steadyDurationMetricName:\s*`p7_\$\{definition\.scenarioId\}_steady_duration`/);
assert.match(formalSource, /steadyRequestMetricName:\s*`p7_\$\{definition\.scenarioId\}_steady_requests`/);

assert.match(baselineSource, /from '\.\/lib\/formal-metrics\.js'/);
assert.match(soakSource, /from '\.\/lib\/formal-metrics\.js'/);
assert.doesNotMatch(baselineSource, /new\s+(Trend|Counter)\(\s*['"]p7_/);
assert.doesNotMatch(soakSource, /new\s+(Trend|Counter)\(\s*['"]p7_/);

const trend = (p95 = 10) => ({ values: { avg: 5, med: 5, 'p(95)': p95, 'p(99)': p95 + 1, max: p95 + 2 } });
const counter = (count) => ({ values: { count, rate: count / 60 } });

assert.equal(classifyMetricEvidence({ rawMetric: undefined, sampleMetric: counter(200) }).classification, 'metric_missing');
assert.equal(classifyMetricEvidence({ rawMetric: trend(), sampleMetric: undefined }).classification, 'sample_counter_missing');
assert.equal(classifyMetricEvidence({ rawMetric: trend(), sampleMetric: counter(0) }).classification, 'insufficient_samples');
assert.equal(classifyMetricEvidence({ rawMetric: trend(), sampleMetric: counter(99) }).classification, 'insufficient_samples');
assert.equal(classifyMetricEvidence({ rawMetric: trend(), sampleMetric: counter(100) }).classification, 'present');
assert.equal(classifyMetricEvidence({ rawMetric: { values: { avg: 5 } }, sampleMetric: counter(100), aggregation: 'p(99)' }).classification, 'summary_stat_missing');

const missingSlo = evaluateAbsoluteSlo({ rawMetric: undefined, sampleMetric: counter(200), threshold: 800 });
assert.equal(missingSlo.evaluationStatus, 'not_evaluable_metric_missing');
assert.equal(missingSlo.verdict, 'not_evaluable');
assert.equal(missingSlo.realAbsoluteSloFailure, false);

const zeroSampleSlo = evaluateAbsoluteSlo({ rawMetric: trend(), sampleMetric: counter(0), threshold: 800 });
assert.equal(zeroSampleSlo.evaluationStatus, 'not_evaluable_insufficient_samples');
assert.equal(zeroSampleSlo.realAbsoluteSloFailure, false);

const realFailure = evaluateAbsoluteSlo({ rawMetric: trend(901), sampleMetric: counter(200), threshold: 800 });
assert.equal(realFailure.evaluationStatus, 'evaluated');
assert.equal(realFailure.verdict, 'failed');
assert.equal(realFailure.realAbsoluteSloFailure, true);

assert.equal(evaluateTargetReached({ loadTargetReached: true }).targetReached, false);
assert.equal(evaluateTargetReached({
  loadTargetReached: true,
  steadyStageEntered: true,
  steadyStageCompleted: true,
  steadyDurationReached: true,
  scenarioCoverageReached: true,
  sampleTargetReached: true,
  sloEvaluationCompleted: true,
}).targetReached, true);

assert.doesNotMatch(wrapperSource, /goroutineStableOrRecovered:\s*cooldown\.goroutines\.recovered/);
assert.doesNotMatch(wrapperSource, /providerStateRecovered:\s*cooldown\.mockProviderState\.recovered/);
assert.doesNotMatch(wrapperSource, /circuitRecovered:\s*cooldown\.circuitState\.recovered/);
assert.doesNotMatch(wrapperSource, /process\.exit\(/);
assert.match(wrapperSource, /wrapperExitedAutomatically:\s*true/);
assert.match(wrapperSource, /manualStopRequired:\s*false/);

console.log(JSON.stringify({ phase: 'P7-V2-R3B-SOAK-SEMANTICS', status: 'passed', fixtures: 28 }, null, 2));
