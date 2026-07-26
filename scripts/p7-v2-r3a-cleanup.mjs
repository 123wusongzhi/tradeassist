import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { DB_PREFIX, root, runWSL, valueOf, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const args = process.argv.slice(2);
const approvalFile = valueOf(args, '--approval-file');
const databaseName = valueOf(args, '--drop-database');
const checkOnly = args.includes('--check-only') || !databaseName;
const safeName = /^trademind_p7v2_[a-zA-Z0-9_-]+$/;

function sha256(value) {
  return crypto.createHash('sha256').update(value).digest('hex');
}

function psql(sql) {
  return runWSL(`psql -h /var/run/postgresql -U root -d postgres -At -F '|' -v ON_ERROR_STOP=1 -c ${JSON.stringify(sql)}`, { timeout: 30000 });
}

function rowsFor(sql) {
  const result = psql(sql);
  return {
    result,
    rows: (result.stdout || '').split(/\r?\n/).filter(Boolean),
  };
}

const before = rowsFor(`SELECT datname FROM pg_database WHERE datname LIKE '${DB_PREFIX}%' ORDER BY datname;`);
const runtimeEnv = fs.existsSync(path.join(root, 'artifacts/p7-v2/runtime.env'))
  ? fs.readFileSync(path.join(root, 'artifacts/p7-v2/runtime.env'), 'utf8')
  : '';
const localHost = psql('SELECT CASE WHEN inet_server_addr() IS NULL THEN true ELSE false END;');
const environmentSafe =
  runtimeEnv.includes('APP_ENV=performance') &&
  runtimeEnv.includes('PERFORMANCE_TEST_MODE=true') &&
  !runtimeEnv.includes('APP_ENV=production') &&
  (localHost.stdout || '').trim() === 't';
const issues = [];
if (!environmentSafe) issues.push('local performance environment guard failed');
if (!checkOnly && !safeName.test(databaseName)) issues.push('database name rejected: exact P7-V2 name required');

let approvalHash = '';
let approvedNames = [];
if (!checkOnly) {
  if (!approvalFile || !fs.existsSync(path.join(root, approvalFile))) {
    issues.push('approval file is required');
  } else {
    const content = fs.readFileSync(path.join(root, approvalFile));
    approvalHash = sha256(content);
    try {
      const approval = JSON.parse(content.toString('utf8'));
      approvedNames = approval.approvedDatabaseNames || [];
      if (approval.approvalObtained !== true) issues.push('approval evidence is not marked obtained');
      if (!approvedNames.includes(databaseName)) issues.push('database is absent from exact approval list');
    } catch {
      issues.push('approval file is not valid JSON');
    }
  }
}

let terminatedPids = [];
let dropped = false;
if (!checkOnly && issues.length === 0) {
  const connections = rowsFor(`SELECT pid FROM pg_stat_activity WHERE datname = '${databaseName}' AND pid <> pg_backend_pid() ORDER BY pid;`);
  terminatedPids = connections.rows.map(Number);
  for (const pid of terminatedPids) {
    const terminate = psql(`SELECT pg_terminate_backend(${pid});`);
    if (terminate.status !== 0 || (terminate.stdout || '').trim() !== 't') issues.push(`failed to terminate pid ${pid}`);
  }
  if (issues.length === 0) {
    const drop = psql(`DROP DATABASE "${databaseName}";`);
    dropped = drop.status === 0;
    if (!dropped) issues.push(`DROP DATABASE failed for ${databaseName}`);
  }
}

const after = rowsFor(`SELECT datname FROM pg_database WHERE datname LIKE '${DB_PREFIX}%' ORDER BY datname;`);
const report = {
  phase: 'P7-V2-R3A',
  component: 'runtime-cleanup',
  commandMode: checkOnly ? 'check-only' : 'single-exact-drop',
  status: issues.length === 0 ? (checkOnly ? 'checked' : dropped ? 'dropped' : 'failed') : 'failed',
  environment: {
    appEnv: runtimeEnv.includes('APP_ENV=performance') ? 'performance' : '',
    performanceTestMode: runtimeEnv.includes('PERFORMANCE_TEST_MODE=true'),
    production: false,
    hostClass: 'wsl2_local_postgresql_socket',
  },
  approval: {
    required: true,
    approvalFile: approvalFile || '',
    approvalEvidenceHash: approvalHash,
    approvedDatabaseNames: approvedNames,
  },
  cleanup: {
    requestedDatabase: databaseName || '',
    databasesBefore: before.rows,
    connectionsTerminated: terminatedPids,
    databaseDropped: dropped ? databaseName : '',
    databasesAfter: after.rows,
    remainingDatabasesWithPrefix: after.rows.length,
  },
  issues,
};
writeJSON('docs/p7-v2-r3a-runtime-cleanup-report.json', report);
writeMarkdown('docs/P7_V2_R3A_RUNTIME_CLEANUP_REPORT.md', `# P7-V2-R3A Runtime Cleanup Report\n\nStatus: **${report.status}**\n\n| Field | Value |\n| --- | --- |\n| Mode | ${report.commandMode} |\n| Requested database | ${report.cleanup.requestedDatabase || '-'} |\n| Remaining prefix databases | ${report.cleanup.remainingDatabasesWithPrefix} |\n| Approval evidence hash | ${approvalHash || '-'} |\n\nIssues:\n${issues.length ? issues.map((issue) => `- ${issue}`).join('\n') : '- none'}\n`);
console.log(JSON.stringify(report, null, 2));
process.exit(issues.length === 0 ? 0 : 1);
