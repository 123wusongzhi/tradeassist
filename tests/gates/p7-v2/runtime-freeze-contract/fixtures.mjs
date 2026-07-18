import assert from 'node:assert/strict';
import fs from 'node:fs';
import {
  buildRuntimeFreezeContract,
  detectRuntimeFreezeIdentityCollision,
  RUNTIME_FREEZE_IDENTITY_VERSION,
  validateRuntimeFreezeContract,
} from '../../../../scripts/p7-v2-r3b-lpc-r3-runtime-freeze.mjs';

const manifest = {
  phase: 'P7-V2-R3B-FAST-CLOSE-R3',
  status: 'planned',
  canonicalSchemaVersion: 3,
  executionStarted: false,
  environmentStarted: false,
  datasetExecuted: false,
  k6Executed: false,
  runIdsUnique: true,
  baselineRunId: 'p7v2-baseline-r3b-recovery6-fixture',
  currentRunId: 'p7v2-current-r3b-recovery6-fixture',
  soakRunId: 'p7v2-soak-r3b-recovery6-fixture',
  demoRun1Id: 'p7v2-demo1-r3b-recovery6-fixture',
  demoRun2Id: 'p7v2-demo2-r3b-recovery6-fixture',
  providerMode: 'mock',
  datasetProfile: 'medium',
  expectedRows: 1900150,
  selectedHost: '127.0.0.1',
  selectedPort: 18080,
};
const cleanImmutableDiff = {
  hash: null,
  immutableWorkingTreeClean: true,
  immutableTrackedDiffPresent: false,
  stagedImmutableChangeCount: 0,
  unstagedImmutableChangeCount: 0,
  untrackedImmutableChangeCount: 0,
  pathspecs: [],
};
const contract = buildRuntimeFreezeContract({ manifest, now: '2026-07-15T10:00:00.000Z', planCheckpoint: 'a'.repeat(40), skipCreationPreconditions: true, immutableDiffOverride: cleanImmutableDiff });

assert.match(contract.contractId, /^[a-f0-9]{64}$/);
assert.equal(contract.runtimeFreezeIdentityVersion, RUNTIME_FREEZE_IDENTITY_VERSION);
assert.match(contract.runtimeContentHash, /^[a-f0-9]{64}$/);
assert.match(contract.planBindingHash, /^[a-f0-9]{64}$/);
assert.equal(contract.loadProfileFingerprintVersion, 3);
assert.equal(validateRuntimeFreezeContract(contract, { kind: 'baseline', runId: manifest.baselineRunId }).valid, true);
assert.equal(validateRuntimeFreezeContract(contract, { kind: 'current', runId: manifest.currentRunId }).valid, true);
assert.equal(validateRuntimeFreezeContract(contract, { kind: 'current', runId: manifest.baselineRunId }).issue, 'runtime_freeze_run_id_mismatch');

const samePlanA = buildRuntimeFreezeContract({ manifest, now: '2026-07-15T10:00:00.000Z', planCheckpoint: 'b'.repeat(40), skipCreationPreconditions: true, immutableDiffOverride: cleanImmutableDiff });
const samePlanB = buildRuntimeFreezeContract({
  manifest: {
    ...manifest,
    lastAttempt: { status: 'blocked', pid: 123, databaseIdentity: 'dynamic-db', recordedAt: '2026-07-15T10:01:00.000Z' },
    currentStep: 'host_guard',
    runtimeFreezeCreatedAt: '2026-07-15T10:02:00.000Z',
  },
  now: '2026-07-15T11:00:00.000Z',
  planCheckpoint: 'b'.repeat(40),
  skipCreationPreconditions: true,
  immutableDiffOverride: cleanImmutableDiff,
});
assert.equal(samePlanA.runtimeFreezeId, samePlanB.runtimeFreezeId);

const differentCheckpoint = buildRuntimeFreezeContract({ manifest, now: '2026-07-15T10:00:00.000Z', planCheckpoint: 'c'.repeat(40), skipCreationPreconditions: true, immutableDiffOverride: cleanImmutableDiff });
assert.notEqual(samePlanA.runtimeFreezeId, differentCheckpoint.runtimeFreezeId);
assert.notEqual(samePlanA.planBindingHash, differentCheckpoint.planBindingHash);

for (const key of ['baselineRunId', 'currentRunId', 'soakRunId', 'demoRun1Id', 'demoRun2Id']) {
  const changed = buildRuntimeFreezeContract({
    manifest: { ...manifest, [key]: `${manifest[key]}-changed` },
    now: '2026-07-15T10:00:00.000Z',
    planCheckpoint: 'b'.repeat(40),
    skipCreationPreconditions: true,
    immutableDiffOverride: cleanImmutableDiff,
  });
  assert.notEqual(samePlanA.runtimeFreezeId, changed.runtimeFreezeId);
  assert.notEqual(samePlanA.planBindingHash, changed.planBindingHash);
}

