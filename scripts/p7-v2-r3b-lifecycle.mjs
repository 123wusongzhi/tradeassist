import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { readJSON, root, writeJSON } from './p7-v2-lib.mjs';

export const RUNTIME_FREEZE_LIFECYCLE_CONTRACT_VERSION = 3;

export const FORMAL_STATES = [
  'planned',
  'ready_for_formal_execution',
  'environment_starting',
  'environment_started',
  'dataset_running',
  'dataset_completed',
  'baseline_running',
  'baseline_completed',
  'baseline_frozen',
  'current_preparing',
  'current_running',
  'current_completed',
  'current_frozen',
  'comparability_running',
  'comparability_passed',
  'regression_running',
  'regression_passed',
  'soak_running',
  'soak_passed',
  'demo1_running',
  'demo1_passed',
  'demo2_running',
  'demo2_passed',
  'stability_running',
  'stability_passed',
  'race_running',
  'race_passed',
  'cleanup_running',
  'cleanup_passed',
  'final_gates_running',
  'completed',
];

export const ALLOWED_TRANSITIONS = new Set(FORMAL_STATES.slice(0, -1).map((state, index) => `${state}->${FORMAL_STATES[index + 1]}`));

function sha256Like(value) {
  return /^[a-f0-9]{64}$/.test(String(value || ''));
}

function fileExists(relPath) {
  return Boolean(relPath && fs.existsSync(path.join(root, relPath)));
}

function registryHasBaseline(runId) {
  const registry = readJSON('docs/baselines/p7-v2-baseline-registry.json') || {};
  return (registry.baselines || []).some((entry) => entry.runId === runId && entry.validForRegression === true && entry.immutable === true);
}

function registryHasCurrent(runId) {
  const registry = readJSON('docs/currents/p7-v2-current-registry.json') || {};
  return (registry.entries || []).some((entry) => entry.runId === runId && entry.validForRegression === true && entry.immutable === true && entry.independentRun === true);
}

function defaultEvidence() {
  const manifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {};
  const baselineRunId = manifest.baselineRunId || '';
  const currentRunId = manifest.currentRunId || '';
  const baselineReport = readJSON('docs/p7-v2-r3-baseline-report.json') || readJSON(`docs/baselines/p7-v2-baseline-${baselineRunId}.json`) || {};
  const currentReport = readJSON('docs/p7-v2-current-load-report.json') || {};
  const baselineFrozen = readJSON(`docs/baselines/frozen/${baselineRunId}/manifest.json`) || {};
  const currentFrozen = readJSON(`docs/currents/frozen/${currentRunId}/manifest.json`) || {};
  const comparability = readJSON('docs/p7-v2-r3b-fast-close-r3-comparability-report.json') || readJSON('docs/p7-v2-r3-comparability-report.json') || {};
  const regression = readJSON('docs/p7-v2-regression-report.json') || {};
  return {
    manifest,
    baselineRunId,
    currentRunId,
    baselineReport,
    currentReport,
    baselineFrozen,
    currentFrozen,
    baselineFrozenManifestExists: fileExists(`docs/baselines/frozen/${baselineRunId}/manifest.json`),
    currentFrozenManifestExists: fileExists(`docs/currents/frozen/${currentRunId}/manifest.json`),
    baselineRegistryEntryExists: registryHasBaseline(baselineRunId),
    currentRegistryEntryExists: registryHasCurrent(currentRunId),
    comparability,
    regression,
  };
}

