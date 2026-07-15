import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { calculateLoadProfileFingerprint } from './p7-v2-load-profile-fingerprint.mjs';
import { jsonHash, runtimeSourceFingerprint, trackedDiffHash } from './p7-v2-r3-lib.mjs';
import { readJSON, root, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import { updateR3BManifest } from './p7-v2-r3b-manifest.mjs';

export const RUNTIME_FREEZE_PATH = 'docs/p7-v2-r3b-lpc-r3-runtime-freeze.json';
const RECOVERY6_RUN_ID = /^p7v2-(baseline|current|soak|demo[12])-r3b-recovery6-[a-z0-9_-]+$/;
const hash = (value) => crypto.createHash('sha256').update(JSON.stringify(value)).digest('hex');

export function validateRuntimeFreezeContract(contract, { kind, runId } = {}) {
  if (!contract || contract.phase !== 'P7-V2-R3B-LPC-R3' || contract.status !== 'passed') return { valid: false, issue: 'missing_runtime_freeze_contract' };
  if (contract.canonicalLoadProfileVersion !== 3 || contract.loadProfileFingerprintVersion !== 3) return { valid: false, issue: 'invalid_runtime_freeze_version' };
  if (!/^[a-f0-9]{64}$/.test(contract.contractId || '') || !/^[a-f0-9]{64}$/.test(contract.loadProfileFingerprint || '')) return { valid: false, issue: 'invalid_runtime_freeze_fingerprint' };
  if (kind && runId && contract.runIds?.[`${kind}RunId`] !== runId) return { valid: false, issue: 'runtime_freeze_run_id_mismatch' };
  if (kind && runId && !RECOVERY6_RUN_ID.test(runId)) return { valid: false, issue: 'invalid_recovery6_run_id' };
  return { valid: true, issue: '' };
}

export function readRuntimeFreezeContract() {
  return readJSON(RUNTIME_FREEZE_PATH) || {};
}

export function buildRuntimeFreezeContract({ manifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {}, now = new Date().toISOString() } = {}) {
  const runIds = {
    baselineRunId: manifest.baselineRunId || '',
    currentRunId: manifest.currentRunId || '',
    soakRunId: manifest.soakRunId || '',
    demoRun1Id: manifest.demoRun1Id || '',
    demoRun2Id: manifest.demoRun2Id || '',
  };
  if (manifest.phase !== 'P7-V2-R3B-FAST-CLOSE-R3' || manifest.status !== 'planned' || manifest.executionStarted !== false || !manifest.runIdsUnique || Object.values(runIds).some((runId) => !RECOVERY6_RUN_ID.test(runId))) {
    throw new Error('a planned Recovery6 manifest with unique run IDs is required before runtime freeze');
  }
  const source = runtimeSourceFingerprint();
  const files = source.files || [];
  const select = (prefix) => files.filter((file) => file.path.startsWith(prefix));
  const runtime = readJSON('docs/p7-v2-runtime-environment.json') || {};
  const loadScriptPath = 'tests/load/p7v2-baseline.js';
  const sha256File = (relativePath) => crypto.createHash('sha256').update(fs.readFileSync(path.join(root, relativePath))).digest('hex');
  const profile = {
    configuredVUs: 10,
    stages: [{ name: 'warmup', duration: '5m', targetVUs: 10 }, { name: 'ramp', duration: '3m', targetVUs: 10 }, { name: 'steady', duration: '10m', targetVUs: 10 }, { name: 'rampdown', duration: '2m', targetVUs: 0 }],
    scenarios: [{ name: 'warmup', executor: 'constant-vus', startTime: '0s', gracefulStop: '0s' }, { name: 'ramp', executor: 'ramping-vus', startTime: '5m', gracefulStop: '0s' }, { name: 'steady', executor: 'constant-vus', startTime: '8m', gracefulStop: '0s' }, { name: 'rampdown', executor: 'ramping-vus', startTime: '18m', gracefulStop: '0s' }, { name: 'security_negative', executor: 'constant-vus', startTime: '0s', gracefulStop: '0s', weight: 1 }],
    requestMix: [['product_list', 20], ['order_list', 20], ['inventory_list', 15], ['task_list', 10], ['webhook_event_list', 8], ['operation_log_list', 7], ['webhook_ingestion', 5], ['provider_mock_flow', 5], ['auth_security', 2]].map(([routeId, weight]) => ({ routeId, method: routeId === 'webhook_ingestion' ? 'POST' : 'GET', weight })),
    credentialMix: [{ role: 'system_admin', weight: 1 }, { role: 'tenant_admin', weight: 1 }, { role: 'operator', weight: 1 }, { role: 'readonly', weight: 1 }],
    loadScript: { path: loadScriptPath, sha256: sha256File(loadScriptPath) },
  };
  const canonical = calculateLoadProfileFingerprint(profile, { repositoryRoot: root });
  const contract = {
    phase: 'P7-V2-R3B-LPC-R3', status: 'passed', frozenAt: now, runIds,
    runtimeSourceTreeHash: source.hash, evidenceToolingHash: jsonHash(select('scripts/')), loadScriptsHash: jsonHash(select('tests/load/')),
    metricSemanticsHash: jsonHash([...select('tests/load/'), ...files.filter((file) => ['scripts/p7-v2-lib.mjs', 'scripts/p7-v2-regression.mjs', 'scripts/p7-v2-regression-metrics.mjs'].includes(file.path))]),
    canonicalLoadProfileVersion: 3, loadProfileFingerprintVersion: 3, loadProfileFingerprint: canonical.loadProfileFingerprint, canonicalLoadProfile: canonical.canonicalProfile,
    datasetGeneratorHash: jsonHash(files.filter((file) => file.path === 'scripts/p7-v2-dataset.mjs')), datasetFingerprint: (readJSON('docs/p7-v2-dataset-report.json') || {}).datasetFingerprint || '',
    configFingerprint: crypto.createHash('sha256').update(JSON.stringify(runtime.env || {})).digest('hex'), sloFingerprint: fs.existsSync(path.join(root, 'docs/SLO.md')) ? sha256File('docs/SLO.md') : '',
    routeCredentialMatrixFingerprint: jsonHash(readJSON('docs/p7-v2-r2-route-credential-matrix.json') || {}), regressionPolicyFingerprint: jsonHash(readJSON('docs/p7-v2-regression-policy-v2.json') || {}),
    trackedDiffHash: trackedDiffHash().hash, selectedHost: '127.0.0.1', selectedPort: 18080, baseUrl: 'http://127.0.0.1:18080', k6Version: runtime.environmentFingerprint?.k6Version || '',
  };
  return { ...contract, contractId: hash(contract) };
}

if (import.meta.url === `file://${process.argv[1]?.replaceAll('\\', '/')}`) {
  const report = buildRuntimeFreezeContract();
  writeJSON(RUNTIME_FREEZE_PATH, report);
  updateR3BManifest({ runtimeFreezeId: report.contractId, status: 'runtime_frozen' });
  writeMarkdown('docs/P7_V2_R3B_LPC_R3_RUNTIME_FREEZE.md', `# P7-V2-R3B-LPC-R3 Runtime Freeze\n\nStatus: **passed**\n\n- Contract ID: \`${report.contractId}\`\n- Canonical schema version: \`${report.canonicalLoadProfileVersion}\`\n- Load-profile fingerprint version: \`${report.loadProfileFingerprintVersion}\`\n`);
  console.log(JSON.stringify(report, null, 2));
}
