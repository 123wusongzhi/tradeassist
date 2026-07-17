import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import {
  DB_PREFIX,
  gitCommit,
  readJSON,
  resolveP7V2PortConfig,
  root,
  runWSL,
  stopP7V2Server,
  valueOf,
  writeJSON,
  writeMarkdown,
} from './p7-v2-lib.mjs';
import { classifyP7V2Database, dbMatchesRunId, summarizeCleanupClassifications } from './p7-v2-r3b-cleanup-classifier.mjs';

const args = process.argv.slice(2);
const mode = args.includes('--execute') ? 'execute' : 'check';
const targetScope = {
  runId: valueOf(args, '--run-id'),
  databaseName: valueOf(args, '--database-name') || valueOf(args, '--db-name'),
  pid: valueOf(args, '--pid'),
  diagnosticRunId: valueOf(args, '--diagnostic-run-id'),
};
const startedAt = new Date().toISOString();
const cleanupAttemptId = `cleanup-v2-${startedAt.replace(/[-:.TZ]/g, '').slice(0, 14)}-${crypto.randomBytes(4).toString('hex')}`;
const attemptsDir = path.join(root, 'artifacts', 'p7-v2', 'cleanup-attempts');
const attemptRelPath = `artifacts/p7-v2/cleanup-attempts/${cleanupAttemptId}.json`;
const attemptPath = path.join(root, attemptRelPath);
const portConfig = resolveP7V2PortConfig();
const manifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {};
const runtime = readJSON('docs/p7-v2-runtime-environment.json') || {};
const issues = [];

function quoteIdent(value) {
  return `"${String(value).replaceAll('"', '""')}"`;
}

function psql(sql, timeout = 30000) {
  const oneLineSql = String(sql).replace(/\s+/g, ' ').trim();
  return runWSL(`psql -h /var/run/postgresql -U root -d postgres -At -v ON_ERROR_STOP=1 -c ${JSON.stringify(oneLineSql)}`, {
    timeout,
    maxBuffer: 10 * 1024 * 1024,
  });
}

function splitRows(stdout) {
  return String(stdout || '').trim().split('\n').filter(Boolean);
}

function databaseInventory() {
  const sql = `
SELECT
  d.datname,
  pg_get_userbyid(d.datdba) AS owner,
  pg_database_size(d.datname) AS size_bytes,
  COALESCE(a.active_connections, 0) AS active_connections
FROM pg_database d
LEFT JOIN (
  SELECT datname, count(*) AS active_connections
  FROM pg_stat_activity
  WHERE datname LIKE '${DB_PREFIX}%'
  GROUP BY datname
) a ON a.datname = d.datname
WHERE d.datname LIKE '${DB_PREFIX}%'
ORDER BY d.datname;`;
  const query = psql(sql);
  if (query.status !== 0) {
    issues.push('database inventory query failed');
    return { queryExecuted: false, databases: [] };
  }
  const databases = splitRows(query.stdout).map((line) => {
    const [databaseName, databaseOwner, databaseSizeBytes, activeConnectionCount] = line.split('|');
    return { databaseName, databaseOwner, databaseSizeBytes: Number(databaseSizeBytes || 0), activeConnectionCount: Number(activeConnectionCount || 0) };
  });
  return { queryExecuted: true, databases };
}

function targetDatabaseMatches(databaseName) {
  if (targetScope.databaseName && databaseName === targetScope.databaseName) return true;
  if (targetScope.runId && dbMatchesRunId(databaseName, targetScope.runId)) return true;
  if (targetScope.diagnosticRunId && dbMatchesRunId(databaseName, targetScope.diagnosticRunId)) return true;
  return false;
}

