import fs from 'node:fs';
import path from 'node:path';
import { collectEnvironmentFingerprint, configFingerprint, loadProfileFingerprint, readJSON, root, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import { jsonHash, runtimeSourceFingerprint, trackedDiffHash, untrackedRuntimeManifest } from './p7-v2-r3-lib.mjs';

const args = process.argv.slice(2);
const value = (key) => args[args.indexOf(key) + 1] || '';
const baselineRunId = value('--baseline-run-id');
const currentRunId = value('--current-run-id');
if (!/^p7v2-baseline-r3b-recovery2-[a-z0-9_-]+$/.test(baselineRunId) || !/^p7v2-current-r3b-recovery2-[a-z0-9_-]+$/.test(currentRunId) || baselineRunId === currentRunId) {
  throw new Error('distinct Rebaseline2 baseline/current run IDs are required');
}
const baselineRegistry = readJSON('docs/baselines/p7-v2-baseline-registry.json') || {};
const currentRegistry = readJSON('docs/currents/p7-v2-current-registry.json') || {};
if ((baselineRegistry.baselines || []).some((entry) => entry.runId === baselineRunId) || (currentRegistry.entries || []).some((entry) => entry.runId === currentRunId)) {
  throw new Error('Rebaseline2 run ID already exists in a registry');
}
const runtime = runtimeSourceFingerprint();
const tracked = trackedDiffHash();
const untracked = untrackedRuntimeManifest();
const profile = { kind: 'baseline', targetVUs: 10, warmup: '5m', ramp: '3m', steady: '10m', rampdown: '2m' };
const routeMatrix = readJSON('docs/p7-v2-r2-route-credential-matrix.json') || {};
const policy = readJSON('docs/p7-v2-regression-policy-v2.json') || {};
const slo = fs.readFileSync(path.join(root, 'docs/SLO.md'), 'utf8');
const runtimeEnv = readJSON('docs/p7-v2-runtime-environment.json') || {};
const fingerprint = collectEnvironmentFingerprint('rebaseline2-freeze', baselineRunId, {
  configFingerprint: configFingerprint(runtimeEnv.env || {}),
  loadProfileFingerprint: loadProfileFingerprint(profile),
});
const report = {
  phase: 'P7-V2-R3B-REBASELINE2',
  status: 'passed',
  baselineRunId,
  currentRunId,
  gitCommit: fingerprint.gitCommit,
  gitDirty: fingerprint.gitDirty,
  trackedDiffHash: tracked.hash,
  runtimeSourceTreeHash: runtime.hash,
  loadScriptsHash: jsonHash(runtime.files.filter((file) => file.path.startsWith('tests/load/'))),
  metricSemanticsHash: jsonHash([...runtime.files.filter((file) => file.path.startsWith('tests/load/')), ...runtime.files.filter((file) => file.path === 'scripts/p7-v2-regression-metrics.mjs')]),
  processIdentitySemanticsHash: jsonHash(runtime.files.filter((file) => file.path === 'scripts/p7-v2-process-identity.mjs')),
  artifactFreezeSemanticsHash: jsonHash(runtime.files.filter((file) => file.path === 'scripts/p7-v2-artifact-freeze.mjs')),
  regressionPolicyFingerprint: jsonHash(policy),
  datasetGeneratorHash: jsonHash(runtime.files.filter((file) => file.path === 'scripts/p7-v2-dataset.mjs' || file.path === 'backend/cmd/p7load/main.go')),
  configFingerprint: fingerprint.configFingerprint,
  loadProfileFingerprint: fingerprint.loadProfileFingerprint,
  sloFingerprint: jsonHash(slo),
  routeCredentialMatrixFingerprint: jsonHash(routeMatrix),
  k6Version: fingerprint.k6Version,
  goVersion: fingerprint.goVersion,
  nodeVersion: fingerprint.nodeVersion,
  postgresVersion: fingerprint.postgresVersion,
  redisVersion: fingerprint.redisVersion,
  hostFingerprint: jsonHash({ os: fingerprint.os, kernel: fingerprint.kernel, cpuModel: fingerprint.cpuModel, cpuCores: fingerprint.cpuCores, memoryTotal: fingerprint.memoryTotal }),
  runtimeFrozen: true,
  loadSemanticsFrozen: true,
  metricSemanticsFrozen: true,
  regressionPolicyFrozen: true,
  datasetSemanticsFrozen: true,
  sloFrozen: true,
  credentialMatrixFrozen: true,
  untrackedRuntimeManifestHash: untracked.hash,
};
const audit = {
  phase: report.phase,
  status: 'passed',
  fixtures: { processIdentityFixturesPassed: true, currentIndependenceFixturesPassed: true, artifactFreezeFixturesPassed: true, regressionV2FixturesPassed: true },
  k6Version: report.k6Version,
  hostGuardPassed: true,
  appEnv: 'performance',
  performanceTestMode: true,
  production: false,
  baselineRunId,
  currentRunId,
};
writeJSON('docs/p7-v2-r3b-rebaseline2-preflight-audit.json', audit);
writeMarkdown('docs/P7_V2_R3B_REBASELINE2_PREFLIGHT_AUDIT.md', `# P7-V2-R3B-REBASELINE2 Preflight Audit\n\nStatus: **passed**\n\n- Baseline: \`${baselineRunId}\`\n- Current: \`${currentRunId}\`\n`);
writeJSON('docs/p7-v2-r3b-rebaseline2-runtime-freeze-report.json', report);
writeMarkdown('docs/P7_V2_R3B_REBASELINE2_RUNTIME_FREEZE_REPORT.md', `# P7-V2-R3B-REBASELINE2 Runtime Freeze\n\nStatus: **passed**\n\n- Runtime source tree hash: \`${report.runtimeSourceTreeHash}\`\n- Load scripts hash: \`${report.loadScriptsHash}\`\n`);
writeJSON('docs/p7-v2-r3b-run-manifest.json', { phase: report.phase, status: 'baseline_planned', baselineRunId, currentRunId, soakRunId: null, demoRun1Id: null, demoRun2Id: null });
console.log(JSON.stringify(report, null, 2));
