import { calculateLoadProfileFingerprint, InvalidLoadProfileError } from './p7-v2-load-profile-fingerprint.mjs';
import { writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const scriptHash = 'a'.repeat(64);
const profile = {
  configuredVUs: 10,
  stages: [
    { name: 'warmup', duration: '5m', targetVUs: 10 },
    { name: 'ramp', duration: '3m', targetVUs: 10 },
    { name: 'steady', duration: '10m', targetVUs: 10 },
    { name: 'rampdown', duration: '2m', targetVUs: 0 },
  ],
  scenarios: [{ name: 'steady', executor: 'constant-vus', startTime: '0s', gracefulStop: '0s', weight: 1 }],
  requestMix: [{ routeId: 'task_list', method: 'GET', weight: 1 }],
  credentialMix: [{ role: 'system_admin', weight: 1 }],
  loadScript: { path: 'tests/load/p7v2-baseline.js', sha256: scriptHash },
};
const fingerprint = (value) => calculateLoadProfileFingerprint(value, { repositoryRoot: process.cwd() }).loadProfileFingerprint;
const original = fingerprint(profile);
const rejects = (value) => {
  try { fingerprint(value); return false; } catch (error) { return error instanceof InvalidLoadProfileError; }
};
const iterations = 20;
const hashes = new Set(Array.from({ length: iterations }, () => fingerprint(profile)));
const report = {
  phase: 'P7-V2-R3B-LPC-R3-GATEFIX', status: 'passed', canonicalSchemaVersion: 3, fingerprintVersion: 3,
  iterations, uniqueFingerprintCount: hashes.size,
  sameProfileDifferentRunKindSameHash: original === fingerprint({ ...profile, kind: 'current' }),
  sameProfileDifferentRunIdSameHash: original === fingerprint({ ...profile, runId: 'different-run' }),
  sameProfileDifferentTimestampSameHash: original === fingerprint({ ...profile, createdAt: '2026-07-15T00:00:00.000Z' }),
  sameProfileDifferentAbsolutePathSameHash: original === fingerprint({ ...profile, loadScript: { ...profile.loadScript, path: `/mnt/d/project/trademind-ai/${profile.loadScript.path}` } }),
  stageOrderChangedDifferentHash: original !== fingerprint({ ...profile, stages: [profile.stages[1], profile.stages[0], ...profile.stages.slice(2)] }),
  stageDurationChangedDifferentHash: original !== fingerprint({ ...profile, stages: profile.stages.map((stage, index) => index === 1 ? { ...stage, duration: '4m' } : stage) }),
  stageTargetChangedDifferentHash: original !== fingerprint({ ...profile, stages: profile.stages.map((stage, index) => index === 1 ? { ...stage, targetVUs: 9 } : stage) }),
  configuredVUsChangedDifferentHash: original !== fingerprint({ ...profile, configuredVUs: 9 }),
  scenarioExecutorChangedDifferentHash: original !== fingerprint({ ...profile, scenarios: [{ ...profile.scenarios[0], executor: 'ramping-vus' }] }),
  scenarioWeightChangedDifferentHash: original !== fingerprint({ ...profile, scenarios: [{ ...profile.scenarios[0], weight: 2 }] }),
  loadScriptChangedDifferentHash: original !== fingerprint({ ...profile, loadScript: { ...profile.loadScript, sha256: 'b'.repeat(64) } }),
  emptyProfileRejected: rejects({}),
  emptyStagesRejected: rejects({ ...profile, stages: [] }),
  missingConfiguredVUsRejected: rejects({ ...profile, configuredVUs: undefined, targetVUs: undefined }),
  invalidStageRejected: rejects({ ...profile, stages: [{ name: 'steady', duration: '1m', targetVUs: null }] }),
  loadScriptHashMissingRejected: rejects({ ...profile, loadScript: { path: profile.loadScript.path } }),
  crossPlatform: { windowsExecuted: true, wslExecuted: false, fingerprintsMatch: null, status: 'not_executed' },
  issues: [],
};
const required = ['sameProfileDifferentRunKindSameHash', 'sameProfileDifferentRunIdSameHash', 'sameProfileDifferentTimestampSameHash', 'sameProfileDifferentAbsolutePathSameHash', 'stageOrderChangedDifferentHash', 'stageDurationChangedDifferentHash', 'stageTargetChangedDifferentHash', 'configuredVUsChangedDifferentHash', 'scenarioExecutorChangedDifferentHash', 'scenarioWeightChangedDifferentHash', 'loadScriptChangedDifferentHash', 'emptyProfileRejected', 'emptyStagesRejected', 'missingConfiguredVUsRejected', 'invalidStageRejected', 'loadScriptHashMissingRejected'];
if (hashes.size !== 1 || required.some((key) => !report[key])) { report.status = 'failed'; report.issues.push('determinism expectation failed'); }
writeJSON('docs/p7-v2-r3b-lpc-r3-determinism-report.json', report);
writeMarkdown('docs/P7_V2_R3B_LPC_R3_DETERMINISM_REPORT.md', `# P7-V2-R3B LPC-R3 Determinism Report\n\nStatus: **${report.status}**\n\n- Iterations: ${iterations}\n- Unique fingerprints: ${hashes.size}\n- Cross-platform verification: not_executed (WSL not executed)\n`);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
