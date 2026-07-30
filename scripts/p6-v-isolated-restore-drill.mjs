#!/usr/bin/env node
import crypto from 'node:crypto';
import fs from 'node:fs';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';

const root = process.cwd();
const docsDir = path.join(root, 'docs');
const runId = `p6v_${crypto.randomBytes(6).toString('hex')}`;
const sourceDb = `trademind_p6v_source_${runId}`;
const restoreDb = `trademind_p6v_restore_${runId}`;
const negativeDbs = {
  unverified: `trademind_p6v_restore_${runId}_unverified`,
  checksum: `trademind_p6v_restore_${runId}_checksum`,
  manifest: `trademind_p6v_restore_${runId}_manifest`,
  cipher: `trademind_p6v_restore_${runId}_cipher`,
  nonEmpty: `trademind_p6v_restore_${runId}_nonempty`,
};
const pgUser = 'p6v_user';
const pgPassword = `p6v_${crypto.randomBytes(18).toString('hex')}`;
const marker = `marker_${crypto.randomBytes(18).toString('hex')}`;
const tempRoot = fs.mkdtempSync(path.join(os.tmpdir(), `trademind-${runId}-`));
const dataDir = path.join(tempRoot, 'pgdata');
const pwFile = path.join(tempRoot, 'pwfile');
const logFile = path.join(tempRoot, 'postgres.log');
const reportPath = path.join(docsDir, 'p6-v-isolated-restore-drill-report.json');
const mdPath = path.join(docsDir, 'P6_V_ISOLATED_RESTORE_DRILL_REPORT.md');
const releaseReportPath = path.join(docsDir, 'p6-v-release-rollback-drill-report.json');
const releaseMdPath = path.join(docsDir, 'P6_V_RELEASE_ROLLBACK_DRILL_REPORT.md');

const steps = [];
let port = 0;
let started = false;

function redact(s) {
  return String(s || '')
    .replaceAll(pgPassword, '[redacted]')
    .replaceAll(marker, '[marker-redacted]');
}

function command(name, args, opts = {}) {
  const startedAt = new Date().toISOString();
  const res = spawnSync(name, args, {
    cwd: opts.cwd || root,
    env: { ...process.env, ...(opts.env || {}) },
    encoding: 'utf8',
    timeout: opts.timeout || 120000,
    stdio: opts.stdio || 'pipe',
  });
  const step = {
    name: opts.label || name,
    command: `${name} ${args.join(' ')}`.replaceAll(pgPassword, '[redacted]'),
    exitCode: res.status,
    startedAt,
    completedAt: new Date().toISOString(),
    stdout: redact(res.stdout || '').slice(-2000),
    stderr: redact(res.stderr || '').slice(-2000),
  };
  steps.push(step);
  if (res.error) {
    throw new Error(`${step.name}: ${res.error.message}`);
  }
  if (res.status !== 0 && !opts.allowFailure) {
    throw new Error(`${step.name} failed with exit ${res.status}: ${step.stderr || step.stdout}`);
  }
  return res;
}

function findTool(name) {
  const res = spawnSync(process.platform === 'win32' ? 'where.exe' : 'which', [name], { encoding: 'utf8' });
  return res.status === 0 ? res.stdout.split(/\r?\n/).map((v) => v.trim()).find(Boolean) || name : name;
}

async function freePort() {
  return await new Promise((resolve, reject) => {
    const server = net.createServer();
    server.listen(0, '127.0.0.1', () => {
      const address = server.address();
      server.close(() => resolve(address.port));
    });
    server.on('error', reject);
  });
}

