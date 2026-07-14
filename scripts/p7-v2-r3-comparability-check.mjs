import fs from 'node:fs';
import path from 'node:path';
import { collectEnvironmentFingerprint, readJSON, root, valueOf } from './p7-v2-lib.mjs';
import { jsonHash, runtimeSourceFingerprint, trackedDiffHash, untrackedRuntimeManifest, writeR3Report } from './p7-v2-r3-lib.mjs';

const args = process.argv.slice(2);
const baselinePath = valueOf(args, '--baseline') || 'docs/p7-v2-r3-baseline-report.json';
const baseline = readJSON(baselinePath);
const dataset = readJSON('docs/p7-v2-dataset-report.json');
const routeMatrix = readJSON('docs/p7-v2-r2-route-credential-matrix.json');
const runtime = readJSON('docs/p7-v2-runtime-environment.json');
const source = runtimeSourceFingerprint();
const diff = trackedDiffHash();
const untracked = untrackedRuntimeManifest();
const current = {
  runtimeSourceTreeHash: source.hash,
  trackedDiffHash: diff.hash,
  untrackedRuntimeManifestHash: untracked.hash,
  datasetFingerprint: dataset?.datasetFingerprint || '',
  configFingerprint: runtime?.environmentFingerprint?.configFingerprint || '',
  loadProfileFingerprint: baseline?.loadProfileFingerprint || '',
  sloFingerprint: jsonHash(fs.readFileSync(path.join(root, 'docs/SLO.md'), 'utf8')),
  routeCredentialMatrixFingerprint: jsonHash(routeMatrix || {}),
  environment: collectEnvironmentFingerprint('r3-comparability', `p7v2-r3-comparability-${Date.now()}`),
};
const checks = [
  ['baseline-passed', baseline?.status === 'passed'],
  ['baseline-has-traffic', Number(baseline?.completedRequests || 0) > 0],
  ['baseline-immutable', baseline?.immutable === true || baselinePath.includes('baselines/')],
  ['runtime-source-tree', baseline?.runtimeSourceTreeHash === current.runtimeSourceTreeHash],
  ['dataset', baseline?.datasetFingerprint === current.datasetFingerprint],
  ['config', baseline?.configFingerprint === current.configFingerprint],
  ['load-profile', Boolean(baseline?.loadProfileFingerprint)],
  ['slo', baseline?.sloFingerprint === current.sloFingerprint],
  ['route-credential-matrix', baseline?.routeCredentialMatrixFingerprint === current.routeCredentialMatrixFingerprint],
  ['k6-version', baseline?.environmentFingerprint?.k6Version === current.environment.k6Version],
  ['postgres-version', baseline?.environmentFingerprint?.postgresVersion === current.environment.postgresVersion],
  ['redis-version', baseline?.environmentFingerprint?.redisVersion === current.environment.redisVersion],
  ['go-version', baseline?.environmentFingerprint?.goVersion === current.environment.goVersion],
];
const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
const report = {
  phase: 'P7-V2-R3',
  status: failed.length ? 'not_comparable' : 'passed',
  baselinePath,
  baselineRunId: baseline?.runId || '',
  current,
  checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  issues: failed,
};
writeR3Report(
  'docs/p7-v2-r3-comparability-report.json',
  'docs/P7_V2_R3_COMPARABILITY_REPORT.md',
  'P7-V2-R3 Comparability Report',
  report,
  [['Baseline', report.baselineRunId], ['Status', report.status], ['Runtime source tree', source.hash], ['Dataset', current.datasetFingerprint]],
);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
