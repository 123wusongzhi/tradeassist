import fs from 'node:fs';
import path from 'node:path';
import { readJSON, root, valueOf, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const runId = valueOf(process.argv.slice(2), '--run-id');
const registry = readJSON('docs/baselines/p7-v2-baseline-registry.json') || { baselines: [] };
const baseline = (registry.baselines || []).find((item) => item.runId === runId);
const required = [
  'runtimeSourceTreeHash',
  'loadScriptHash',
  'datasetFingerprint',
  'configFingerprint',
  'loadProfileFingerprint',
  'sloFingerprint',
  'routeCredentialMatrixFingerprint',
];
const rawPath = baseline?.rawArtifactPath ? path.join(root, baseline.rawArtifactPath) : '';
const checks = {
  baselinePassed: baseline?.status === 'passed',
  baselineValidForRegression: baseline?.validForRegression === true,
  baselineImmutable: baseline?.immutable === true,
  rawArtifactExists: Boolean(rawPath && fs.existsSync(rawPath)),
  rawArtifactHashVerified: baseline?.rawArtifactHashVerified === true,
  requests: Number(baseline?.completedRequests || 0) > 0,
  scenarioCoverage: baseline?.scenarioCoverage === true,
  fingerprints: required.every((key) => Boolean(baseline?.[key])),
};
const failed = Object.entries(checks).filter(([, value]) => !value).map(([key]) => key);
const report = {
  phase: 'P7-V2-R3A',
  status: failed.length ? 'not_comparable' : 'passed',
  baselineRunId: runId,
  baselineValidForRegression: checks.baselineValidForRegression,
  baselineImmutable: checks.baselineImmutable,
  rawArtifactHashVerified: checks.rawArtifactHashVerified,
  runtimeSourceTreeFingerprintReady: Boolean(baseline?.runtimeSourceTreeHash),
  loadScriptsFingerprintReady: Boolean(baseline?.loadScriptsHash || baseline?.loadScriptHash),
  datasetFingerprintReady: Boolean(baseline?.datasetFingerprint),
  configFingerprintReady: Boolean(baseline?.configFingerprint),
  loadProfileFingerprintReady: Boolean(baseline?.loadProfileFingerprint),
  sloFingerprintReady: Boolean(baseline?.sloFingerprint),
  routeCredentialMatrixFingerprintReady: Boolean(baseline?.routeCredentialMatrixFingerprint),
  expectedCurrentRuntimeSourceTreeHash: baseline?.runtimeSourceTreeHash || '',
  expectedCurrentLoadScriptsHash: baseline?.loadScriptHash || '',
  expectedCurrentDatasetFingerprint: baseline?.datasetFingerprint || '',
  expectedCurrentConfigFingerprint: baseline?.configFingerprint || '',
  expectedCurrentLoadProfileFingerprint: baseline?.loadProfileFingerprint || '',
  expectedCurrentSloFingerprint: baseline?.sloFingerprint || '',
  expectedCurrentRouteCredentialMatrixFingerprint: baseline?.routeCredentialMatrixFingerprint || '',
  currentExecutionAllowed: failed.length === 0,
  issues: failed,
};
writeJSON('docs/p7-v2-r3a-comparability-precondition-report.json', report);
writeMarkdown('docs/P7_V2_R3A_COMPARABILITY_PRECONDITION_REPORT.md', `# P7-V2-R3A Comparability Precondition\n\nStatus: **${report.status}**\n\n- Baseline: \`${runId}\`\n- Current execution allowed: ${report.currentExecutionAllowed}\n\nIssues:\n${failed.length ? failed.map((item) => `- ${item}`).join('\n') : '- none'}\n`);
console.log(JSON.stringify(report, null, 2));
process.exit(failed.length ? 1 : 0);
