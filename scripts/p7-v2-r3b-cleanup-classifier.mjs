import fs from 'node:fs';
import path from 'node:path';
import { DB_PREFIX, readJSON, root, safeDbName } from './p7-v2-lib.mjs';

function normalized(value) {
  return String(value || '')
    .toLowerCase()
    .replace(/[^a-z0-9_]+/g, '_')
    .replace(/^_+|_+$/g, '');
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

function collectKnownDiagnosticRuns() {
  const finalGate = readJSON('docs/p7-v2-r3b-sql-fingerprint-diagnostics-final-gate.json') || {};
  const pairResult = readJSON('docs/p7-v2-r3b-sql-fingerprint-diagnostics-pair-result.json') || {};
  const runs = [];
  for (const role of ['baseline', 'current']) {
    const item = pairResult?.[role] || {};
    const runId = item.runId || finalGate?.[`diagnostic${role === 'baseline' ? 'Baseline' : 'Current'}RunId`] || '';
    if (!runId) continue;
    runs.push({
      runId,
      databaseIdentity: item.databaseIdentity || safeDbName(runId),
      role,
      finalGatePassed: finalGate.status === 'passed' && Number(finalGate.failedCount || 0) === 0,
      formal: finalGate.formal === true,
      validForClosure: finalGate.validForClosure === true,
      rawJsonlPath: item.durableJsonlPath || '',
      diagnosticReportMatch: true,
    });
  }
  return runs;
}

export function dbMatchesRunId(databaseName, runId) {
  const db = normalized(databaseName);
  const run = normalized(runId);
  return db === normalized(safeDbName(runId)) || db.includes(run) || normalized(safeDbName(runId)).startsWith(db);
}

export function artifactFrozenForRunId(runId) {
  return fs.existsSync(path.join(root, 'docs', 'baselines', 'frozen', runId)) ||
    fs.existsSync(path.join(root, 'docs', 'currents', 'frozen', runId));
}

function runEvidencePath(runId) {
  const candidates = [
    path.join(root, 'docs', 'runs', `p7-v2-current-${runId}.json`),
    path.join(root, 'docs', 'baselines', `p7-v2-baseline-${runId}.json`),
    path.join(root, 'docs', 'currents', `p7-v2-current-${runId}.json`),
  ];
  return candidates.find((candidate) => fs.existsSync(candidate)) || '';
}

function baseResult(databaseName, extra) {
  return {
    databaseName,
    databaseOwner: extra.databaseOwner || '',
    databaseSizeBytes: Number(extra.databaseSizeBytes || 0),
    activeConnectionCount: Number(extra.activeConnectionCount || 0),
    derivedRunId: extra.derivedRunId || '',
    manifestMatch: Boolean(extra.manifestMatch),
    environmentEvidenceMatch: Boolean(extra.environmentEvidenceMatch),
    artifactMatch: Boolean(extra.artifactMatch),
    diagnosticReportMatch: Boolean(extra.diagnosticReportMatch),
    associatedRuntimeFreezeId: extra.associatedRuntimeFreezeId || '',
    classification: extra.classification,
    retentionDecision: extra.retentionDecision,
    cleanupEligible: Boolean(extra.cleanupEligible),
    reason: extra.reason || '',
  };
}

export function classifyP7V2Database(databaseName, {
  databaseOwner = '',
  databaseSizeBytes = 0,
  activeConnectionCount = 0,
  manifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {},
  runtime = readJSON('docs/p7-v2-runtime-environment.json') || {},
  registryRunIds = collectRegistryRunIds(),
  diagnosticRuns = collectKnownDiagnosticRuns(),
  targetRunId = '',
  targetDiagnosticRunId = '',
} = {}) {
  const common = { databaseOwner, databaseSizeBytes, activeConnectionCount };
  if (!String(databaseName || '').startsWith(DB_PREFIX)) {
    return baseResult(databaseName, {
      ...common,
      classification: 'unknown_database',
      retentionDecision: 'retain_and_stop',
      reason: 'database does not use the P7-V2 prefix',
    });
  }

  const manifestRunIds = collectManifestRunIds(manifest);
  const manifestRunId = manifestRunIds.find((runId) => dbMatchesRunId(databaseName, runId));
  const runtimeDbMatch = runtime?.dbName === databaseName;
  const explicitTargetRunMatch = targetRunId && dbMatchesRunId(databaseName, targetRunId);
  if (manifestRunId || runtimeDbMatch || explicitTargetRunMatch) {
    const derivedRunId = manifestRunId || targetRunId || runtime?.runId || '';
    return baseResult(databaseName, {
      ...common,
      derivedRunId,
      manifestMatch: Boolean(manifestRunId),
      environmentEvidenceMatch: Boolean(runtimeDbMatch),
      artifactMatch: artifactFrozenForRunId(derivedRunId),
      associatedRuntimeFreezeId: manifest.runtimeFreezeId || '',
      classification: 'old_formal_pair_residue',
      retentionDecision: explicitTargetRunMatch ? 'cleanup_when_execute_scope_matches' : 'cleanup_after_explicit_scope',
      cleanupEligible: Boolean(explicitTargetRunMatch),
      reason: 'database is bound to the stale formal pair manifest/runtime evidence',
    });
  }

  const diagnostic = diagnosticRuns.find((item) => dbMatchesRunId(databaseName, item.runId) || normalized(databaseName) === normalized(item.databaseIdentity));
  if (diagnostic) {
    const rawJsonlPreserved = diagnostic.rawJsonlPath ? fs.existsSync(diagnostic.rawJsonlPath.replace(/^\/mnt\/d\//, 'D:/').replace(/\//g, path.sep)) : false;
    const completedEphemeral =
      diagnostic.finalGatePassed &&
      diagnostic.formal === false &&
      diagnostic.validForClosure === false &&
      rawJsonlPreserved &&
      activeConnectionCount === 0;
    return baseResult(databaseName, {
      ...common,
      derivedRunId: diagnostic.runId,
      diagnosticReportMatch: diagnostic.diagnosticReportMatch,
      artifactMatch: rawJsonlPreserved,
      classification: completedEphemeral ? 'completed_diagnostic_ephemeral' : 'active_or_required_diagnostic_evidence',
      retentionDecision: targetDiagnosticRunId === diagnostic.runId && completedEphemeral ? 'cleanup_when_execute_scope_matches' : 'retain',
      cleanupEligible: targetDiagnosticRunId === diagnostic.runId && completedEphemeral,
      reason: completedEphemeral
        ? 'diagnostic final gate and raw JSONL evidence are preserved'
        : 'diagnostic evidence is still active or insufficiently checkpointed',
    });
  }

  const registeredRunId = [...registryRunIds].find((runId) => dbMatchesRunId(databaseName, runId));
  if (registeredRunId) {
    return baseResult(databaseName, {
      ...common,
      derivedRunId: registeredRunId,
      artifactMatch: artifactFrozenForRunId(registeredRunId) || Boolean(runEvidencePath(registeredRunId)),
      classification: 'historical_formal_evidence_db',
      retentionDecision: 'retain',
      reason: 'registered baseline/current evidence is retained',
    });
  }

  const db = normalized(databaseName);
  if (
    db.includes('baseline_r3b') ||
    db.includes('current_r3b') ||
    db.includes('port_r2') ||
    db.includes('diagnostic') ||
    db.includes('diag_')
  ) {
    return baseResult(databaseName, {
      ...common,
      artifactMatch: true,
      classification: 'historical_formal_evidence_db',
      retentionDecision: 'retain',
      reason: 'database name matches a known historical P7-V2 formal/diagnostic evidence family',
    });
  }

  return baseResult(databaseName, {
    ...common,
    classification: 'unknown_database',
    retentionDecision: 'retain_and_stop',
    reason: 'no run manifest, registry, diagnostic report, or historical evidence binding matched this database',
  });
}

export function summarizeCleanupClassifications(classifications = []) {
  const count = (classification) => classifications.filter((item) => item.classification === classification).length;
  const knownRetainedDiagnosticDatabaseCount = classifications.filter(
    (item) =>
      item.classification === 'active_or_required_diagnostic_evidence' ||
      (item.classification === 'completed_diagnostic_ephemeral' && item.retentionDecision === 'retain'),
  ).length;
  return {
    currentFormalResidualCount: count('old_formal_pair_residue'),
    oldFormalPairResidueCount: count('old_formal_pair_residue'),
    historicalEvidenceDatabaseCount: count('historical_formal_evidence_db'),
    historicalFormalEvidenceDatabaseCount: count('historical_formal_evidence_db'),
    diagnosticDatabaseCount: count('completed_diagnostic_ephemeral') + count('active_or_required_diagnostic_evidence'),
    completedDiagnosticEphemeralCount: count('completed_diagnostic_ephemeral'),
    knownRetainedDiagnosticDatabaseCount,
    activeRequiredDiagnosticDatabaseCount: count('active_or_required_diagnostic_evidence'),
    unknownDatabaseCount: count('unknown_database'),
  };
}
