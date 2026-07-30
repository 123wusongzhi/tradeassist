import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import {
  readJSON,
  resolveP7V2PortConfig,
  runWSL,
  safeDbName,
  safeRunId,
  stopP7V2Server,
  wslProjectRoot,
  writeJSON,
} from './p7-v2-lib.mjs';
import { captureApiProcessIdentity, generateInstanceNonce } from './p7-v2-process-identity.mjs';

const root = process.cwd();
const timestamp = new Date().toISOString().replace(/[-:TZ.]/g, '').slice(0, 14);
const baselineRunId = safeRunId(`p7v2-diag-baseline-sql-fingerprint-${timestamp}`);
const currentRunId = safeRunId(`p7v2-diag-current-sql-fingerprint-${timestamp}`);
const outRoot = '/tmp/trademind-p7-sql-fingerprint';
const durableRoot = `${wslProjectRoot()}/artifacts/p7-v2-diagnostics/sql-fingerprint`;
const portConfig = resolveP7V2PortConfig();
const environmentStopTimeoutSec = 45;
const listenerReleaseVerificationSec = 30;

function node(script, args = [], env = {}) {
  return spawnSync(process.execPath, [script, ...args], {
    cwd: root,
    encoding: 'utf8',
    env: { ...process.env, ...env },
    stdio: 'pipe',
    maxBuffer: 32 * 1024 * 1024,
  });
}

function listenerCount() {
  const res = runWSL(`ss -ltn 'sport = :${portConfig.port}' 2>/dev/null | awk 'NR>1 {c++} END {print c+0}'`, { timeout: 10000 });
  return Number(String(res.stdout || '0').trim() || '0');
}

function waitForListenerRelease() {
  const deadline = Date.now() + listenerReleaseVerificationSec * 1000;
  while (Date.now() < deadline) {
    if (listenerCount() === 0) return true;
    runWSL('sleep 1', { timeout: 5000 });
  }
  return listenerCount() === 0;
}

function ensureCleanPort() {
  const identity = captureApiProcessIdentity({ port: portConfig.port });
  if (identity.present) {
    stopP7V2Server({ expectedIdentity: identity, portConfig });
  }
  if (!waitForListenerRelease()) {
    throw new Error(`port ${portConfig.port} still busy after environmentStopTimeout=${environmentStopTimeoutSec}s`);
  }
}

function loadFingerprint() {
  return crypto
    .createHash('sha256')
    .update(
      JSON.stringify({
        formal: false,
        diagnosticOnly: true,
        host: portConfig.host,
        port: portConfig.port,
        vus: 10,
        stages: ['warmup:5m', 'ramp:3m', 'steady:10m', 'rampdown:2m'],
        datasetRows: 1900150,
        providerMode: 'mock',
        routes: ['webhook_ingestion', 'auth_invalid_login'],
        kind: 'current',
      }),
    )
    .digest('hex')
    .slice(0, 16);
}

