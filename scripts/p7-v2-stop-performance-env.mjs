import { spawnSync } from 'node:child_process';
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
import { classifyP7V2Database, summarizeCleanupClassifications } from './p7-v2-r3b-cleanup-classifier.mjs';

const args = process.argv.slice(2);
const portConfig = resolveP7V2PortConfig();
const runtime = readJSON('docs/p7-v2-runtime-environment.json') || {};
const dbName = valueOf(args, '--db-name') || runtime.dbName || '';
const checkOnly = args.includes('--check-only');

const issues = [];
if (!dbName.startsWith(DB_PREFIX)) issues.push(`database must use prefix ${DB_PREFIX}`);

function psql(sql) {
  return runWSL(`psql -h /var/run/postgresql -U root -d postgres -At -v ON_ERROR_STOP=1 -c ${JSON.stringify(sql)}`);
}

const remaining = psql(`SELECT datname FROM pg_database WHERE datname LIKE '${DB_PREFIX}%' ORDER BY datname;`);
const rows = remaining.status === 0 ? (remaining.stdout || '').trim().split('\n').filter(Boolean) : [];
const beforeClassifications = rows.map((databaseName) => classifyP7V2Database(databaseName));
const droppedDatabases = [];

if (!checkOnly) {
  stopP7V2Server({ portConfig });
  const targets = beforeClassifications
    .filter((item) => item.cleanupEligible === true || item.databaseName === dbName)
    .map((item) => item.databaseName);
  for (const target of [...new Set(targets)]) {
    const drop = psql(`DROP DATABASE IF EXISTS "${target.replaceAll('"', '""')}";`);
    if (drop.status !== 0) issues.push(`failed to drop database ${target}`);
    else droppedDatabases.push(target);
  }
}

const live = psql(`SELECT datname FROM pg_database WHERE datname LIKE '${DB_PREFIX}%' ORDER BY datname;`);
const liveRows = live.status === 0 ? (live.stdout || '').trim().split('\n').filter(Boolean) : [];
const classifications = liveRows.map((databaseName) => classifyP7V2Database(databaseName));
const summary = summarizeCleanupClassifications(classifications);
const proc = runWSL("pgrep -af 'p7v2|p7load|p7verify|k6.*p7v2' 2>/dev/null || true");
const procCount = (proc.stdout || '').trim().split('\n').filter((l) => l && !l.includes('pgrep -af')).length;
const ports = runWSL(`ss -ltn 'sport = :${portConfig.port}' 2>/dev/null | awk 'NR>1 {print $4}' || true`);
const portCount = (ports.stdout || '').trim().split('\n').filter(Boolean).length;
const cleanupPassed = issues.length === 0 &&
  summary.currentFormalResidualCount === 0 &&
  summary.failedAttemptResidualCount === 0 &&
  summary.unknownDatabaseCount === 0 &&
  procCount === 0 &&
  portCount === 0;

const report = {
  phase: 'P7-V2',
  component: 'runtime-cleanup',
  status: cleanupPassed ? 'passed' : issues.length ? 'failed' : 'incomplete',
  checkedAt: new Date().toISOString(),
  gitCommit: gitCommit(),
  databasePrefix: DB_PREFIX,
  droppedDatabase: checkOnly ? '' : dbName,
  droppedDatabases,
  remainingDatabases: liveRows,
  remainingDatabasesWithPrefix: liveRows.length,
  databaseClassifications: classifications,
  cleanupGateSemanticsValid: summary.unknownDatabaseCount === 0,
  ...summary,
  processesRemaining: procCount,
  portsRemaining: portCount,
  queryExecuted: remaining.status === 0,
  issues,
};

writeJSON('docs/p7-v2-runtime-cleanup-report.json', report);
writeMarkdown(
  'docs/P7_V2_RUNTIME_CLEANUP_REPORT.md',
  `# P7-V2 Runtime Cleanup Report

Status: ${report.status}

| Field | Value |
| --- | --- |
| Remaining DBs | ${report.remainingDatabasesWithPrefix} |
| Current formal residual DBs | ${report.currentFormalResidualCount} |
| Failed attempt residual DBs | ${report.failedAttemptResidualCount} |
| Historical evidence DBs | ${report.historicalEvidenceDatabaseCount} |
| Unknown DBs | ${report.unknownDatabaseCount} |
| Processes | ${report.processesRemaining} |
| Ports | ${report.portsRemaining} |
`,
);

console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
