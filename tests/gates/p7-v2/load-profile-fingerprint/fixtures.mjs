import assert from 'node:assert/strict';
import { calculateLoadProfileFingerprint, InvalidLoadProfileError } from '../../../../scripts/p7-v2-load-profile-fingerprint.mjs';

const scriptHash = 'a'.repeat(64);
const base = {
  targetVUs: 10,
  warmup: '5m',
  ramp: '3m',
  steady: '10m',
  rampdown: '2m',
  scenarios: [{ name: 'steady', executor: 'constant-vus', startTime: '0s', gracefulStop: '0s' }],
  loadScript: { path: 'D:\\project\\trademind-ai\\tests\\load\\p7.js', sha256: scriptHash },
};
const fingerprint = (profile) => calculateLoadProfileFingerprint(profile, { repositoryRoot: 'D:\\project\\trademind-ai' }).loadProfileFingerprint;

assert.equal(fingerprint({ ...base, kind: 'baseline', databaseName: 'one', createdAt: 'a' }), fingerprint({ databaseName: 'two', kind: 'current', createdAt: 'b', ...base }));
assert.equal(fingerprint(base), fingerprint({ ...base, warmup: '300s', ramp: '180000ms', steady: 600000, rampdown: '120000' }));
assert.equal(fingerprint(base), fingerprint({ ...base, targetVUs: '10' }));
assert.equal(fingerprint(base), fingerprint({ ...base, loadScript: { path: '/mnt/d/project/trademind-ai/tests/load/p7.js', sha256: scriptHash } }));
assert.equal(fingerprint(base), fingerprint({ ...base, requestMix: [], credentialMix: [] }));
assert.notEqual(fingerprint(base), fingerprint({ ...base, stages: [
  { name: 'ramp', duration: '3m', targetVUs: 10 },
  { name: 'warmup', duration: '5m', targetVUs: 10 },
] }));
assert.notEqual(fingerprint(base), fingerprint({ ...base, steady: '11m' }));
assert.notEqual(fingerprint(base), fingerprint({ ...base, targetVUs: 11 }));
assert.notEqual(fingerprint({ ...base, scenarios: [{ name: 'steady', executor: 'constant-vus', startTime: '0s', gracefulStop: '0s', weight: 1 }] }), fingerprint({ ...base, scenarios: [{ name: 'steady', executor: 'constant-vus', startTime: '0s', gracefulStop: '0s', weight: 2 }] }));
assert.notEqual(fingerprint(base), fingerprint({ ...base, loadScript: { ...base.loadScript, sha256: 'b'.repeat(64) } }));
assert.throws(() => fingerprint({ ...base, targetVUs: undefined }), InvalidLoadProfileError);
const unknown = calculateLoadProfileFingerprint({ ...base, unrecognized: true }, { repositoryRoot: 'D:\\project\\trademind-ai' });
assert.deepEqual(unknown.unknownFields, ['unrecognized']);
const repeated = new Set(Array.from({ length: 10 }, () => fingerprint(base)));
assert.equal(repeated.size, 1);

console.log(JSON.stringify({ phase: 'P7-V2-R3B-LPF-V2', status: 'passed', fixtures: 14, uniqueFingerprintCount: repeated.size }, null, 2));
