import assert from 'node:assert/strict';
import {
  buildRuntimeFreezeContract,
  revalidateRuntimeFreezeImmutableInputs,
  validateRuntimeFreezeCreationPreconditions,
} from '../../../../scripts/p7-v2-r3b-lpc-r3-runtime-freeze.mjs';
import { validateFormalExecutionLifecycle } from '../../../../scripts/p7-v2-r3b-lifecycle.mjs';
import { buildFormalConfigFingerprint, buildRuntimeFreezeSourceManifest } from '../../../../scripts/p7-v2-runtime-freeze-scope.mjs';
import { gitCommit } from '../../../../scripts/p7-v2-lib.mjs';

const plannedManifest = {
  phase: 'P7-V2-R3B-FAST-CLOSE-R3',
  status: 'planned',
  executionStarted: false,
  environmentStarted: false,
  datasetExecuted: false,
  k6Executed: false,
  runIdsUnique: true,
  baselineRunId: 'p7v2-baseline-r3b-recovery6-lifecycle',
  currentRunId: 'p7v2-current-r3b-recovery6-lifecycle',
  soakRunId: 'p7v2-soak-r3b-recovery6-lifecycle',
  demoRun1Id: 'p7v2-demo1-r3b-recovery6-lifecycle',
  demoRun2Id: 'p7v2-demo2-r3b-recovery6-lifecycle',
};

const currentHead = gitCommit();
assert.equal(validateRuntimeFreezeCreationPreconditions({ ...plannedManifest, planCheckpoint: currentHead }).issues.includes('plan_checkpoint_not_current_head'), false);
assert.equal(validateRuntimeFreezeCreationPreconditions({ ...plannedManifest, status: 'baseline_frozen' }).valid, false);

const cleanImmutableDiff = {
  hash: null,
  immutableWorkingTreeClean: true,
  immutableTrackedDiffPresent: false,
  stagedImmutableChangeCount: 0,
  unstagedImmutableChangeCount: 0,
  untrackedImmutableChangeCount: 0,
  pathspecs: [],
};
const freeze = buildRuntimeFreezeContract({ manifest: { ...plannedManifest, planCheckpoint: currentHead }, now: '2026-07-15T12:00:00.000Z', bindRunIds: false, skipCreationPreconditions: true, planCheckpoint: currentHead, immutableDiffOverride: cleanImmutableDiff });
assert.equal(freeze.runtimeFreezeIdentityVersion, 2);
assert.equal(freeze.runtimeFreezeLifecycleVersion, 3);
for (const status of ['baseline_frozen', 'current_frozen', 'comparability_passed', 'regression_passed', 'soak_passed', 'completed']) {
  const rebuilt = revalidateRuntimeFreezeImmutableInputs({ runtimeFreeze: freeze, manifest: { ...plannedManifest, status }, immutableDiffOverride: cleanImmutableDiff });
  assert.equal(rebuilt.runtimeFreezeId, freeze.runtimeFreezeId);
  assert.equal(rebuilt.runtimeContentHash, freeze.runtimeContentHash);
  assert.equal(rebuilt.planBindingHash, freeze.planBindingHash);
  assert.equal(rebuilt.runtimeSourceTreeHash, freeze.runtimeSourceTreeHash);
  assert.equal(rebuilt.configFingerprint, freeze.configFingerprint);
}

const baselineEvidence = {
  baselineRunId: plannedManifest.baselineRunId,
  baselineFrozenManifestExists: true,
  baselineRegistryEntryExists: true,
  baselineFrozen: {
    sha256: 'a'.repeat(64),
    sizeBytes: 123,
    validForRegression: true,
  },
};
assert.equal(validateFormalExecutionLifecycle({ previousState: 'baseline_completed', nextState: 'baseline_frozen', evidence: baselineEvidence }).status, 'passed');
assert.equal(validateFormalExecutionLifecycle({ previousState: 'baseline_frozen', nextState: 'planned', evidence: baselineEvidence }).issues[0].classification, 'lifecycle_transition_invalid');
assert.equal(validateFormalExecutionLifecycle({ previousState: 'dataset_completed', nextState: 'current_running', evidence: baselineEvidence }).issues[0].classification, 'lifecycle_transition_invalid');
assert.equal(validateFormalExecutionLifecycle({ previousState: 'baseline_completed', nextState: 'baseline_frozen', evidence: { baselineFrozenManifestExists: false } }).issues.some((issue) => issue.classification === 'lifecycle_evidence_missing'), true);

const currentEvidence = {
  ...baselineEvidence,
  currentRunId: plannedManifest.currentRunId,
  currentFrozenManifestExists: true,
  currentFrozen: { independentRun: true },
};
assert.equal(validateFormalExecutionLifecycle({ previousState: 'current_completed', nextState: 'current_frozen', evidence: currentEvidence }).status, 'passed');
assert.equal(validateFormalExecutionLifecycle({ previousState: 'current_frozen', nextState: 'comparability_running', evidence: currentEvidence }).status, 'passed');
assert.equal(validateFormalExecutionLifecycle({
  previousState: 'comparability_running',
  nextState: 'comparability_passed',
  evidence: { ...currentEvidence, comparability: { status: 'passed', mismatchCount: 0, notComparableCount: 0 } },
}).status, 'passed');
assert.equal(validateFormalExecutionLifecycle({ previousState: 'comparability_passed', nextState: 'regression_running', evidence: currentEvidence }).status, 'passed');
assert.equal(validateFormalExecutionLifecycle({
  previousState: 'regression_running',
  nextState: 'regression_passed',
  evidence: { ...currentEvidence, comparability: { status: 'passed' }, regression: { status: 'passed', failedMetricCount: 0, notComparableCount: 0, invalidMetricCount: 0 } },
}).status, 'passed');

const sourceA = buildRuntimeFreezeSourceManifest({ fileMap: { 'backend/cmd/server/main.go': 'package main\n', 'scripts/p7-v2-a.mjs': 'export default 1;\n' } }).manifestSha256;
const sourceB = buildRuntimeFreezeSourceManifest({ fileMap: { 'backend/cmd/server/main.go': 'package main\nfunc changed() {}\n', 'scripts/p7-v2-a.mjs': 'export default 1;\n' } }).manifestSha256;
assert.notEqual(sourceA, sourceB);

const configA = buildFormalConfigFingerprint({ network: { host: '127.0.0.1', port: 18080, baseUrl: 'http://127.0.0.1:18080' }, loadProfileFingerprint: '1'.repeat(64), datasetGeneratorHash: '2'.repeat(64), sloFingerprint: '3'.repeat(64), regressionPolicyFingerprint: '4'.repeat(64), routeCredentialMatrixFingerprint: '5'.repeat(64) }).hash;
const configB = buildFormalConfigFingerprint({ network: { host: '127.0.0.1', port: 18081, baseUrl: 'http://127.0.0.1:18081' }, loadProfileFingerprint: '1'.repeat(64), datasetGeneratorHash: '2'.repeat(64), sloFingerprint: '3'.repeat(64), regressionPolicyFingerprint: '4'.repeat(64), routeCredentialMatrixFingerprint: '5'.repeat(64) }).hash;
assert.notEqual(configA, configB);

console.log(JSON.stringify({ phase: 'P7-V2-R3B-RUNTIME-FREEZE-LIFECYCLE-FIX', status: 'passed', fixtures: 13 }, null, 2));
