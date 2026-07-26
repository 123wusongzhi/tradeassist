import fs from 'node:fs';
import path from 'node:path';
import { readJSON, root, writeJSON } from './p7-v2-lib.mjs';

function exists(rel) {
  return fs.existsSync(path.join(root, rel));
}

const cleanup = readJSON('docs/p7-v2-runtime-cleanup-report.json') || {};
const hostGuard = readJSON('docs/p7-v2-load-host-guard-report.json') || {};
const repair = readJSON('docs/p7-v2-r3b-pre-formal-cleanup-v2-repair.json') || {};
const incident = readJSON('docs/p7-v2-r3b-cleanup-unclassified-db-delete-incident.json') || {};
const attemptsDir = path.join(root, 'artifacts', 'p7-v2', 'cleanup-attempts');
const attempts = fs.existsSync(attemptsDir)
  ? fs.readdirSync(attemptsDir).filter((name) => name.endsWith('.json')).sort()
  : [];
const executeAttempts = attempts
  .map((name) => ({ name, data: readJSON(`artifacts/p7-v2/cleanup-attempts/${name}`) || {} }))
  .filter((item) => item.data.mode === 'execute');
const latestAttemptExists = cleanup.latestAttemptPath ? exists(cleanup.latestAttemptPath) : false;
const executeActionPreserved = executeAttempts.some((item) => (item.data.executedActions || []).some((action) => action.type === 'drop_database'));

const checks = [
  ['cleanupContractVersion', cleanup.cleanupContractVersion === 2 && repair.cleanupContractVersion === 2],
  ['checkModeNonDestructive', cleanup.latestMode === 'check' && cleanup.destructiveActionCount === 0],
  ['targetScopeBindingPassed', repair.oldCurrentRunId === 'p7v2-current-r3b-recovery6-20260717051755'],
  ['unknownResourceExecutionBlocked', repair.unknownDatabasesAfter === 0 && cleanup.unknownDatabaseCount === 0],
  ['attemptHistoryAppendOnly', attempts.length >= 2 && latestAttemptExists],
  ['checkDoesNotOverwriteExecute', cleanup.checkDoesNotOverwriteExecute === true && executeActionPreserved],
  ['staleServerResolved', hostGuard.listener18080Count === 0 && cleanup.listener18080Count === 0],
  ['oldCurrentDatabaseResolved', cleanup.currentFormalResidualCount === 0],
  ['unknownDatabaseCount', cleanup.unknownDatabaseCount === 0],
  ['unknownProcessCount', cleanup.unknownProcessCount === 0 && hostGuard.unknownProcessCount === 0],
  ['unknownConnectionCount', cleanup.unknownConnectionCount === 0],
  ['listener18080Count', hostGuard.listener18080Count === 0],
  ['historicalFrozenEvidencePreserved', repair.formalFrozenEvidencePreserved === true],
  ['historicalEvidencePreserved', repair.historicalEvidencePreserved === true],
  ['unexpectedDatabaseDeletedCount', repair.unexpectedDatabaseDeletedCount === 0],
  ['unexpectedProcessKilledCount', repair.unexpectedProcessKilledCount === 0],
  ['previousUnclassifiedDeleteIncidentRecorded', incident.destructiveActionOccurred === true],
];

const failed = checks.filter(([, passed]) => !passed).map(([id]) => id);
const report = {
  phase: 'P7-V2-R3B-PRE-FORMAL-CLEANUP-V2-FINAL-GATE',
  status: failed.length === 0 ? 'passed' : 'failed',
  cleanupContractVersion: 2,
  checkModeNonDestructive: checks.find(([id]) => id === 'checkModeNonDestructive')?.[1] === true,
  targetScopeBindingPassed: checks.find(([id]) => id === 'targetScopeBindingPassed')?.[1] === true,
  unknownResourceExecutionBlocked: checks.find(([id]) => id === 'unknownResourceExecutionBlocked')?.[1] === true,
  attemptHistoryAppendOnly: checks.find(([id]) => id === 'attemptHistoryAppendOnly')?.[1] === true,
  checkDoesNotOverwriteExecute: checks.find(([id]) => id === 'checkDoesNotOverwriteExecute')?.[1] === true,
  staleServerResolved: checks.find(([id]) => id === 'staleServerResolved')?.[1] === true,
  oldCurrentDatabaseResolved: checks.find(([id]) => id === 'oldCurrentDatabaseResolved')?.[1] === true,
  unknownDatabaseCount: cleanup.unknownDatabaseCount ?? null,
  unknownProcessCount: cleanup.unknownProcessCount ?? null,
  unknownConnectionCount: cleanup.unknownConnectionCount ?? null,
  listener18080Count: hostGuard.listener18080Count ?? cleanup.listener18080Count ?? null,
  historicalFrozenEvidencePreserved: repair.formalFrozenEvidencePreserved === true,
  historicalEvidencePreserved: repair.historicalEvidencePreserved === true,
  unexpectedDatabaseDeletedCount: repair.unexpectedDatabaseDeletedCount ?? null,
  unexpectedProcessKilledCount: repair.unexpectedProcessKilledCount ?? null,
  failed,
  failedCount: failed.length,
  attempts: attempts.map((name) => `artifacts/p7-v2/cleanup-attempts/${name}`),
  generatedAt: new Date().toISOString(),
};

writeJSON('docs/p7-v2-r3b-pre-formal-cleanup-v2-final-gate.json', report);
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