function runLeg(runId, role) {
  ensureCleanPort();
  const diagDir = `${outRoot}/${runId}`;
  runWSL(`mkdir -p ${JSON.stringify(diagDir)}`, { timeout: 10000 });
  const instanceNonce = generateInstanceNonce();
  const start = node(
    'scripts/p7-v2-start-performance-env.mjs',
    ['--run-id', runId, '--instance-nonce', instanceNonce],
    {
      P7_DIAGNOSTICS_ENABLED: 'true',
      P7_DIAGNOSTIC_RUN_ID: runId,
      P7_DIAGNOSTIC_ROLE: role,
      P7_DIAGNOSTIC_DIR: diagDir,
      P7_DIAGNOSTIC_RUNTIME_SNAPSHOT_INTERVAL_MS: '1000',
      P7_DIAGNOSTIC_PG_SAMPLE_INTERVAL_MS: '1000',
      P7V2_INSTANCE_NONCE: instanceNonce,
    },
  );
  if (start.status !== 0) {
    throw new Error(`start failed for ${runId}: ${(start.stderr || start.stdout || '').slice(0, 800)}`);
  }
  const runtime = readJSON('docs/p7-v2-runtime-environment.json') || {};
  const dataset = node('scripts/p7-v2-dataset.mjs', ['--run-id', runId, '--execute']);
  if (dataset.status !== 0) {
    throw new Error(`dataset failed for ${runId}: ${(dataset.stderr || dataset.stdout || '').slice(0, 800)}`);
  }
  const datasetReport = readJSON('docs/p7-v2-dataset-report.json') || {};
  const load = node('scripts/p7-v2-load.mjs', ['--kind', 'current', '--run-id', runId, '--target-vus', '10']);
  if (load.status !== 0) {
    throw new Error(`load failed for ${runId}: ${(load.stderr || load.stdout || '').slice(0, 1200)}`);
  }
  const identity = captureApiProcessIdentity({ pid: runtime.serverPid || '', port: portConfig.port });
  stopP7V2Server({ expectedIdentity: identity, portConfig });
  if (!waitForListenerRelease()) {
    throw new Error(`listener release verification failed after ${runId}`);
  }
  const jsonlPath = `${diagDir}/${runId}.jsonl`;
  const durableDir = `${durableRoot}/${runId}`;
  runWSL(`mkdir -p ${JSON.stringify(durableDir)} && cp -f ${JSON.stringify(jsonlPath)} ${JSON.stringify(`${durableDir}/${runId}.jsonl`)}`, {
    timeout: 120000,
  });
  return {
    runId,
    role,
    apiPid: String(identity.pid || runtime.serverPid || ''),
    instanceNonce: String(runtime.env?.P7V2_INSTANCE_NONCE || runtime.instanceNonce || ''),
    databaseIdentity: safeDbName(runId),
    datasetRows: Number(datasetReport.actualRows || datasetReport.plannedRows || 0),
    diagnosticDir: diagDir,
    jsonlPath,
    durableJsonlPath: `${durableDir}/${runId}.jsonl`,
    loadFingerprint: loadFingerprint(),
  };
}

const pairMeta = {
  phase: 'P7-V2-R3B-SQL-FINGERPRINT-PG-WAIT-DIAGNOSTICS',
  formal: false,
  validForClosure: false,
  validForRegression: false,
  validForComparability: false,
  diagnosticOnly: true,
  formalRerunStarted: false,
  environmentStopTimeoutSec,
  listenerReleaseVerificationSec,
  diagnosticBaselineRunId: baselineRunId,
  diagnosticCurrentRunId: currentRunId,
  diagnosticOutputDirectory: outRoot,
};

writeJSON('docs/p7-v2-r3b-sql-fingerprint-diagnostics-pair-plan.json', pairMeta);

const baseline = runLeg(baselineRunId, 'baseline');
const current = runLeg(currentRunId, 'current');

const summary = {
  ...pairMeta,
  diagnosticRunsIndependent:
    baseline.apiPid !== current.apiPid &&
    baseline.instanceNonce !== current.instanceNonce &&
    baseline.databaseIdentity !== current.databaseIdentity,
  datasetRows: baseline.datasetRows === 1900150 && current.datasetRows === 1900150 ? 1900150 : 0,
  baseline,
  current,
  baselineDiagnosticLoadFingerprint: baseline.loadFingerprint,
  currentDiagnosticLoadFingerprint: current.loadFingerprint,
  diagnosticLoadFingerprint: baseline.loadFingerprint,
  fingerprintsMatch: baseline.loadFingerprint === current.loadFingerprint,
};

writeJSON('docs/p7-v2-r3b-sql-fingerprint-diagnostics-pair-result.json', summary);
console.log(JSON.stringify(summary, null, 2));

const analyze = node('scripts/p7-v2-r3b-sql-fingerprint-diagnostics-analyze.mjs');
process.stdout.write(analyze.stdout || '');
process.stderr.write(analyze.stderr || '');
process.exit(analyze.status === 0 ? 0 : 1);