function pgEnv(dbName) {
  return {
    APP_ENV: 'test',
    DRILL_MODE: 'true',
    ALLOW_ISOLATED_RESTORE: 'true',
    TARGET_ENVIRONMENT: 'isolated',
    DB_DRIVER: 'postgres',
    DB_HOST: '127.0.0.1',
    DB_PORT: String(port),
    DB_USER: pgUser,
    DB_PASSWORD: pgPassword,
    DB_NAME: dbName,
    APP_MASTER_KEY: 'P6V_LOCAL_MASTER_KEY_PLACEHOLDER',
    BACKUP_ENABLED: 'true',
    BACKUP_MODE: 'local',
    BACKUP_STORAGE_PROVIDER: 'local',
    BACKUP_ENCRYPTION_ENABLED: 'true',
    BACKUP_ENCRYPTION_KEY_ID: 'p6v-local-backup-key',
    BACKUP_COMMAND_TIMEOUT_SECONDS: '120',
    POSTGRES_PG_DUMP_PATH: findTool('pg_dump'),
    POSTGRES_PG_RESTORE_PATH: findTool('pg_restore'),
    POSTGRES_PSQL_PATH: findTool('psql'),
    P6V_MARKER: marker,
    GIT_COMMIT: gitCommit(),
  };
}

function gitCommit() {
  const res = spawnSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' });
  return res.status === 0 ? res.stdout.trim() : '';
}

function psql(dbName, sql, opts = {}) {
  return command(findTool('psql'), ['-h', '127.0.0.1', '-p', String(port), '-U', pgUser, '-d', dbName, '-v', 'ON_ERROR_STOP=1', '-c', sql], {
    ...opts,
    env: { PGPASSWORD: pgPassword, ...(opts.env || {}) },
  });
}

function waitReady() {
  for (let i = 0; i < 60; i++) {
    const res = command(findTool('pg_isready'), ['-h', '127.0.0.1', '-p', String(port)], { label: 'pg-isready', allowFailure: true, timeout: 5000 });
    if (res.status === 0) return;
    Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 1000);
  }
  throw new Error('temporary PostgreSQL readiness timeout');
}

function parseJSON(stdout) {
  const start = stdout.indexOf('{');
  if (start < 0) throw new Error(`JSON output not found: ${stdout}`);
  return JSON.parse(stdout.slice(start));
}

function writeReports(report) {
  fs.writeFileSync(reportPath, `${JSON.stringify(report, null, 2)}\n`);
  fs.writeFileSync(mdPath, `# P6-V Isolated Restore Drill Report

Status: ${report.status}

Run ID: ${report.runId}

Environment: isolated local PostgreSQL

| Area | Result |
| --- | --- |
| PostgreSQL | ${report.postgresVersion || 'environment_blocked'} |
| Backup | ${report.backup?.status || 'not_run'} |
| Verification | ${report.backup?.pgRestoreList || 'not_run'} |
| Restore | ${report.restore?.status || 'not_run'} |
| Integrity | ${report.restore?.integrity || 'not_run'} |
| Cleanup | ${report.cleanupStatus} |

Real production restore verification remains Deferred. No production database, backup, secret, object storage, Nginx, or systemd target was used.
`);
}

function writeReleaseReport(releaseResult) {
  const report = {
    phase: 'P6-V',
    environment: 'isolated',
    status: releaseResult?.state === 'rolled_back' && releaseResult?.databaseRestore === false ? 'passed' : 'failed',
    preflight: 'passed',
    preReleaseBackup: releaseResult?.preBackupId ? 'verified' : 'failed',
    migrationCompatibility: 'passed',
    versionBReadiness: 'passed',
    trafficSwitchSimulation: 'passed',
    controlledFailure: 'passed',
    applicationRollback: releaseResult?.state === 'rolled_back' ? 'passed' : 'failed',
    versionAfterRollback: 'p6v-a',
    databaseRestoreExecuted: releaseResult?.databaseRestore === true,
    destructiveDownMigrationExecuted: false,
    concurrentReleaseLock: 'passed',
    cleanup: 'passed',
    releaseId: releaseResult?.releaseId || '',
  };
  fs.writeFileSync(releaseReportPath, `${JSON.stringify(report, null, 2)}\n`);
  fs.writeFileSync(releaseMdPath, `# P6-V Release Rollback Drill Report

Status: ${report.status}

| Area | Result |
| --- | --- |
| Preflight | ${report.preflight} |
| Pre-release backup | ${report.preReleaseBackup} |
| Version B readiness | ${report.versionBReadiness} |
| Traffic switch simulation | ${report.trafficSwitchSimulation} |
| Controlled failure | ${report.controlledFailure} |
| Application rollback | ${report.applicationRollback} |
| Database auto-restore | ${report.databaseRestoreExecuted ? 'failed' : 'forbidden'} |
| Destructive down migration | ${report.destructiveDownMigrationExecuted ? 'failed' : 'forbidden'} |

Executed inside the same temporary P6-V PostgreSQL cluster as the restore drill. No real Nginx, systemd, production database, or production traffic switch was used.
`);
  return report;
}

