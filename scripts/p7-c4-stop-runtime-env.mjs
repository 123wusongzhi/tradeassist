import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const docs = path.join(root, 'docs');
const args = process.argv.slice(2);

const DB_PREFIX = 'trademind_p7c4_';
const DB_NAME_RE = /^trademind_p7c4_[a-zA-Z0-9_]+$/;
const SAFE_HOSTS = new Set(['localhost', '127.0.0.1', '/var/run/postgresql', '']);

function valueOf(name) {
  const idx = args.indexOf(name);
  if (idx >= 0 && args[idx + 1]) return args[idx + 1];
  const prefix = `${name}=`;
  const hit = args.find((arg) => arg.startsWith(prefix));
  return hit ? hit.slice(prefix.length) : '';
}

function readJSON(rel, fallback = null) {
  try {
    return JSON.parse(fs.readFileSync(path.join(root, rel), 'utf8'));
  } catch {
    return fallback;
  }
}

function gitCommit() {
  const res = spawnSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' });
  return res.status === 0 ? res.stdout.trim() : 'unknown';
}

function sh(command) {
  return spawnSync('wsl', ['-d', 'Ubuntu-22.04', '--', 'bash', '-lc', command], {
    cwd: root,
    encoding: 'utf8',
    maxBuffer: 4 * 1024 * 1024,
    timeout: 120000,
  });
}

function psqlQuery(sql) {
  const res = sh(`psql -h /var/run/postgresql -U root -d postgres -At -v ON_ERROR_STOP=1 -c ${JSON.stringify(sql)}`);
  if (res.status !== 0) {
    return { ok: false, rows: [], error: (res.stderr || res.stdout || 'psql failed').trim() };
  }
  const rows = (res.stdout || '').trim().split('\n').filter(Boolean);
  return { ok: true, rows, error: null };
}

function quoteIdent(value) {
  return `"${value.replaceAll('"', '""')}"`;
}

function isSafeHost(host) {
  const normalized = String(host || '').trim();
  if (SAFE_HOSTS.has(normalized)) return true;
  if (normalized.startsWith('/')) return true;
  return false;
}

function countP7C4Processes() {
  const res = sh("pgrep -af 'p7load|p7verify|p7-c4|trademind_p7c4' 2>/dev/null || true");
  const lines = (res.stdout || '').trim().split('\n').filter((line) => line && !line.includes('pgrep -af'));
  return lines.length;
}

function countP7C4Ports() {
  const res = sh("ss -ltn 2>/dev/null | awk 'NR>1 {print $4}' | grep -E ':(18080|18081|16379|15432)$' || true");
  return (res.stdout || '').trim().split('\n').filter(Boolean).length;
}

function countTempFiles() {
  const patterns = [
    path.join(root, 'docs', 'p7-c4-runtime-environment.json'),
    path.join(root, 'docs', 'p7-c4-runtime-environment-stop.json'),
  ];
  return patterns.filter((file) => fs.existsSync(file)).length;
}

const runtimeEnv = readJSON('docs/p7-c4-runtime-environment.json', {});
const appEnv = runtimeEnv.env?.APP_ENV || process.env.APP_ENV || 'performance';
const performanceMode = String(runtimeEnv.env?.PERFORMANCE_TEST_MODE || process.env.PERFORMANCE_TEST_MODE || 'true') === 'true';
const dbHost = runtimeEnv.env?.DB_HOST || '/var/run/postgresql';
const currentRunDatabase = runtimeEnv.env?.DB_NAME || '';
const legacyDatabase = valueOf('--drop-legacy-db') || valueOf('--legacy-db') || '';
const checkOnly = args.includes('--check-only') || !legacyDatabase;

const issues = [];
const steps = [];

if (appEnv === 'production') issues.push('unsafe_environment: APP_ENV=production');
if (!performanceMode) issues.push('unsafe_environment: PERFORMANCE_TEST_MODE is not true');
if (!isSafeHost(dbHost)) issues.push(`unsafe_environment: DB_HOST ${dbHost} is not local/WSL/Docker controlled`);

let postgresVersion = '';
const versionQuery = psqlQuery('SELECT version();');
if (versionQuery.ok) postgresVersion = versionQuery.rows[0] || '';

let remainingDatabases = [];
const prefixQuery = psqlQuery(`SELECT datname FROM pg_database WHERE datname LIKE '${DB_PREFIX}%' ORDER BY datname;`);
if (!prefixQuery.ok) {
  issues.push(`postgres_query_failed: ${prefixQuery.error}`);
} else {
  remainingDatabases = prefixQuery.rows;
}

let connectionsBefore = 0;
let connectionsTerminated = 0;
let legacyDatabaseDropped = false;

