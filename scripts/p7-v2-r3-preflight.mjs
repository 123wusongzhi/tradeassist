import fs from 'node:fs';
import path from 'node:path';
import { collectEnvironmentFingerprint, discoverK6, readJSON, root } from './p7-v2-lib.mjs';
import { jsonHash, runtimeSourceFingerprint, trackedDiffHash, untrackedRuntimeManifest, writeR3Report } from './p7-v2-r3-lib.mjs';

const baselineRunId = 'p7v2-baseline-20260714181000';
const baselinePath = 'docs/baselines/p7-v2-baseline-p7v2-baseline-20260714181000.json';
const baseline = readJSON(baselinePath);
const routeMatrix = readJSON('docs/p7-v2-r2-route-credential-matrix.json');
const dataset = readJSON('docs/p7-v2-dataset-report.json');
const historicalSummary = path.join(root, 'artifacts', 'p7-v2', 'baseline', baselineRunId, 'baseline.summary.json');
const k6 = discoverK6();
const runtimeSource = runtimeSourceFingerprint();
const trackedDiff = trackedDiffHash();
const untracked = untrackedRuntimeManifest();
const missingHistoricalFingerprints = [
  'trackedDiffHash',
  'untrackedRuntimeManifestHash',
  'runtimeSourceTreeHash',
  'apiSourceHash',
  'loadScriptHash',
  'sloFingerprint',
  'routeCredentialMatrixFingerprint',
].filter((key) => !baseline?.[key] && !baseline?.environmentFingerprint?.[key]);
const hasRawSummary = fs.existsSync(historicalSummary);
const rawSummary = hasRawSummary ? JSON.parse(fs.readFileSync(historicalSummary, 'utf8')) : null;
const rawTraffic = Number((rawSummary?.metrics?.http_reqs?.values || rawSummary?.metrics?.http_reqs || {}).count || 0);
const issues = [];
if (!baseline || baseline.status !== 'passed') issues.push('historical baseline is missing or not passed');
if (!k6.executable || !String(k6.version).includes('v0.57.0')) issues.push('required k6 v0.57.0 is unavailable');
if (!dataset || dataset.actualRows !== 1900150 || dataset.duplicateRows !== 0 || dataset.failedRows !== 0) issues.push('medium dataset evidence is incomplete');
if (!routeMatrix?.routes?.length) issues.push('route credential matrix is missing');
if (missingHistoricalFingerprints.length) issues.push(`historical baseline lacks immutable fingerprints: ${missingHistoricalFingerprints.join(', ')}`);
if (!hasRawSummary) issues.push('historical baseline raw k6 summary is missing');
if (rawTraffic <= 0) issues.push('historical baseline does not prove non-zero k6 traffic');

const report = {
  phase: 'P7-V2-R3',
  component: 'preflight-audit',
  status: k6.executable && dataset?.actualRows === 1900150 ? 'passed' : 'failed',
  baselineExists: Boolean(baseline),
  baselineStatus: baseline?.status || 'missing',
  baselineRunId,
  baselineImmutable: true,
  baselineComparable: issues.length === 0,
  baselineRebuildRequired: issues.length > 0,
  rawK6SummaryPath: hasRawSummary ? path.relative(root, historicalSummary).replaceAll('\\', '/') : '',
  historicalRawTraffic: rawTraffic,
  k6Available: k6.executable === true,
  k6Version: k6.version || '',
  currentHarnessAvailable: fs.existsSync(path.join(root, 'scripts/p7-v2-current.mjs')),
  regressionHarnessAvailable: fs.existsSync(path.join(root, 'scripts/p7-v2-performance-regression.mjs')),
  soakHarnessAvailable: fs.existsSync(path.join(root, 'scripts/p7-v2-soak.mjs')),
  demoHarnessAvailable: fs.existsSync(path.join(root, 'scripts/p7-v2-demo-acceptance.mjs')),
  finalGatesAvailable: fs.existsSync(path.join(root, 'scripts/p1-p7-final-gate.mjs')) && fs.existsSync(path.join(root, 'scripts/p7-v2-final-closure-gate.mjs')),
  currentRuntime: {
    trackedDiffHash: trackedDiff.hash,
    untrackedRuntimeManifestHash: untracked.hash,
    runtimeSourceTreeHash: runtimeSource.hash,
    runtimeSourceFileCount: runtimeSource.fileCount,
    datasetFingerprint: dataset?.datasetFingerprint || '',
    sloFingerprint: jsonHash(fs.readFileSync(path.join(root, 'docs/SLO.md'), 'utf8')),
    routeCredentialMatrixFingerprint: jsonHash(routeMatrix || {}),
  },
  environmentFingerprint: collectEnvironmentFingerprint('r3-preflight', `p7v2-r3-preflight-${Date.now()}`),
  existingUntrackedExcluded: ['.agents/', 'skills-lock.json'],
  issues,
};

writeR3Report(
  'docs/p7-v2-r3-preflight-audit.json',
  'docs/P7_V2_R3_PREFLIGHT_AUDIT.md',
  'P7-V2-R3 Preflight Audit',
  report,
  [
    ['Baseline run ID', baselineRunId],
    ['Historical baseline comparable', report.baselineComparable],
    ['k6', report.k6Version],
    ['Runtime source hash', runtimeSource.hash],
    ['Dataset fingerprint', report.currentRuntime.datasetFingerprint],
  ],
);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
