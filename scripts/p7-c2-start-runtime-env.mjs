import { spawnSync } from 'node:child_process';
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');

function runId() {
  const raw = process.env.P7_C2_RUN_ID || `p7c2-${new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14)}`;
  return raw.toLowerCase().replace(/[^a-z0-9_-]+/g, '-').replace(/^-+|-+$/g, '');
}

function safeDbName(id) {
  const key = id.toLowerCase().replace(/[^a-z0-9_]+/g, '_').replace(/^_+|_+$/g, '');
  return `trademind_p7c2_${key}`.slice(0, 63);
}

function sh(command) {
  const res = spawnSync('wsl', ['-d', 'Ubuntu-22.04', '--', 'bash', '-lc', command], {
    encoding: 'utf8',
    maxBuffer: 1024 * 1024 * 10,
  });
  return { status: res.status ?? 1, stdout: res.stdout || '', stderr: res.stderr || '' };
}

function gitCommit() {
  const res = spawnSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' });
  return res.status === 0 ? res.stdout.trim() : 'unknown';
}

function writeReports(report) {
  fs.writeFileSync(path.join(docs, 'p7-c2-runtime-environment.json'), `${JSON.stringify(report, null, 2)}\n`, 'utf8');
  fs.writeFileSync(
    path.join(docs, 'P7_C2_RUNTIME_ENVIRONMENT.md'),
    `# P7-C2 Runtime Environment\n\nStatus: ${report.status}\n\n- Run ID: \`${report.runId}\`\n- Host class: \`${report.hostClass}\`\n- Database name hash: \`${report.databaseNameHash}\`\n- PostgreSQL version: \`${report.postgreSQLVersion || 'unknown'}\`\n- Dataset profile: \`${report.datasetProfile}\`\n\n${(report.issues || []).map((item) => `- ${item}`).join('\n')}\n`,
    'utf8',
  );
}

const id = runId();
const dbName = safeDbName(id);
const appEnv = (process.env.APP_ENV || 'performance').trim();
const issues = [];

if (appEnv === 'production') {
  issues.push('APP_ENV=production is rejected for P7-C2 runtime environment.');
}
if (!dbName.startsWith('trademind_p7c2_')) {
  issues.push('Database name does not use the required trademind_p7c2_ prefix.');
}

let status = 'failed';
let pgVersion = '';
if (issues.length === 0) {
  const start = sh('service postgresql start >/dev/null 2>&1 || /etc/init.d/postgresql start >/dev/null 2>&1');
  if (start.status !== 0) {
    issues.push(`PostgreSQL service start failed: ${start.stderr.trim() || start.stdout.trim()}`);
  } else {
    const roleExists = sh(`sudo -u postgres psql -At -d postgres -c "SELECT 1 FROM pg_roles WHERE rolname = 'root';"`);
    if (roleExists.status !== 0) {
      issues.push(`PostgreSQL root role check failed: ${roleExists.stderr.trim() || roleExists.stdout.trim()}`);
    } else if (roleExists.stdout.trim() !== '1') {
      const role = sh(`sudo -u postgres psql -v ON_ERROR_STOP=1 -d postgres -c "CREATE ROLE root LOGIN SUPERUSER;"`);
      if (role.status !== 0) {
        issues.push(`PostgreSQL root role setup failed: ${role.stderr.trim() || role.stdout.trim()}`);
      }
    }
    const create = sh(`sudo -u postgres psql -v ON_ERROR_STOP=1 -d postgres -c "CREATE DATABASE ${dbName};" 2>/tmp/p7c2_createdb.err || (grep -q "already exists" /tmp/p7c2_createdb.err && true)`);
    if (create.status !== 0) {
      issues.push(`Database create failed for isolated P7-C2 database.`);
    }
    const version = sh(`sudo -u postgres psql -At -d postgres -c "select version();"`);
    if (version.status === 0) {
      pgVersion = version.stdout.trim();
    }
  }
}

status = issues.length === 0 ? 'passed' : 'failed';
const report = {
  phase: 'P7-C2',
  status,
  generatedAt: new Date().toISOString(),
  runId: id,
  hostClass: 'wsl2_local_postgresql_socket',
  port: 5432,
  databaseNameHash: crypto.createHash('sha256').update(dbName).digest('hex').slice(0, 16),
  postgreSQLVersion: pgVersion,
  gitCommit: gitCommit(),
  schemaVersion: 'AutoMigrate',
  migrationVersion: 'AutoMigrate+p7',
  datasetProfile: 'medium',
  env: {
    APP_ENV: 'performance',
    PERFORMANCE_TEST_MODE: 'true',
    ALLOW_PERFORMANCE_DATASET: 'true',
    EXTERNAL_PROVIDER_MODE: 'mock',
    DOUYIN_WRITE_ENABLED: 'false',
    AUTO_LISTING_ENABLED: 'false',
    DB_DRIVER: 'postgres',
    DB_HOST: '/var/run/postgresql',
    DB_PORT: '5432',
    DB_USER: 'root',
    DB_NAME: dbName,
  },
  secretsRecorded: false,
  productionResourceAccess: false,
  realProviderAccess: false,
  realDouyinWrite: false,
  issues,
};

writeReports(report);
console.log(JSON.stringify(report, null, 2));
process.exit(status === 'passed' ? 0 : 1);
