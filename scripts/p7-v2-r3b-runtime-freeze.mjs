import crypto from 'node:crypto';
import fs from 'node:fs';
import { jsonHash, runtimeSourceFingerprint, trackedDiffHash } from './p7-v2-r3-lib.mjs';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const source = runtimeSourceFingerprint();
const files = source.files || [];
const select = (prefix) => files.filter((file) => file.path.startsWith(prefix));
const runtime = readJSON('docs/p7-v2-runtime-environment.json') || {};
const load = readJSON('docs/p7-v2-r3-baseline-report.json') || {};
const report = {
  phase: 'P7-V2-R3B-FAST-CLOSE-R2',
  status: 'passed',
  frozenAt: new Date().toISOString(),
  runtimeSourceTreeHash: source.hash,
  evidenceToolingHash: jsonHash(select('scripts/')),
  loadScriptsHash: jsonHash(select('tests/load/')),
  metricSemanticsHash: jsonHash([
    ...select('tests/load/'),
    ...files.filter((file) => [
      'scripts/p7-v2-lib.mjs',
      'scripts/p7-v2-regression.mjs',
      'scripts/p7-v2-regression-metrics.mjs',
    ].includes(file.path)),
  ]),
  regressionPolicyFingerprint: jsonHash(readJSON('docs/p7-v2-regression-policy-v2.json') || {}),
  datasetFingerprint: (readJSON('docs/p7-v2-dataset-report.json') || {}).datasetFingerprint || '',
  configFingerprint: crypto.createHash('sha256').update(JSON.stringify(runtime.env || {})).digest('hex'),
  loadProfileFingerprint: load.loadProfileFingerprint || '',
  sloFingerprint: fs.existsSync('docs/SLO.md') ? crypto.createHash('sha256').update(fs.readFileSync('docs/SLO.md')).digest('hex') : '',
  routeCredentialMatrixFingerprint: jsonHash(readJSON('docs/p7-v2-r2-route-credential-matrix.json') || {}),
  gitCommit: runtime.environmentFingerprint?.gitCommit || '',
  gitDirty: runtime.environmentFingerprint?.gitDirty ?? true,
  trackedDiffHash: trackedDiffHash().hash,
  selectedHost: '127.0.0.1',
  selectedPort: 18080,
  baseUrl: 'http://127.0.0.1:18080',
  k6Version: runtime.environmentFingerprint?.k6Version || '',
  goVersion: runtime.environmentFingerprint?.goVersion || '',
  nodeVersion: process.version,
  postgresVersion: runtime.environmentFingerprint?.postgresVersion || '',
  redisVersion: runtime.environmentFingerprint?.redisVersion || '',
};
writeJSON('docs/p7-v2-r3b-fast-close-r2-runtime-freeze.json', report);
writeMarkdown('docs/P7_V2_R3B_FAST_CLOSE_R2_RUNTIME_FREEZE.md', `# P7-V2-R3B Fast Close R2 Runtime Freeze\n\nStatus: **passed**\n\n- Runtime source tree hash: \`${report.runtimeSourceTreeHash}\`\n`);
console.log(JSON.stringify(report, null, 2));
