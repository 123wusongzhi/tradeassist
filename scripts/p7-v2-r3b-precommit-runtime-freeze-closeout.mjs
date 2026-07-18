import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { gitCommit, readJSON, root, runWSL, safeDbName, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';
import { RUNTIME_FREEZE_PATH } from './p7-v2-r3b-lpc-r3-runtime-freeze.mjs';
import { freezeCurrentContract } from './p7-v2-runtime-freeze-scope.mjs';

export const PRECOMMIT_FREEZE_CLOSEOUT_JSON = 'docs/p7-v2-r3b-precommit-runtime-freeze-closeout.json';
export const PRECOMMIT_FREEZE_CLOSEOUT_MD = 'docs/P7_V2_R3B_PRECOMMIT_RUNTIME_FREEZE_CLOSEOUT.md';

function exists(relPath) {
  return fs.existsSync(path.join(root, relPath));
}

function jsonContainsRunId(relPath, runId) {
  const value = readJSON(relPath);
  return Boolean(value && JSON.stringify(value).includes(runId));
}

function databaseExists(runId) {
  const dbName = safeDbName(runId);
  const res = runWSL(
    `psql -h /var/run/postgresql -U root -d postgres -At -v ON_ERROR_STOP=1 -c "SELECT datname FROM pg_database WHERE datname='${dbName}'" 2>/dev/null || true`,
    { timeout: 30000 },
  );
  return String(res.stdout || '').trim() === dbName;
}

function listenerExists() {
  const listener = runWSL(`ss -ltn 'sport = :18080' 2>/dev/null | awk 'NR>1 {found=1} END {print found ? 1 : 0}'`, { timeout: 10000 });
  return Number(String(listener.stdout || '').trim()) > 0;
}

function pidExists(runId) {
  const pidFile = path.join(root, 'artifacts/p7-v2/server.pid');
  if (!fs.existsSync(pidFile)) return false;
  const runtimeEnv = fs.existsSync(path.join(root, 'artifacts/p7-v2/runtime.env'))
    ? fs.readFileSync(path.join(root, 'artifacts/p7-v2/runtime.env'), 'utf8')
    : '';
  return runtimeEnv.includes(runId);
}

export function auditRunIdConsumption(manifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {}) {
  const runIds = [
    manifest.baselineRunId,
    manifest.currentRunId,
    manifest.soakRunId,
    manifest.demoRun1Id,
    manifest.demoRun2Id,
  ].filter(Boolean);
  const listener = listenerExists();
  const audits = runIds.map((runId) => {
    const baselineLoad = exists(`artifacts/p7-v2/baseline/${runId}/baseline.summary.json`) || jsonContainsRunId(`docs/baselines/p7-v2-baseline-${runId}.json`, runId);
    const currentLoad = exists(`artifacts/p7-v2/current/${runId}/current.summary.json`) || jsonContainsRunId(`docs/currents/p7-v2-current-${runId}.json`, runId) || jsonContainsRunId(`docs/runs/p7-v2-current-${runId}.json`, runId);
    const frozenBaseline = exists(`docs/baselines/frozen/${runId}/manifest.json`);
    const frozenCurrent = exists(`docs/currents/frozen/${runId}/manifest.json`);
    return {
      runId,
      databaseExists: databaseExists(runId),
      pidExists: pidExists(runId),
      listenerExists: listener,
      environmentArtifactExists: jsonContainsRunId('docs/p7-v2-runtime-environment.json', runId) || jsonContainsRunId('docs/p7-v2-environment-fingerprint.json', runId),
      datasetArtifactExists: jsonContainsRunId('docs/p7-v2-dataset-report.json', runId),
      loadArtifactExists: baselineLoad || currentLoad,
      frozenLoadArtifactExists: frozenBaseline || frozenCurrent,
      registryConsumed: jsonContainsRunId('docs/baselines/p7-v2-baseline-registry.json', runId) ||
        jsonContainsRunId('docs/currents/p7-v2-current-registry.json', runId) ||
        jsonContainsRunId('docs/fingerprints/p7-v2/load-profile-registry.json', runId),
    };
  });
  const runIdsConsumed = audits.some((audit) =>
    audit.databaseExists ||
    audit.pidExists ||
    audit.listenerExists ||
    audit.environmentArtifactExists ||
    audit.datasetArtifactExists ||
    audit.loadArtifactExists ||
    audit.frozenLoadArtifactExists ||
    audit.registryConsumed);
  return {
    runIds,
    audits,
    runIdsConsumed,
    runIdsRetained: !runIdsConsumed,
    newRunIdsCreated: false,
  };
}

function supersedeCurrentFreeze(oldFreezeId) {
  const doc = readJSON(RUNTIME_FREEZE_PATH) || {};
  const current = freezeCurrentContract(doc) || {};
  const history = Array.isArray(doc.history) ? [...doc.history] : [];
  if (!current.runtimeFreezeId || current.runtimeFreezeId !== oldFreezeId) return false;
  const superseded = {
    ...current,
    status: 'superseded',
    active: false,
    validForHistoricalAudit: true,
    validForFormalExecution: false,
    validForClosure: false,
    superseded: true,
    reason: 'runtime_freeze_created_from_uncommitted_immutable_state',
    supersededAt: new Date().toISOString(),
  };
  writeJSON(RUNTIME_FREEZE_PATH, { current: superseded, history });
  return true;
}

function restoreManifestNoActiveFreeze(manifest, audit, oldRuntimeFreezeId) {
  const next = {
    ...manifest,
    status: 'planned',
    active: true,
    validForExecution: true,
    formalExecutionStarted: false,
    executionStarted: false,
    environmentStarted: false,
    datasetExecuted: false,
    baselineExecuted: false,
    currentExecuted: false,
    comparabilityExecuted: false,
    regressionExecuted: false,
    runtimeFreezeId: null,
    runtimeFreezeCreated: false,
    runtimeFreezeLifecycleVersion: 3,
    oldRuntimeFreezeId,
    oldRuntimeFreezeSuperseded: true,
    runIdsConsumed: audit.runIdsConsumed,
    runIdsRetained: audit.runIdsRetained,
    newRunIdsCreated: false,
    reason: 'precommit_runtime_freeze_superseded_before_formal_execution',
    updatedAt: new Date().toISOString(),
  };
  writeJSON('docs/p7-v2-r3b-run-manifest.json', next);
  return next;
}

export function closeoutPrecommitRuntimeFreeze({ updateManifest = true } = {}) {
  const manifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {};
  const revalidation = readJSON('docs/p7-v2-r3b-runtime-freeze-revalidation.json') || {};
  const freezeDoc = readJSON(RUNTIME_FREEZE_PATH) || {};
  const currentFreeze = freezeCurrentContract(freezeDoc) || {};
  const oldRuntimeFreezeId = manifest.runtimeFreezeId || currentFreeze.runtimeFreezeId || '';
  const capturedHead = currentFreeze.freezeCreationGitHead || currentFreeze.git?.commit || currentFreeze.planBindingPayload?.planCheckpoint || '';
  const currentHead = gitCommit();
  const headMatch = Boolean(capturedHead && capturedHead === currentHead);
  const immutableInputMatch = revalidation.runtimeFreezeStillValid === true && revalidation.gitCommitMatch === true && headMatch;
  const audit = auditRunIdConsumption(manifest);
  const superseded = supersedeCurrentFreeze(oldRuntimeFreezeId);
  const manifestAfterCloseout = updateManifest ? restoreManifestNoActiveFreeze(manifest, audit, oldRuntimeFreezeId) : manifest;
  const report = {
    phase: 'P7-V2-R3B-PRECOMMIT-RUNTIME-FREEZE-CLOSEOUT',
    status: superseded ? 'passed' : 'failed',
    oldRuntimeFreezeId,
    primaryRootCause: 'runtime_freeze_created_from_uncommitted_immutable_state',
    createdFromCleanCommittedHead: false,
    createdFromUncommittedImmutableState: true,
    capturedHead,
    currentHead,
    headMatch,
    immutableInputMatch,
    passedRevalidationEvidenceCreatedBeforeCommit: revalidation.status === 'passed',
    passedRevalidationValidForCurrentHead: false,
    validForHistoricalAudit: true,
    validForFormalExecution: false,
    validForClosure: false,
    superseded,
    formalExecutionStarted: manifest.formalExecutionStarted === true,
    runIdsConsumed: audit.runIdsConsumed,
    runIdsRetained: audit.runIdsRetained,
    newRunIdsCreated: audit.newRunIdsCreated,
    runIdAudit: audit.audits,
    manifestStatusBefore: manifest.status || '',
    manifestStatusAfter: manifestAfterCloseout.status || '',
    runtimeFreezeCreatedAfter: manifestAfterCloseout.runtimeFreezeCreated === true,
    generatedAt: new Date().toISOString(),
  };
  writeJSON(PRECOMMIT_FREEZE_CLOSEOUT_JSON, report);
  writeMarkdown(
    PRECOMMIT_FREEZE_CLOSEOUT_MD,
    `# P7-V2-R3B Precommit Runtime Freeze Closeout\n\nStatus: **${report.status}**\n\n- Old runtime freeze ID: \`${report.oldRuntimeFreezeId}\`\n- Created from clean committed HEAD: \`${report.createdFromCleanCommittedHead}\`\n- Created from uncommitted immutable state: \`${report.createdFromUncommittedImmutableState}\`\n- Captured HEAD: \`${report.capturedHead || 'unknown'}\`\n- Current HEAD: \`${report.currentHead}\`\n- Valid for historical audit: \`${report.validForHistoricalAudit}\`\n- Valid for formal execution: \`${report.validForFormalExecution}\`\n- Superseded: \`${report.superseded}\`\n- Run IDs consumed: \`${report.runIdsConsumed}\`\n`,
  );
  return report;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = closeoutPrecommitRuntimeFreeze({ updateManifest: !process.argv.includes('--no-manifest-update') });
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}