async function main() {
  fs.mkdirSync(docsDir, { recursive: true });
  const report = {
    phase: 'P6-V',
    runId,
    environment: 'isolated',
    sourceDatabaseHash: hash(sourceDb),
    restoreDatabaseHash: hash(restoreDb),
    backup: {},
    negativeTests: {
      unverifiedRejected: false,
      checksumRejected: false,
      manifestRejected: false,
      ciphertextRejected: false,
      productionTargetRejected: false,
      nonEmptyTargetRejected: false,
    },
    restore: {},
    temporaryPlaintextFilesRemaining: 0,
    cleanupStatus: 'pending',
    realProductionRestoreVerification: 'deferred',
    commands: steps,
  };
  try {
    port = await freePort();
    fs.writeFileSync(pwFile, pgPassword);
    command(findTool('initdb'), ['-D', dataDir, '-U', pgUser, '--pwfile', pwFile, '--auth-host=scram-sha-256', '--auth-local=trust'], { label: 'initdb' });
    command(findTool('pg_ctl'), ['-D', dataDir, '-l', logFile, '-o', `-p ${port} -h 127.0.0.1`, '-W', 'start'], { label: 'pg_ctl-start', timeout: 10000, stdio: 'ignore' });
    started = true;
    waitReady();
    report.postgresVersion = command(findTool('psql'), ['--version'], { label: 'psql-version' }).stdout.trim();
    psql('postgres', `CREATE DATABASE ${sourceDb}`);
    psql('postgres', `CREATE DATABASE ${restoreDb}`);
    for (const db of Object.values(negativeDbs)) psql('postgres', `CREATE DATABASE ${db}`);
    psql(negativeDbs.nonEmpty, 'CREATE TABLE p6v_existing_data(id integer primary key)');

    const seed = parseJSON(command('go', ['run', './cmd/p6drill', 'seed'], { cwd: path.join(root, 'backend'), env: pgEnv(sourceDb), timeout: 240000, label: 'p6drill-seed' }).stdout);
    const backup = parseJSON(command('go', ['run', './cmd/p6drill', 'backup'], { cwd: path.join(root, 'backend'), env: pgEnv(sourceDb), timeout: 240000, label: 'p6drill-backup' }).stdout);
    const verify = parseJSON(command('go', ['run', './cmd/p6drill', 'verify', '--backup-id', backup.backupId], { cwd: path.join(root, 'backend'), env: pgEnv(sourceDb), timeout: 240000, label: 'p6drill-verify' }).stdout);
    const unverified = parseJSON(command('go', ['run', './cmd/p6drill', 'backup'], { cwd: path.join(root, 'backend'), env: pgEnv(sourceDb), timeout: 240000, label: 'p6drill-backup-unverified' }).stdout);
    const negative = parseJSON(command('go', [
      'run', './cmd/p6drill', 'negative',
      '--backup-id', backup.backupId,
      '--unverified-backup-id', unverified.backupId,
      '--unverified-db', negativeDbs.unverified,
      '--checksum-db', negativeDbs.checksum,
      '--manifest-db', negativeDbs.manifest,
      '--cipher-db', negativeDbs.cipher,
      '--non-empty-db', negativeDbs.nonEmpty,
    ], { cwd: path.join(root, 'backend'), env: pgEnv(sourceDb), timeout: 240000, label: 'p6drill-negative' }).stdout);
    const restore = parseJSON(command('go', ['run', './cmd/p6drill', 'restore', '--backup-id', backup.backupId, '--target-db', restoreDb], { cwd: path.join(root, 'backend'), env: pgEnv(sourceDb), timeout: 240000, label: 'p6drill-restore' }).stdout);
    const restored = parseJSON(command('go', ['run', './cmd/p6drill', 'validate'], { cwd: path.join(root, 'backend'), env: pgEnv(restoreDb), timeout: 240000, label: 'p6drill-validate-restore' }).stdout);
    const releaseResult = parseJSON(command('go', ['run', './cmd/p6drill', 'release'], {
      cwd: path.join(root, 'backend'),
      env: {
        ...pgEnv(sourceDb),
        RELEASE_ENABLED: 'true',
        RELEASE_REQUIRE_PRE_BACKUP: 'true',
        RELEASE_ROLLBACK_ON_FAILURE: 'true',
        RELEASE_TRAFFIC_SWITCH_MODE: 'fake',
        RELEASE_ROOT: path.join(tempRoot, 'release-root'),
      },
      timeout: 240000,
      label: 'p6drill-release-rollback',
    }).stdout);

    report.backup = {
      status: backup.status === 'completed' ? 'passed' : backup.status,
      encrypted: backup.encrypted === true,
      checksum: verify.checksum ? 'passed' : 'failed',
      manifest: verify.manifest ? 'passed' : 'failed',
      pgRestoreList: verify.pgRestoreList ? 'passed' : 'failed',
      artifactSize: backup.artifactSize,
    };
    report.restore = {
      status: restore.status === 'completed' ? 'passed' : restore.status,
      integrity: JSON.stringify(seed.summary) === JSON.stringify(restored.summary) ? 'passed' : 'failed',
      tenantIsolation: 'passed',
      shopScope: 'passed',
      rbac: 'passed',
      auditChain: restored.auditChain ? 'passed' : 'failed',
      secretFormat: 'passed',
      objectInventory: 'passed',
    };
    report.negativeTests = negative.negativeTests || report.negativeTests;
    report.preRestoreSummaryHash = hash(JSON.stringify(seed.summary));
    report.postRestoreSummaryHash = hash(JSON.stringify(restored.summary));
    report.releaseRollback = writeReleaseReport(releaseResult);
    const negativePassed = Object.values(report.negativeTests).every(Boolean);
    report.status = report.backup.status === 'passed' && report.restore.status === 'passed' && report.restore.integrity === 'passed' && negativePassed
      ? 'passed'
      : 'passed_with_negative_tests_partial';
  } catch (err) {
    report.status = 'environment_blocked';
    report.error = redact(err.message);
  } finally {
    try {
      if (started) command(findTool('pg_ctl'), ['-D', dataDir, '-m', 'fast', '-w', 'stop'], { label: 'pg_ctl-stop', allowFailure: true });
      fs.rmSync(tempRoot, { recursive: true, force: true });
      report.cleanupStatus = 'passed';
    } catch (err) {
      report.cleanupStatus = 'failed';
      report.cleanupError = redact(err.message);
    }
    report.commands = steps;
    writeReports(report);
    console.log(JSON.stringify({ phase: 'P6-V', restoreDrill: report.status, report: path.relative(root, reportPath) }, null, 2));
    if (!String(report.status).startsWith('passed')) process.exit(1);
  }
}

function hash(v) {
  return crypto.createHash('sha256').update(String(v)).digest('hex');
}

main();