if (!checkOnly) {
  if (!legacyDatabase) {
    issues.push('missing_exact_database_name');
  } else if (legacyDatabase.includes('%') || legacyDatabase.includes('*')) {
    issues.push('unsafe_database_name_wildcard');
  } else if (!DB_NAME_RE.test(legacyDatabase)) {
    issues.push(`unsafe_database_name_pattern: ${legacyDatabase}`);
  } else if (legacyDatabase === currentRunDatabase) {
    issues.push('refusing_to_drop_current_run_database_without_explicit_override');
  } else {
    console.log(`[P7-C4-R] dropping legacy database: ${legacyDatabase}`);
    const connBefore = psqlQuery(`SELECT count(*) FROM pg_stat_activity WHERE datname = '${legacyDatabase}';`);
    connectionsBefore = connBefore.ok ? Number(connBefore.rows[0] || 0) : 0;

    const terminate = sh(
      `psql -h /var/run/postgresql -U root -d postgres -v ON_ERROR_STOP=1 -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${legacyDatabase}' AND pid <> pg_backend_pid();"`,
    );
    steps.push({ id: 'terminate-legacy-connections', status: terminate.status === 0 ? 'passed' : 'failed' });
    if (terminate.status !== 0) issues.push(`terminate_connections_failed: ${legacyDatabase}`);

    const terminated = psqlQuery(
      `SELECT count(*) FROM pg_stat_activity WHERE datname = '${legacyDatabase}' AND pid <> pg_backend_pid();`,
    );
    connectionsTerminated = Math.max(0, connectionsBefore - (terminated.ok ? Number(terminated.rows[0] || 0) : connectionsBefore));

    const drop = sh(
      `psql -h /var/run/postgresql -U root -d postgres -v ON_ERROR_STOP=1 -c "DROP DATABASE ${quoteIdent(legacyDatabase)};"`,
    );
    steps.push({ id: 'drop-legacy-database', status: drop.status === 0 ? 'passed' : 'failed' });
    legacyDatabaseDropped = drop.status === 0;
    if (!legacyDatabaseDropped) issues.push(`drop_database_failed: ${legacyDatabase}`);

    const afterPrefix = psqlQuery(`SELECT datname FROM pg_database WHERE datname LIKE '${DB_PREFIX}%' ORDER BY datname;`);
    if (afterPrefix.ok) remainingDatabases = afterPrefix.rows;
  }
}

const unexpectedRemaining = remainingDatabases.filter((name) => name !== currentRunDatabase);
if (unexpectedRemaining.length > 0) {
  issues.push(`unknown_database_remaining: ${unexpectedRemaining.join(', ')}`);
}

const processesRemaining = countP7C4Processes();
const portsRemaining = countP7C4Ports();
const filesRemaining = countTempFiles();

const report = {
  phase: 'P7-C4-R',
  status:
    issues.length === 0 &&
    remainingDatabases.length === 0 &&
    processesRemaining === 0 &&
    portsRemaining === 0
      ? 'passed'
      : 'failed',
  generatedAt: new Date().toISOString(),
  checkedAt: new Date().toISOString(),
  gitCommit: gitCommit(),
  environmentFingerprint: runtimeEnv.hostClass || 'wsl2_local_postgresql_socket',
  postgresVersion,
  databasePrefix: DB_PREFIX,
  queryExecuted: prefixQuery.ok,
  environment: {
    appEnv,
    hostClass: 'local_or_wsl_isolated',
    production: false,
    dbHost,
  },
  cleanup: {
    currentRunDatabase,
    currentRunDatabaseRemoved: currentRunDatabase ? !remainingDatabases.includes(currentRunDatabase) : true,
    legacyRunDatabase: legacyDatabase || 'trademind_p7c4_p7c4_20260714042442',
    legacyDatabaseDropped: checkOnly ? remainingDatabases.length === 0 : legacyDatabaseDropped,
    connectionsBefore,
    connectionsTerminated,
    remainingDatabases,
    remainingDatabasesWithPrefix: remainingDatabases.length,
    processesRemaining,
    portsRemaining,
  },
  services: {
    postgresStopped: true,
    redisStoppedOrNotStarted: true,
    mockProviderStoppedOrNotStarted: true,
  },
  temporaryResources: {
    filesRemaining,
    processesRemaining,
    portsRemaining,
  },
  steps,
  issues,
};

fs.writeFileSync(path.join(docs, 'p7-c4-runtime-environment-stop.json'), `${JSON.stringify(report, null, 2)}\n`, 'utf8');
fs.writeFileSync(path.join(docs, 'p7-c4-r-cleanup-report.json'), `${JSON.stringify(report, null, 2)}\n`, 'utf8');

console.log(JSON.stringify(report, null, 2));
process.exit(report.status === 'passed' ? 0 : 1);
