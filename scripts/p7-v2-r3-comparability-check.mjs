import { readJSON } from './p7-v2-lib.mjs';
import { writeR3Report } from './p7-v2-r3-lib.mjs';
import { resolveActiveBaseline, resolveActiveCurrent } from './p7-v2-evidence-resolver.mjs';

const resolvedBaseline = resolveActiveBaseline();
const resolvedCurrent = resolveActiveCurrent();
const baseline = resolvedBaseline.baseline || {};
const current = resolvedCurrent.entry || {};
const baselineManifest = readJSON(`docs/baselines/frozen/${baseline.runId || ''}/manifest.json`) || {};
const currentManifest = resolvedCurrent.manifest || {};
const comparableKeys = [
  'runtimeSourceTreeHash',
  'loadScriptsHash',
  'metricSemanticsHash',
  'datasetFingerprint',
  'configFingerprint',
  'loadProfileFingerprint',
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
function values(manifest, entry) {
  const environment = manifest.environmentFingerprint || entry.environmentFingerprint || {};
  return {
    runtimeSourceTreeHash: manifest.runtimeSourceTreeHash || '',
    loadScriptsHash: manifest.loadScriptsHash || '',
    metricSemanticsHash: manifest.metricSemanticsHash || '',
    datasetFingerprint: manifest.datasetFingerprint || '',
    configFingerprint: manifest.configFingerprint || '',
    loadProfileFingerprint: manifest.loadProfileFingerprint || '',
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
const baselineValues = values(baselineManifest, baseline);
const currentValues = values(currentManifest, current);
const checks = [
  ['baseline-registry', resolvedBaseline.valid],
  ['current-registry', resolvedCurrent.valid],
  ['baseline-manifest', baselineManifest.immutable === true && baselineManifest.validForRegression === true],
  ['current-manifest', currentManifest.immutable === true && currentManifest.validForRegression === true],
  ['different-run-id', baseline.runId && current.runId && baseline.runId !== current.runId],
  ['different-artifact', resolvedBaseline.baseline?.rawArtifactSha256 && resolvedCurrent.actualHash && resolvedBaseline.baseline.rawArtifactSha256 !== resolvedCurrent.actualHash],
  ['current-independent', current.independentRun === true && current.baselineRunId === baseline.runId],
  ...comparableKeys.map((key) => [key, Boolean(baselineValues[key]) && baselineValues[key] === currentValues[key]]),
];
const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
const report = {
  phase: 'P7-V2-R3B-CI-RG',
  status: failed.length ? 'not_comparable' : 'passed',
  baselineRunId: baseline.runId || '',
  currentRunId: current.runId || '',
  baseline: baselineValues,
  current: currentValues,
  mismatchCount: failed.filter((item) => comparableKeys.includes(item)).length,
  notComparableCount: failed.filter((item) => !comparableKeys.includes(item)).length,
  checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  issues: [...(resolvedBaseline.valid ? [] : resolvedBaseline.issues), ...failed],
};
writeR3Report(
  'docs/p7-v2-r3b-rebaseline2-comparability-report.json',
  'docs/P7_V2_R3B_REBASELINE2_COMPARABILITY_REPORT.md',
  'P7-V2-R3B-REBASELINE2 Comparability Report',
  report,
  [['Baseline', report.baselineRunId], ['Current', report.currentRunId], ['Status', report.status], ['Mismatch count', report.mismatchCount]],
);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
