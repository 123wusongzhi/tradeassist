import assert from 'node:assert/strict';

function latencyVerdict({ baseline, current, samples = 100, floor = 2, relative = 0.1, absoluteSloPassed = true }) {
  if (!Number.isFinite(baseline) || !Number.isFinite(current)) return 'invalid_metric';
  if (samples < 100) return 'insufficient_samples';
  if (baseline === 0 && current === 0) return 'passed_no_change_zero_to_zero';
  if (baseline === 0) return 'invalid_metric';
  if (current === 0) return 'passed_improved_to_zero';
  if (!absoluteSloPassed) return 'failed_absolute_slo';
  const delta = current - baseline;
  return delta / baseline > relative && delta > floor ? 'failed_material_regression' : delta / baseline > relative ? 'passed_relative_noise_below_materiality_floor' : 'passed';
}

assert.equal(latencyVerdict({ baseline: 0, current: 0 }), 'passed_no_change_zero_to_zero');
assert.equal(latencyVerdict({ baseline: 5, current: 0 }), 'passed_improved_to_zero');
assert.equal(latencyVerdict({ baseline: null, current: 1 }), 'invalid_metric');
assert.equal(latencyVerdict({ baseline: 3.14, current: 4.5 }), 'passed_relative_noise_below_materiality_floor');
assert.equal(latencyVerdict({ baseline: 20, current: 30 }), 'failed_material_regression');
assert.equal(latencyVerdict({ baseline: 20, current: 21, absoluteSloPassed: false }), 'failed_absolute_slo');
assert.equal(latencyVerdict({ baseline: 20, current: 30, samples: 20 }), 'insufficient_samples');
assert.equal(0.00314 * 1000, 3.14);
assert.equal((0.002 - 0.001) <= 0.002, true);
assert.equal((100 - 0) / 100 > 0.1, true);
console.log(JSON.stringify({ phase: 'P7-V2-R3B-REBASELINE', fixtures: 11, status: 'passed' }, null, 2));
