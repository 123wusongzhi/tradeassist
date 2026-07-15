import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { readJSON, root } from './p7-v2-lib.mjs';

export const REGISTRY_PATH = 'docs/baselines/p7-v2-baseline-registry.json';
export const PRIORITY = ['frozen_registry', 'r3a_final', 'r3b_latest', 'r3_legacy', 'r2_historical'];
export const SUPPORTED_CANONICAL_LOAD_PROFILE_SCHEMA_VERSIONS = [1, 2, 3];
export const SUPPORTED_LOAD_PROFILE_FINGERPRINT_VERSIONS = [1, 2, 3];

export function validateLoadProfileEvidence(manifest = {}) {
  const schemaVersion = manifest.canonicalLoadProfileVersion ?? manifest.canonicalSchemaVersion ?? 1;
  const fingerprintVersion = manifest.loadProfileFingerprintVersion ?? 1;
  if (!SUPPORTED_CANONICAL_LOAD_PROFILE_SCHEMA_VERSIONS.includes(schemaVersion)) return { valid: false, issue: 'unsupported_canonical_load_profile_schema_version' };
  if (!SUPPORTED_LOAD_PROFILE_FINGERPRINT_VERSIONS.includes(fingerprintVersion)) return { valid: false, issue: 'unsupported_load_profile_fingerprint_version' };
  if (fingerprintVersion === 3 && (schemaVersion !== 3 || !/^[a-f0-9]{64}$/.test(manifest.loadProfileFingerprint || ''))) {
    return { valid: false, issue: 'invalid_v3_load_profile_evidence' };
  }
  return { valid: true, issue: '' };
}

export function sha256File(relativePath) {
  const file = path.join(root, relativePath);
  return fs.existsSync(file) ? crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex') : '';
}

export function baselineRequests(baseline = {}) {
  return Number(baseline.completedRequests ?? baseline.requests ?? 0);
}

export function validateFrozenBaseline(baseline, { verifyArtifact = true } = {}) {
  const issues = [];
  if (baseline?.status !== 'passed') issues.push('baseline status is not passed');
  if (!baseline?.runId) issues.push('baseline run ID is missing');
  if (baselineRequests(baseline) <= 0) issues.push('baseline has zero requests');
  if (baseline?.superseded === true) issues.push('baseline is superseded');
  if (baseline?.validForRegression !== true) issues.push('baseline is not valid for regression');
  if (baseline?.immutable !== true) issues.push('baseline is not immutable');
  if (baseline?.scenarioCoverage !== true) issues.push('baseline scenario coverage is incomplete');
  if (!baseline?.rawArtifactSha256) issues.push('baseline raw artifact hash is missing');
  if (baseline?.rawArtifactHashVerified !== true) issues.push('baseline raw artifact has not been verified');
  if (verifyArtifact) {
    const manifestPath = `docs/baselines/frozen/${baseline?.runId || ''}/manifest.json`;
    const manifest = readJSON(manifestPath) || {};
    const profileEvidence = validateLoadProfileEvidence(manifest);
    if (!profileEvidence.valid) issues.push(profileEvidence.issue);
    const relativePath = manifest?.rawArtifact?.relativePath || (manifest.frozenPath ? path.basename(manifest.frozenPath) : '') || 'raw-summary.json';
    const frozenArtifact = `docs/baselines/frozen/${baseline?.runId || ''}/${relativePath}`;
    const actualHash = sha256File(frozenArtifact);
    if (!actualHash) issues.push('frozen baseline raw artifact is missing');
    else if (
      actualHash !== baseline.rawArtifactSha256 ||
      actualHash !== (manifest?.rawArtifact?.sha256 || manifest.sha256) ||
      Number(fs.statSync(path.join(root, frozenArtifact)).size) !== Number(manifest?.rawArtifact?.sizeBytes ?? manifest.sizeBytes)
    ) issues.push('frozen baseline raw artifact hash or size mismatch');
  }
  return { valid: issues.length === 0, issues };
}

export const CURRENT_REGISTRY_PATH = 'docs/currents/p7-v2-current-registry.json';

export function resolveActiveCurrent({ verifyArtifact = true } = {}) {
  const registry = readJSON(CURRENT_REGISTRY_PATH) || {};
  const runId = registry.activeRegressionCurrent || '';
  const entry = (registry.entries || []).find((candidate) => candidate.runId === runId) || null;
  const manifest = entry ? readJSON(`docs/currents/frozen/${entry.runId}/manifest.json`) : null;
  const relativePath = manifest?.rawArtifact?.relativePath || (manifest?.frozenPath ? path.basename(manifest.frozenPath) : '') || 'raw-summary.json';
  const artifactPath = entry ? `docs/currents/frozen/${entry.runId}/${relativePath}` : '';
  const actualHash = artifactPath ? sha256File(artifactPath) : '';
  const size = artifactPath && fs.existsSync(path.join(root, artifactPath)) ? fs.statSync(path.join(root, artifactPath)).size : 0;
  const valid = Boolean(
    entry &&
      entry.status === 'passed' &&
      entry.independentRun === true &&
      entry.immutable === true &&
      entry.validForRegression === true &&
      manifest?.runId === entry.runId &&
      (!verifyArtifact || (actualHash && actualHash === entry.rawArtifactSha256 && actualHash === (manifest?.rawArtifact?.sha256 || manifest.sha256) && size === Number(manifest?.rawArtifact?.sizeBytes ?? manifest.sizeBytes))),
  );
  return { registry, entry, manifest, artifactPath, actualHash, valid, issues: valid ? [] : ['active frozen Current is invalid'] };
}

export function resolveActiveBaseline({ verifyArtifact = true } = {}) {
  const registry = readJSON(REGISTRY_PATH) || {};
  const configured = registry.activeRegressionBaseline || registry.activeBaselineRunId || registry.activeBaseline || '';
  const entries = Array.isArray(registry.baselines) ? registry.baselines : [];
  const candidate = typeof configured === 'object'
    ? configured
    : entries.find((entry) => entry.runId === configured) ||
      entries.find((entry) => entry.activeRegressionBaseline === true) ||
      [...entries].reverse().find((entry) => entry.status === 'passed' && entry.validForRegression === true && entry.superseded !== true && baselineRequests(entry) > 0);
  const reportPath = candidate?.reportPath || candidate?.baselineReportPath || 'docs/p7-v2-r3-baseline-report.json';
  const report = candidate?.completedRequests !== undefined ? candidate : readJSON(reportPath);
  const baseline = { ...(report || {}), ...(candidate || {}) };
  const validation = validateFrozenBaseline(baseline, { verifyArtifact });
  return {
    source: 'frozen_registry',
    priority: PRIORITY,
    registry,
    entry: candidate || null,
    reportPath,
    baseline,
    ...validation,
  };
}

export function readR3BManifest() {
  return readJSON('docs/p7-v2-r3b-run-manifest.json') || {};
}
