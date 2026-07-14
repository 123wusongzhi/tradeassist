import { spawnSync } from 'node:child_process';
import { readJSON, runWSL, safeRunId, valueOf, writeJSON } from './p7-v2-lib.mjs';

const args = process.argv.slice(2);
const runtime = readJSON('docs/p7-v2-runtime-environment.json') || {};
const runId = safeRunId(valueOf(args, '--run-id') || `p7v2-restart-${Date.now()}`);
const previousApiPid = String(runtime.serverPid || '');
const previousWorkerPid = String(runtime.workerPid || '');
const previousDatabaseName = String(runtime.dbName || '');
const dbNameBefore = runWSL('cat /mnt/d/project/trademind-ai/artifacts/p7-v2/server.pid 2>/dev/null || true', { timeout: 10000 });
const start = spawnSync(process.execPath, ['scripts/p7-v2-start-performance-env.mjs', '--run-id', runId], { stdio: 'pipe', encoding: 'utf8' });
const rebuilt = start.status === 0 ? spawnSync(process.execPath, ['scripts/p7-v2-dataset.mjs', '--run-id', runId, '--execute'], { stdio: 'pipe', encoding: 'utf8' }) : { status: 1 };
const currentRuntime = readJSON('docs/p7-v2-runtime-environment.json') || {};
const dataset = readJSON('docs/p7-v2-dataset-report.json') || {};
const redis = runWSL('redis-cli FLUSHALL >/dev/null 2>&1 && echo flushed || echo failed', { timeout: 15000 });
const newApiPid = String(currentRuntime.serverPid || '');
const currentDatabaseName = String(currentRuntime.dbName || '');
const authProbe = readJSON('docs/p7-v2-r2-auth-probe-report.json') || {};
const routeProbe = readJSON('docs/p7-v2-r2-route-probe-report.json') || {};
const databaseStateReset =
  start.status === 0 &&
  Boolean(currentDatabaseName) &&
  currentDatabaseName !== previousDatabaseName &&
  currentRuntime.readiness?.migrationsComplete === true &&
  rebuilt.status === 0 &&
  dataset.status === 'passed' &&
  dataset.databaseRunId === runId;

const report = {
  phase: 'P7-V2',
  component: 'environment-restart',
  runId,
  previousApiPid,
  newApiPid,
  apiProcessChanged: Boolean(previousApiPid && newApiPid && previousApiPid !== newApiPid) || (Boolean(newApiPid) && dbNameBefore.stdout.trim() !== newApiPid),
  previousWorkerPid,
  newWorkerPid: String(currentRuntime.workerPid || ''),
  workerProcessChanged: Boolean(previousWorkerPid && currentRuntime.workerPid && previousWorkerPid !== currentRuntime.workerPid),
  redisRestarted: false,
  redisFlushed: redis.stdout.trim() === 'flushed',
  mockProviderRestarted: false,
  previousDatabaseName,
  currentDatabaseName,
  databaseStateReset,
  databaseResetMethod: databaseStateReset ? 'new_isolated_database_rebuilt_and_dataset_verified' : '',
  migrationCompleted: currentRuntime.readiness?.migrationsComplete === true,
  datasetRebuiltOrRestored: rebuilt.status === 0,
  datasetVerified: dataset.status === 'passed' && dataset.databaseRunId === runId,
  datasetFingerprint: dataset.fullDatasetFingerprint || dataset.datasetFingerprint || '',
  bootstrapPassed: currentRuntime.readiness?.bootstrapCompleted === true,
  authProbePassed: currentRuntime.readiness?.authProbePassed === true && authProbe.status === 'passed',
  routeProbePassed: currentRuntime.readiness?.routeProbePassed === true && routeProbe.status === 'passed',
  serverReady: currentRuntime.readiness?.loadReady === true,
  restartPerformed: start.status === 0,
  currentRunIndependent: false,
  serverStarted: currentRuntime.serverStarted === true,
  serverPid: newApiPid,
  issues: [
    ...(start.status === 0 ? [] : ['isolated environment restart failed']),
    ...(rebuilt.status === 0 ? [] : ['deterministic dataset rebuild failed']),
    'worker and mock-provider lifecycle probes are not yet implemented',
  ],
  generatedAt: new Date().toISOString(),
};
report.currentRunIndependent =
  report.restartPerformed &&
  report.apiProcessChanged &&
  report.databaseStateReset &&
  report.bootstrapPassed &&
  report.authProbePassed &&
  report.routeProbePassed &&
  report.datasetVerified;

writeJSON('docs/p7-v2-environment-restart-report.json', report);
console.log(JSON.stringify(report, null, 2));
process.exit(report.currentRunIndependent ? 0 : 1);
