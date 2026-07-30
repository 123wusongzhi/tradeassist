import assert from 'node:assert/strict';
import { validateLoadProfileFingerprintEvidence } from '../../../../scripts/p7-v2-regression-fingerprint.mjs';

const hash = 'a'.repeat(64);
const profile = {
  schemaVersion: 3,
  load: { configuredVUs: 10, stages: [{ name: 'steady', durationMs: 600000, targetVUs: 10 }] },
  loadScript: { relativePath: 'tests/load/p7v2-baseline.js', sha256: 'b'.repeat(64) },
};
const v3 = (overrides = {}) => ({ loadProfileFingerprintVersion: 3, loadProfileFingerprint: hash, canonicalLoadProfile: profile, ...overrides });
const accepted = (baseline, current) => assert.equal(validateLoadProfileFingerprintEvidence(baseline, current).classification, 'accepted');
const rejected = (classification, baseline, current) => assert.equal(validateLoadProfileFingerprintEvidence(baseline, current).classification, classification);

accepted(v3({ runKind: 'baseline' }), v3({ runKind: 'current' }));
accepted(v3({ runId: 'baseline-id' }), v3({ runId: 'current-id' }));
rejected('fingerprint_version_mismatch', { loadProfileFingerprintVersion: 2, loadProfileFingerprint: hash }, v3());
rejected('load_profile_fingerprint_mismatch', v3(), v3({ loadProfileFingerprint: 'c'.repeat(64) }));
rejected('missing_load_profile_fingerprint', v3({ loadProfileFingerprint: '' }), v3());
for (const bad of ['a'.repeat(63), `${'a'.repeat(63)}g`, '']) rejected(bad ? 'invalid_load_profile_fingerprint' : 'missing_load_profile_fingerprint', v3({ loadProfileFingerprint: bad }), v3());
rejected('invalid_canonical_load_profile', v3({ canonicalLoadProfile: {} }), v3());
rejected('invalid_canonical_load_profile', v3({ canonicalLoadProfile: { ...profile, load: { ...profile.load, stages: [] } } }), v3());
for (const stage of [{ durationMs: 0, targetVUs: 1 }, { durationMs: 1, targetVUs: -1 }, { durationMs: 1, targetVUs: 1.5 }, { durationMs: 1, targetVUs: null }]) {
  rejected('invalid_canonical_stage', v3({ canonicalLoadProfile: { ...profile, load: { ...profile.load, stages: [stage] } } }), v3());
}
rejected('unsupported_fingerprint_version', v3({ loadProfileFingerprintVersion: 4 }), v3());
console.log(JSON.stringify({ phase: 'P7-V2-R3B-LPC-R3-GATEFIX', status: 'passed', fixtures: 14 }, null, 2));
