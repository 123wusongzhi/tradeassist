import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import {
  DB_PREFIX,
  assertDbNameSafe,
  collectEnvironmentFingerprint,
  configFingerprint,
  docsDir,
  gitCommit,
  readJSON,
  root,
  runWSL,
  safeDbName,
  safeRunId,
  valueOf,
  writeJSON,
  writeMarkdown,
} from './p7-v2-lib.mjs';

const args = process.argv.slice(2);
const runId = safeRunId(valueOf(args, '--run-id') || process.env.P7_V2_RUN_ID);
const dbName = safeDbName(runId);
const issues = [...assertDbNameSafe(dbName)];
if ((process.env.APP_ENV || 'performance') === 'production') issues.push('APP_ENV=production rejected');

const startedAt = new Date().toISOString();
if (issues.length === 0) {
  runWSL('service postgresql start >/dev/null 2>&1 || /etc/init.d/postgresql start >/dev/null 2>&1');
  const create = runWSL(
    `sudo -u postgres psql -v ON_ERROR_STOP=1 -d postgres -c "CREATE DATABASE ${dbName};" 2>/tmp/p7v2_createdb.err || (grep -q "already exists" /tmp/p7v2_createdb.err && true)`,
  );
  if (create.status !== 0) issues.push('failed to create isolated PostgreSQL database');
  runWSL('redis-server --daemonize yes --port 6379 >/dev/null 2>&1 || service redis-server start >/dev/null 2>&1 || true');
}

const pgVersion = runWSL(`psql -h /var/run/postgresql -U root -At -d postgres -c "select version();"`);
const redisVersion = runWSL('redis-cli --version 2>/dev/null || true');

const env = {
  APP_ENV: 'performance',
  PERFORMANCE_TEST_MODE: 'true',
  ALLOW_PERFORMANCE_DATASET: 'true',
  EXTERNAL_PROVIDER_MODE: 'mock',
  DOUYIN_WRITE_ENABLED: 'false',
  AUTO_LISTING_ENABLED: 'false',
  METRICS_ENABLED: 'true',
  TRACING_ENABLED: 'true',
  AUDIT_ENABLED: 'true',
  OPERATION_LOG_ENABLED: 'true',
  PPROF_ENABLED: 'true',
  PPROF_INTERNAL_ONLY: 'true',
  DB_NAME: dbName,
  DB_DRIVER: 'postgres',
  DB_HOST: '/var/run/postgresql',
  DB_PORT: '5432',
  DB_USER: 'root',
  REDIS_ADDR: '127.0.0.1:6379',
};

const fingerprint = collectEnvironmentFingerprint('environment-start', runId, {
  startedAt,
  databaseNameHash: crypto.createHash('sha256').update(dbName).digest('hex').slice(0, 16),
  configFingerprint: configFingerprint(env),
  migrationVersion: 'AutoMigrate+p7',
  datasetProfile: 'medium',
});

const report = {
  phase: 'P7-V2',
  status: issues.length === 0 ? 'passed' : 'failed',
  runId,
  dbName,
  databaseNameHash: fingerprint.databaseNameHash,
  hostClass: 'wsl2_local_postgresql_socket',
  port: 8080,
  postgreSQLVersion: (pgVersion.stdout || '').trim(),
  redisVersion: (redisVersion.stdout || '').trim(),
  gitCommit: gitCommit(),
  schemaVersion: 'AutoMigrate',
  migrationVersion: 'AutoMigrate+p7',
  datasetProfile: 'medium',
  plannedRows: 1900150,
  env,
  environmentFingerprint: fingerprint,
  issues,
  generatedAt: new Date().toISOString(),
};

writeJSON('docs/p7-v2-runtime-environment.json', report);
writeJSON('docs/p7-v2-environment-fingerprint.json', { runs: [fingerprint] });
writeMarkdown(
  'docs/P7_V2_ENVIRONMENT_FINGERPRINT.md',
  `# P7-V2 Environment Fingerprint

Status: ${report.status}

- Run ID: \`${runId}\`
- Database hash: \`${fingerprint.databaseNameHash}\`
- Config fingerprint: \`${fingerprint.configFingerprint}\`
`,
);
writeMarkdown(
  'docs/P7_V2_RUNTIME_ENVIRONMENT.md',
  `# P7-V2 Runtime Environment

Status: ${report.status}

- Database prefix: \`${DB_PREFIX}\`
- Run ID: \`${runId}\`
`,
);

console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
