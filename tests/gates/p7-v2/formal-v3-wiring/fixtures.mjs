import assert from 'node:assert/strict';
import fs from 'node:fs';
import {
  buildRuntimeFreezeContract,
  FORMAL_PHASE,
  RUNTIME_FREEZE_PATH,
  validateRuntimeFreezeContract,
} from '../../../../scripts/p7-v2-r3b-lpc-r3-runtime-freeze.mjs';
import { resolveFormalPairEvidence } from '../../../../scripts/p7-v2-evidence-resolver.mjs';

const manifest = {
  phase: 'P7-V2-R3B-FAST-CLOSE-R3',
  status: 'planned',
  executionStarted: false,
  runIdsUnique: true,
  baselineRunId: 'p7v2-baseline-r3b-recovery6-fixture',
  currentRunId: 'p7v2-current-r3b-recovery6-fixture',
  soakRunId: 'p7v2-soak-r3b-recovery6-fixture',
  demoRun1Id: 'p7v2-demo1-r3b-recovery6-fixture',
  demoRun2Id: 'p7v2-demo2-r3b-recovery6-fixture',
};
const contract = buildRuntimeFreezeContract({ manifest, now: '2026-07-15T10:00:00.000Z' });

assert.equal(FORMAL_PHASE, 'P7-V2-R3B-FAST-CLOSE-R3-FORMAL');
assert.equal(RUNTIME_FREEZE_PATH, 'docs/p7-v2-r3b-fast-close-r3-runtime-freeze.json');
assert.equal(contract.phase, FORMAL_PHASE);
assert.equal(contract.canonicalSchemaVersion, 3);
assert.equal(contract.loadProfileFingerprintVersion, 3);
assert.match(contract.runtimeFreezeId, /^[a-f0-9]{64}$/);
assert.equal(validateRuntimeFreezeContract(contract, { kind: 'baseline', runId: manifest.baselineRunId }).valid, true);

const source = (file) => fs.readFileSync(file, 'utf8');
assert.match(source('scripts/p7-v2-evidence-resolver.mjs'), /resolveFormalPairEvidence/);
assert.match(source('scripts/p7-v2-regression.mjs'), /resolveFormalPairEvidence\(\{ requireFrozen: true, requireComparability: true \}\)/);
assert.match(source('scripts/p7-v2-regression.mjs'), /p7-v2-r3b-fast-close-r3-comparability-report\.json/);
assert.match(source('scripts/p7-v2-r3-comparability-check.mjs'), /baselineArtifactSha256/);
assert.match(source('scripts/p7-v2-r3-comparability-check.mjs'), /runtimeFreezeId/);

const unresolved = resolveFormalPairEvidence({
  baselineRunId: 'p7v2-baseline-r3b-recovery6-missing',
  currentRunId: 'p7v2-current-r3b-recovery6-missing',
  requireFrozen: true,
  requireComparability: false,
});
assert.equal(unresolved.regressionAllowed, false);
assert.equal(unresolved.classification, 'formal_pair_evidence_missing');

console.log(JSON.stringify({ phase: FORMAL_PHASE, status: 'passed', fixtures: 12 }, null, 2));
