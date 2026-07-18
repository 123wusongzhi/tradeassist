import { readJSON } from './p7-v2-lib.mjs';
import { writeR3Report } from './p7-v2-r3-lib.mjs';
import { resolveActiveBaseline, resolveActiveCurrent, resolveFormalPairEvidence } from './p7-v2-evidence-resolver.mjs';

const args = process.argv.slice(2);
const fingerprintVersion = Number(args[args.indexOf('--fingerprint-version') + 1] || 1);
if (![1, 2, 3, 4].includes(fingerprintVersion)) throw new Error('fingerprint version must be 1, 2, 3, or 4');
const resolvedBaseline = resolveActiveBaseline();
const resolvedCurrent = resolveActiveCurrent();
const formalMode = fingerprintVersion >= 3;
const formalPair = formalMode ? resolveFormalPairEvidence({ requireFrozen: true, requireComparability: false }) : null;
const baseline = formalMode ? (formalPair.baselineRegistryEntry || {}) : (resolvedBaseline.baseline || {});
const current = formalMode ? (formalPair.currentRegistryEntry || {}) : (resolvedCurrent.entry || {});
const baselineManifest = formalMode ? (formalPair.baselineFrozenManifest || {}) : (readJSON(`docs/baselines/frozen/${baseline.runId || ''}/manifest.json`) || {});
const currentManifest = formalMode ? (formalPair.currentFrozenManifest || {}) : (resolvedCurrent.manifest || {});
const sidecarVersion = fingerprintVersion >= 4 ? 3 : fingerprintVersion;
const sidecarRoot = `docs/fingerprints/p7-v2/load-profile/v${sidecarVersion}`;
const baselineVersionedProfile = fingerprintVersion >= 2 ? readJSON(`${sidecarRoot}/${baseline.runId || ''}.json`) || {} : {};
const currentVersionedProfile = fingerprintVersion >= 2 ? readJSON(`${sidecarRoot}/${current.runId || ''}.json`) || {} : {};
const versionedFingerprintKey = `loadProfileFingerprintV${fingerprintVersion}`;
const comparableKeys = [
  'runtimeSourceTreeHash',
  'loadScriptsHash',
  'metricSemanticsHash',
  'datasetFingerprint',
  'configFingerprint',
  fingerprintVersion >= 2 ? versionedFingerprintKey : 'loadProfileFingerprint',
  'sloFingerprint',
  'routeCredentialMatrixFingerprint',
  'regressionPolicyFingerprint',
  'k6Version',
  'goVersion',
  'postgresVersion',
  'redisVersion',
  'hostClass',
  'selectedHost',
  'selectedPort',
  'baseUrl',
];
const comparableV4Keys = [
  'formalBinaryProvenanceVersion',
  'formalInputSequenceBindingVersion',
  'requestSequenceHash',
  'webhookSequenceHash',
  'authSequenceHash',
  'webhookDuplicateSequenceHash',
  'webhookBranchMixFingerprint',
  'authBranchMixFingerprint',
  'branchMixFingerprint',
  'inputSequenceManifestHash',
];
function values(manifest, entry, versionedProfile) {
  const environment = manifest.environmentFingerprint || entry.environmentFingerprint || {};
  return {
    runtimeSourceTreeHash: manifest.runtimeSourceTreeHash || '',
    loadScriptsHash: manifest.loadScriptsHash || '',
    metricSemanticsHash: manifest.metricSemanticsHash || '',
    datasetFingerprint: manifest.datasetFingerprint || '',
    configFingerprint: manifest.configFingerprint || '',
    loadProfileFingerprint: manifest.loadProfileFingerprint || '',
    [versionedFingerprintKey]: versionedProfile.loadProfileFingerprint || '',
    sloFingerprint: manifest.sloFingerprint || '',
    routeCredentialMatrixFingerprint: manifest.routeCredentialMatrixFingerprint || '',
    regressionPolicyFingerprint: manifest.regressionPolicyFingerprint || '',
    k6Version: environment.k6Version || '',
    goVersion: environment.goVersion || '',
    postgresVersion: environment.postgresVersion || environment.postgreSQLVersion || '',
    redisVersion: environment.redisVersion || '',
    hostClass: entry.hostClass || environment.hostClass || 'wsl2_local_postgresql_socket',
    selectedHost: manifest.selectedHost || entry.selectedHost || '',
    selectedPort: String(manifest.selectedPort || entry.selectedPort || ''),
    baseUrl: manifest.baseUrl || entry.baseUrl || '',
    formalBinaryProvenanceVersion: manifest.formalBinaryProvenanceVersion || entry.formalBinaryProvenanceVersion || '',
    formalInputSequenceBindingVersion: manifest.formalInputSequenceBindingVersion || entry.formalInputSequenceBindingVersion || '',
    binarySha256: manifest.binarySha256 || manifest.serverBinarySha256 || entry.serverBinarySha256 || '',
    expectedBinarySha256: manifest.expectedBinarySha256 || entry.expectedBinarySha256 || '',
    processExecutableSha256: manifest.processExecutableSha256 || entry.processExecutableSha256 || '',
    processExecutableSha256Match: manifest.processExecutableSha256Match ?? entry.processExecutableSha256Match ?? null,
    requestSequenceHash: manifest.requestSequenceHash || entry.requestSequenceHash || '',
    webhookSequenceHash: manifest.webhookSequenceHash || entry.webhookSequenceHash || '',
    authSequenceHash: manifest.authSequenceHash || entry.authSequenceHash || '',
    webhookDuplicateSequenceHash: manifest.webhookDuplicateSequenceHash || entry.webhookDuplicateSequenceHash || '',
    webhookBranchMixFingerprint: manifest.webhookBranchMixFingerprint || entry.webhookBranchMixFingerprint || '',
    authBranchMixFingerprint: manifest.authBranchMixFingerprint || entry.authBranchMixFingerprint || '',
    branchMixFingerprint: manifest.branchMixFingerprint || entry.branchMixFingerprint || '',
    inputSequenceManifestHash: manifest.inputSequenceManifestHash || entry.inputSequenceManifestHash || '',
  };
}
const baselineValues = values(baselineManifest, baseline, baselineVersionedProfile);
const currentValues = values(currentManifest, current, currentVersionedProfile);
const versionedProfileChecks = fingerprintVersion >= 2 ? [
  ['baseline-versioned-sidecar', baselineVersionedProfile.fingerprintVersion === sidecarVersion && baselineVersionedProfile.derivedEvidence === true && baselineVersionedProfile.sourceArtifactsModified === false],
  ['current-versioned-sidecar', currentVersionedProfile.fingerprintVersion === sidecarVersion && currentVersionedProfile.derivedEvidence === true && currentVersionedProfile.sourceArtifactsModified === false],
  ['baseline-canonical-profile', Array.isArray(baselineVersionedProfile.canonicalProfile?.load?.stages) && baselineVersionedProfile.canonicalProfile.load.configuredVUs > 0 && Array.isArray(baselineVersionedProfile.canonicalProfile?.scenarios) && baselineVersionedProfile.canonicalProfile.scenarios.length > 0 && Boolean(baselineVersionedProfile.canonicalProfile?.loadScript?.sha256)],
  ['current-canonical-profile', Array.isArray(currentVersionedProfile.canonicalProfile?.load?.stages) && currentVersionedProfile.canonicalProfile.load.configuredVUs > 0 && Array.isArray(currentVersionedProfile.canonicalProfile?.scenarios) && currentVersionedProfile.canonicalProfile.scenarios.length > 0 && Boolean(currentVersionedProfile.canonicalProfile?.loadScript?.sha256)],
  ['versioned-fingerprint-shape', /^[a-f0-9]{64}$/.test(baselineVersionedProfile.loadProfileFingerprint || '') && /^[a-f0-9]{64}$/.test(currentVersionedProfile.loadProfileFingerprint || '')],
] : [];
const runtimeFreeze = formalPair?.runtimeFreeze?.current || formalPair?.runtimeFreeze || {};
const runtimeFreezeChecks = fingerprintVersion >= 3 ? [
  ['baseline-runtime-freeze-metadata', /^[a-f0-9]{64}$/.test(baselineManifest.runtimeFreezeId || '') && baselineManifest.runtimeFreezeId === baselineManifest.runtimeFreezeContractHash && baselineManifest.runtimeFreezeRunId === baseline.runId],
  ['current-runtime-freeze-metadata', /^[a-f0-9]{64}$/.test(currentManifest.runtimeFreezeId || '') && currentManifest.runtimeFreezeId === currentManifest.runtimeFreezeContractHash && currentManifest.runtimeFreezeRunId === current.runId],
  ['matching-runtime-freeze-contract', baselineManifest.runtimeFreezeId === currentManifest.runtimeFreezeId],
] : [];
const binaryAndInputChecks = fingerprintVersion >= 4 ? [
  ['binary-provenance-version', baselineValues.formalBinaryProvenanceVersion === 2 && currentValues.formalBinaryProvenanceVersion === 2],
  ['input-sequence-version', baselineValues.formalInputSequenceBindingVersion === 1 && currentValues.formalInputSequenceBindingVersion === 1],
  ['baseline-binary-role-binding', baselineValues.binarySha256 && baselineValues.binarySha256 === runtimeFreeze.baselineBinarySha256],
  ['current-binary-role-binding', currentValues.binarySha256 && currentValues.binarySha256 === runtimeFreeze.currentBinarySha256],
  ['baseline-process-executable-binding', baselineValues.processExecutableSha256Match === true && baselineValues.processExecutableSha256 === runtimeFreeze.baselineBinarySha256],
  ['current-process-executable-binding', currentValues.processExecutableSha256Match === true && currentValues.processExecutableSha256 === runtimeFreeze.currentBinarySha256],
  ['runtime-commit-binding', Boolean(runtimeFreeze.baselineRuntimeCommit && runtimeFreeze.currentRuntimeCommit)],
  ['binary-provenance-hash-binding', /^[a-f0-9]{64}$/.test(runtimeFreeze.baselineBinaryProvenanceHash || '') && /^[a-f0-9]{64}$/.test(runtimeFreeze.currentBinaryProvenanceHash || '')],
  ['input-sequence-freeze-binding', baselineValues.inputSequenceManifestHash === runtimeFreeze.inputSequenceManifestHash && currentValues.inputSequenceManifestHash === runtimeFreeze.inputSequenceManifestHash],
] : [];
const checks = [
  ['baseline-registry', fingerprintVersion === 3 ? Boolean(formalPair.baselineRegistryEntry) : resolvedBaseline.valid],
  ['current-registry', fingerprintVersion === 3 ? Boolean(formalPair.currentRegistryEntry) : resolvedCurrent.valid],
  ['baseline-manifest', baselineManifest.immutable === true && baselineManifest.validForRegression === true],
  ['current-manifest', currentManifest.immutable === true && currentManifest.validForRegression === true],
  ['different-run-id', baseline.runId && current.runId && baseline.runId !== current.runId],
  ['different-artifact', fingerprintVersion === 3
    ? formalPair.selectedBaselineArtifactSha256 && formalPair.selectedCurrentArtifactSha256 && formalPair.selectedBaselineArtifactSha256 !== formalPair.selectedCurrentArtifactSha256
    : resolvedBaseline.baseline?.rawArtifactSha256 && resolvedCurrent.actualHash && resolvedBaseline.baseline.rawArtifactSha256 !== resolvedCurrent.actualHash],
  ['current-independent', current.independentRun === true && current.baselineRunId === baseline.runId],
  ...versionedProfileChecks,
  ...runtimeFreezeChecks,
  ...binaryAndInputChecks,
  ...comparableKeys.map((key) => [key, Boolean(baselineValues[key]) && baselineValues[key] === currentValues[key]]),
  ...(fingerprintVersion >= 4 ? comparableV4Keys.map((key) => [key, Boolean(baselineValues[key]) && baselineValues[key] === currentValues[key]]) : []),
];
const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
const report = {
  phase: fingerprintVersion >= 3 ? 'P7-V2-R3B-FAST-CLOSE-R3-FORMAL' : fingerprintVersion === 2 ? 'P7-V2-R3B-LPF-V2' : 'P7-V2-R3B-CI-RG',
  status: failed.length ? 'not_comparable' : 'passed',
  baselineRunId: baseline.runId || '',
  currentRunId: current.runId || '',
  baselineArtifactSha256: formalMode ? formalPair.selectedBaselineArtifactSha256 : (resolvedBaseline.baseline?.rawArtifactSha256 || ''),
  currentArtifactSha256: formalMode ? formalPair.selectedCurrentArtifactSha256 : (resolvedCurrent.actualHash || ''),
  baselineFrozenManifestPath: formalMode ? formalPair.baselineFrozenManifestPath : `docs/baselines/frozen/${baseline.runId || ''}/manifest.json`,
  currentFrozenManifestPath: formalMode ? formalPair.currentFrozenManifestPath : `docs/currents/frozen/${current.runId || ''}/manifest.json`,
  runtimeFreezeId: formalMode ? formalPair.runtimeFreezeId : (baselineManifest.runtimeFreezeId || currentManifest.runtimeFreezeId || ''),
  canonicalSchemaVersion: formalMode ? 3 : undefined,
  loadProfileFingerprintVersion: fingerprintVersion,
  loadProfileFingerprint: fingerprintVersion >= 2 ? baselineValues[versionedFingerprintKey] : baselineValues.loadProfileFingerprint,
  baseline: baselineValues,
  current: currentValues,
  mismatchCount: failed.filter((item) => comparableKeys.includes(item) || comparableV4Keys.includes(item)).length,
  notComparableCount: failed.filter((item) => !comparableKeys.includes(item) && !comparableV4Keys.includes(item)).length,
  previousComparabilityStatus: fingerprintVersion === 2 ? (readJSON('docs/p7-v2-r3b-rebaseline2-comparability-report.json') || {}).status || '' : undefined,
  previousMismatchField: fingerprintVersion === 2 ? 'loadProfileFingerprint' : undefined,
  previousFingerprintVersion: fingerprintVersion === 2 ? 1 : undefined,
  currentFingerprintVersion: fingerprintVersion,
  loadProfileFingerprintMatch: fingerprintVersion >= 2 ? baselineValues[versionedFingerprintKey] === currentValues[versionedFingerprintKey] : baselineValues.loadProfileFingerprint === currentValues.loadProfileFingerprint,
  checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  binaryRoleBindingPassed: fingerprintVersion >= 4 ? binaryAndInputChecks.filter(([id]) => id.includes('binary') || id.includes('process')).every(([, ok]) => ok) : undefined,
  runtimeCommitBindingPassed: fingerprintVersion >= 4 ? Boolean(runtimeFreeze.baselineRuntimeCommit && runtimeFreeze.currentRuntimeCommit) : undefined,
  inputSequenceHashMatch: fingerprintVersion >= 4 ? comparableV4Keys.filter((key) => key.includes('Sequence') || key.includes('sequence')).every((key) => baselineValues[key] && baselineValues[key] === currentValues[key]) : undefined,
  webhookSequenceHashMatch: fingerprintVersion >= 4 ? baselineValues.webhookSequenceHash === currentValues.webhookSequenceHash : undefined,
  authSequenceHashMatch: fingerprintVersion >= 4 ? baselineValues.authSequenceHash === currentValues.authSequenceHash : undefined,
  branchMixFingerprintMatch: fingerprintVersion >= 4 ? baselineValues.branchMixFingerprint === currentValues.branchMixFingerprint : undefined,
  pairBinding: formalMode ? formalPair : undefined,
  issues: [...(formalMode ? formalPair.issues : (resolvedBaseline.valid ? [] : resolvedBaseline.issues)), ...failed],
};
const output = fingerprintVersion >= 3
  ? ['docs/p7-v2-r3b-fast-close-r3-comparability-report.json', 'docs/P7_V2_R3B_FAST_CLOSE_R3_COMPARABILITY_REPORT.md', 'P7-V2-R3B-FAST-CLOSE-R3-FORMAL Comparability Report']
  : fingerprintVersion === 2
  ? ['docs/p7-v2-r3b-lpf-comparability-v2-report.json', 'docs/P7_V2_R3B_LPF_COMPARABILITY_V2_REPORT.md', 'P7-V2-R3B-LPF-V2 Comparability Report']
  : ['docs/p7-v2-r3b-rebaseline2-comparability-report.json', 'docs/P7_V2_R3B_REBASELINE2_COMPARABILITY_REPORT.md', 'P7-V2-R3B-REBASELINE2 Comparability Report'];
writeR3Report(
  ...output,
  report,
  [['Baseline', report.baselineRunId], ['Current', report.currentRunId], ['Status', report.status], ['Mismatch count', report.mismatchCount]],
);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