function evidenceCheck(nextState, evidence) {
  const e = { ...defaultEvidence(), ...(evidence || {}) };
  const failures = [];
  if (nextState === 'baseline_completed') {
    if (Number(e.baselineReport?.k6ExitCode) !== 0) failures.push('baseline_k6_exit_code_not_zero');
    if (Number(e.baselineReport?.completedRequests || e.baselineReport?.requests || 0) <= 0) failures.push('baseline_requests_missing');
    if (!fileExists(`artifacts/p7-v2/baseline/${e.baselineRunId}/baseline.summary.json`) && !e.rawArtifactExists) failures.push('baseline_raw_artifact_missing');
  }
  if (nextState === 'baseline_frozen') {
    if (!e.baselineFrozenManifestExists) failures.push('baseline_frozen_manifest_missing');
    if (!sha256Like(e.baselineFrozen?.sha256 || e.baselineFrozen?.rawArtifact?.sha256)) failures.push('baseline_artifact_sha256_invalid');
    if (Number(e.baselineFrozen?.sizeBytes || e.baselineFrozen?.rawArtifact?.sizeBytes || 0) <= 0) failures.push('baseline_artifact_size_missing');
    if (!e.baselineRegistryEntryExists) failures.push('baseline_registry_entry_missing');
    if (e.baselineFrozen?.validForRegression !== true) failures.push('baseline_frozen_manifest_not_regression_valid');
  }
  if (nextState === 'current_running') {
    if (!e.baselineFrozenManifestExists || !e.baselineRegistryEntryExists) failures.push('baseline_frozen_required_before_current');
  }
  if (nextState === 'current_frozen') {
    if (!e.currentFrozenManifestExists) failures.push('current_frozen_manifest_missing');
    if (e.currentFrozen?.independentRun !== true && e.currentReport?.currentRunIndependent !== true) failures.push('current_independence_missing');
  }
  if (nextState === 'comparability_passed') {
    if (!e.baselineFrozenManifestExists || !e.currentFrozenManifestExists) failures.push('frozen_pair_required_for_comparability');
    if (e.comparability?.status !== 'passed') failures.push('comparability_not_passed');
    if (Number(e.comparability?.mismatchCount || 0) !== 0) failures.push('comparability_mismatch_count_nonzero');
    if (Number(e.comparability?.notComparableCount || 0) !== 0) failures.push('comparability_not_comparable_count_nonzero');
  }
  if (nextState === 'regression_passed') {
    if (e.comparability?.status !== 'passed') failures.push('comparability_required_before_regression');
    if (e.regression?.status !== 'passed') failures.push('regression_not_passed');
    if (Number(e.regression?.failedMetricCount || 0) !== 0) failures.push('regression_failed_metric_count_nonzero');
    if (Number(e.regression?.notComparableCount || 0) !== 0) failures.push('regression_not_comparable_count_nonzero');
    if (Number(e.regression?.invalidMetricCount || 0) !== 0) failures.push('regression_invalid_metric_count_nonzero');
  }
  return failures;
}

export function validateFormalExecutionLifecycle({ previousState, nextState, evidence = {} } = {}) {
  const transition = `${previousState}->${nextState}`;
  const issues = [];
  if (!FORMAL_STATES.includes(previousState) || !FORMAL_STATES.includes(nextState) || !ALLOWED_TRANSITIONS.has(transition)) {
    issues.push({ classification: 'lifecycle_transition_invalid', issue: transition });
  }
  const evidenceFailures = evidenceCheck(nextState, evidence);
  for (const issue of evidenceFailures) issues.push({ classification: 'lifecycle_evidence_missing', issue });
  return {
    status: issues.length ? 'failed' : 'passed',
    runtimeFreezeLifecycleContractVersion: RUNTIME_FREEZE_LIFECYCLE_CONTRACT_VERSION,
    previousState,
    nextState,
    transitionValid: !issues.some((issue) => issue.classification === 'lifecycle_transition_invalid'),
    evidenceComplete: !issues.some((issue) => issue.classification === 'lifecycle_evidence_missing'),
    issues,
  };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const previousState = process.argv[process.argv.indexOf('--from') + 1] || '';
  const nextState = process.argv[process.argv.indexOf('--to') + 1] || '';
  const report = validateFormalExecutionLifecycle({ previousState, nextState });
  if (process.argv.includes('--write')) writeJSON('docs/p7-v2-r3b-runtime-freeze-lifecycle-validation.json', report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}
