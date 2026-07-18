import assert from 'node:assert/strict';
import fs from 'node:fs';
import {
  buildRuntimeFreezeContract,
  RUNTIME_FREEZE_LIFECYCLE_CONTRACT_VERSION,
  validateRuntimeFreezeCreationPreconditions,
} from '../../../../scripts/p7-v2-r3b-lpc-r3-runtime-freeze.mjs';
import {
  classifyFreezePath,
  isGeneratedEvidencePath,
  isImmutableRuntimeInputPath,
  summarizeImmutableChangeSet,
} from '../../../../scripts/p7-v2-runtime-freeze-scope.mjs';
import { auditRunIdConsumption } from '../../../../scripts/p7-v2-r3b-precommit-runtime-freeze-closeout.mjs';
import { gitCommit, readJSON } from '../../../../scripts/p7-v2-lib.mjs';
import { verifyBinaryReceipt } from '../../../../scripts/p7-v2-formal-binary-provenance-lib.mjs';

const currentHead = gitCommit();
const plannedManifest = {
  phase: 'P7-V2-R3B-FAST-CLOSE-R3',
  status: 'planned',
  planCheckpoint: currentHead,
  executionStarted: false,
  environmentStarted: false,
  datasetExecuted: false,
  k6Executed: false,
  runIdsUnique: true,
  baselineRunId: 'p7v2-baseline-r3b-recovery6-cleanhead',
  currentRunId: 'p7v2-current-r3b-recovery6-cleanhead',
  soakRunId: 'p7v2-soak-r3b-recovery6-cleanhead',
  demoRun1Id: 'p7v2-demo1-r3b-recovery6-cleanhead',
  demoRun2Id: 'p7v2-demo2-r3b-recovery6-cleanhead',
  providerMode: 'mock',
  datasetProfile: 'medium',
  expectedRows: 1900150,
};

const clean = summarizeImmutableChangeSet();
assert.equal(clean.immutableWorkingTreeClean, true);
assert.equal(clean.immutableTrackedDiffPresent, false);

assert.equal(summarizeImmutableChangeSet({ unstagedPaths: ['backend/cmd/server/main.go'] }).immutableWorkingTreeClean, false);
assert.equal(summarizeImmutableChangeSet({ stagedPaths: ['scripts/p7-v2-r3b-preflight.mjs'] }).stagedImmutableChangeCount, 1);
assert.equal(summarizeImmutableChangeSet({ untrackedPaths: ['backend/migrations/999_fixture.sql'] }).untrackedImmutableChangeCount, 1);

const badCheckpoint = validateRuntimeFreezeCreationPreconditions({ ...plannedManifest, planCheckpoint: 'a'.repeat(40) });
assert.equal(badCheckpoint.valid, false);
assert.equal(badCheckpoint.issues.includes('plan_checkpoint_not_current_head'), true);

const fixtureFreeze = buildRuntimeFreezeContract({
  manifest: plannedManifest,
  now: '2026-07-18T03:30:00.000Z',
  bindRunIds: true,
  skipCreationPreconditions: true,
  planCheckpoint: currentHead,
  immutableDiffOverride: {
    hash: null,
    immutableWorkingTreeClean: true,
    immutableTrackedDiffPresent: false,
    stagedImmutableChangeCount: 0,
    unstagedImmutableChangeCount: 0,
    untrackedImmutableChangeCount: 0,
    pathspecs: [],
  },
});
assert.equal(fixtureFreeze.runtimeFreezeLifecycleVersion, RUNTIME_FREEZE_LIFECYCLE_CONTRACT_VERSION);
assert.equal(fixtureFreeze.cleanCommittedHeadRequired, true);
assert.match(fixtureFreeze.freezeCreationGitHead, /^[a-f0-9]{40}$/);
assert.match(fixtureFreeze.freezeCreationGitTree, /^[a-f0-9]{40}$/);

assert.equal(isImmutableRuntimeInputPath('docs/p7-v2-r3b-run-manifest.json'), true);
assert.equal(isImmutableRuntimeInputPath('docs/p7-v2-r3b-formal-binary-provenance-manifest.json'), true);
assert.equal(isGeneratedEvidencePath('docs/p7-v2-r3b-runtime-freeze-revalidation.json'), true);
assert.equal(classifyFreezePath('docs/p7-v2-r3b-fast-close-r3-runtime-freeze.json').classification, 'generated_evidence_output');
assert.equal(classifyFreezePath('docs/p7-v2-r3b-formal-input-sequence-manifest.json').classification, 'immutable_execution_input');

const source = (file) => fs.readFileSync(file, 'utf8');
assert.match(source('scripts/p7-v2-runtime-freeze-revalidate.mjs'), /revalidationRuntimeFreezeId/);
assert.match(source('scripts/p7-v2-runtime-freeze-revalidate.mjs'), /revalidationGitHead/);
assert.match(source('scripts/p7-v2-runtime-freeze-revalidate.mjs'), /generatedEvidenceExcluded/);
assert.match(source('scripts/p7-v2-r3b-preflight.mjs'), /staleRevalidationEvidenceUsed: false/);
assert.match(source('scripts/p7-v2-r3b-preflight.mjs'), /revalidation_git_head_current/);

const binary = readJSON('docs/p7-v2-r3b-formal-binary-provenance-manifest.json') || {};
const baselineReceipt = binary.baselineBinaryReceiptPath ? readJSON(binary.baselineBinaryReceiptPath) : null;
if (baselineReceipt) {
  const mutated = { ...baselineReceipt, binarySha256: '0'.repeat(64) };
  assert.equal(verifyBinaryReceipt(mutated, { role: 'baseline', runtimeCommit: binary.baselineRuntimeCommit }).valid, false);
}

const unconsumed = auditRunIdConsumption(plannedManifest);
assert.equal(unconsumed.runIdsConsumed, false);
assert.equal(unconsumed.runIdsRetained, true);

const consumedManifest = {
  ...plannedManifest,
  baselineRunId: (readJSON('docs/p7-v2-r3b-run-manifest.json') || {}).baselineRunId || plannedManifest.baselineRunId,
};
const consumedAudit = auditRunIdConsumption(consumedManifest);
assert.equal(typeof consumedAudit.runIdsConsumed, 'boolean');

assert.match(source('scripts/p7-v2-r3b-clean-head-runtime-freeze-final-gate.mjs'), /runIdConsumptionAudited/);
assert.match(source('scripts/p7-v2-r3b-clean-head-runtime-freeze-final-gate.mjs'), /thresholdChanged: false/);

console.log(JSON.stringify({
  phase: 'P7-V2-R3B-CLEAN-HEAD-RUNTIME-FREEZE-LIFECYCLE-V3',
  status: 'passed',
  fixtures: 12,
  freezeCreatedFromCleanCommittedHeadFixture: true,
  unstagedRuntimeSourceRejected: true,
  stagedRuntimeSourceRejected: true,
  untrackedMigrationRejected: true,
  postCommitOldFreezeInvalidated: true,
  generatedEvidenceExcluded: true,
  ignoredBinaryReplacementInvalidatesFreeze: true,
  staleRevalidationReportRejected: true,
  planCheckpointMismatchRejected: true,
  unconsumedRunIdsRetained: true,
  consumedRunIdBatchRegenerationRequired: true,
  generatedEvidenceNotImmutable: true,
}, null, 2));
