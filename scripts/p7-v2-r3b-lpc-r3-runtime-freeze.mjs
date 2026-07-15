import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { calculateLoadProfileFingerprint } from './p7-v2-load-profile-fingerprint.mjs';
import { jsonHash, runtimeSourceFingerprint, trackedDiffHash } from './p7-v2-r3-lib.mjs';
import { readJSON, root, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const source = runtimeSourceFingerprint();
const files = source.files || [];
const select = (prefix) => files.filter((file) => file.path.startsWith(prefix));
const runtime = readJSON('docs/p7-v2-runtime-environment.json') || {};
const loadScriptPath = 'tests/load/p7v2-baseline.js';
const sha256File = (relativePath) => crypto.createHash('sha256').update(fs.readFileSync(path.join(root, relativePath))).digest('hex');
const profile = {
  configuredVUs: 10,
  stages: [
    { name: 'warmup', duration: '5m', targetVUs: 10 },
    { name: 'ramp', duration: '3m', targetVUs: 10 },
    { name: 'steady', duration: '10m', targetVUs: 10 },
    { name: 'rampdown', duration: '2m', targetVUs: 0 },
  ],
  scenarios: [
    { name: 'warmup', executor: 'constant-vus', startTime: '0s', gracefulStop: '0s' },
    { name: 'ramp', executor: 'ramping-vus', startTime: '5m', gracefulStop: '0s' },
    { name: 'steady', executor: 'constant-vus', startTime: '8m', gracefulStop: '0s' },
    { name: 'rampdown', executor: 'ramping-vus', startTime: '18m', gracefulStop: '0s' },
    { name: 'security_negative', executor: 'constant-vus', startTime: '0s', gracefulStop: '0s', weight: 1 },
  ],
  requestMix: [
    ['product_list', 20], ['order_list', 20], ['inventory_list', 15], ['task_list', 10],
    ['webhook_event_list', 8], ['operation_log_list', 7], ['webhook_ingestion', 5],
    ['provider_mock_flow', 5], ['auth_security', 2],
  ].map(([routeId, weight]) => ({ routeId, method: routeId === 'webhook_ingestion' ? 'POST' : 'GET', weight })),
  credentialMix: [
    { role: 'system_admin', weight: 1 }, { role: 'tenant_admin', weight: 1 },
    { role: 'operator', weight: 1 }, { role: 'readonly', weight: 1 },
  ],
  loadScript: { path: loadScriptPath, sha256: sha256File(loadScriptPath) },
};
const canonical = calculateLoadProfileFingerprint(profile, { repositoryRoot: root });
const report = {
  phase: 'P7-V2-R3B-LPC-R3',
  status: 'passed',
  frozenAt: new Date().toISOString(),
  runtimeSourceTreeHash: source.hash,
  evidenceToolingHash: jsonHash(select('scripts/')),
  loadScriptsHash: jsonHash(select('tests/load/')),
  metricSemanticsHash: jsonHash([...select('tests/load/'), ...files.filter((file) => [
    'scripts/p7-v2-lib.mjs', 'scripts/p7-v2-regression.mjs', 'scripts/p7-v2-regression-metrics.mjs',
  ].includes(file.path))]),
  canonicalLoadProfileVersion: 3,
  loadProfileFingerprintVersion: 3,
  loadProfileFingerprint: canonical.loadProfileFingerprint,
  canonicalLoadProfile: canonical.canonicalProfile,
  datasetGeneratorHash: jsonHash(files.filter((file) => file.path === 'scripts/p7-v2-dataset.mjs')),
  datasetFingerprint: (readJSON('docs/p7-v2-dataset-report.json') || {}).datasetFingerprint || '',
  configFingerprint: crypto.createHash('sha256').update(JSON.stringify(runtime.env || {})).digest('hex'),
  sloFingerprint: fs.existsSync(path.join(root, 'docs/SLO.md')) ? sha256File('docs/SLO.md') : '',
  routeCredentialMatrixFingerprint: jsonHash(readJSON('docs/p7-v2-r2-route-credential-matrix.json') || {}),
  regressionPolicyFingerprint: jsonHash(readJSON('docs/p7-v2-regression-policy-v2.json') || {}),
  trackedDiffHash: trackedDiffHash().hash,
  selectedHost: '127.0.0.1',
  selectedPort: 18080,
  baseUrl: 'http://127.0.0.1:18080',
  k6Version: runtime.environmentFingerprint?.k6Version || '',
};
writeJSON('docs/p7-v2-r3b-lpc-r3-runtime-freeze.json', report);
writeMarkdown('docs/P7_V2_R3B_LPC_R3_RUNTIME_FREEZE.md', `# P7-V2-R3B-LPC-R3 Runtime Freeze\n\nStatus: **passed**\n\n- Canonical schema version: \`${report.canonicalLoadProfileVersion}\`\n- Load-profile fingerprint version: \`${report.loadProfileFingerprintVersion}\`\n- Load-profile fingerprint: \`${report.loadProfileFingerprint}\`\n`);
console.log(JSON.stringify(report, null, 2));
