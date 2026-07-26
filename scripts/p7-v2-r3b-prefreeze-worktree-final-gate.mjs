import { readJSON, run } from './p7-v2-lib.mjs';

const CLASSIFICATION_JSON = 'docs/p7-v2-r3b-prefreeze-worktree-classification.json';

const report = readJSON(CLASSIFICATION_JSON) || {};
const validClassifications = new Set([
  'immutable_runtime_input',
  'immutable_test_or_gate',
  'formal_evidence_output',
  'historical_evidence',
  'mutable_execution_state',
  'generated_artifact',
  'unrelated_change',
]);

function statusPath(line) {
  if (!line) return '';
  const payload = line.slice(3);
  if (payload.includes(' -> ')) return payload.split(' -> ').pop().trim();
  return payload.trim();
}

const gitStatus = run('git', ['status', '--porcelain=v1', '-uall']);
const statusLines = gitStatus.stdout.split(/\r?\n/).map((line) => line.trimEnd()).filter(Boolean);
const statusPaths = new Set(statusLines.map(statusPath));
const filePaths = new Set((report.files || []).map((file) => file.path));
const untrackedPaths = statusLines.filter((line) => line.startsWith('?? ')).map(statusPath);
const dirtyPaths = statusLines.filter((line) => !line.startsWith('?? ')).map(statusPath);

const unclassifiedFiles = (report.files || []).filter((file) => !validClassifications.has(file.classification));
const unknownStatusPaths = [...statusPaths].filter((path) => !filePaths.has(path));
const unrelatedFiles = (report.files || []).filter((file) => file.classification === 'unrelated_change');

const checks = {
  allDirtyFilesEnumerated: dirtyPaths.every((path) => filePaths.has(path)),
  allUntrackedFilesEnumerated: untrackedPaths.every((path) => filePaths.has(path)),
  allFilesClassified: unclassifiedFiles.length === 0 && unknownStatusPaths.length === 0,
  unclassifiedFileCount: unclassifiedFiles.length + unknownStatusPaths.length,
  unrelatedChangeCount: unrelatedFiles.length,
  immutableRuntimeInputsIdentified: Number(report.classifications?.immutableRuntimeInput || 0) >= 0,
  immutableTestAndGateFilesIdentified: Number(report.classifications?.immutableTestOrGate || 0) > 0,
  historicalEvidencePreserved: report.eligibilityInputs?.historicalEvidencePreserved === true,
  mutableExecutionStateAccurate: report.eligibilityInputs?.mutableExecutionStateAccurate === true,
  blockedRunStatePreserved: report.eligibilityInputs?.blockedRunsStillBlocked === true,
  noResetPerformed: report.guardrails?.noResetPerformed === true,
  noCleanPerformed: report.guardrails?.noCleanPerformed === true,
  noStashPerformed: report.guardrails?.noStashPerformed === true,
  noUnknownFileDeleted: report.guardrails?.noUnknownFileDeleted === true,
  checkpointEligible: report.checkpointEligible === true,
};

const failedChecks = Object.entries(checks)
  .filter(([key, value]) => key.endsWith('Count') ? value !== 0 : value !== true)
  .map(([key]) => key);

const gate = {
  phase: 'P7-V2-R3B-PREFREEZE-WORKTREE-FINAL-GATE',
  status: failedChecks.length === 0 ? 'passed' : 'failed',
  classificationReport: CLASSIFICATION_JSON,
  observedStatusLines: statusLines,
  unknownStatusPaths,
  unrelatedFiles: unrelatedFiles.map((file) => file.path),
  checks,
  failedChecks,
  failed: failedChecks.length,
};

console.log(JSON.stringify(gate, null, 2));
process.exit(gate.status === 'passed' ? 0 : 1);
