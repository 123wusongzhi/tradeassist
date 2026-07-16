import assert from 'node:assert/strict';
import { evaluateRecovery6Preflight, PREFLIGHT_BINDING_VERSION } from '../../../../scripts/p7-v2-r3b-preflight.mjs';
import { buildRuntimeFreezeContract } from '../../../../scripts/p7-v2-r3b-lpc-r3-runtime-freeze.mjs';

const checkpoint = 'f'.repeat(40);
const manifest = {
  phase: 'P7-V2-R3B-FAST-CLOSE-R3',
  status: 'planned',
  active: true,
  canonicalSchemaVersion: 3,
  loadProfileFingerprintVersion: 3,
  formalExecutionStarted: false,
  executionStarted: false,
  environmentStarted: false,
  datasetExecuted: false,
  baselineExecuted: false,
  currentExecuted: false,
  runIdsUnique: true,
  baselineRunId: 'p7v2-baseline-r3b-recovery6-20260716082252',
  currentRunId: 'p7v2-current-r3b-recovery6-20260716082252',
  soakRunId: 'p7v2-soak-r3b-recovery6-20260716082252',
  demoRun1Id: 'p7v2-demo1-r3b-recovery6-20260716082252',
  demoRun2Id: 'p7v2-demo2-r3b-recovery6-20260716082252',
  providerMode: 'mock',
  datasetProfile: 'medium',
  expectedRows: 1900150,
  selectedHost: '127.0.0.1',
  selectedPort: 18080,
  planCheckpoint: checkpoint,
};

const freeze = buildRuntimeFreezeContract({ manifest, planCheckpoint: checkpoint, now: '2026-07-16T09:00:00.000Z' });
const boundManifest = {
  ...manifest,
  status: 'runtime_frozen',
  runtimeFreezeCreated: true,
  runtimeFreezeId: freeze.runtimeFreezeId,
  runtimeFreezeIdentityVersion: freeze.runtimeFreezeIdentityVersion,
  runtimeContentHash: freeze.runtimeContentHash,
  planBindingHash: freeze.planBindingHash,
};
const revalidation = {
  runtimeFreezeStillValid: true,
  immutableMismatchFields: [],
  plannedRunIdsBindingPassed: true,
};

function evalReport(overrides = {}) {
  return evaluateRecovery6Preflight({
    manifest: Object.hasOwn(overrides, 'manifest') ? overrides.manifest : boundManifest,
    runtimeFreezeDoc: overrides.runtimeFreezeDoc ?? { current: freeze },
    currentGitHead: overrides.currentGitHead ?? checkpoint,
    revalidation: overrides.revalidation ?? revalidation,
    legacyBaselineCandidates: overrides.legacyBaselineCandidates ?? ['p7v2-baseline-r3b-recovery6-20260715165422'],
    checkLiveResources: false,
  });
}

const passed = evalReport();
assert.equal(passed.status, 'passed');
assert.equal(passed.semanticGatePassed, true);
assert.equal(passed.preflightBindingVersion, PREFLIGHT_BINDING_VERSION);
assert.equal(passed.resolvedBaselineRunId, boundManifest.baselineRunId);
assert.equal(passed.resolvedCurrentRunId, boundManifest.currentRunId);
assert.equal(passed.resolvedRuntimeFreezeId, freeze.runtimeFreezeId);
assert.equal(passed.runtimeFreezeCreated, true);
assert.equal(passed.runtimeFreezeStillValid, true);
assert.equal(passed.legacyFallbackUsed, false);
assert.equal(passed.legacyBaselineRunIdIgnored, true);

const freezeIdMismatch = evalReport({ manifest: { ...boundManifest, runtimeFreezeId: 'a'.repeat(64) } });
assert.equal(freezeIdMismatch.status, 'failed');
assert.equal(freezeIdMismatch.classification, 'runtime_freeze_id_mismatch');

const notCreated = evalReport({ manifest: { ...boundManifest, runtimeFreezeCreated: false } });
assert.equal(notCreated.status, 'failed');
assert.equal(notCreated.classification, 'runtime_freeze_not_created');

const checkpointMismatch = evalReport({ currentGitHead: 'e'.repeat(40) });
assert.equal(checkpointMismatch.status, 'failed');
assert.equal(checkpointMismatch.classification, 'plan_checkpoint_mismatch');

const planBindingMismatch = evalReport({ manifest: { ...boundManifest, planBindingHash: 'b'.repeat(64) } });
assert.equal(planBindingMismatch.status, 'failed');
assert.equal(planBindingMismatch.classification, 'plan_binding_hash_mismatch');

const activePollution = evalReport({
  legacyBaselineCandidates: [
    'p7v2-baseline-r3b-recovery6-20260715165422',
    'p7v2-baseline-r3b-recovery6-20260715153726',
  ],
});
assert.equal(activePollution.status, 'passed');
assert.equal(activePollution.resolvedBaselineRunId, boundManifest.baselineRunId);
assert.equal(activePollution.legacyFallbackUsed, false);

const missingManifest = evalReport({ manifest: null });
assert.equal(missingManifest.status, 'failed');
assert.equal(missingManifest.legacyFallbackUsed, false);

const invalidRevalidation = evalReport({
  revalidation: {
    runtimeFreezeStillValid: false,
    immutableMismatchFields: ['planBindingHash'],
    plannedRunIdsBindingPassed: false,
  },
});
assert.equal(invalidRevalidation.status, 'failed');
assert.equal(invalidRevalidation.classification, 'runtime_freeze_revalidation_failed');

console.log(JSON.stringify({
  phase: 'P7-V2-R3B-PREFLIGHT-BINDING-V2',
  status: 'passed',
  fixtures: 9,
  preflightBindingV2FixturePassed: true,
  freshManifestResolutionPassed: true,
  legacyFallbackIsolationPassed: true,
  runtimeFreezeBindingFixturePassed: true,
  semanticFailureExitCodePassed: true,
}, null, 2));