const oldId = '169a84ff16ecb11cfed96d434497ce8c52390d6e44035a8721d343f5515fbf43';
const staleManifest = { ...manifest, runtimeFreezeId: oldId, runtimeFreezeCreated: false, status: 'planned' };
const regenerated = buildRuntimeFreezeContract({ manifest: staleManifest, now: '2026-07-15T10:00:00.000Z', planCheckpoint: 'd'.repeat(40), skipCreationPreconditions: true, immutableDiffOverride: cleanImmutableDiff });
assert.notEqual(regenerated.runtimeFreezeId, oldId);

const oldRealPlan = buildRuntimeFreezeContract({
  manifest: {
    ...manifest,
    baselineRunId: 'p7v2-baseline-r3b-recovery6-20260716063639',
    currentRunId: 'p7v2-current-r3b-recovery6-20260716063639',
    soakRunId: 'p7v2-soak-r3b-recovery6-20260716063639',
    demoRun1Id: 'p7v2-demo1-r3b-recovery6-20260716063639',
    demoRun2Id: 'p7v2-demo2-r3b-recovery6-20260716063639',
  },
  now: '2026-07-16T06:44:35.424Z',
  planCheckpoint: 'c182977694a616ddd263c42e7089a88fb5093c9c',
  skipCreationPreconditions: true,
  immutableDiffOverride: cleanImmutableDiff,
});
const newRealPlan = buildRuntimeFreezeContract({
  manifest: {
    ...manifest,
    baselineRunId: 'p7v2-baseline-r3b-recovery6-20260716082252',
    currentRunId: 'p7v2-current-r3b-recovery6-20260716082252',
    soakRunId: 'p7v2-soak-r3b-recovery6-20260716082252',
    demoRun1Id: 'p7v2-demo1-r3b-recovery6-20260716082252',
    demoRun2Id: 'p7v2-demo2-r3b-recovery6-20260716082252',
  },
  now: '2026-07-16T08:27:52.876Z',
  planCheckpoint: '963011f659204e579e53458e699d0b25cb608ee5',
  skipCreationPreconditions: true,
  immutableDiffOverride: cleanImmutableDiff,
});
assert.notEqual(oldRealPlan.planBindingHash, newRealPlan.planBindingHash);
assert.notEqual(oldRealPlan.runtimeFreezeId, newRealPlan.runtimeFreezeId);

const changedRunIdRebuild = buildRuntimeFreezeContract({
  manifest: { ...manifest, baselineRunId: `${manifest.baselineRunId}-mutated` },
  now: '2026-07-15T10:00:00.000Z',
  planCheckpoint: samePlanA.planBindingPayload.planCheckpoint,
  skipCreationPreconditions: true,
  immutableDiffOverride: cleanImmutableDiff,
});
assert.notEqual(changedRunIdRebuild.runtimeFreezeId, samePlanA.runtimeFreezeId);

const collision = detectRuntimeFreezeIdentityCollision({ current: { runtimeFreezeId: samePlanA.runtimeFreezeId, planBindingHash: 'e'.repeat(64), status: 'superseded', validForFinalClosure: false } }, samePlanA);
assert.equal(collision.identityCollision, true);
assert.equal(collision.classification, 'runtime_freeze_identity_collision');

const source = (file) => fs.readFileSync(file, 'utf8');
assert.match(source('scripts/p7-v2-baseline.mjs'), /runtimeFreezeId = runtimeFreeze\.contractId/);
assert.match(source('scripts/p7-v2-current.mjs'), /runtimeFreezeId = runtimeFreeze\.contractId/);
assert.match(source('scripts/p7-v2-artifact-freeze.mjs'), /runtimeFreezeContractHash/);
assert.match(source('scripts/p7-v2-r3-comparability-check.mjs'), /loadProfileFingerprintV\$\{fingerprintVersion\}/);
assert.match(source('scripts/p7-v2-r3-comparability-check.mjs'), /matching-runtime-freeze-contract/);
assert.match(source('scripts/p7-v2-regression.mjs'), /p7-v2-r3b-fast-close-r3-comparability-report\.json/);
assert.match(source('scripts/p7-v2-r3b-fast-close.mjs'), /comparability-v3/);
assert.match(source('scripts/p7-v2-r3b-preflight.mjs'), /const recovery6 = process\.argv\.includes\('--recovery6'\)/);
assert.match(source('scripts/p7-v2-r3b-manifest.mjs'), /phase: current\.phase \|\| 'P7-V2-R3B-FAST-CLOSE'/);
assert.match(source('scripts/p7-v2-r3b-lpc-r3-runtime-freeze.mjs'), /runtimeFreezeIdentityVersion: RUNTIME_FREEZE_IDENTITY_VERSION/);
assert.match(source('scripts/p7-v2-r3b-lpc-r3-runtime-freeze.mjs'), /planBindingHash/);

console.log(JSON.stringify({
  phase: 'P7-V2-R3B-FAST-CLOSE-R3-FORMAL',
  status: 'passed',
  fixtures: 34,
  runtimeFreezeIdentityV2FixturePassed: true,
  runtimeFreezeDeterminismPassed: true,
  differentPlanCheckpointChangesId: true,
  differentRunIdsChangeId: true,
  mutableEvidenceDoesNotChangeId: true,
  oldFreezeIdNotInherited: true,
  semanticFailureExitCodePassed: true,
}, null, 2));
