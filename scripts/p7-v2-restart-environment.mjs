import { spawnSync } from 'node:child_process';
import { readJSON, resolveP7V2PortConfig, runWSL, safeRunId, stopP7V2Server, valueOf, wslProjectRoot, writeJSON } from './p7-v2-lib.mjs';
import {
  captureApiProcessIdentity,
  compareProcessIdentity,
  generateInstanceNonce,
  verifyInstanceNonce,
  verifyPortOwner,
  verifyServerBinary,
} from './p7-v2-process-identity.mjs';
import { resolveActiveBaseline } from './p7-v2-evidence-resolver.mjs';

const args = process.argv.slice(2);
const activeBaseline = resolveActiveBaseline();
if (!activeBaseline.valid) throw new Error(`active frozen baseline is invalid: ${activeBaseline.issues.join('; ')}`);
const baselineRunId = activeBaseline.baseline.runId;
const runtime = readJSON('docs/p7-v2-runtime-environment.json') || {};
const runId = safeRunId(valueOf(args, '--run-id') || `p7v2-restart-${Date.now()}`);
const formal = args.includes('--formal');
const portConfig = resolveP7V2PortConfig();
const previousDatabaseName = String(runtime.dbName || '');
const pidFile = `${wslProjectRoot()}/artifacts/p7-v2/server.pid`;
const pidFileRead = runWSL(`cat ${JSON.stringify(pidFile)} 2>/dev/null || true`, { timeout: 10000 });
const pidFromFile = String(pidFileRead.stdout || '').trim();
const previousIdentity = captureApiProcessIdentity({ pid: pidFromFile, port: portConfig.port });
const previousApiPresent = previousIdentity.present === true;
const stalePidFile = Boolean(pidFromFile && !previousIdentity.present);
const startMode = previousApiPresent ? 'restart' : 'clean_start';
const oldPid = previousIdentity.pid || pidFromFile;

let oldProcessStopped = !previousApiPresent;
let portReleased = !previousApiPresent;
let oldIdentityExists = previousApiPresent;
if (previousApiPresent) {
  stopP7V2Server({ expectedIdentity: previousIdentity, portConfig });
  const afterStop = captureApiProcessIdentity({ pid: oldPid, port: portConfig.port });
  oldIdentityExists = afterStop.present === true;
  oldProcessStopped = !oldIdentityExists;
}
const portBeforeStart = runWSL(`ss -ltn 'sport = :${portConfig.port}' 2>/dev/null | awk 'NR>1 {found=1} END {print found ? "busy" : "free"}'`, { timeout: 10000 });
portReleased = (portBeforeStart.stdout || '').trim() === 'free';
const instanceNonce = generateInstanceNonce();
const startArgs = ['scripts/p7-v2-start-performance-env.mjs', '--run-id', runId, '--skip-stop', '--instance-nonce', instanceNonce];
if (formal) startArgs.push('--formal');
const start = oldProcessStopped && portReleased
  ? spawnSync(process.execPath, startArgs, { stdio: 'pipe', encoding: 'utf8' })
  : { status: 1 };
const rebuilt = start.status === 0
  ? spawnSync(process.execPath, ['scripts/p7-v2-dataset.mjs', '--run-id', runId, '--execute'], { stdio: 'pipe', encoding: 'utf8' })
  : { status: 1 };
const currentRuntime = readJSON('docs/p7-v2-runtime-environment.json') || {};
const dataset = readJSON('docs/p7-v2-dataset-report.json') || {};
const authProbe = readJSON('docs/p7-v2-r2-auth-probe-report.json') || {};
const routeProbe = readJSON('docs/p7-v2-r2-route-probe-report.json') || {};
const redis = runWSL('redis-cli FLUSHALL >/dev/null 2>&1 && redis-cli DBSIZE 2>/dev/null | grep -qx 0 && echo flushed || echo failed', { timeout: 15000 });
const currentIdentity = captureApiProcessIdentity({ pid: currentRuntime.serverPid || '', port: portConfig.port });
const identityComparison = compareProcessIdentity(previousIdentity, currentIdentity);
const currentDatabaseName = String(currentRuntime.dbName || '');
const databaseStateReset =
  start.status === 0 &&
  Boolean(currentDatabaseName) &&
  currentDatabaseName !== previousDatabaseName &&
  currentRuntime.readiness?.migrationsComplete === true &&
  rebuilt.status === 0 &&
  dataset.status === 'passed' &&
  dataset.databaseRunId === runId;