function listenerAudit() {
  const script = String.raw`
port=__PORT__
pid=$(ss -ltnp 2>/dev/null | sed -n "s/.*127\.0\.0\.1:__PORT__.*pid=\([0-9]\+\).*/\1/p" | head -n1)
count=$(ss -ltnp 2>/dev/null | awk '$4 ~ /127\.0\.0\.1:__PORT__$/ {c++} END {print c+0}')
if [ -z "$pid" ]; then
  printf 'listener18080Count=%s\n' "$count"
  exit 0
fi
printf 'listener18080Count=%s\n' "$count"
printf 'pid=%s\n' "$pid"
printf 'exe=%s\n' "$(readlink -f "/proc/$pid/exe" 2>/dev/null || true)"
printf 'cwd=%s\n' "$(readlink -f "/proc/$pid/cwd" 2>/dev/null || true)"
printf 'startedAt=%s\n' "$(ps -o lstart= -p "$pid" 2>/dev/null | sed 's/^ *//')"
printf 'cmdline=%s\n' "$(tr '\0' ' ' < "/proc/$pid/cmdline" 2>/dev/null)"
tr '\0' '\n' < "/proc/$pid/environ" 2>/dev/null |
  grep -E '^(DB_NAME|P7V2_INSTANCE_NONCE|P7_V2_API_PORT|P7_BASE_URL|P7_V2_API_HOST)=' || true
`.replaceAll('__PORT__', String(portConfig.port));
  const res = runWSL(script, { timeout: 30000 });
  const out = Object.fromEntries(splitRows(res.stdout).map((line) => {
    const idx = line.indexOf('=');
    return idx >= 0 ? [line.slice(0, idx), line.slice(idx + 1)] : [line, ''];
  }));
  const listener18080Count = Number(out.listener18080Count || 0);
  const pid = out.pid || '';
  const dbName = out.DB_NAME || '';
  const executable = out.exe || '';
  const cwd = out.cwd || '';
  const runIdMatch = targetScope.runId ? dbMatchesRunId(dbName, targetScope.runId) : false;
  const databaseMatch = targetScope.databaseName ? dbName === targetScope.databaseName : false;
  const pidMatch = targetScope.pid ? pid === targetScope.pid : false;
  const processLooksP7 = executable.endsWith('/artifacts/p7-v2/server') && cwd.replace(/\\/g, '/').endsWith('/trademind-ai');
  let ownership = 'unknown_process';
  if (!pid) ownership = 'none';
  else if (processLooksP7 && (runIdMatch || databaseMatch || pidMatch)) ownership = 'old_formal_current_owned_process';
  else if (processLooksP7) ownership = 'other_known_p7_process';
  else ownership = 'non_p7_process';
  return {
    listener18080Count,
    listenerPid: pid,
    listenerOwnership: ownership,
    listenerRunId: targetScope.runId || '',
    listenerDatabaseName: dbName,
    listenerExecutable: executable,
    listenerWorkingDirectory: cwd,
    listenerStartedAt: out.startedAt || '',
    listenerInstanceNonce: out.P7V2_INSTANCE_NONCE || '',
    listenerCommandLine: out.cmdline || '',
    queryStatus: res.status,
    queryStderr: res.stderr || '',
  };
}

function connectionAudit(databaseName) {
  if (!databaseName) return { connections: [], unknownConnectionCount: 0 };
  const dbLiteral = `'${databaseName.replaceAll("'", "''")}'`;
  const sql = `
SELECT
  pid,
  usename,
  application_name,
  COALESCE(client_addr::text, ''),
  COALESCE(client_port::text, ''),
  backend_start,
  state,
  COALESCE(wait_event_type, ''),
  COALESCE(wait_event, ''),
  query_start,
  left(query, 200)
FROM pg_stat_activity
WHERE datname = ${dbLiteral}
ORDER BY pid;`;
  const query = psql(sql);
  if (query.status !== 0) {
    return { connections: [], unknownConnectionCount: 1, issue: 'database connection audit failed' };
  }
  const connections = splitRows(query.stdout).map((line) => {
    const [connectionPid, usename, applicationName, clientAddr, clientPort, backendStart, state, waitEventType, waitEvent, queryStart, queryPreview] = line.split('|');
    return {
      connectionPid,
      usename,
      applicationName,
      clientAddr,
      clientPort,
      backendStart,
      state,
      waitEventType,
      waitEvent,
      queryStart,
      queryPreview,
      connectionOwnership: targetDatabaseMatches(databaseName) ? 'old_current_server_connection' : 'unknown_connection',
    };
  });
  return {
    connections,
    unknownConnectionCount: connections.filter((item) => item.connectionOwnership === 'unknown_connection').length,
  };
}

function plannedActionsFor(classifications, listener) {
  const actions = [];
  const targetDatabases = classifications.filter((item) => item.cleanupEligible && targetDatabaseMatches(item.databaseName));
  if (listener.listenerOwnership === 'old_formal_current_owned_process') {
    actions.push({
      type: 'stop_process',
      resourceType: 'process',
      resourceId: listener.listenerPid,
      classification: listener.listenerOwnership,
      exactIdentity: {
        pid: listener.listenerPid,
        executable: listener.listenerExecutable,
        workingDirectory: listener.listenerWorkingDirectory,
        databaseName: listener.listenerDatabaseName,
        instanceNonce: listener.listenerInstanceNonce,
      },
    });
  }
  for (const db of targetDatabases) {
    actions.push({
      type: 'terminate_database_connections',
      resourceType: 'database_connections',
      resourceId: db.databaseName,
      classification: db.classification,
      exactIdentity: { databaseName: db.databaseName },
    });
    actions.push({
      type: 'drop_database',
      resourceType: 'database',
      resourceId: db.databaseName,
      classification: db.classification,
      exactIdentity: { databaseName: db.databaseName },
    });
  }
  return actions;
}

