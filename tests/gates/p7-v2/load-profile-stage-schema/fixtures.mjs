import assert from 'node:assert/strict';
import {
  calculateLoadProfileFingerprint,
  InvalidLoadProfileError,
  parseDurationToMilliseconds,
} from '../../../../scripts/p7-v2-load-profile-fingerprint.mjs';

const scriptHash = 'a'.repeat(64);
const base = {
  configuredVUs: 10,
  stages: [
    { name: 'warmup', duration: '5m', target: 10 },
    { name: 'ramp', duration: '3m', targetVUs: 10 },
    { name: 'steady', duration: '10m', targetVUs: 10 },
    { name: 'rampdown', duration: '2m', target: 0 },
  ],
  scenarios: [{ name: 'steady', executor: 'constant-vus', startTime: '0s', gracefulStop: '0s', weight: 1 }],
  requestMix: [{ routeId: 'task_list', method: 'GET', weight: 1 }],
  credentialMix: [{ role: 'system_admin', weight: 1 }],
  loadScript: { path: 'tests/load/p7v2-baseline.js', sha256: scriptHash },
};
const result = (profile) => calculateLoadProfileFingerprint(profile, { repositoryRoot: process.cwd() });
const stage = (raw) => result({ ...base, stages: [{ name: 'warmup', duration: '5m', ...raw }, ...base.stages.slice(1)] });

assert.equal(result(base).canonicalProfile.load.stages[0].durationMs, 300000);
assert.equal(result(base).canonicalProfile.load.stages[0].targetVUs, 10);
assert.equal(stage({ targetVUs: 10 }).canonicalProfile.load.stages[0].targetVUs, 10);
assert.equal(stage({ target: 10, targetVUs: 10 }).canonicalProfile.load.stages[0].duplicateEquivalentFields, true);
assert.equal(stage({ target: '10' }).canonicalProfile.load.stages[0].targetVUs, 10);
assert.equal(stage({ target: 0 }).canonicalProfile.load.stages[0].targetVUs, 0);
assert.throws(() => stage({ target: 10, targetVUs: 8 }), /conflicting_stage_target_fields/);
assert.throws(() => stage({}), /missing_required_stage_target/);
assert.throws(() => stage({ target: null }), /invalid_stage_target/);
for (const value of [NaN, Infinity, -Infinity, -1, 1.5, '', '10vu', {}, []]) {
  assert.throws(() => stage({ target: value }), /invalid_stage_target/);
}
for (const value of ['5m', '300s', '300000ms', 300000, '300000']) {
  assert.equal(parseDurationToMilliseconds(value), 300000);
}
for (const value of [0, -1, null, undefined, NaN, Infinity, '', '2x']) {
  assert.throws(() => parseDurationToMilliseconds(value), /invalid_duration/);
}
const reordered = { ...base, stages: [base.stages[1], base.stages[0], ...base.stages.slice(2)] };
assert.notEqual(result(base).loadProfileFingerprint, result(reordered).loadProfileFingerprint);
assert.equal(result({ ...base, kind: 'baseline', runId: 'baseline-a' }).loadProfileFingerprint, result({ ...base, kind: 'current', runId: 'current-b' }).loadProfileFingerprint);

console.log(JSON.stringify({ phase: 'P7-V2-R3B-LPC-R3', status: 'passed', fixtures: 15 }, null, 2));
