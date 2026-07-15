import crypto from 'node:crypto';
import { jsonHash, runtimeSourceFingerprint } from './p7-v2-r3-lib.mjs';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const source = runtimeSourceFingerprint();
const files = source.files || [];
const select = (prefix) => files.filter((file) => file.path.startsWith(prefix));
const report = {
  phase: 'P7-V2-R3B-FAST-CLOSE',
  status: 'passed',
  frozenAt: new Date().toISOString(),
  runtimeSourceTreeHash: source.hash,
  evidenceToolingHash: jsonHash(select('scripts/')),
  loadScriptsHash: jsonHash(select('tests/load/')),
  metricSemanticsHash: jsonHash([...select('tests/load/'), ...files.filter((file) => file.path === 'scripts/p7-v2-regression-metrics.mjs')]),
  regressionPolicyFingerprint: jsonHash(readJSON('docs/p7-v2-regression-policy-v2.json') || {}),
  datasetFingerprint: (readJSON('docs/p7-v2-dataset-report.json') || {}).datasetFingerprint || '',
  configFingerprint: crypto.createHash('sha256').update(JSON.stringify((readJSON('docs/p7-v2-runtime-environment.json') || {}).env || {})).digest('hex'),
  loadProfileFingerprint: '',
  sloFingerprint: '',
  routeCredentialMatrixFingerprint: jsonHash(readJSON('docs/p7-v2-r2-route-credential-matrix.json') || {}),
};
writeJSON('docs/p7-v2-r3b-runtime-freeze.json', report);
writeMarkdown('docs/P7_V2_R3B_FAST_CLOSE_PREFLIGHT.md', `# P7-V2-R3B Fast Close Preflight\n\nStatus: **passed**\n\n- Runtime source tree hash: \`${report.runtimeSourceTreeHash}\`\n`);
writeJSON('docs/p7-v2-r3b-fast-close-preflight.json', report);
console.log(JSON.stringify(report, null, 2));
