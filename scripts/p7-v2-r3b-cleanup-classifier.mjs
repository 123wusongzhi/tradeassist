import fs from 'node:fs';
import path from 'node:path';
import { DB_PREFIX, readJSON, root, safeDbName } from './p7-v2-lib.mjs';

function normalized(value) {
  return String(value || '').toLowerCase().replace(/[^a-z0-9_]+/g, '_').replace(/^_+|_+$/g, '');
}

function collectRegistryRunIds() {
  const baselineRegistry = readJSON('docs/baselines/p7-v2-baseline-registry.json') || {};
  const currentRegistry = readJSON('docs/currents/p7-v2-current-registry.json') || {};
  const baselineIds = (baselineRegistry.baselines || []).map((item) => item.runId).filter(Boolean);
  const currentIds = (currentRegistry.entries || []).map((item) => item.runId).filter(Boolean);
  return new Set([...baselineIds, ...currentIds]);
}

function collectManifestRunIds(manifest) {
  const keys = ['baselineRunId', 'currentRunId', 'soakRunId', 'demoRun1Id', 'demoRun2Id'];
  return keys.map((key) => manifest?.[key]).filter(Boolean);
}

function dbMatchesRunId(databaseName, runId) {
  const db = normalized(databaseName);
  const run = normalized(runId);
  return db === normalized(safeDbName(runId)) || db.includes(run);
}

function artifactFrozenForRunId(runId) {
  return fs.existsSync(path.join(root, 'docs', 'baselines', 'frozen', runId)) ||
    fs.existsSync(path.join(root, 'docs', 'currents', 'frozen', runId));
}

export function classifyP7V2Database(databaseName, {
  manifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {},
  finalReport = readJSON('docs/p7-v2-r3b-soak-semantics-final-report.json') || {},
  registryRunIds = collectRegistryRunIds(),
} = {}) {
  if (!String(databaseName || '').startsWith(DB_PREFIX)) {
    return {
      databaseName,
      associatedRunId: '',
      associatedRuntimeFreezeId: '',
      registryStatus: 'not_trademind_p7v2',
      artifactFrozen: false,
      evidenceRetentionRequired: false,
      classification: 'non_trademind_resource',
      cleanupEligible: false,
      reason: 'database does not use the P7-V2 prefix',
    };
  }

  const currentRunIds = collectManifestRunIds(manifest);
  const currentRunId = currentRunIds.find((runId) => dbMatchesRunId(databaseName, runId));
  if (currentRunId) {
    return {
      databaseName,
      associatedRunId: currentRunId,
      associatedRuntimeFreezeId: manifest.runtimeFreezeId || '',
      registryStatus: 'current_manifest',
      artifactFrozen: artifactFrozenForRunId(currentRunId),
      evidenceRetentionRequired: false,
      classification: 'current_formal_run_resource',
      cleanupEligible: true,
      reason: 'database belongs to the current formal R3B manifest',
    };
  }

  const failedBaselineRunId = finalReport.baselineRunId || '';
  if (failedBaselineRunId && dbMatchesRunId(databaseName, failedBaselineRunId) && finalReport.status === 'incomplete') {
    return {
      databaseName,
      associatedRunId: failedBaselineRunId,
      associatedRuntimeFreezeId: finalReport.runtimeFreezeId || '',
      registryStatus: 'failed_attempt',
      artifactFrozen: false,
      evidenceRetentionRequired: false,
      classification: 'failed_current_attempt_resource',
      cleanupEligible: true,
      reason: finalReport.failureClassification || 'failed current formal attempt',
    };
  }

  const registeredRunId = [...registryRunIds].find((runId) => dbMatchesRunId(databaseName, runId));
  if (registeredRunId) {
    return {
      databaseName,
      associatedRunId: registeredRunId,
      associatedRuntimeFreezeId: '',
      registryStatus: 'registered_historical_evidence',
      artifactFrozen: artifactFrozenForRunId(registeredRunId),
      evidenceRetentionRequired: true,
      classification: 'historical_evidence_resource',
      cleanupEligible: false,
      reason: 'registered baseline/current evidence is retained and not counted as current residual',
    };
  }

  const db = normalized(databaseName);
  if (
    db.includes('baseline_r3b') ||
    db.includes('current_r3b') ||
    db.includes('port_r2') ||
    db.includes('diagnostic')
  ) {
    return {
      databaseName,
      associatedRunId: '',
      associatedRuntimeFreezeId: '',
      registryStatus: 'superseded_or_diagnostic_history',
      artifactFrozen: false,
      evidenceRetentionRequired: true,
      classification: 'superseded_run_resource',
      cleanupEligible: false,
      reason: 'known superseded P7-V2 formal/diagnostic database retained for historical audit context',
    };
  }

  return {
    databaseName,
    associatedRunId: '',
    associatedRuntimeFreezeId: '',
    registryStatus: 'unclassified',
    artifactFrozen: false,
    evidenceRetentionRequired: false,
    classification: 'unknown_resource',
    cleanupEligible: false,
    reason: 'no registry, manifest, failed-attempt, or known diagnostic binding matched this database',
  };
}

export function summarizeCleanupClassifications(classifications = []) {
  const count = (classification) => classifications.filter((item) => item.classification === classification).length;
  return {
    currentFormalResidualCount: count('current_formal_run_resource'),
    failedAttemptResidualCount: count('failed_current_attempt_resource'),
    historicalEvidenceDatabaseCount: count('historical_evidence_resource'),
    supersededRunDatabaseCount: count('superseded_run_resource'),
    unknownDatabaseCount: count('unknown_resource'),
    nonTradeMindDatabaseCount: count('non_trademind_resource'),
  };
}