function executeActions(actions) {
  const executed = [];
  for (const action of actions) {
    const startedAtAction = new Date().toISOString();
    if (action.type === 'stop_process') {
      const stop = stopP7V2Server({ expectedIdentity: { pid: action.resourceId }, portConfig });
      executed.push({
        ...action,
        startedAt: startedAtAction,
        completedAt: new Date().toISOString(),
        status: stop.stopped ? 'passed' : 'failed',
        result: stop,
      });
      if (!stop.stopped) issues.push(`failed to stop process ${action.resourceId}`);
      continue;
    }
    if (action.type === 'terminate_database_connections') {
      const sql = `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${action.resourceId.replaceAll("'", "''")}' AND pid <> pg_backend_pid();`;
      const res = psql(sql);
      executed.push({
        ...action,
        startedAt: startedAtAction,
        completedAt: new Date().toISOString(),
        status: res.status === 0 ? 'passed' : 'failed',
        result: { status: res.status, stdout: res.stdout.trim(), stderr: res.stderr.trim() },
      });
      if (res.status !== 0) issues.push(`failed to terminate database connections for ${action.resourceId}`);
      continue;
    }
    if (action.type === 'drop_database') {
      const res = psql(`DROP DATABASE IF EXISTS ${quoteIdent(action.resourceId)};`, 60000);
      executed.push({
        ...action,
        startedAt: startedAtAction,
        completedAt: new Date().toISOString(),
        status: res.status === 0 ? 'passed' : 'failed',
        dropDatabasePlanned: true,
        dropDatabaseExecuted: res.status === 0,
        result: { status: res.status, stdout: res.stdout.trim(), stderr: res.stderr.trim() },
      });
      if (res.status !== 0) issues.push(`failed to drop database ${action.resourceId}`);
    }
  }
  return executed;
}

function verifyAfter(plannedActions) {
  const live = databaseInventory();
  const liveNames = new Set(live.databases.map((item) => item.databaseName));
  const listener = listenerAudit();
  const dropped = plannedActions.filter((action) => action.type === 'drop_database').map((action) => action.resourceId);
  const dropVerification = dropped.map((databaseName) => ({
    databaseName,
    dropDatabaseVerified: !liveNames.has(databaseName),
  }));
  return {
    queryExecuted: live.queryExecuted,
    listener18080Count: listener.listener18080Count,
    targetResidualCount: dropped.filter((databaseName) => liveNames.has(databaseName)).length,
    dropVerification,
  };
}

const inventory = databaseInventory();
const classifications = inventory.databases.map((item) =>
  classifyP7V2Database(item.databaseName, {
    ...item,
    manifest,
    runtime,
    targetRunId: targetScope.runId,
    targetDiagnosticRunId: targetScope.diagnosticRunId,
  }),
);
const summary = summarizeCleanupClassifications(classifications);
const listener = listenerAudit();
const targetDatabaseName =
  targetScope.databaseName ||
  classifications.find((item) => item.cleanupEligible && targetDatabaseMatches(item.databaseName))?.databaseName ||
  listener.listenerDatabaseName ||
  runtime.dbName ||
  '';
const connectionAuditResult = connectionAudit(targetDatabaseName);
if (connectionAuditResult.issue) issues.push(connectionAuditResult.issue);

const discoveredResources = [
  ...classifications.map((item) => ({ resourceType: 'database', ...item })),
  ...(listener.listenerPid ? [{ resourceType: 'process', ...listener }] : []),
  ...connectionAuditResult.connections.map((item) => ({ resourceType: 'database_connection', databaseName: targetDatabaseName, ...item })),
];
const plannedActions = plannedActionsFor(classifications, listener);
const unknownProcessCount = listener.listenerOwnership === 'unknown_process' ? 1 : 0;
const unknownConnectionCount = connectionAuditResult.unknownConnectionCount;
const explicitScopePresent = Boolean(targetScope.runId || targetScope.databaseName || targetScope.pid || targetScope.diagnosticRunId);
const destructivePlanCount = plannedActions.length;
const blockers = [
  ...(inventory.queryExecuted ? [] : ['database inventory unavailable']),
  ...(summary.unknownDatabaseCount > 0 ? ['unknown databases block execution'] : []),
  ...(unknownProcessCount > 0 ? ['unknown process blocks execution'] : []),
  ...(unknownConnectionCount > 0 ? ['unknown database connection blocks execution'] : []),
  ...(mode === 'execute' && !explicitScopePresent ? ['execute requires an explicit --run-id, --database-name, --pid, or --diagnostic-run-id scope'] : []),
  ...(mode === 'execute' && explicitScopePresent && destructivePlanCount === 0 ? ['execute scope matched no cleanup-eligible resources'] : []),
];
issues.push(...blockers);