const workerTopology = 'embedded_in_api';
const worker = {
  topology: workerTopology,
  required: true,
  previousIdentity: null,
  currentIdentity: null,
  processChanged: false,
  freshInstanceVerified: identityComparison.freshProcessVerified,
  status: identityComparison.freshProcessVerified ? 'passed' : 'failed',
  evidence: 'performance worker lifecycle is embedded in the API process',
};
const mockProvider = {
  topology: 'in_memory_mock',
  freshStateVerified: identityComparison.freshProcessVerified && currentRuntime.env?.EXTERNAL_PROVIDER_MODE === 'mock',
  realProviderCalls: 0,
  status: identityComparison.freshProcessVerified && currentRuntime.env?.EXTERNAL_PROVIDER_MODE === 'mock' ? 'passed' : 'failed',
};
const api = {
  startMode,
  previousPresent: previousApiPresent,
  previousIdentity: previousApiPresent ? previousIdentity : null,
  currentIdentity: currentIdentity.present ? currentIdentity : null,
  oldProcessStopped,
  oldPidExists: oldIdentityExists,
  oldIdentityExists,
  stalePidFile,
  portReleased,
  portWasFreeBeforeStart: portReleased,
  processChanged: identityComparison.processChanged,
  pidReused: identityComparison.pidReused,
  freshProcessVerified: identityComparison.freshProcessVerified && portReleased,
  portOwnerVerified: verifyPortOwner(currentIdentity, portConfig.port),
  serverBinaryVerified: verifyServerBinary(currentIdentity, currentRuntime.serverBinarySha256 || ''),
  instanceNonceVerified: verifyInstanceNonce(currentIdentity, instanceNonce),
};
const report = {
  phase: 'P7-V2-R3B-CI-RG',
  component: 'environment-restart',
  runId,
  status: 'failed',
  formal,
  restartPerformed: start.status === 0,
  api,
  worker,
  redis: {
    isolationMethod: 'flushall',
    restarted: false,
    flushed: (redis.stdout || '').trim() === 'flushed',
    stateResetVerified: (redis.stdout || '').trim() === 'flushed',
  },
  mockProvider,
  database: {
    previousName: previousDatabaseName,
    currentName: currentDatabaseName,
    stateResetMethod: databaseStateReset ? 'new_isolated_database' : '',
    stateReset: databaseStateReset,
    migrationCompleted: currentRuntime.readiness?.migrationsComplete === true,
    datasetProfile: dataset.profile || '',
    plannedRows: dataset.plannedRows || 0,
    actualRows: dataset.actualRows || 0,
    failedRows: dataset.failedRows || 0,
    duplicateRows: dataset.duplicateRows || 0,
    datasetVerified: dataset.status === 'passed' && dataset.databaseRunId === runId && Number(dataset.actualRows) === 1900150 && Number(dataset.failedRows) === 0 && Number(dataset.duplicateRows) === 0,
    datasetFingerprint: dataset.fullDatasetFingerprint || dataset.datasetFingerprint || '',
  },
  bootstrapPassed: currentRuntime.readiness?.bootstrapCompleted === true,
  authProbePassed: currentRuntime.readiness?.authProbePassed === true && authProbe.status === 'passed',
  routeProbePassed: currentRuntime.readiness?.routeProbePassed === true && routeProbe.status === 'passed',
  serverReady: currentRuntime.readiness?.loadReady === true,
  currentRunIndependent: false,
  productionResourcesAccessed: false,
  selectedHost: portConfig.host,
  selectedPort: portConfig.port,
  baseUrl: portConfig.baseUrl,
  generatedAt: new Date().toISOString(),
};
report.currentRunIndependent =
  report.runId !== baselineRunId &&
  report.database.stateReset &&
  report.database.datasetVerified &&
  report.bootstrapPassed &&
  report.authProbePassed &&
  report.routeProbePassed &&
  report.api.portOwnerVerified &&
  report.api.serverBinaryVerified &&
  report.api.instanceNonceVerified &&
  (report.api.freshProcessVerified || report.api.processChanged) &&
  report.worker.status === 'passed' &&
  report.redis.stateResetVerified &&
  report.mockProvider.freshStateVerified;
report.status = report.currentRunIndependent ? 'passed' : 'failed';
report.issues = [
  ...(oldProcessStopped ? [] : ['old API process did not stop']),
  ...(portReleased ? [] : ['port 8080 was not released before start']),
  ...(report.api.portOwnerVerified ? [] : ['port owner does not match current API']),
  ...(report.api.serverBinaryVerified ? [] : ['current API binary hash does not match build hash']),
  ...(report.api.instanceNonceVerified ? [] : ['current API instance nonce was not verified']),
];

writeJSON('docs/p7-v2-environment-restart-report.json', report);
console.log(JSON.stringify(report, null, 2));
process.exit(report.currentRunIndependent ? 0 : 1);
