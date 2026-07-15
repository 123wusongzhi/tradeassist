import { readJSON } from './p7-v2-lib.mjs';
import { writeR3Report } from './p7-v2-r3-lib.mjs';
import { resolveActiveBaseline, resolveActiveCurrent } from './p7-v2-evidence-resolver.mjs';

const args = process.argv.slice(2);
const fingerprintVersion = Number(args[args.indexOf('--fingerprint-version') + 1] || 1);
if (![1, 2, 3].includes(fingerprintVersion)) throw new Error('fingerprint version must be 1, 2, or 3');
const resolvedBaseline = resolveActiveBaseline();
const resolvedCurrent = resolveActiveCurrent();
const baseline = resolvedBaseline.baseline || {};
const current = resolvedCurrent.entry || {};
const baselineManifest = readJSON(`docs/baselines/frozen/${baseline.runId || ''}/manifest.json`) || {};
const currentManifest = resolvedCurrent.manifest || {};
const sidecarRoot = `docs/fingerprints/p7-v2/load-profile/v${fingerprintVersion}`;
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
  };
}
const baselineValues = values(baselineManifest, baseline, baselineVersionedProfile);
const currentValues = values(currentManifest, current, currentVersionedProfile);
const versionedProfileChecks = fingerprintVersion >= 2 ? [
  ['baseline-versioned-sidecar', baselineVersionedProfile.fingerprintVersion === fingerprintVersion && baselineVersionedProfile.derivedEvidence === true && baselineVersionedProfile.sourceArtifactsModified === false],
  ['current-versioned-sidecar', currentVersionedProfile.fingerprintVersion === fingerprintVersion && currentVersionedProfile.derivedEvidence === true && currentVersionedProfile.sourceArtifactsModified === false],
  ['baseline-canonical-profile', Array.isArray(baselineVersionedProfile.canonicalProfile?.load?.stages) && baselineVersionedProfile.canonicalProfile.load.configuredVUs > 0 && Array.isArray(baselineVersionedProfile.canonicalProfile?.scenarios) && baselineVersionedProfile.canonicalProfile.scenarios.length > 0 && Boolean(baselineVersionedProfile.canonicalProfile?.loadScript?.sha256)],
  ['current-canonical-profile', Array.isArray(currentVersionedProfile.canonicalProfile?.load?.stages) && currentVersionedProfile.canonicalProfile.load.configuredVUs > 0 && Array.isArray(currentVersionedProfile.canonicalProfile?.scenarios) && currentVersionedProfile.canonicalProfile.scenarios.length > 0 && Boolean(currentVersionedProfile.canonicalProfile?.loadScript?.sha256)],
  ['versioned-fingerprint-shape', /^[a-f0-9]{64}$/.test(baselineVersionedProfile.loadProfileFingerprint || '') && /^[a-f0-9]{64}$/.test(currentVersionedProfile.loadProfileFingerprint || '')],
] : [];
const runtimeFreezeChecks = fingerprintVersion === 3 ? [
  ['baseline-runtime-freeze-metadata', /^[a-f0-9]{64}$/.test(baselineManifest.runtimeFreezeId || '') && baselineManifest.runtimeFreezeId === baselineManifest.runtimeFreezeContractHash && baselineManifest.runtimeFreezeRunId === baseline.runId],
  ['current-runtime-freeze-metadata', /^[a-f0-9]{64}$/.test(currentManifest.runtimeFreezeId || '') && currentManifest.runtimeFreezeId === currentManifest.runtimeFreezeContractHash && currentManifest.runtimeFreezeRunId === current.runId],
  ['matching-runtime-freeze-contract', baselineManifest.runtimeFreezeId === currentManifest.runtimeFreezeId],
] : [];
const checks = [
  ['baseline-registry', resolvedBaseline.valid],
  ['current-registry', resolvedCurrent.valid],
  ['baseline-manifest', baselineManifest.immutable === true && baselineManifest.validForRegression === true],
  ['current-manifest', currentManifest.immutable === true && currentManifest.validForRegression === true],
  ['different-run-id', baseline.runId && current.runId && baseline.runId !== current.runId],
  ['different-artifact', resolvedBaseline.baseline?.rawArtifactSha256 && resolvedCurrent.actualHash && resolvedBaseline.baseline.rawArtifactSha256 !== resolvedCurrent.actualHash],
  ['current-independent', current.independentRun === true && current.baselineRunId === baseline.runId],
  ...versionedProfileChecks,
  ...runtimeFreezeChecks,
  ...comparableKeys.map((key) => [key, Boolean(baselineValues[key]) && baselineValues[key] === currentValues[key]]),
];
const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
const report = {
  phase: fingerprintVersion === 3 ? 'P7-V2-R3B-LPC-R3' : fingerprintVersion === 2 ? 'P7-V2-R3B-LPF-V2' : 'P7-V2-R3B-CI-RG',
  status: failed.length ? 'not_comparable' : 'passed',
  baselineRunId: baseline.runId || '',
  currentRunId: current.runId || '',
  baseline: baselineValues,
  current: currentValues,
  mismatchCount: failed.filter((item) => comparableKeys.includes(item)).length,
  notComparableCount: failed.filter((item) => !comparableKeys.includes(item)).length,
  previousComparabilityStatus: fingerprintVersion === 2 ? (readJSON('docs/p7-v2-r3b-rebaseline2-comparability-report.json') || {}).status || '' : undefined,
  previousMismatchField: fingerprintVersion === 2 ? 'loadProfileFingerprint' : undefined,
  previousFingerprintVersion: fingerprintVersion === 2 ? 1 : undefined,
  currentFingerprintVersion: fingerprintVersion,
  loadProfileFingerprintMatch: fingerprintVersion >= 2 ? baselineValues[versionedFingerprintKey] === currentValues[versionedFingerprintKey] : baselineValues.loadProfileFingerprint === currentValues.loadProfileFingerprint,
  checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  issues: [...(resolvedBaseline.valid ? [] : resolvedBaseline.issues), ...failed],
};
const output = fingerprintVersion === 3
  ? ['docs/p7-v2-r3b-lpc-r3-comparability-report.json', 'docs/P7_V2_R3B_LPC_R3_COMPARABILITY_REPORT.md', 'P7-V2-R3B-LPC-R3 Comparability Report']
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