let executedActions = [];
let verification = {
  queryExecuted: inventory.queryExecuted,
  listener18080Count: listener.listener18080Count,
  targetResidualCount: plannedActions.filter((action) => action.type === 'drop_database').length,
  dropVerification: [],
};
if (mode === 'execute' && blockers.length === 0) {
  executedActions = executeActions(plannedActions);
  verification = verifyAfter(plannedActions);
  for (const item of verification.dropVerification) {
    const executed = executedActions.find((action) => action.type === 'drop_database' && action.resourceId === item.databaseName);
    if (executed) executed.dropDatabaseVerified = item.dropDatabaseVerified;
  }
  if (verification.targetResidualCount > 0) issues.push('database still exists after execute');
  if (verification.listener18080Count > 0 && plannedActions.some((action) => action.type === 'stop_process')) issues.push('listener still active after execute');
}

const destructiveActionCount = mode === 'execute' ? executedActions.length : 0;
const unexpectedDatabaseDeletedCount = executedActions.filter((action) => action.type === 'drop_database' && action.classification === 'unknown_database').length;
const unexpectedProcessKilledCount = executedActions.filter((action) => action.type === 'stop_process' && action.classification !== 'old_formal_current_owned_process').length;
const globalCheckRequiresClean = mode === 'check' && !explicitScopePresent;
const semanticGatePassed =
  issues.length === 0 &&
  unexpectedDatabaseDeletedCount === 0 &&
  unexpectedProcessKilledCount === 0 &&
  (mode !== 'execute' || executedActions.every((action) => action.status === 'passed')) &&
  (!globalCheckRequiresClean || (summary.currentFormalResidualCount === 0 && listener.listener18080Count === 0));
const status = semanticGatePassed ? 'passed' : blockers.length > 0 ? 'blocked' : 'failed';
const completedAt = new Date().toISOString();

const attempt = {
  cleanupContractVersion: 2,
  cleanupAttemptId,
  mode,
  startedAt,
  completedAt,
  targetScope,
  discoveredResources,
  classificationSummary: {
    ...summary,
    unknownProcessCount,
    unknownConnectionCount,
    listener18080Count: listener.listener18080Count,
  },
  plannedActions,
  executedActions,
  verification,
  semanticGatePassed,
  status,
  destructiveActionCount,
  unexpectedDatabaseDeletedCount,
  unexpectedProcessKilledCount,
  historicalFrozenEvidencePreserved: true,
  historicalEvidencePreserved: true,
  issues,
};

writeJSON(attemptRelPath, attempt);
const historicalAttemptCount = fs.existsSync(attemptsDir)
  ? fs.readdirSync(attemptsDir).filter((name) => name.endsWith('.json')).length
  : 1;
const summaryReport = {
  cleanupContractVersion: 2,
  latestAttemptId: cleanupAttemptId,
  latestAttemptPath: attemptRelPath,
  historicalAttemptCount,
  latestStatus: status,
  latestMode: mode,
  semanticGatePassed,
  currentFormalResidualCount: summary.currentFormalResidualCount,
  unknownDatabaseCount: summary.unknownDatabaseCount,
  unknownProcessCount,
  unknownConnectionCount,
  listener18080Count: verification.listener18080Count,
  knownRetainedDiagnosticDatabaseCount: summary.knownRetainedDiagnosticDatabaseCount,
  completedDiagnosticEphemeralCount: summary.completedDiagnosticEphemeralCount,
  destructiveActionCount,
  droppedDatabases: executedActions.filter((action) => action.type === 'drop_database' && action.status === 'passed').map((action) => action.resourceId),
  actionHistoryPreserved: true,
  checkDoesNotOverwriteExecute: true,
};
writeJSON('docs/p7-v2-runtime-cleanup-report.json', summaryReport);
writeMarkdown(
  'docs/P7_V2_RUNTIME_CLEANUP_REPORT.md',
  `# P7-V2 Runtime Cleanup Report

Status: ${status}

| Field | Value |
| --- | --- |
| Cleanup contract | 2 |
| Latest attempt | ${cleanupAttemptId} |
| Latest mode | ${mode} |
| Attempt path | ${attemptRelPath} |
| Historical attempts | ${historicalAttemptCount} |
| Current formal residuals | ${summary.currentFormalResidualCount} |
| Unknown databases | ${summary.unknownDatabaseCount} |
| Unknown processes | ${unknownProcessCount} |
| Unknown connections | ${unknownConnectionCount} |
| Listener ${portConfig.port} count | ${verification.listener18080Count} |
| Known retained diagnostic DBs | ${summary.knownRetainedDiagnosticDatabaseCount} |
| Completed diagnostic ephemeral DBs | ${summary.completedDiagnosticEphemeralCount} |
| Destructive actions | ${destructiveActionCount} |
`,
);

console.log(JSON.stringify(attempt, null, 2));
process.exit(status === 'passed' ? 0 : 1);
