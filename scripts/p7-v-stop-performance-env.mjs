import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const args = process.argv.slice(2);
const runId = valueOf('--run-id') || '';
const stopContainers = args.includes('--stop-containers');
const reportPath = path.join(docs, 'p7-v-performance-environment-stop.json');
const startReport = readJSON('docs/p7-v-performance-environment.json') || {};
const dbName = valueOf('--db-name') || (runId ? `trademind_p7_${safeName(runId)}` : startReport.databaseName || '');
const tempDataDir = valueOf('--data-dir') || startReport.tempDataDir || '';

function valueOf(name) {
  const idx = args.indexOf(name);
  if (idx >= 0 && args[idx + 1]) return args[idx + 1];
  const prefix = `${name}=`;
  const hit = args.find((arg) => arg.startsWith(prefix));
  return hit ? hit.slice(prefix.length) : '';
}

function safeName(value) {
  return String(value).toLowerCase().replace(/[^a-z0-9_]+/g, '_').replace(/^_+|_+$/g, '').slice(0, 48) || 'run';
}

function run(command, commandArgs, timeout = 120000) {
  const res = spawnSync(command, commandArgs, {
    cwd: root,
    env: process.env,
    encoding: 'utf8',
    timeout,
    maxBuffer: 4 * 1024 * 1024,
  });
  return {
    command: `${command} ${commandArgs.join(' ')}`,
    status: res.status ?? 1,
    stdout: (res.stdout || '').slice(0, 4000),
    stderr: (res.stderr || '').slice(0, 4000),
  };
}

const steps = [];
const issues = [];
if (!dbName || !/^trademind_p7_[a-z0-9_]+$/.test(dbName)) {
  issues.push(`refusing to drop database without safe trademind_p7_ name: ${dbName || '<empty>'}`);
} else {
  const terminateSQL = `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${dbName}' AND pid <> pg_backend_pid();`;
  const dropSQL = `DROP DATABASE IF EXISTS ${quoteIdent(dbName)};`;
  let terminate = run('docker', ['exec', 'trademind-postgres', 'psql', '-U', 'trademind', '-d', 'postgres', '-v', 'ON_ERROR_STOP=1', '-c', terminateSQL]);
  let drop = run('docker', ['exec', 'trademind-postgres', 'psql', '-U', 'trademind', '-d', 'postgres', '-v', 'ON_ERROR_STOP=1', '-c', dropSQL]);
  if (terminate.status !== 0 || drop.status !== 0) {
    terminate = runWithEnv('psql', [
      '-h',
      process.env.P7_DB_HOST || startReport.env?.DB_HOST || '127.0.0.1',
      '-p',
      process.env.P7_DB_PORT || startReport.env?.DB_PORT || '5432',
      '-U',
      process.env.P7_DB_USER || process.env.DB_USER || startReport.env?.DB_USER || 'trademind',
      '-d',
      'postgres',
      '-v',
      'ON_ERROR_STOP=1',
      '-c',
      terminateSQL,
    ], { PGPASSWORD: process.env.P7_DB_PASSWORD || process.env.DB_PASSWORD || (startReport.environmentMode === 'local_ephemeral_postgres' ? 'p7v_local_password' : 'trademind') });
    drop = runWithEnv('psql', [
      '-h',
      process.env.P7_DB_HOST || startReport.env?.DB_HOST || '127.0.0.1',
      '-p',
      process.env.P7_DB_PORT || startReport.env?.DB_PORT || '5432',
      '-U',
      process.env.P7_DB_USER || process.env.DB_USER || startReport.env?.DB_USER || 'trademind',
      '-d',
      'postgres',
      '-v',
      'ON_ERROR_STOP=1',
      '-c',
      dropSQL,
    ], { PGPASSWORD: process.env.P7_DB_PASSWORD || process.env.DB_PASSWORD || (startReport.environmentMode === 'local_ephemeral_postgres' ? 'p7v_local_password' : 'trademind') });
  }
  steps.push({ id: 'terminate-database-connections', ...terminate });
  steps.push({ id: 'drop-database', ...drop });
  if (drop.status !== 0) issues.push('failed to drop isolated PostgreSQL database');
}

if (stopContainers) {
  const stop = run('docker', ['compose', 'stop', 'postgres', 'redis'], 300000);
  steps.push({ id: 'docker-compose-stop', ...stop });
  if (stop.status !== 0) issues.push('failed to stop Docker postgres/redis containers');
}

if (tempDataDir) {
  const stopPg = run('pg_ctl', ['-D', tempDataDir, '-m', 'fast', '-w', 'stop'], 30000);
  steps.push({ id: 'ephemeral-pg-ctl-stop', ...stopPg });
  if (stopPg.status !== 0) issues.push('failed to stop temporary PostgreSQL cluster');
}

const report = {
  phase: 'P7-V',
  status: issues.length === 0 ? 'stopped' : 'blocked',
  runId,
  databaseName: dbName,
  stoppedContainers: stopContainers,
  tempDataDir,
  steps,
  issues,
  productionReady: false,
  finishedAt: new Date().toISOString(),
};

fs.mkdirSync(docs, { recursive: true });
fs.writeFileSync(reportPath, `${JSON.stringify(report, null, 2)}\n`);
console.log(JSON.stringify({ phase: 'P7-V', status: report.status, databaseName: dbName, report: path.relative(root, reportPath) }, null, 2));
process.exit(report.status === 'stopped' ? 0 : 1);

function quoteIdent(value) {
  return `"${value.replaceAll('"', '""')}"`;
}

function readJSON(rel) {
  try {
    return JSON.parse(fs.readFileSync(path.join(root, rel), 'utf8'));
  } catch {
    return null;
  }
}

function runWithEnv(command, commandArgs, env, timeout = 120000) {
  const res = spawnSync(command, commandArgs, {
    cwd: root,
    env: { ...process.env, ...env },
    encoding: 'utf8',
    timeout,
    maxBuffer: 4 * 1024 * 1024,
  });
  return {
    command: `${command} ${commandArgs.join(' ')}`,
    status: res.status ?? 1,
    stdout: (res.stdout || '').slice(0, 4000),
    stderr: (res.stderr || '').slice(0, 4000),
  };
}
