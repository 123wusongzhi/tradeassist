import { calculateLoadProfileFingerprint, InvalidLoadProfileError } from './p7-v2-load-profile-fingerprint.mjs';
import { writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const base = {
  configuredVUs: 10,
  stages: [{ name: 'steady', duration: '10m', targetVUs: 10 }],
  scenarios: [{ name: 'steady', executor: 'constant-vus', startTime: '0s', gracefulStop: '0s' }],
  requestMix: [{ routeId: 'task_list', method: 'GET', weight: 1 }],
  credentialMix: [{ role: 'system_admin', weight: 1 }],
  loadScript: { path: 'tests/load/p7v2-baseline.js', sha256: 'a'.repeat(64) },
};
const calculate = (value) => calculateLoadProfileFingerprint(value, { repositoryRoot: process.cwd() });
const rejects = (stage) => {
  try { calculate({ ...base, stages: [stage] }); return false; } catch (error) { return error instanceof InvalidLoadProfileError; }
};
const result = calculate(base);
const report = {
  phase: 'P7-V2-R3B-LPC-R3-GATEFIX', status: 'passed',
  canonicalLoadProfile: {
    canonicalSchemaVersion: result.canonicalProfile.schemaVersion, loadProfileFingerprintVersion: result.fingerprintVersion,
    configuredVUs: result.canonicalProfile.load.configuredVUs, stageCount: result.canonicalProfile.load.stages.length,
    allStagesValid: result.canonicalProfile.load.stages.every((stage) => stage.durationMs > 0 && Number.isInteger(stage.targetVUs) && stage.targetVUs >= 0),
    runKindExcluded: result.loadProfileFingerprint === calculate({ ...base, kind: 'current' }).loadProfileFingerprint,
    runIdExcluded: result.loadProfileFingerprint === calculate({ ...base, runId: 'another-run' }).loadProfileFingerprint,
    emptyProfileRejected: rejects({ name: 'steady', duration: '10m' }),
  },
  stageSemantics: {
    targetAliasSupported: calculate({ ...base, stages: [{ name: 'steady', duration: '10m', target: 10 }] }).canonicalProfile.load.stages[0].targetVUs === 10,
    targetVUsSupported: result.canonicalProfile.load.stages[0].targetVUs === 10, legalZeroSupported: calculate({ ...base, stages: [{ name: 'rampdown', duration: '1m', targetVUs: 0 }] }).canonicalProfile.load.stages[0].targetVUs === 0,
    missingTargetRejected: rejects({ name: 'steady', duration: '1m' }), nullTargetRejected: rejects({ name: 'steady', duration: '1m', targetVUs: null }),
    nonFiniteTargetRejected: rejects({ name: 'steady', duration: '1m', targetVUs: Infinity }), negativeTargetRejected: rejects({ name: 'steady', duration: '1m', targetVUs: -1 }),
    fractionalTargetRejected: rejects({ name: 'steady', duration: '1m', targetVUs: 1.5 }), conflictingTargetFieldsRejected: rejects({ name: 'steady', duration: '1m', target: 1, targetVUs: 2 }),
  },
  execution: { runtimeFreezeCreated: false, environmentStarted: false, datasetExecuted: false, k6Executed: false, registryActiveEntryModified: false },
  issues: [],
};
const values = [...Object.values(report.canonicalLoadProfile), ...Object.values(report.stageSemantics)];
if (values.some((value) => value === false) || report.canonicalLoadProfile.stageCount <= 0) { report.status = 'failed'; report.issues.push('canonical profile preflight expectation failed'); }
writeJSON('docs/p7-v2-r3b-lpc-r3-preflight-audit.json', report);
writeMarkdown('docs/P7_V2_R3B_LPC_R3_PREFLIGHT_AUDIT.md', `# P7-V2-R3B LPC-R3 Preflight Audit\n\nStatus: **${report.status}**\n\n- Canonical schema: ${report.canonicalLoadProfile.canonicalSchemaVersion}\n- Fingerprint version: ${report.canonicalLoadProfile.loadProfileFingerprintVersion}\n- Stages: ${report.canonicalLoadProfile.stageCount}\n`);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
