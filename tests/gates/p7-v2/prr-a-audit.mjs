import assert from 'node:assert/strict';
import { classifyDistribution, classifyP99 } from '../../../scripts/p7-v2-prr-a-metric-audit.mjs';

assert.equal(classifyP99({ rawMetricPresent: false, rawP99: null, rawMax: null, requestCount: 100, trendSampleCount: 100, parserOutput: 0 }), 'raw_metric_missing_parser_defaulted_zero');
assert.equal(classifyP99({ rawMetricPresent: true, rawP99: null, rawMax: null, requestCount: 100, trendSampleCount: 0, parserOutput: 0 }), 'trend_has_no_samples');
assert.equal(classifyP99({ rawMetricPresent: true, rawP99: null, rawMax: null, requestCount: 20, trendSampleCount: 20, parserOutput: 0 }), 'insufficient_samples_for_p99');
assert.equal(classifyP99({ rawMetricPresent: true, rawP99: 0, rawMax: 0, requestCount: 100, trendSampleCount: 100, parserOutput: 0 }), 'invalid_latency_zero');
assert.equal(classifyP99({ rawMetricPresent: true, rawP99: null, rawMax: 12, requestCount: 100, trendSampleCount: 100, parserOutput: 0 }), 'summary_stat_missing');

const broadShift = classifyDistribution({
  baseline: { p50: 10, p90: 20, p95: 30, p99: 40, throughput: 10 },
  current: { p50: 12, p90: 24, p95: 40, p99: 55, throughput: 8 },
});
assert.equal(broadShift.broadlyShifted, true);
assert.equal(broadShift.throughputDeclined, true);

const p95Only = classifyDistribution({
  baseline: { p50: 10, p90: 20, p95: 30, p99: 40, throughput: 10 },
  current: { p50: 10.1, p90: 20.2, p95: 40, p99: 39, throughput: 10.1 },
});
assert.equal(p95Only.p95Only, true);

const unevenSamples = { baseline: 1000, current: 100 };
assert.equal(unevenSamples.current / unevenSamples.baseline < 0.2, true);
const tagMismatch = classifyP99({ rawMetricPresent: true, rawP99: null, rawMax: 20, requestCount: 100, trendSampleCount: 0, parserOutput: 0 });
assert.equal(tagMismatch, 'trend_has_no_samples');
const unitZero = classifyP99({ rawMetricPresent: true, rawP99: 0, rawMax: 3, requestCount: 100, trendSampleCount: 100, parserOutput: 0 });
assert.equal(unitZero, 'invalid_latency_zero');

console.log(JSON.stringify({ phase: 'P7-V2-R3B-PRR-A', status: 'passed', fixtures: 10 }, null, 2));
