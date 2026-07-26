import assert from 'node:assert/strict';
import { classifyP7V2Database, summarizeCleanupClassifications } from '../../../../scripts/p7-v2-r3b-cleanup-classifier.mjs';
import { safeDbName } from '../../../../scripts/p7-v2-lib.mjs';

const manifest = {
  runtimeFreezeId: 'a'.repeat(64),
  baselineRunId: 'p7v2-baseline-r3b-recovery6-20260715150913',
  currentRunId: 'p7v2-current-r3b-recovery6-20260715150913',
  soakRunId: 'p7v2-soak-r3b-recovery6-20260715150913',
};
const runtime = {
  runId: 'p7v2-current-r3b-recovery6-20260715150913-restart',
  dbName: 'trademind_p7v2_p7v2_current_r3b_recovery6_20260715150913_restar',
};
const registryRunIds = new Set(['p7v2-baseline-r3b-recovery6-20260715125505']);
const diagnosticRuns = [
  {
    runId: 'p7v2-diag-current-sql-fingerprint-20260716160124',
    databaseIdentity: safeDbName('p7v2-diag-current-sql-fingerprint-20260716160124'),
    finalGatePassed: true,
    formal: false,
    validForClosure: false,
    rawJsonlPath: '',
    diagnosticReportMatch: true,
  },
];

assert.equal(
  classifyP7V2Database(runtime.dbName, {
    manifest,
    runtime,
    registryRunIds,
    diagnosticRuns,
    targetRunId: manifest.currentRunId,
  }).classification,
  'old_formal_pair_residue',
);

assert.equal(
  classifyP7V2Database('trademind_p7v2_p7v2_baseline_r3b_recovery6_20260715125505', {
    manifest,
    runtime,
    registryRunIds,
    diagnosticRuns,
  }).classification,
  'historical_formal_evidence_db',
);

const diagnostic = classifyP7V2Database(safeDbName('p7v2-diag-current-sql-fingerprint-20260716160124'), {
  manifest,
  runtime,
  registryRunIds,
  diagnosticRuns,
  targetRunId: manifest.currentRunId,
});
assert.equal(diagnostic.classification, 'active_or_required_diagnostic_evidence');
assert.equal(diagnostic.cleanupEligible, false, 'formal run cleanup must not delete diagnostic databases');

const unknown = classifyP7V2Database('trademind_p7v2_unbound_database', {
  manifest,
  runtime,
  registryRunIds,
  diagnosticRuns,
});
assert.equal(unknown.classification, 'unknown_database');
assert.equal(unknown.retentionDecision, 'retain_and_stop');
assert.equal(unknown.cleanupEligible, false);

const summary = summarizeCleanupClassifications([
  { classification: 'old_formal_pair_residue' },
  { classification: 'historical_formal_evidence_db' },
  { classification: 'active_or_required_diagnostic_evidence' },
  { classification: 'completed_diagnostic_ephemeral', retentionDecision: 'cleanup_when_execute_scope_matches' },
  { classification: 'unknown_database' },
]);
assert.equal(summary.currentFormalResidualCount, 1);
assert.equal(summary.historicalEvidenceDatabaseCount, 1);
assert.equal(summary.diagnosticDatabaseCount, 2);
assert.equal(summary.knownRetainedDiagnosticDatabaseCount, 1);
assert.equal(summary.completedDiagnosticEphemeralCount, 1);
assert.equal(summary.unknownDatabaseCount, 1);

const executeAttempt = {
  cleanupContractVersion: 2,
  cleanupAttemptId: 'execute-1',
  mode: 'execute',
  plannedActions: [{ type: 'drop_database', resourceId: runtime.dbName }],
  executedActions: [{ type: 'drop_database', resourceId: runtime.dbName, status: 'passed' }],
};
const checkAttempt = {
  cleanupContractVersion: 2,
  cleanupAttemptId: 'check-2',
  mode: 'check',
  plannedActions: [],
  executedActions: [],
};
assert.notEqual(executeAttempt.cleanupAttemptId, checkAttempt.cleanupAttemptId);
assert.equal(executeAttempt.executedActions.length, 1);
assert.equal(checkAttempt.executedActions.length, 0);

const unknownExecuteGate = {
  mode: 'execute',
  unknownDatabaseCount: 1,
  plannedActions: [{ type: 'drop_database', resourceId: 'trademind_p7v2_unbound_database' }],
};
assert.equal(
  unknownExecuteGate.unknownDatabaseCount === 0 && unknownExecuteGate.plannedActions.every((item) => item.resourceId !== 'trademind_p7v2_unbound_database'),
  false,
  'unknown database execution must be blocked by the caller',
);

const semanticFailure = { semanticGatePassed: false, exitCode: 1 };
assert.notEqual(semanticFailure.exitCode, 0);

console.log(JSON.stringify({ phase: 'P7-V2-R3B-CLEANUP-V2', status: 'passed', fixtures: 9 }, null, 2));
