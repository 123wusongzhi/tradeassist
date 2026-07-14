import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { readJSON, root, valueOf, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const runId = valueOf(process.argv.slice(2), '--run-id');
if (!/^p7v2-baseline-r3a-[a-z0-9_-]+$/.test(runId)) throw new Error('R3A baseline run ID is required');

const reportPath = `docs/baselines/p7-v2-baseline-${runId}.json`;
const report = readJSON(reportPath);
const rawPath = path.join(root, 'artifacts', 'p7-v2', 'baseline', runId, 'baseline.summary.json');
const frozenDir = path.join(root, 'docs', 'baselines', 'frozen', runId);
const registryPath = 'docs/baselines/p7-v2-baseline-registry.json';
const registry = readJSON(registryPath) || { phase: 'P7-V2-R3A', baselines: [] };
const raw = fs.existsSync(rawPath) ? fs.readFileSync(rawPath) : null;
const rawSummary = raw ? JSON.parse(raw.toString('utf8')) : null;
const requests = Number(rawSummary?.metrics?.http_reqs?.values?.count || rawSummary?.metrics?.http_reqs?.count || 0);
const scenarioCoverage = (report?.scenarios || []).length >= 9 && (report?.scenarios || []).every((item) => Number(item.requests) > 0);
const issues = [];
if (!report || report.status !== 'passed') issues.push('baseline report is not passed');
if (!raw || raw.length === 0) issues.push('raw k6 artifact missing');
if (requests <= 0) issues.push('raw k6 artifact has zero requests');
if (!scenarioCoverage) issues.push('scenario coverage is incomplete');
if (report?.thresholdsPassed !== true) issues.push('k6 thresholds did not pass');
if (report?.absoluteSloPassed !== true) issues.push('absolute SLO did not pass');
if (!report?.runtimeSourceTreeHash || !report?.loadScriptHash || !report?.datasetFingerprint || !report?.configFingerprint || !report?.loadProfileFingerprint || !report?.sloFingerprint || !report?.routeCredentialMatrixFingerprint) issues.push('required baseline fingerprints are missing');
if (fs.existsSync(path.join(frozenDir, 'baseline.json'))) issues.push('frozen baseline artifact already exists');
if ((registry.baselines || []).some((item) => item.runId === runId)) issues.push('baseline registry already contains this run ID');
if (issues.length) {
  console.error(JSON.stringify({ runId, issues }, null, 2));
  process.exit(1);
}

const sha256 = crypto.createHash('sha256').update(raw).digest('hex');
const frozen = {
  ...report,
  immutable: true,
  validForRegression: true,
  freezeMode: 'immediate_post_run',
  rawArtifactPath: path.relative(root, rawPath).replaceAll('\\', '/'),
  rawArtifactSha256: sha256,
  rawArtifactSizeBytes: raw.length,
  rawArtifactHashVerified: true,
  scenarioCoverage: true,
  frozenAt: new Date().toISOString(),
  supersedes: ['p7v2-baseline-20260714181000'],
};
fs.mkdirSync(frozenDir, { recursive: true });
writeJSON(path.relative(root, path.join(frozenDir, 'baseline.json')), frozen);
writeJSON(path.relative(root, path.join(frozenDir, 'manifest.json')), {
  runId,
  rawArtifactPath: frozen.rawArtifactPath,
  rawArtifactSha256: sha256,
  rawArtifactSizeBytes: raw.length,
  frozenAt: frozen.frozenAt,
});
fs.writeFileSync(path.join(frozenDir, 'raw-artifact.sha256'), `${sha256}  ${frozen.rawArtifactPath}\n`, 'utf8');
writeJSON(path.relative(root, path.join(frozenDir, 'source-fingerprint.json')), {
  runtimeSourceTreeHash: frozen.runtimeSourceTreeHash,
  loadScriptsHash: frozen.loadScriptHash,
  datasetFingerprint: frozen.datasetFingerprint,
  configFingerprint: frozen.configFingerprint,
  loadProfileFingerprint: frozen.loadProfileFingerprint,
  sloFingerprint: frozen.sloFingerprint,
  routeCredentialMatrixFingerprint: frozen.routeCredentialMatrixFingerprint,
});
registry.baselines = [...(registry.baselines || []), frozen];
writeJSON(registryPath, registry);
writeJSON('docs/p7-v2-r3a-baseline-freeze-report.json', { phase: 'P7-V2-R3A', status: 'passed', ...frozen });
writeMarkdown('docs/P7_V2_R3A_BASELINE_FREEZE_REPORT.md', `# P7-V2-R3A Baseline Freeze Report\n\nStatus: **passed**\n\n- Run ID: \`${runId}\`\n- Raw artifact SHA-256: \`${sha256}\`\n- Immutable: true\n- Freeze mode: immediate_post_run\n`);
console.log(JSON.stringify({ runId, status: 'passed', sha256 }, null, 2));
