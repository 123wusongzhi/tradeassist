import fs from 'node:fs';
import path from 'node:path';
import { readJSON, root } from './p7-v2-lib.mjs';
import { jsonHash, runtimeSourceFingerprint, trackedDiffHash, untrackedRuntimeManifest, writeR3Report } from './p7-v2-r3-lib.mjs';

const baselineRunId = 'p7v2-baseline-20260714181000';
const baselineReportPath = 'docs/baselines/p7-v2-baseline-p7v2-baseline-20260714181000.json';
const baseline = readJSON(baselineReportPath);
const preflight = readJSON('docs/p7-v2-r3-preflight-audit.json');
const currentRuntime = preflight?.currentRuntime || {};
const runtime = runtimeSourceFingerprint();
const diff = trackedDiffHash();
const untracked = untrackedRuntimeManifest();
const rawPath = path.join(root, 'artifacts', 'p7-v2', 'baseline', baselineRunId, 'baseline.summary.json');
const derivationEvidence = [
  baselineReportPath,
  'docs/p7-v2-r2-closure-report.json',
  'docs/P7_V2_R2_FORMAL_BASELINE_REPORT.md',
  'docs/p7-v2-r3-preflight-audit.json',
].filter((rel) => fs.existsSync(path.join(root, rel)));
const missing = !baseline || !preflight?.baselineComparable;
const report = {
  phase: 'P7-V2-R3',
  status: missing ? 'not_comparable' : 'passed',
  baselineRunId,
  baselineReportPath,
  rawK6SummaryPath: fs.existsSync(rawPath) ? path.relative(root, rawPath).replaceAll('\\', '/') : '',
  gitCommit: baseline?.environmentFingerprint?.gitCommit || '',
  gitDirty: Boolean(baseline?.environmentFingerprint?.gitDirty),
  trackedDiffHash: diff.hash,
  untrackedRuntimeManifestHash: untracked.hash,
  runtimeSourceTreeHash: runtime.hash,
  apiSourceHash: jsonHash(runtime.files.filter((f) => f.path.startsWith('backend/'))),
  loadScriptHash: jsonHash(runtime.files.filter((f) => f.path.startsWith('tests/load/'))),
  loadProfileFingerprint: baseline?.loadProfileFingerprint || '',
  datasetFingerprint: baseline?.datasetFingerprint || '',
  configFingerprint: baseline?.configFingerprint || '',
  sloFingerprint: currentRuntime.sloFingerprint || '',
  routeCredentialMatrixFingerprint: currentRuntime.routeCredentialMatrixFingerprint || '',
  k6Version: baseline?.environmentFingerprint?.k6Version || '',
  postgresVersion: baseline?.environmentFingerprint?.postgresVersion || '',
  redisVersion: baseline?.environmentFingerprint?.redisVersion || '',
  hostFingerprint: jsonHash({
    os: baseline?.environmentFingerprint?.os,
    kernel: baseline?.environmentFingerprint?.kernel,
    cpuModel: baseline?.environmentFingerprint?.cpuModel,
    cpuCores: baseline?.environmentFingerprint?.cpuCores,
    memoryTotal: baseline?.environmentFingerprint?.memoryTotal,
  }),
  createdAt: new Date().toISOString(),
  immutable: true,
  derivedAfterBaseline: true,
  derivationEvidence,
  baselineComparable: !missing,
  issues: missing ? ['historical baseline cannot be proven comparable; a fresh R3 baseline is required'] : [],
};
writeR3Report(
  'docs/p7-v2-r3-baseline-freeze-report.json',
  'docs/P7_V2_R3_BASELINE_FREEZE_REPORT.md',
  'P7-V2-R3 Baseline Freeze Report',
  report,
  [
    ['Baseline run ID', report.baselineRunId],
    ['Immutable', report.immutable],
    ['Comparable', report.baselineComparable],
    ['Runtime source tree hash', report.runtimeSourceTreeHash],
    ['Derived after baseline', report.derivedAfterBaseline],
  ],
);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
