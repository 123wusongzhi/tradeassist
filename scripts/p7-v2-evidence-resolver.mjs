import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { readJSON, root } from './p7-v2-lib.mjs';

export const REGISTRY_PATH = 'docs/baselines/p7-v2-baseline-registry.json';
export const PRIORITY = ['frozen_registry', 'r3a_final', 'r3b_latest', 'r3_legacy', 'r2_historical'];

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
    const frozenArtifact = `docs/baselines/frozen/${baseline?.runId || ''}/baseline.summary.json`;
    const actualHash = sha256File(frozenArtifact);
    if (!actualHash) issues.push('frozen baseline raw artifact is missing');
    else if (actualHash !== baseline.rawArtifactSha256) issues.push('frozen baseline raw artifact hash mismatch');
  }
  return { valid: issues.length === 0, issues };
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
