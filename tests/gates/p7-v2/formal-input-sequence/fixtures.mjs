import assert from 'node:assert/strict';
import { buildFormalInputSequenceManifest, compareFormalInputSequences } from '../../../../scripts/p7-v2-formal-input-sequence.mjs';
import { writeJSON } from '../../../../scripts/p7-v2-lib.mjs';

const baseline = buildFormalInputSequenceManifest({
  loadSeed: 'seed-a',
  scenarioSeed: 'scenario-a',
});
const current = buildFormalInputSequenceManifest({
  loadSeed: 'seed-a',
  scenarioSeed: 'scenario-a',
});

assert.equal(baseline.requestSequenceHash, current.requestSequenceHash);
assert.equal(baseline.webhookSequenceHash, current.webhookSequenceHash);
assert.equal(baseline.authSequenceHash, current.authSequenceHash);
assert.equal(compareFormalInputSequences(baseline, current).status, 'passed');

const seedDrift = buildFormalInputSequenceManifest({ loadSeed: 'seed-b', scenarioSeed: 'scenario-a' });
assert.equal(compareFormalInputSequences(baseline, seedDrift).notComparable, true);

const duplicateDrift = buildFormalInputSequenceManifest({
  loadSeed: 'seed-a',
  scenarioSeed: 'scenario-a',
  duplicateConflictTargetCount: 126,
});
assert.equal(compareFormalInputSequences(baseline, duplicateDrift).notComparable, true);

const authDrift = buildFormalInputSequenceManifest({
  loadSeed: 'seed-a',
  scenarioSeed: 'scenario-a',
  wrongPasswordTargetCount: 121,
});
assert.equal(compareFormalInputSequences(baseline, authDrift).notComparable, true);

const branchCountDeviation = {
  ...baseline,
  webhookBranchMix: {
    ...baseline.webhookBranchMix,
    normalInsertActualCount: baseline.webhookBranchMix.normalInsertActualCount - 1,
  },
};
const semantic = compareFormalInputSequences(branchCountDeviation, current);
assert.equal(semantic.semanticGatePassed, false);
assert.equal(semantic.status, 'not_comparable');

const changedList = {
  ...baseline,
  requestSequenceHash: 'f'.repeat(64),
};
assert.equal(compareFormalInputSequences(changedList, current).notComparable, true);

writeJSON('docs/p7-v2-formal-input-sequence-fixture-report.json', {
  phase: 'P7-V2-R3B-FORMAL-INPUT-SEQUENCE-BINDING-V1',
  status: 'passed',
  fixtures: 6,
  sameSeedHashMatch: true,
  seedDriftNotComparable: true,
  webhookDuplicateDriftNotComparable: true,
  authBranchMixDriftNotComparable: true,
  branchCountDeviationSemanticGateFailed: true,
  requestListDriftNotComparable: true,
});

console.log(JSON.stringify({ phase: 'P7-V2-R3B-FORMAL-INPUT-SEQUENCE-BINDING-V1', status: 'passed', fixtures: 6 }, null, 2));
