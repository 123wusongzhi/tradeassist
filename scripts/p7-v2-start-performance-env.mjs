import crypto from 'node:crypto';
import { spawnSync } from 'node:child_process';
import {
  assertDbNameSafe,
  collectEnvironmentFingerprint,
  configFingerprint,
  performanceEnvDefaults,
  probePerformanceEndpoints,
  runAuthProbe,
  runWSL,
  safeDbName,
  safeRunId,
  startP7V2Server,
  stopP7V2Server,
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
let migrationsComplete = false;
let bootstrapCompleted = false;
let authProbePassed = false;
let routeProbePassed = false;

if (issues.length === 0) {
  runWSL('service postgresql start >/dev/null 2>&1 || /etc/init.d/postgresql start >/dev/null 2>&1');
  const create = runWSL(
    `sudo -u postgres psql -v ON_ERROR_STOP=1 -d postgres -c "CREATE DATABASE ${dbName};" 2>/tmp/p7v2_createdb.err || (grep -q "already exists" /tmp/p7v2_createdb.err && true)`,
  );
  if (create.status !== 0) issues.push('failed to create isolated PostgreSQL database');
  runWSL('redis-server --daemonize yes --port 6379 >/dev/null 2>&1 || service redis-server start >/dev/null 2>&1 || true');
  migrationsComplete = create.status === 0;
}

const pgVersion = runWSL(`psql -h /var/run/postgresql -U root -At -d postgres -c "select version();"`);
const redisVersion = runWSL('redis-cli --version 2>/dev/null || true');

const env = performanceEnvDefaults({
  DB_NAME: dbName,
  DB_DRIVER: 'postgres',
  DB_HOST: '/var/run/postgresql',
  DB_PORT: '5432',
  DB_USER: 'root',
  REDIS_ADDR: '127.0.0.1:6379',
  APP_HTTP_ADDR: '127.0.0.1:8080',
});

let server = { ok: false, pid: '', issues: [] };
if (issues.length === 0 && !args.includes('--skip-server')) {
  server = startP7V2Server(env);
  if (!server.ok) {
    issues.push(...(server.issues || ['failed to start API server']));
  } else {
    bootstrapCompleted = true;
    const authProbe = runAuthProbe('http://127.0.0.1:8080', env);
    writeJSON('docs/p7-v2-r2-auth-probe-report.json', authProbe);
    authProbePassed = authProbe.status === 'passed';
    if (!authProbePassed) {
      issues.push(`auth probe failed: positive=${authProbe.positiveScenariosFailed} negative=${authProbe.negativeScenariosUnexpected}`);
    }
    const routeProbeRes = spawnSync(process.execPath, ['scripts/p7-v2-r2-route-probe.mjs'], { stdio: 'pipe', encoding: 'utf8' });
    const routeProbe = (() => {
      try {
        return JSON.parse(routeProbeRes.stdout || '{}');
      } catch {
        return { status: 'failed', routeNotFound: 1 };
      }
    })();
    routeProbePassed = routeProbe.status === 'passed' && routeProbe.routeNotFound === 0;
    if (!routeProbePassed) issues.push('route probe failed');
  }
}

const loadReady = migrationsComplete && bootstrapCompleted && authProbePassed && routeProbePassed && server.ok;

const fingerprint = collectEnvironmentFingerprint('environment-start', runId, {
  startedAt,
  databaseNameHash: crypto.createHash('sha256').update(dbName).digest('hex').slice(0, 16),
  configFingerprint: configFingerprint(env),
  migrationVersion: 'AutoMigrate+p7',
  datasetProfile: 'medium',
});

const redactedEnv = {
  ...env,
  ADMIN_BOOTSTRAP_PASSWORD: '[redacted]',
  P7V2_PERF_ADMIN_PASSWORD: '[redacted]',
  P7V2_PERF_TENANT_ADMIN_PASSWORD: '[redacted]',
  P7V2_PERF_OPERATOR_PASSWORD: '[redacted]',
  P7V2_PERF_READONLY_PASSWORD: '[redacted]',
  P7V2_WEBHOOK_TEST_SECRET: '[redacted]',
};

const report = {
  phase: 'P7-V2-R2',
  status: issues.length === 0 && loadReady ? 'passed' : 'failed',
  runId,
  dbName,
  databaseNameHash: fingerprint.databaseNameHash,
  hostClass: 'wsl2_local_postgresql_socket',
  port: 8080,
  postgreSQLVersion: (pgVersion.stdout || '').trim(),
  redisVersion: (redisVersion.stdout || '').trim(),
  schemaVersion: 'AutoMigrate',
  migrationVersion: 'AutoMigrate+p7',
  datasetProfile: 'medium',
  plannedRows: 1900150,
  env: redactedEnv,
  serverPid: server.pid || '',
  serverStarted: server.ok,
  readiness: {
    migrationsComplete,
    bootstrapCompleted,
    performanceAdminReady: bootstrapCompleted,
    authProbePassed,
    routeProbePassed,
    loadReady,
  },
  environmentFingerprint: fingerprint,
  issues,
  generatedAt: new Date().toISOString(),
};

writeJSON('docs/p7-v2-runtime-environment.json', report);
writeJSON('docs/p7-v2-environment-fingerprint.json', { runs: [fingerprint] });
writeMarkdown(
  'docs/P7_V2_RUNTIME_ENVIRONMENT.md',
  `# P7-V2 Runtime Environment\n\nStatus: ${report.status}\n\n- Run ID: \`${runId}\`\n- loadReady: ${loadReady}\n`,
);

console.log(JSON.stringify(report, null, 2));
if (!loadReady || issues.length) {
  process.exit(1);
}
