import { fileURLToPath } from 'node:url';
import path from 'node:path';
import {
  readRuntimeFreezeContract,
  revalidateRuntimeFreezeImmutableInputs,
  RUNTIME_FREEZE_LIFECYCLE_CONTRACT_VERSION,
  RUNTIME_FREEZE_PATH,
} from './p7-v2-r3b-lpc-r3-runtime-freeze.mjs';
import { readJSON, writeJSON } from './p7-v2-lib.mjs';
import { CONFIG_FINGERPRINT_VERSION, freezeCurrentContract, RUNTIME_FREEZE_SCOPE_VERSION } from './p7-v2-runtime-freeze-scope.mjs';

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
  let rebuilt = {};
  let rebuildError = '';
  try {
    rebuilt = revalidateRuntimeFreezeImmutableInputs({ runtimeFreeze: stored, manifest: readJSON('docs/p7-v2-r3b-run-manifest.json') || {} });
  } catch (error) {
    rebuildError = error.message;
  }
  const compare = (key, storedValue, rebuiltValue) => ({
    key,
    stored: storedValue || '',
    rebuilt: rebuiltValue || '',
    match: Boolean(storedValue && rebuiltValue && storedValue === rebuiltValue),
  });
  const checks = [
    compare('runtimeFreezeId', stored.runtimeFreezeId, rebuilt.runtimeFreezeId),
    compare('runtimeContentHash', stored.runtimeContentHash, rebuilt.runtimeContentHash),
    compare('planBindingHash', stored.planBindingHash, rebuilt.planBindingHash),
    compare('runtimeSourceTreeHash', stored.runtimeSourceTreeHash, rebuilt.runtimeSourceTreeHash),
    compare('evidenceToolingHash', stored.evidenceToolingHash, rebuilt.evidenceToolingHash),
    compare('loadScriptsHash', stored.loadScriptsHash, rebuilt.loadScriptsHash),
    compare('metricSemanticsHash', stored.metricSemanticsHash, rebuilt.metricSemanticsHash),
    compare('datasetGeneratorHash', stored.datasetGeneratorHash, rebuilt.datasetGeneratorHash),
    compare('configFingerprint', stored.configFingerprint, rebuilt.configFingerprint),
    compare('sloFingerprint', stored.sloFingerprint, rebuilt.sloFingerprint),
    compare('routeCredentialMatrixFingerprint', stored.routeCredentialMatrixFingerprint, rebuilt.routeCredentialMatrixFingerprint),
    compare('regressionPolicyFingerprint', stored.regressionPolicyFingerprint, rebuilt.regressionPolicyFingerprint),
    compare('immutableTrackedDiffHash', stored.git?.immutableTrackedDiffHash || stored.git?.trackedDiffHash, rebuilt.git?.immutableTrackedDiffHash || rebuilt.git?.trackedDiffHash),
  ];
  const versionsValid =
    (stored.runtimeFreezeIdentityVersion ?? 1) === 2 &&
    (stored.runtimeFreezeScopeVersion ?? 1) === RUNTIME_FREEZE_SCOPE_VERSION &&
    (stored.configFingerprintVersion ?? 1) === CONFIG_FINGERPRINT_VERSION &&
    (stored.runtimeFreezeLifecycleContractVersion ?? 1) === RUNTIME_FREEZE_LIFECYCLE_CONTRACT_VERSION &&
    stored.canonicalSchemaVersion === 3 &&
    stored.loadProfileFingerprintVersion === 3;
  const runtimeFreezeStillValid = !rebuildError && versionsValid && checks.every((check) => check.match);
  const report = {
    phase: 'P7-V2-R3B-RUNTIME-FREEZE-LIFECYCLE-FIX',
    component: 'runtime-freeze-revalidation',
    status: runtimeFreezeStillValid ? 'passed' : 'failed',
    validationMode: mode,
    runtimeFreezeStillValid,
    runtimeFreezeScopeVersion: stored.runtimeFreezeScopeVersion ?? null,
    configFingerprintVersion: stored.configFingerprintVersion ?? null,
    runtimeFreezeLifecycleContractVersion: stored.runtimeFreezeLifecycleContractVersion ?? null,
    storedRuntimeFreezeId: stored.runtimeFreezeId || '',
    rebuiltRuntimeFreezeId: rebuilt.runtimeFreezeId || '',
    runtimeFreezeIdentityVersion: stored.runtimeFreezeIdentityVersion ?? null,
    runtimeContentHash: stored.runtimeContentHash || '',
    rebuiltRuntimeContentHash: rebuilt.runtimeContentHash || '',
    planBindingHash: stored.planBindingHash || '',
    rebuiltPlanBindingHash: rebuilt.planBindingHash || '',
    runtimeContentHashMatch: Boolean(stored.runtimeContentHash && rebuilt.runtimeContentHash && stored.runtimeContentHash === rebuilt.runtimeContentHash),
    planBindingHashMatch: Boolean(stored.planBindingHash && rebuilt.planBindingHash && stored.planBindingHash === rebuilt.planBindingHash),
    plannedManifestBindingPassed: Boolean(stored.planBindingHash && rebuilt.planBindingHash && stored.planBindingHash === rebuilt.planBindingHash),
    plannedRunIdsBindingPassed: ['baselineRunId', 'currentRunId', 'soakRunId', 'demoRun1Id', 'demoRun2Id'].every((key) => stored.planBindingPayload?.[key] && stored.planBindingPayload?.[key] === rebuilt.planBindingPayload?.[key]),
    gitCommitMatch: Boolean(stored.planBindingPayload?.planCheckpoint && stored.planBindingPayload?.planCheckpoint === rebuilt.planBindingPayload?.planCheckpoint),
    immutableMismatchFields: checks.filter((check) => !check.match).map((check) => check.key),
    storedTrackedDiffHash: stored.git?.immutableTrackedDiffHash || stored.git?.trackedDiffHash || '',
    rebuiltTrackedDiffHash: rebuilt.git?.immutableTrackedDiffHash || rebuilt.git?.trackedDiffHash || '',
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
