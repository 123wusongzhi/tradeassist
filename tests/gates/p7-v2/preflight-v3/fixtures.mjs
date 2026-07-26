import assert from 'node:assert/strict';
import { evaluateRecovery6Preflight, PREFLIGHT_BINDING_VERSION } from '../../../../scripts/p7-v2-r3b-preflight.mjs';
import { buildRuntimeFreezeContract } from '../../../../scripts/p7-v2-r3b-lpc-r3-runtime-freeze.mjs';
import { FORMAL_INVOCATION_CONTRACT_VERSION } from '../../../../scripts/p7-v2-formal-invocation-lib.mjs';

const checkpoint = 'f'.repeat(40);
const tree = 'e'.repeat(40);
const sha = 'a'.repeat(64);
const currentSha = 'b'.repeat(64);
const manifest = {
  phase: 'P7-V2-R3B-FAST-CLOSE-R3',
  status: 'planned',
  active: true,
  validForExecution: true,
  canonicalSchemaVersion: 3,
  loadProfileFingerprintVersion: 3,
  formalInvocationContractVersion: FORMAL_INVOCATION_CONTRACT_VERSION,
  preflightBindingVersion: PREFLIGHT_BINDING_VERSION,
  formalExecutionStarted: false,
  executionStarted: false,
  environmentStarted: false,
  datasetExecuted: false,
  baselineExecuted: false,
  currentExecuted: false,
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
  planCheckpoint: checkpoint,
  controlToolingCommit: checkpoint,
  runtimeFreezeLifecycleVersion: 3,
  formalBinaryProvenanceVersion: 2,
  binaryProvenanceBound: true,
  baselineBinarySha256: sha,
  currentBinarySha256: currentSha,
  baselineRuntimeCommit: 'c'.repeat(40),
  currentRuntimeCommit: 'd'.repeat(40),
  formalInputSequenceBindingVersion: 1,
  inputSequenceBound: true,
  inputSequenceManifestHash: '1'.repeat(64),
  requestSequenceHash: '2'.repeat(64),
  webhookSequenceHash: '3'.repeat(64),
  authSequenceHash: '4'.repeat(64),
  webhookBranchMixFingerprint: '5'.repeat(64),
  authBranchMixFingerprint: '6'.repeat(64),
  branchMixFingerprint: '7'.repeat(64),
};
const freeze = buildRuntimeFreezeContract({
  manifest,
  planCheckpoint: checkpoint,
  now: '2026-07-18T04:30:00.000Z',
  skipCreationPreconditions: true,
  immutableDiffOverride: {
    hash: '8'.repeat(64),
    immutableWorkingTreeClean: true,
    immutableTrackedDiffPresent: false,
  },
});
freeze.freezeCreationGitHead = checkpoint;
freeze.freezeCreationGitTree = tree;
freeze.planBindingPayload.planCheckpoint = checkpoint;
const boundManifest = {
  ...manifest,
  status: 'runtime_frozen',
  runtimeFreezeCreated: true,
  runtimeFreezeId: freeze.runtimeFreezeId,
  runtimeContentHash: freeze.runtimeContentHash,
  planBindingHash: freeze.planBindingHash,
};
const revalidation = {
  runtimeFreezeStillValid: true,
  revalidationRuntimeFreezeId: freeze.runtimeFreezeId,
  revalidationGitHead: checkpoint,
  revalidationGitTree: tree,
  revalidationPlanCheckpoint: checkpoint,
  revalidationCreatedAfterFreeze: true,
  generatedEvidenceExcluded: true,
  immutableMismatchFields: [],
  plannedRunIdsBindingPassed: true,
  baselineBinaryBindingPassed: true,
  currentBinaryBindingPassed: true,
  baselineRuntimeCommitBindingPassed: true,
  currentRuntimeCommitBindingPassed: true,
  inputSequenceBindingPassed: true,
  branchMixBindingPassed: true,
};

function evaluate(overrides = {}) {
  return evaluateRecovery6Preflight({
    manifest: overrides.manifest ?? boundManifest,
    runtimeFreezeDoc: overrides.runtimeFreezeDoc ?? { current: freeze },
    currentGitHead: overrides.currentGitHead ?? checkpoint,
    currentGitTree: overrides.currentGitTree ?? tree,
    revalidation: overrides.revalidation ?? revalidation,
    legacyBaselineCandidates: overrides.legacyBaselineCandidates ?? ['p7v2-baseline-r3b-recovery6-legacy'],
    checkLiveResources: false,
  });
}

const passed = evaluate();
assert.equal(passed.status, 'passed');
assert.equal(passed.preflightBindingVersion, 3);
assert.equal(passed.formalInvocationContractVersion, 2);
assert.equal(passed.selectedManifestReason, 'canonical_active_recovery6_binary_bound_manifest');
assert.equal(passed.legacyFallbackUsed, false);
assert.equal(passed.ignoredLegacyCandidates.length, 1);
assert.equal(passed.resolvedBaselineRunId, boundManifest.baselineRunId);
assert.equal(passed.resolvedCurrentRunId, boundManifest.currentRunId);

const legacyManifest = evaluate({ manifest: { ...boundManifest, formalInvocationContractVersion: 1 } });
assert.equal(legacyManifest.status, 'failed');
assert.equal(legacyManifest.classification, 'legacy_manifest_not_valid_for_binary_bound_formal_execution');

const staleRevalidation = evaluate({
  revalidation: { ...revalidation, revalidationGitHead: '0'.repeat(40) },
});
assert.equal(staleRevalidation.status, 'failed');
assert.equal(staleRevalidation.classification, 'stale_revalidation_evidence_rejected');

const runIdMismatch = evaluate({ manifest: { ...boundManifest, baselineRunId: 'p7v2-baseline-r3b-recovery6-other' } });
assert.equal(runIdMismatch.status, 'failed');

const generatedEvidenceDirtyAllowed = evaluate({
  revalidation: { ...revalidation, generatedEvidenceExcluded: true, immutableMismatchFields: [] },
});
assert.equal(generatedEvidenceDirtyAllowed.status, 'passed');

console.log(JSON.stringify({
  phase: 'P7-V2-R3B-PREFLIGHT-BINDING-V3',
  status: 'passed',
  fixtures: 5,
  canonicalManifestSelected: true,
  legacyFallbackUsed: false,
  staleRevalidationRejected: true,
}, null, 2));
