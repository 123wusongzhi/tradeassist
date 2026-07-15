import { readJSON } from './p7-v2-lib.mjs';
import { writeR3Report } from './p7-v2-r3-lib.mjs';
import { resolveActiveBaseline, resolveActiveCurrent } from './p7-v2-evidence-resolver.mjs';

const args = process.argv.slice(2);
const fingerprintVersion = Number(args[args.indexOf('--fingerprint-version') + 1] || 1);
if (![1, 2].includes(fingerprintVersion)) throw new Error('fingerprint version must be 1 or 2');
const resolvedBaseline = resolveActiveBaseline();
const resolvedCurrent = resolveActiveCurrent();
const baseline = resolvedBaseline.baseline || {};
const current = resolvedCurrent.entry || {};
const baselineManifest = readJSON(`docs/baselines/frozen/${baseline.runId || ''}/manifest.json`) || {};
const currentManifest = resolvedCurrent.manifest || {};
const sidecarRoot = 'docs/fingerprints/p7-v2/load-profile/v2';
const baselineV2 = fingerprintVersion === 2 ? readJSON(`${sidecarRoot}/${baseline.runId || ''}.json`) || {} : {};
const currentV2 = fingerprintVersion === 2 ? readJSON(`${sidecarRoot}/${current.runId || ''}.json`) || {} : {};
const comparableKeys = [
  'runtimeSourceTreeHash',
  'loadScriptsHash',
  'metricSemanticsHash',
  'datasetFingerprint',
  'configFingerprint',
  fingerprintVersion === 2 ? 'loadProfileFingerprintV2' : 'loadProfileFingerprint',
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
function values(manifest, entry, v2) {
  const environment = manifest.environmentFingerprint || entry.environmentFingerprint || {};
  return {
    runtimeSourceTreeHash: manifest.runtimeSourceTreeHash || '',
    loadScriptsHash: manifest.loadScriptsHash || '',
    metricSemanticsHash: manifest.metricSemanticsHash || '',
    datasetFingerprint: manifest.datasetFingerprint || '',
    configFingerprint: manifest.configFingerprint || '',
    loadProfileFingerprint: manifest.loadProfileFingerprint || '',
    loadProfileFingerprintV2: v2.loadProfileFingerprint || '',
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
const baselineValues = values(baselineManifest, baseline, baselineV2);
const currentValues = values(currentManifest, current, currentV2);
const v2ProfileChecks = fingerprintVersion === 2 ? [
  ['baseline-v2-sidecar', baselineV2.fingerprintVersion === 2 && baselineV2.derivedEvidence === true && baselineV2.sourceArtifactsModified === false],
  ['current-v2-sidecar', currentV2.fingerprintVersion === 2 && currentV2.derivedEvidence === true && currentV2.sourceArtifactsModified === false],
  ['baseline-canonical-profile', Array.isArray(baselineV2.canonicalProfile?.load?.stages) && baselineV2.canonicalProfile.load.configuredVUs > 0 && Array.isArray(baselineV2.canonicalProfile?.scenarios) && baselineV2.canonicalProfile.scenarios.length > 0 && Boolean(baselineV2.canonicalProfile?.loadScript?.sha256)],
  ['current-canonical-profile', Array.isArray(currentV2.canonicalProfile?.load?.stages) && currentV2.canonicalProfile.load.configuredVUs > 0 && Array.isArray(currentV2.canonicalProfile?.scenarios) && currentV2.canonicalProfile.scenarios.length > 0 && Boolean(currentV2.canonicalProfile?.loadScript?.sha256)],
  ['v2-fingerprint-shape', /^[a-f0-9]{64}$/.test(baselineV2.loadProfileFingerprint || '') && /^[a-f0-9]{64}$/.test(currentV2.loadProfileFingerprint || '')],
] : [];
const checks = [
  ['baseline-registry', resolvedBaseline.valid],
  ['current-registry', resolvedCurrent.valid],
  ['baseline-manifest', baselineManifest.immutable === true && baselineManifest.validForRegression === true],
  ['current-manifest', currentManifest.immutable === true && currentManifest.validForRegression === true],
  ['different-run-id', baseline.runId && current.runId && baseline.runId !== current.runId],
  ['different-artifact', resolvedBaseline.baseline?.rawArtifactSha256 && resolvedCurrent.actualHash && resolvedBaseline.baseline.rawArtifactSha256 !== resolvedCurrent.actualHash],
  ['current-independent', current.independentRun === true && current.baselineRunId === baseline.runId],
  ...v2ProfileChecks,
  ...comparableKeys.map((key) => [key, Boolean(baselineValues[key]) && baselineValues[key] === currentValues[key]]),
];
const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
const report = {
  phase: fingerprintVersion === 2 ? 'P7-V2-R3B-LPF-V2' : 'P7-V2-R3B-CI-RG',
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
  loadProfileFingerprintMatch: fingerprintVersion === 2 ? baselineValues.loadProfileFingerprintV2 === currentValues.loadProfileFingerprintV2 : baselineValues.loadProfileFingerprint === currentValues.loadProfileFingerprint,
  checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  issues: [...(resolvedBaseline.valid ? [] : resolvedBaseline.issues), ...failed],
};
const output = fingerprintVersion === 2
  ? ['docs/p7-v2-r3b-lpf-comparability-v2-report.json', 'docs/P7_V2_R3B_LPF_COMPARABILITY_V2_REPORT.md', 'P7-V2-R3B-LPF-V2 Comparability Report']
  : ['docs/p7-v2-r3b-rebaseline2-comparability-report.json', 'docs/P7_V2_R3B_REBASELINE2_COMPARABILITY_REPORT.md', 'P7-V2-R3B-REBASELINE2 Comparability Report'];
writeR3Report(
  ...output,
  report,
  [['Baseline', report.baselineRunId], ['Current', report.currentRunId], ['Status', report.status], ['Mismatch count', report.mismatchCount]],
);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
