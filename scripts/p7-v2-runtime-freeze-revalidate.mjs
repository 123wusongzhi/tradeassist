import { fileURLToPath } from 'node:url';
import path from 'node:path';
import {
  readRuntimeFreezeContract,
  revalidateRuntimeFreezeImmutableInputs,
  BINARY_PROVENANCE_BINDING_VERSION,
  INPUT_SEQUENCE_BINDING_VERSION,
  RUNTIME_FREEZE_LIFECYCLE_CONTRACT_VERSION,
  RUNTIME_FREEZE_PATH,
} from './p7-v2-r3b-lpc-r3-runtime-freeze.mjs';
import { readJSON, writeJSON } from './p7-v2-lib.mjs';
import { CONFIG_FINGERPRINT_VERSION, freezeCurrentContract, generatedEvidenceDiffAudit, RUNTIME_FREEZE_SCOPE_VERSION } from './p7-v2-runtime-freeze-scope.mjs';

export const REVALIDATION_PATH = 'docs/p7-v2-r3b-runtime-freeze-revalidation.json';

export function revalidateRuntimeFreeze({ writeReport = false, mode = 'revalidate' } = {}) {
  if (mode !== 'revalidate') {
    const report = {
      phase: 'P7-V2-R3B-RUNTIME-FREEZE-LIFECYCLE-FIX',
      component: 'runtime-freeze-revalidation',
      status: 'failed',
      validationMode: mode,
      classification: 'unsupported_runtime_freeze_validation_mode',
      runtimeFreezeStillValid: false,
    };
    if (writeReport) writeJSON(REVALIDATION_PATH, report);
    return report;
  }
  const storedDoc = readJSON(RUNTIME_FREEZE_PATH) || {};
  const stored = freezeCurrentContract(storedDoc) || {};
  const manifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {};
  const reportCreatedAt = new Date().toISOString();
  let rebuilt = {};
  let rebuildError = '';
  try {
    rebuilt = revalidateRuntimeFreezeImmutableInputs({ runtimeFreeze: stored, manifest });
  } catch (error) {
    rebuildError = error.message;
  }
  const compare = (key, storedValue, rebuiltValue, { allowNull = false } = {}) => {
    const hasStored = allowNull ? storedValue !== undefined : Boolean(storedValue);
    const hasRebuilt = allowNull ? rebuiltValue !== undefined : Boolean(rebuiltValue);
    return {
      key,
      stored: storedValue ?? '',
      rebuilt: rebuiltValue ?? '',
      match: hasStored && hasRebuilt && storedValue === rebuiltValue,
    };
  };
  const generatedEvidence = generatedEvidenceDiffAudit();
  const checks = [
    compare('runtimeFreezeId', stored.runtimeFreezeId, rebuilt.runtimeFreezeId),
    compare('runtimeContentHash', stored.runtimeContentHash, rebuilt.runtimeContentHash),
    compare('planBindingHash', stored.planBindingHash, rebuilt.planBindingHash),
    compare('freezeCreationGitHead', stored.freezeCreationGitHead || stored.git?.commit, rebuilt.freezeCreationGitHead || rebuilt.git?.commit),
    compare('freezeCreationGitTree', stored.freezeCreationGitTree || stored.git?.tree, rebuilt.freezeCreationGitTree || rebuilt.git?.tree),
    compare('runtimeSourceTreeHash', stored.runtimeSourceTreeHash, rebuilt.runtimeSourceTreeHash),
    compare('evidenceToolingHash', stored.evidenceToolingHash, rebuilt.evidenceToolingHash),
    compare('loadScriptsHash', stored.loadScriptsHash, rebuilt.loadScriptsHash),
    compare('metricSemanticsHash', stored.metricSemanticsHash, rebuilt.metricSemanticsHash),
    compare('datasetGeneratorHash', stored.datasetGeneratorHash, rebuilt.datasetGeneratorHash),
    compare('configFingerprint', stored.configFingerprint, rebuilt.configFingerprint),
    compare('sloFingerprint', stored.sloFingerprint, rebuilt.sloFingerprint),
    compare('routeCredentialMatrixFingerprint', stored.routeCredentialMatrixFingerprint, rebuilt.routeCredentialMatrixFingerprint),
    compare('regressionPolicyFingerprint', stored.regressionPolicyFingerprint, rebuilt.regressionPolicyFingerprint),
    compare('baselineBinaryProvenanceHash', stored.baselineBinaryProvenanceHash, rebuilt.baselineBinaryProvenanceHash),
    compare('currentBinaryProvenanceHash', stored.currentBinaryProvenanceHash, rebuilt.currentBinaryProvenanceHash),
    compare('baselineBinarySha256', stored.baselineBinarySha256, rebuilt.baselineBinarySha256),
    compare('currentBinarySha256', stored.currentBinarySha256, rebuilt.currentBinarySha256),
    compare('inputSequenceManifestHash', stored.inputSequenceManifestHash, rebuilt.inputSequenceManifestHash),
    compare('branchMixFingerprint', stored.branchMixFingerprint, rebuilt.branchMixFingerprint),
    compare('immutableTrackedDiffHash', stored.git?.immutableTrackedDiffHash ?? stored.git?.trackedDiffHash ?? stored.immutableTrackedDiffHash, rebuilt.git?.immutableTrackedDiffHash ?? rebuilt.git?.trackedDiffHash ?? rebuilt.immutableTrackedDiffHash, { allowNull: true }),
  ];
  const versionsValid =
    (stored.runtimeFreezeIdentityVersion ?? 1) === 2 &&
    (stored.runtimeFreezeScopeVersion ?? 1) === RUNTIME_FREEZE_SCOPE_VERSION &&
    (stored.configFingerprintVersion ?? 1) === CONFIG_FINGERPRINT_VERSION &&
    (stored.runtimeFreezeLifecycleContractVersion ?? 1) === RUNTIME_FREEZE_LIFECYCLE_CONTRACT_VERSION &&
    (stored.runtimeFreezeLifecycleVersion ?? stored.runtimeFreezeLifecycleContractVersion ?? 1) === RUNTIME_FREEZE_LIFECYCLE_CONTRACT_VERSION &&
    (stored.binaryProvenanceBindingVersion ?? BINARY_PROVENANCE_BINDING_VERSION) === BINARY_PROVENANCE_BINDING_VERSION &&
    (stored.inputSequenceBindingVersion ?? INPUT_SEQUENCE_BINDING_VERSION) === INPUT_SEQUENCE_BINDING_VERSION &&
    stored.canonicalSchemaVersion === 3 &&
    stored.loadProfileFingerprintVersion === 3;
  const runtimeFreezeIdMatchesManifest = Boolean(manifest.runtimeFreezeId && manifest.runtimeFreezeId === stored.runtimeFreezeId);
  const freezeCreationGitHeadMatch = Boolean(stored.freezeCreationGitHead && stored.freezeCreationGitHead === rebuilt.freezeCreationGitHead);
  const freezeCreationGitTreeMatch = Boolean(stored.freezeCreationGitTree && stored.freezeCreationGitTree === rebuilt.freezeCreationGitTree);
  const planCheckpointMatch = Boolean(manifest.planCheckpoint && manifest.planCheckpoint === stored.planBindingPayload?.planCheckpoint && manifest.planCheckpoint === rebuilt.planBindingPayload?.planCheckpoint);
  const immutableWorkingTreeClean = rebuilt.immutableWorkingTreeClean === true && rebuilt.immutableTrackedDiffPresent === false;
  const createdAfterFreeze = Boolean(stored.createdAt && Date.parse(reportCreatedAt) >= Date.parse(stored.createdAt));
  const runtimeFreezeStillValid =
    !rebuildError &&
    versionsValid &&
    checks.every((check) => check.match) &&
    runtimeFreezeIdMatchesManifest &&
    freezeCreationGitHeadMatch &&
    freezeCreationGitTreeMatch &&
    planCheckpointMatch &&
    immutableWorkingTreeClean &&
    generatedEvidence.generatedEvidenceExcluded === true &&
    createdAfterFreeze;
  const report = {
    phase: 'P7-V2-R3B-RUNTIME-FREEZE-LIFECYCLE-FIX',
    component: 'runtime-freeze-revalidation',
    status: runtimeFreezeStillValid ? 'passed' : 'failed',
    validationMode: mode,
    runtimeFreezeStillValid,
    generatedAt: reportCreatedAt,
    runtimeFreezeScopeVersion: stored.runtimeFreezeScopeVersion ?? null,
    configFingerprintVersion: stored.configFingerprintVersion ?? null,
    runtimeFreezeLifecycleContractVersion: stored.runtimeFreezeLifecycleContractVersion ?? null,
    runtimeFreezeLifecycleVersion: stored.runtimeFreezeLifecycleVersion ?? stored.runtimeFreezeLifecycleContractVersion ?? null,
    binaryProvenanceBindingVersion: stored.binaryProvenanceBindingVersion ?? null,
    inputSequenceBindingVersion: stored.inputSequenceBindingVersion ?? null,
    storedRuntimeFreezeId: stored.runtimeFreezeId || '',
    rebuiltRuntimeFreezeId: rebuilt.runtimeFreezeId || '',
    revalidationRuntimeFreezeId: stored.runtimeFreezeId || '',
    currentManifestRuntimeFreezeId: manifest.runtimeFreezeId || '',
    revalidationGitHead: rebuilt.freezeCreationGitHead || rebuilt.git?.commit || '',
    revalidationGitTree: rebuilt.freezeCreationGitTree || rebuilt.git?.tree || '',
    revalidationPlanCheckpoint: rebuilt.planBindingPayload?.planCheckpoint || '',
    currentManifestPlanCheckpoint: manifest.planCheckpoint || '',
    revalidationCreatedAfterFreeze: createdAfterFreeze,
    runtimeFreezeIdMatchesManifest,
    freezeCreationGitHeadMatch,
    freezeCreationGitTreeMatch,
    planCheckpointMatch,
    runtimeFreezeIdentityVersion: stored.runtimeFreezeIdentityVersion ?? null,
    runtimeContentHash: stored.runtimeContentHash || '',
    rebuiltRuntimeContentHash: rebuilt.runtimeContentHash || '',
    planBindingHash: stored.planBindingHash || '',
    rebuiltPlanBindingHash: rebuilt.planBindingHash || '',
    runtimeContentHashMatch: Boolean(stored.runtimeContentHash && rebuilt.runtimeContentHash && stored.runtimeContentHash === rebuilt.runtimeContentHash),
    planBindingHashMatch: Boolean(stored.planBindingHash && rebuilt.planBindingHash && stored.planBindingHash === rebuilt.planBindingHash),
    plannedManifestBindingPassed: Boolean(stored.planBindingHash && rebuilt.planBindingHash && stored.planBindingHash === rebuilt.planBindingHash),
    plannedRunIdsBindingPassed: ['baselineRunId', 'currentRunId', 'soakRunId', 'demoRun1Id', 'demoRun2Id'].every((key) => stored.planBindingPayload?.[key] && stored.planBindingPayload?.[key] === rebuilt.planBindingPayload?.[key]),
    baselineBinaryBindingPassed: Boolean(stored.baselineBinarySha256 && rebuilt.baselineBinarySha256 && stored.baselineBinarySha256 === rebuilt.baselineBinarySha256),
    currentBinaryBindingPassed: Boolean(stored.currentBinarySha256 && rebuilt.currentBinarySha256 && stored.currentBinarySha256 === rebuilt.currentBinarySha256),
    baselineRuntimeCommitBindingPassed: Boolean(stored.baselineRuntimeCommit && rebuilt.baselineRuntimeCommit && stored.baselineRuntimeCommit === rebuilt.baselineRuntimeCommit),
    currentRuntimeCommitBindingPassed: Boolean(stored.currentRuntimeCommit && rebuilt.currentRuntimeCommit && stored.currentRuntimeCommit === rebuilt.currentRuntimeCommit),
    inputSequenceBindingPassed: Boolean(stored.inputSequenceManifestHash && rebuilt.inputSequenceManifestHash && stored.inputSequenceManifestHash === rebuilt.inputSequenceManifestHash),
    branchMixBindingPassed: Boolean(stored.branchMixFingerprint && rebuilt.branchMixFingerprint && stored.branchMixFingerprint === rebuilt.branchMixFingerprint),
    gitCommitMatch: freezeCreationGitHeadMatch,
    immutableWorkingTreeClean,
    immutableTrackedDiffPresent: rebuilt.immutableTrackedDiffPresent === true,
    stagedImmutableChangeCount: rebuilt.stagedImmutableChangeCount || 0,
    unstagedImmutableChangeCount: rebuilt.unstagedImmutableChangeCount || 0,
    untrackedImmutableChangeCount: rebuilt.untrackedImmutableChangeCount || 0,
    generatedEvidenceExcluded: generatedEvidence.generatedEvidenceExcluded === true,
    generatedEvidenceDoesNotInvalidateFreeze: generatedEvidence.workingTreeGloballyClean === false && generatedEvidence.generatedEvidenceExcluded === true ? runtimeFreezeStillValid : generatedEvidence.generatedEvidenceExcluded === true,
    workingTreeGloballyClean: generatedEvidence.workingTreeGloballyClean,
    generatedEvidenceChangeCount: generatedEvidence.generatedEvidenceChangeCount,
    unexpectedChangeCount: generatedEvidence.unexpectedChangeCount,
    unexpectedPaths: generatedEvidence.unexpectedPaths,
    immutableMismatchFields: checks.filter((check) => !check.match).map((check) => check.key),
    storedTrackedDiffHash: stored.git?.immutableTrackedDiffHash ?? stored.git?.trackedDiffHash ?? '',
    rebuiltTrackedDiffHash: rebuilt.git?.immutableTrackedDiffHash ?? rebuilt.git?.trackedDiffHash ?? '',
    storedConfigFingerprint: stored.configFingerprint || '',
    rebuiltConfigFingerprint: rebuilt.configFingerprint || '',
    rebuildError,
    checks,
  };
  if (writeReport) writeJSON(REVALIDATION_PATH, report);
  return report;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const modeIndex = process.argv.indexOf('--mode');
  const mode = modeIndex >= 0 ? process.argv[modeIndex + 1] : 'revalidate';
  const report = revalidateRuntimeFreeze({ writeReport: process.argv.includes('--write'), mode });
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}

export default readRuntimeFreezeContract;
