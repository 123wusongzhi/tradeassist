import { spawnSync } from 'node:child_process';
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');

function runId() {
  const raw = process.env.P7_C4_RUN_ID || `p7c4-${new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14)}`;
  return raw.toLowerCase().replace(/[^a-z0-9_-]+/g, '-').replace(/^-+|-+$/g, '');
}

function safeDbName(id) {
  const key = id.toLowerCase().replace(/[^a-z0-9_]+/g, '_').replace(/^_+|_+$/g, '');
  return `trademind_p7c4_${key}`.slice(0, 63);
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

const id = runId();
const dbName = safeDbName(id);
const issues = [];
if ((process.env.APP_ENV || 'performance').trim() === 'production') issues.push('APP_ENV=production rejected');
if (!dbName.startsWith('trademind_p7c4_')) issues.push('Database prefix must be trademind_p7c4_');

let pgVersion = '';
if (issues.length === 0) {
  sh('service postgresql start >/dev/null 2>&1 || /etc/init.d/postgresql start >/dev/null 2>&1');
  const create = sh(`sudo -u postgres psql -v ON_ERROR_STOP=1 -d postgres -c "CREATE DATABASE ${dbName};" 2>/tmp/p7c4_createdb.err || (grep -q "already exists" /tmp/p7c4_createdb.err && true)`);
  if (create.status !== 0) issues.push('Isolated database create failed');
  const version = sh(`sudo -u postgres psql -At -d postgres -c "select version();"`);
  if (version.status === 0) pgVersion = version.stdout.trim();
}

const report = {
  phase: 'P7-C4',
  status: issues.length === 0 ? 'passed' : 'failed',
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
  plannedRows: 1900150,
  env: {
    APP_ENV: 'performance',
    PERFORMANCE_TEST_MODE: 'true',
    ALLOW_PERFORMANCE_DATASET: 'true',
    EXTERNAL_PROVIDER_MODE: 'mock',
    DOUYIN_WRITE_ENABLED: 'false',
    AUTO_LISTING_ENABLED: 'false',
    DB_NAME: dbName,
    DB_DRIVER: 'postgres',
    DB_HOST: '/var/run/postgresql',
    DB_PORT: '5432',
    DB_USER: 'root',
  },
  issues,
};

fs.writeFileSync(path.join(docs, 'p7-c4-runtime-environment.json'), `${JSON.stringify(report, null, 2)}\n`, 'utf8');
fs.writeFileSync(path.join(docs, 'P7_C4_RUNTIME_ENVIRONMENT.md'), `# P7-C4 Runtime Environment\n\nStatus: ${report.status}\n\n- Run ID: \`${report.runId}\`\n- Database hash: \`${report.databaseNameHash}\`\n`, 'utf8');
console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
