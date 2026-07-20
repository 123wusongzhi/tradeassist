import fs from 'node:fs';
import path from 'node:path';
import { gitCommit, readJSON, root, writeJSON } from './p7-v2-lib.mjs';
import { auditRunIdConsumption } from './p7-v2-r3b-precommit-runtime-freeze-closeout.mjs';

const required = [
  'docs/p7-v2-r3b-lpc-r3-preflight-audit.json',
  'docs/p7-v2-r3b-lpc-r3-determinism-report.json',
  'docs/p7-v2-r3b-lpc-r3-consumer-compatibility.json',
  'docs/p7-v2-r3b-formal-binary-provenance-manifest.json',
  'docs/p7-v2-r3b-formal-input-sequence-manifest.json',
];
if (required.some((file) => readJSON(file)?.status !== 'passed')) throw new Error('passed Formal Wiring evidence is required before planning Recovery6');
const binaryProvenance = readJSON('docs/p7-v2-r3b-formal-binary-provenance-manifest.json') || {};
const inputSequence = readJSON('docs/p7-v2-r3b-formal-input-sequence-manifest.json') || {};
const currentPlan = readJSON('docs/p7-v2-r3b-run-manifest.json') || {};
const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
const existingRunIds = {
  baselineRunId: currentPlan.baselineRunId || '',
  currentRunId: currentPlan.currentRunId || '',
  soakRunId: currentPlan.soakRunId || '',
  demoRun1Id: currentPlan.demoRun1Id || '',
  demoRun2Id: currentPlan.demoRun2Id || '',
};
const canRetainExistingRunIds =
  !process.argv.includes('--force-new-run-ids') &&
  currentPlan.runIdsConsumed === false &&
  Object.values(existingRunIds).every((runId) => /^p7v2-(baseline|current|soak|demo[12])-r3b-recovery6-[a-z0-9_-]+$/.test(runId)) &&
  auditRunIdConsumption(currentPlan).runIdsConsumed === false;
const runIds = canRetainExistingRunIds
  ? existingRunIds
  : {
      baselineRunId: `p7v2-baseline-r3b-recovery6-${stamp}`, currentRunId: `p7v2-current-r3b-recovery6-${stamp}`,
      soakRunId: `p7v2-soak-r3b-recovery6-${stamp}`, demoRun1Id: `p7v2-demo1-r3b-recovery6-${stamp}`, demoRun2Id: `p7v2-demo2-r3b-recovery6-${stamp}`,
    };
const allRunIds = Object.values(runIds);
if (new Set(allRunIds).size !== allRunIds.length) throw new Error('Recovery6 run IDs are not unique');
const registries = [
  readJSON('docs/baselines/p7-v2-baseline-registry.json') || {},
  readJSON('docs/currents/p7-v2-current-registry.json') || {},
  readJSON('docs/fingerprints/p7-v2/load-profile-registry.json') || {},
];
for (const runId of allRunIds) {
  if (fs.existsSync(path.join(root, 'artifacts', 'p7-v2', 'baseline', runId)) || fs.existsSync(path.join(root, 'artifacts', 'p7-v2', 'current', runId)) ||
      fs.existsSync(path.join(root, 'docs', 'baselines', 'frozen', runId)) || fs.existsSync(path.join(root, 'docs', 'currents', 'frozen', runId)) ||
      JSON.stringify(registries).includes(runId)) throw new Error(`Recovery6 run ID already exists: ${runId}`);
}
const supersededPlan = currentPlan?.baselineRunId
  ? {
      ...currentPlan,
      status: 'superseded_before_formal_execution',
      active: false,
      validForExecution: false,
      executionStarted: false,
      reason: currentPlan.runtimeFreezeId ? 'runtime_freeze_revalidation_failed' : 'formal_pair_wiring_completed_after_plan_creation',
      supersededAt: new Date().toISOString(),
    }
  : {
      phase: 'P7-V2-R3B-FAST-CLOSE-R2',
      baselineRunId: 'p7v2-baseline-r3b-recovery5-20260715091700',
      status: 'aborted_before_execution',
      active: false,
      validForExecution: false,
      executionStarted: false,
      baselineArtifactCreated: false,
      currentExecuted: false,
      reason: 'canonical_load_profile_stage_validation_failed',
    };
writeJSON('docs/p7-v2-r3b-recovery6-superseded-plan.json', supersededPlan);
const manifest = {
  phase: 'P7-V2-R3B-FAST-CLOSE-R3', status: 'planned', canonicalSchemaVersion: 3, loadProfileFingerprintVersion: 3,
  ...runIds, selectedHost: '127.0.0.1', selectedPort: 18080, baseUrl: 'http://127.0.0.1:18080',
  controlToolingCommit: gitCommit(),
  planCheckpoint: gitCommit(),
  runtimeFreezeLifecycleVersion: 3,
  baselineRuntimeCommit: binaryProvenance.baselineRuntimeCommit || '',
  currentRuntimeCommit: binaryProvenance.currentRuntimeCommit || '',
  baselineBinarySha256: binaryProvenance.baselineBinarySha256 || '',
  currentBinarySha256: binaryProvenance.currentBinarySha256 || '',
  baselineBinaryPath: binaryProvenance.baselineBinaryPath || '',
  currentBinaryPath: binaryProvenance.currentBinaryPath || '',
  baselineBinaryReceiptPath: binaryProvenance.baselineBinaryReceiptPath || '',
  currentBinaryReceiptPath: binaryProvenance.currentBinaryReceiptPath || '',
  baselineBinaryProvenanceHash: binaryProvenance.baselineBinaryProvenanceHash || '',
  currentBinaryProvenanceHash: binaryProvenance.currentBinaryProvenanceHash || '',
  binaryProvenance,
  formalBinaryProvenanceVersion: binaryProvenance.formalBinaryProvenanceVersion || null,
  binaryProvenanceBound: binaryProvenance.status === 'passed',
  formalInputSequenceBindingVersion: inputSequence.formalInputSequenceBindingVersion || null,
  inputSequenceManifestHash: inputSequence.inputSequenceManifestHash || '',
  requestSequenceHash: inputSequence.requestSequenceHash || '',
  webhookSequenceHash: inputSequence.webhookSequenceHash || '',
  authSequenceHash: inputSequence.authSequenceHash || '',
  webhookDuplicateSequenceHash: inputSequence.webhookDuplicateSequenceHash || '',
  webhookBranchMixFingerprint: inputSequence.webhookBranchMixFingerprint || '',
  authBranchMixFingerprint: inputSequence.authBranchMixFingerprint || '',
  branchMixFingerprint: inputSequence.branchMixFingerprint || '',
  inputSequenceBound: inputSequence.status === 'passed',
  runtimeFreezeId: null,
  formal: true,
  active: true,
  validForExecution: true,
  executionStarted: false,
  formalExecutionStarted: false,
  runtimeFreezeCreated: false,
  environmentStarted: false,
  datasetExecuted: false,
  baselineExecuted: false,
  currentExecuted: false,
  comparabilityExecuted: false,
  regressionExecuted: false,
  soakExecuted: false,
  demoExecuted: false,
  providerMode: 'mock',
  datasetProfile: 'medium',
  expectedRows: 1900150,
  runIdsUnique: true,
  runIdsConsumed: false,
  runIdsRetained: canRetainExistingRunIds,
  newRunIdsCreated: !canRetainExistingRunIds,
  previousPlan: supersededPlan,
  updatedAt: new Date().toISOString(),
};
writeJSON('docs/p7-v2-r3b-run-manifest.json', manifest);
console.log(JSON.stringify(manifest, null, 2));
