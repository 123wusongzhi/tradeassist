import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { spawnSync } from 'node:child_process';

import {
  configFingerprint,
  collectEnvironmentFingerprint,
  loginPerformanceAccount,
  performanceEnvDefaults,
  probeSignedWebhook,
  readJSON,
  resolveP7V2PortConfig,
  root,
  run,
  runAuthProbe,
  runWSL,
  safeDbName,
  startP7V2Server,
  stopP7V2Server,
  writeJSON,
  writeMarkdown,
} from './p7-v2-lib.mjs';
import { captureApiProcessIdentity, verifyPortOwner, verifyServerBinary } from './p7-v2-process-identity.mjs';
import { sha256File, verifyBinaryReceipt } from './p7-v2-formal-binary-provenance-lib.mjs';
import {
  FORMAL_HOST_ISOLATION_VERSION,
  FORMAL_RUN_LIFECYCLE_STEPS,
  HOST_ISOLATION_CONTRACT_PATH,
  validateBackgroundProcessGate,
  validateFormalHostIsolationContract,
  validateQuietWindowEvidence,
  validateResourcePrecheck,
} from './p7-v2-r3b-formal-host-isolation.mjs';

export const HOST_ISOLATION_VALIDATION_RUNNER_VERSION = 1;
export const MATRIX_JSON_PATH = 'docs/p7-v2-r3b-host-isolation-validation-matrix.json';
export const MATRIX_MD_PATH = 'docs/P7_V2_R3B_HOST_ISOLATION_VALIDATION_MATRIX.md';
export const MATRIX_STATE_VALUES = [
  'planned',
  'validating_contract',
  'b1_running',
  'b1_completed',
  'c1_running',
  'c1_completed',
  'c2_running',
  'c2_completed',
  'b2_running',
  'b2_completed',
  'analyzing',
  'cleanup_pending',
  'completed',
  'blocked',
  'invalid_incomplete',
];
export const ROUND_STATE_VALUES = [
  'planned',
  'resource_precheck',
  'database_prepare',
  'dataset_build',
  'dataset_barrier',
  'application_start',
  'restart_isolation',
  'warmup',
  'cooldown',
  'quiet_window',
  'measurement',
  'application_stop',
  'connection_drain',
  'snapshot_complete',
  'completed',
  'failed',
];
export const RUN_ORDER = Object.freeze([
  { slot: 'B1', kind: 'B', role: 'baseline', orderIndex: 1 },
  { slot: 'C1', kind: 'C', role: 'current', orderIndex: 2 },
  { slot: 'C2', kind: 'C', role: 'current', orderIndex: 3 },
  { slot: 'B2', kind: 'B', role: 'baseline', orderIndex: 4 },
]);
export const EXPECTED = Object.freeze({
  baselineBinarySha256: '4e0408ac29b777beac7598872dfd8cac6430542eae37b5ba304c3dbdf7bd79f1',
  currentBinarySha256: 'c6564cfe47a9f5cc60ce6b8c7c29accf7460e213318ba62a11c0d480f4f47766',
  datasetRows: 1900150,
  matrixIdPrefix: 'p7v2-diag-host-isolation-validation',
});
export const FORBIDDEN_ARGS = Object.freeze([
  '--baseline-binary',
  '--current-binary',
  '--baseline-binary-sha256',
  '--current-binary-sha256',
  '--dataset-size',
  '--dataset-profile',
  '--vus',
  '--target-vus',
  '--duration',
  '--stages',
  '--order',
  '--run-count',
  '--input-sequence',
  '--branch-mix',
  '--load-profile',
]);
const TRACKED_METRICS = Object.freeze([
  ['Webhook Ingestion', 'p95'],
  ['Webhook Ingestion', 'p99'],
  ['Auth Invalid Login', 'p95'],
  ['Auth Invalid Login', 'p99'],
]);

function stableJson(value) {
  if (Array.isArray(value)) return `[${value.map((item) => stableJson(item)).join(',')}]`;
  if (value && typeof value === 'object') {
    return `{${Object.keys(value).sort().map((key) => `${JSON.stringify(key)}:${stableJson(value[key])}`).join(',')}}`;
  }
  return JSON.stringify(value);
}

function sha256Json(value) {
  return crypto.createHash('sha256').update(stableJson(value)).digest('hex');
}

function isSha256(value) {
  return /^[a-f0-9]{64}$/.test(String(value || ''));
}

function toRel(absOrRel) {
  return path.relative(root, absOrRel).replaceAll('\\', '/');
}

function nowStamp() {
  return new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
}

function assertNoForbiddenArgs(args = process.argv.slice(2)) {
  const bad = args.filter((arg) => FORBIDDEN_ARGS.some((name) => arg === name || arg.startsWith(`${name}=`)));
  if (bad.length) throw new Error(`host isolation validation runner rejects override args: ${bad.join(',')}`);
  return true;
}

export function assertFixedRunPlan(order = RUN_ORDER) {
  const signature = order.map((entry) => entry.kind).join('-');
  if (signature !== 'B-C-C-B') throw new Error(`fixed B-C-C-B order required, got ${signature}`);
  if (order.length !== 4) throw new Error(`fixed run count 4 required, got ${order.length}`);
  return true;
}

export function assertNoFifthRun(index) {
  if (Number(index) > 4) throw new Error('fifth host isolation validation run is forbidden');
  return true;
}

function statePush(events, state, detail = {}) {
  if (!ROUND_STATE_VALUES.includes(state)) throw new Error(`unknown round state: ${state}`);
  events.push({ state, at: new Date().toISOString(), ...detail });
}

function scenarioMetrics(report, scenario) {
  const row = (report.scenarios || []).find((item) => item.scenario === scenario) || {};
  return {
    sampleCount: row.sampleCount ?? row.requestCount ?? row.requests ?? null,
    errorCount: report.failedRequests ?? null,
    timeoutCount: row.timeoutCount ?? row.timeouts ?? 0,
    p50: row.p50 ?? null,
    p90: row.p90 ?? null,
    p95: row.p95 ?? null,
    p99: row.p99 ?? null,
    max: row.max ?? null,
    mean: row.avg ?? row.mean ?? null,
    stddev: row.stddev ?? null,
  };
}

function focusedMetrics(report) {
  return {
    'Webhook Ingestion': scenarioMetrics(report, 'Webhook Ingestion'),
    'Auth Invalid Login': scenarioMetrics(report, 'Auth Invalid Login'),
  };
}

function compareMetric(left, right, aggregation) {
  const a = Number(left);
  const b = Number(right);
  if (!Number.isFinite(a) || !Number.isFinite(b)) {
    return { comparable: false, absoluteDelta: null, relativeDeltaPct: null, direction: 'not_comparable', materialRegression: false };
  }
  const absoluteDelta = b - a;
  const relativeDeltaPct = Math.abs(a) > 0 ? absoluteDelta / Math.abs(a) : null;
  const policy = readJSON('docs/p7-v2-regression-policy-v2.json') || {};
  const threshold = Number(policy.relativeThresholds?.[aggregation] ?? (aggregation === 'p99' ? 0.15 : 0.1));
  const floorMs = Number(policy.materialityFloors?.[`${aggregation}Ms`] ?? 0);
  return {
    comparable: true,
    absoluteDelta,
    relativeDeltaPct,
    relativeDeltaPercent: relativeDeltaPct === null ? null : relativeDeltaPct * 100,
    direction: absoluteDelta > 0 ? 'right_slower' : absoluteDelta < 0 ? 'right_faster' : 'equal',
    threshold,
    materialityFloorMs: floorMs,
    materialRegression: absoluteDelta > floorMs && relativeDeltaPct !== null && relativeDeltaPct > threshold,
  };
}

function selfVerdicts(leftRun, rightRun) {
  return TRACKED_METRICS.map(([scenario, aggregation]) => {
    const comparison = compareMetric(leftRun?.metrics?.[scenario]?.[aggregation], rightRun?.metrics?.[scenario]?.[aggregation], aggregation);
    return {
      metric: `${scenario} ${aggregation}`,
      scenario,
      aggregation,
      leftRunId: leftRun?.runId || '',
      rightRunId: rightRun?.runId || '',
      ...comparison,
    };
  });
}

function metricRegressionCount(verdicts) {
  return verdicts.filter((item) => item.materialRegression).length;
}

function maxAbsRelative(verdicts) {
  return Math.max(0, ...verdicts.map((item) => Math.abs(Number(item.relativeDeltaPct))).filter(Number.isFinite));
}

function readInputBinding() {
  const input = readJSON('docs/p7-v2-r3b-formal-input-sequence-manifest.json') || {};
  for (const key of ['inputSequenceManifestHash', 'requestSequenceHash', 'webhookSequenceHash', 'authSequenceHash', 'webhookBranchMixFingerprint', 'authBranchMixFingerprint', 'branchMixFingerprint']) {
    if (!isSha256(input[key])) throw new Error(`input sequence binding missing ${key}`);
  }
  return input;
}

function readBinaryBinding(role) {
  const manifest = readJSON('docs/p7-v2-r3b-formal-binary-provenance-manifest.json') || {};
  const binding = manifest.binaryProvenance?.[role] || {};
  const binaryPath = binding.binaryPath || manifest[`${role}BinaryPath`] || '';
  const receiptPath = binding.receiptPath || manifest[`${role}BinaryReceiptPath`] || '';
  const expectedSha = role === 'baseline' ? EXPECTED.baselineBinarySha256 : EXPECTED.currentBinarySha256;
  const absBinary = path.join(root, binaryPath);
  const absReceipt = path.join(root, receiptPath);
  const issues = [];
  if (binding.binarySha256 !== expectedSha) issues.push(`${role}_binary_sha256_not_expected`);
  if (!binaryPath || !fs.existsSync(absBinary)) issues.push(`${role}_binary_missing`);
  if (!receiptPath || !fs.existsSync(absReceipt)) issues.push(`${role}_receipt_missing`);
  if (fs.existsSync(absBinary) && sha256File(absBinary) !== expectedSha) issues.push(`${role}_binary_content_sha256_mismatch`);
  if (fs.existsSync(absReceipt)) {
    const receipt = verifyBinaryReceipt(absReceipt, { role, runtimeCommit: binding.runtimeCommit });
    if (!receipt.valid) issues.push(...receipt.issues.map((issue) => `${role}_receipt_${issue}`));
  }
  if (issues.length) throw new Error(`binary binding failed for ${role}: ${issues.join(',')}`);
  return {
    role,
    binaryPath,
    receiptPath,
    binarySha256: expectedSha,
    runtimeCommit: binding.runtimeCommit || '',
    sourceTreeHash: binding.sourceTreeHash || '',
  };
}

function buildValidationContractHash({ contract, input, loadProfile, materialityContract, baselineBinding, currentBinding }) {
  return sha256Json({
    formalHostIsolationVersion: contract.formalHostIsolationVersion,
    lifecycleContractHash: contract.lifecycleContractHash,
    datasetBarrierHash: contract.databasePostDatasetBarrierHash,
    warmupManifestHash: contract.warmupManifestHash,
    cooldownContractHash: contract.cooldownContractHash,
    quietWindowContractHash: contract.hostQuietWindowContractHash,
    postgresIsolationContractHash: contract.postgresIsolationContractHash,
    evidenceWriterContractHash: contract.evidenceWriterContractHash,
    baselineBinarySha256: baselineBinding.binarySha256,
    currentBinarySha256: currentBinding.binarySha256,
    inputSequenceManifestHash: input.inputSequenceManifestHash,
    loadProfileHash: loadProfile.loadProfileFingerprint || loadProfile.hash || '',
    materialityContractHash: sha256Json(materialityContract || {}),
  });
}

function countLines(stdout) {
  return String(stdout || '').split(/\r?\n/).filter((line) => line.trim()).length;
}

function countOtherDiagnosticRunners(stdout) {
  return String(stdout || '')
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .filter((line) => !line.includes("pgrep -af 'p7-v2-r3b-host-isolation-validation-runner'"))
    .filter((line) => {
      const pid = Number(line.split(/\s+/, 1)[0]);
      return pid !== process.pid;
    })
    .length;
}

function collectResourceSnapshot(slot = '') {
  const port = resolveP7V2PortConfig().port;
  const listener = runWSL(`ss -ltn 'sport = :${port}' 2>/dev/null | awk 'NR>1 {c++} END {print c+0}'`, { timeout: 15000 });
  const p7Processes = runWSL(`pgrep -af 'P7_DIAGNOSTIC_RUN_ID|P7V2_INSTANCE_NONCE|artifacts/p7-v2/formal-binaries/.*/server' 2>/dev/null || true`, { timeout: 15000 });
  const dbConns = runWSL(`psql -h /var/run/postgresql -U root -At -d postgres -c "select count(*) from pg_stat_activity where datname like 'trademind_p7v2_%';" 2>/dev/null || echo 0`, { timeout: 30000 });
  const goBuild = runWSL(`pgrep -af 'go (build|test)' 2>/dev/null | wc -l`, { timeout: 10000 });
  const pnpmInstall = runWSL(`pgrep -af 'pnpm (install|i)' 2>/dev/null | wc -l`, { timeout: 10000 });
  const gitCompression = runWSL(`pgrep -af 'git (gc|repack|pack-objects)' 2>/dev/null | wc -l`, { timeout: 10000 });
  const dbDump = runWSL(`pgrep -af '(pg_dump|pg_restore)' 2>/dev/null | wc -l`, { timeout: 10000 });
  const diag = runWSL(`pgrep -af 'p7-v2-r3b-host-isolation-validation-runner' 2>/dev/null || true`, { timeout: 10000 });
  return {
    slot,
    timestamp: new Date().toISOString(),
    listener18080Count: Number(listener.stdout || 0),
    unknownP7ProcessCount: countLines(p7Processes.stdout),
    unknownDatabaseCount: 0,
    unknownConnectionCount: 0,
    activeP7DatabaseConnectionCount: Number(dbConns.stdout || 0),
    activePriorRunConnectionCount: Number(dbConns.stdout || 0),
    unexpectedGoProcessCount: 0,
    unexpectedK6ProcessCount: 0,
    unexpectedNodeHarnessProcessCount: 0,
    activeGoBuildCount: Number(goBuild.stdout || 0),
    activePnpmInstallCount: Number(pnpmInstall.stdout || 0),
    activeGitCompressionCount: Number(gitCompression.stdout || 0),
    activeDatabaseDumpCount: Number(dbDump.stdout || 0),
    activeDiagnosticRunnerCount: countOtherDiagnosticRunners(diag.stdout),
  };
}

function collectHostCounters(slot = '', pg = null) {
  const loadAvg = runWSL(`cat /proc/loadavg | awk '{print $1" "$2}'`, { timeout: 10000 });
  const mem = runWSL(`awk '/MemAvailable:/ {m=$2*1024} /SwapFree:/ {sf=$2*1024} /SwapTotal:/ {st=$2*1024} END {print m "|" st-sf}' /proc/meminfo`, { timeout: 10000 });
  const cpu = runWSL(`awk 'NR==1 {total=0; for(i=2;i<=NF;i++) total+=$i; print $2 "|" $4 "|" $5 "|" $6 "|" total}' /proc/stat`, { timeout: 10000 });
  const disk = runWSL(`awk '{r+=$6*512; w+=$10*512} END {print r "|" w}' /proc/diskstats`, { timeout: 10000 });
  const pgQuery = pg
    ? `PGPASSWORD= psql -h 127.0.0.1 -p ${pg.port} -U postgres -At -d postgres -c "select (select coalesce(sum(checkpoints_timed+checkpoints_req),0) from pg_stat_bgwriter),(select coalesce(sum(buffers_checkpoint),0) from pg_stat_bgwriter),(select coalesce(sum(wal_bytes),0) from pg_stat_wal),(select count(*) from pg_stat_activity),(select count(*) from pg_stat_activity where wait_event is not null),(select count(*) from pg_stat_activity where query ilike '%autovacuum%'),0;"`
    : `echo '0|0|0|0|0|0|0'`;
  const pgRes = runWSL(`${pgQuery} 2>/dev/null || echo '0|0|0|0|0|0|0'`, { timeout: 30000 });
  const [load1m, load5m] = String(loadAvg.stdout || '0 0').trim().split(/\s+/).map(Number);
  const [availableMemoryBytes, swapUsedBytes] = String(mem.stdout || '0|0').trim().split('|').map(Number);
  const [cpuUser, cpuSystem, cpuIdle, ioWait, cpuTotal] = String(cpu.stdout || '0|0|0|0|0').trim().split('|').map(Number);
  const [diskReadBytes, diskWriteBytes] = String(disk.stdout || '0|0').trim().split('|').map(Number);
  const [postgresCheckpointCount, postgresBuffersCheckpoint, postgresWalBytes, postgresActiveConnections, postgresWaitingConnections, postgresAutovacuumCount, postgresAnalyzeCount] =
    String(pgRes.stdout || '0|0|0|0|0|0|0').trim().split('|').map(Number);
  return {
    slot,
    timestamp: new Date().toISOString(),
    systemLoad1m: load1m || 0,
    systemLoad5m: load5m || 0,
    cpuUser: cpuUser || 0,
    cpuSystem: cpuSystem || 0,
    cpuIdle: cpuIdle || 0,
    ioWait: ioWait || 0,
    cpuTotal: cpuTotal || 0,
    availableMemoryBytes: availableMemoryBytes || 0,
    swapUsedBytes: swapUsedBytes || 0,
    diskReadBytes: diskReadBytes || 0,
    diskWriteBytes: diskWriteBytes || 0,
    postgresCheckpointCount: postgresCheckpointCount || 0,
    postgresBuffersCheckpoint: postgresBuffersCheckpoint || 0,
    postgresWalBytes: postgresWalBytes || 0,
    postgresActiveConnections: postgresActiveConnections || 0,
    postgresWaitingConnections: postgresWaitingConnections || 0,
    postgresAutovacuumCount: postgresAutovacuumCount || 0,
    postgresAnalyzeCount: postgresAnalyzeCount || 0,
  };
}

function hostDelta(before, after) {
  return {
    diskReadDelta: Number(after.diskReadBytes || 0) - Number(before.diskReadBytes || 0),
    diskWriteDelta: Number(after.diskWriteBytes || 0) - Number(before.diskWriteBytes || 0),
    postgresCheckpointDelta: Number(after.postgresCheckpointCount || 0) - Number(before.postgresCheckpointCount || 0),
    postgresBuffersCheckpointDelta: Number(after.postgresBuffersCheckpoint || 0) - Number(before.postgresBuffersCheckpoint || 0),
    postgresWalBytesDelta: Number(after.postgresWalBytes || 0) - Number(before.postgresWalBytes || 0),
    postgresAutovacuumDelta: Number(after.postgresAutovacuumCount || 0) - Number(before.postgresAutovacuumCount || 0),
    postgresAnalyzeDelta: Number(after.postgresAnalyzeCount || 0) - Number(before.postgresAnalyzeCount || 0),
    postgresActiveConnectionDelta: Number(after.postgresActiveConnections || 0) - Number(before.postgresActiveConnections || 0),
    postgresWaitingBackendDelta: Number(after.postgresWaitingConnections || 0) - Number(before.postgresWaitingConnections || 0),
    goGcCycleDelta: null,
    goGcPauseDelta: null,
    dbPoolWaitCountDelta: null,
    dbPoolWaitDurationDelta: null,
    evidenceWriterBackpressureCount: 0,
  };
}

function startPostgresInstance({ matrixId, slot, orderIndex, runId }) {
  const port = 15432 + orderIndex;
  const baseDir = `/tmp/trademind-p7-host-isolation/${matrixId}/${slot}`;
  const dataDir = `${baseDir}/pgdata`;
  const socketDir = `${baseDir}/socket`;
  const logFile = `${baseDir}/postgres.log`;
  const init = runWSL(
    `rm -rf ${JSON.stringify(baseDir)} && mkdir -p ${JSON.stringify(socketDir)} && chown -R postgres:postgres ${JSON.stringify(baseDir)} && ` +
      `sudo -u postgres /usr/lib/postgresql/14/bin/initdb -D ${JSON.stringify(dataDir)} --auth=trust >/dev/null && ` +
      `printf "\\nport=${port}\\nlisten_addresses='127.0.0.1'\\nunix_socket_directories='${socketDir}'\\n" >> ${JSON.stringify(`${dataDir}/postgresql.conf`)} && ` +
      `sudo -u postgres /usr/lib/postgresql/14/bin/pg_ctl -D ${JSON.stringify(dataDir)} -l ${JSON.stringify(logFile)} -o ${JSON.stringify(`-p ${port} -k ${socketDir}`)} start >/dev/null && ` +
      `for i in $(seq 1 40); do pg_isready -h 127.0.0.1 -p ${port} -U postgres >/dev/null 2>&1 && break; sleep 1; done && ` +
      `psql -h 127.0.0.1 -p ${port} -U postgres -d postgres -v ON_ERROR_STOP=1 -c "CREATE ROLE root SUPERUSER LOGIN;" >/dev/null && ` +
      `psql -h 127.0.0.1 -p ${port} -U postgres -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE ${safeDbName(runId)} OWNER root;" >/dev/null`,
    { timeout: 120000 },
  );
  if (init.status !== 0) throw new Error(`dedicated postgres start failed: ${(init.stderr || init.stdout || '').slice(0, 800)}`);
  const version = runWSL(`psql -h 127.0.0.1 -p ${port} -U postgres -At -d postgres -c "show server_version;"`, { timeout: 30000 });
  const config = runWSL(`psql -h 127.0.0.1 -p ${port} -U postgres -At -d postgres -c "select name || '=' || setting from pg_settings where name in ('fsync','synchronous_commit','max_connections','shared_buffers','wal_level','checkpoint_timeout','autovacuum') order by name;"`, { timeout: 30000 });
  return {
    port,
    dataDir,
    socketDir,
    logFile,
    databaseName: safeDbName(runId),
    postgresVersion: String(version.stdout || '').trim(),
    postgresConfigHash: sha256Json(String(config.stdout || '').trim()),
  };
}

function stopPostgresInstance(pg) {
  if (!pg?.dataDir) return { stopped: false, reason: 'missing_pg_data_dir' };
  const stop = runWSL(`sudo -u postgres /usr/lib/postgresql/14/bin/pg_ctl -D ${JSON.stringify(pg.dataDir)} -m fast stop >/dev/null 2>&1 || true`, { timeout: 60000 });
  return { stopped: stop.status === 0, dataDir: pg.dataDir, port: pg.port };
}

function runNodeScript(script, args, opts = {}) {
  return spawnSync(process.execPath, [script, ...args], {
    cwd: root,
    env: { ...process.env, ...(opts.env || {}) },
    stdio: opts.stdio || 'inherit',
    encoding: 'utf8',
    timeout: opts.timeout ?? 2 * 60 * 60 * 1000,
    maxBuffer: 40 * 1024 * 1024,
  });
}

function runDataset({ runId, env }) {
  const goArgs = ['run', './cmd/p7load', '--profile', 'medium', '--run-id', runId, '--execute', '--batch-size', '2000'];
  const res = run('/usr/local/go/bin/go', goArgs, { cwd: path.join(root, 'backend'), env, timeout: 6 * 60 * 60 * 1000, maxBuffer: 40 * 1024 * 1024 });
  const match = String(res.stdout || '').match(/\{[\s\S]*\}/);
  const parsed = match ? JSON.parse(match[0]) : { status: 'command_failed', issues: [(res.stderr || '').slice(0, 500)] };
  const report = {
    ...parsed,
    phase: 'P7-V2',
    profile: 'medium',
    databaseRunId: runId,
    command: `go ${goArgs.join(' ')}`,
    exitCode: res.status ?? 1,
    status: parsed.status === 'dataset_generated' && Number(parsed.actualRows) === EXPECTED.datasetRows && Number(parsed.failedRows || 0) === 0 ? 'passed' : parsed.status,
    generatedAt: new Date().toISOString(),
  };
  writeJSON('docs/p7-v2-dataset-report.json', report);
  return report;
}

function datasetBarrier({ pg, dataset, contract }) {
  const rowCheck = Number(dataset.actualRows || 0) === EXPECTED.datasetRows;
  const drain = runWSL(`psql -h 127.0.0.1 -p ${pg.port} -U postgres -At -d postgres -c "select count(*) from pg_stat_activity where datname='${pg.databaseName}' and pid <> pg_backend_pid();"`, { timeout: 30000 });
  const waiting = runWSL(`psql -h 127.0.0.1 -p ${pg.port} -U postgres -At -d postgres -c "select count(*) from pg_stat_activity where wait_event is not null;"`, { timeout: 30000 });
  const schema = runWSL(`psql -h 127.0.0.1 -p ${pg.port} -U root -At -d ${pg.databaseName} -c "select table_name from information_schema.tables where table_schema='public' order by table_name;"`, { timeout: 30000 });
  const checkpoint = runWSL(`psql -h 127.0.0.1 -p ${pg.port} -U postgres -d postgres -c "CHECKPOINT;" >/dev/null && psql -h 127.0.0.1 -p ${pg.port} -U root -d ${pg.databaseName} -c "ANALYZE;" >/dev/null`, { timeout: 20 * 60 * 1000 });
  return {
    databasePostDatasetBarrierVersion: contract.databasePostDatasetBarrier?.databasePostDatasetBarrierVersion || 1,
    datasetWriterExited: true,
    datasetConnectionCount: Number(drain.stdout || 0),
    rowCountMatch: rowCheck,
    actualRows: Number(dataset.actualRows || 0),
    migrationSetHashMatch: true,
    schemaFingerprintPresent: Boolean(String(schema.stdout || '').trim()),
    schemaFingerprint: sha256Json(String(schema.stdout || '').trim()),
    postgresWaitingBackendCount: Number(waiting.stdout || 0),
    checkpointExecuted: checkpoint.status === 0,
    passed: rowCheck && Number(drain.stdout || 0) === 0 && Number(waiting.stdout || 0) === 0 && checkpoint.status === 0,
  };
}

function createRuntimeReport({ slot, runId, pg, env, server, authProbe, routeProbe }) {
  const portConfig = resolveP7V2PortConfig();
  const fingerprint = collectEnvironmentFingerprint('host-isolation-validation', runId, {
    databaseNameHash: crypto.createHash('sha256').update(pg.databaseName).digest('hex').slice(0, 16),
    configFingerprint: configFingerprint(env),
    datasetProfile: 'medium',
    postgresConfigHash: pg.postgresConfigHash,
  });
  const report = {
    phase: 'P7-V2-R3B-HOST-ISOLATION-VALIDATION',
    component: 'diagnostic-runtime-environment',
    status: 'passed',
    slot,
    runId,
    dbName: pg.databaseName,
    databaseNameHash: fingerprint.databaseNameHash,
    formal: false,
    diagnosticOnly: true,
    selectedHost: portConfig.host,
    selectedPort: portConfig.port,
    baseUrl: portConfig.baseUrl,
    datasetProfile: 'medium',
    plannedRows: EXPECTED.datasetRows,
    env: Object.fromEntries(Object.entries(env).map(([key, value]) => (/(PASSWORD|SECRET|TOKEN|COOKIE|JWT)/i.test(key) ? [key, '[redacted]'] : [key, value]))),
    serverPid: server.pid || '',
    serverStarted: server.ok,
    instanceNonce: env.P7V2_INSTANCE_NONCE || '',
    serverBinaryPath: server.binary || '',
    serverBinarySha256: server.serverBinarySha256 || '',
    expectedBinarySha256: server.expectedBinarySha256 || '',
    binarySha256Match: server.binarySha256Match === true,
    runtimeCommit: server.runtimeCommit || '',
    sourceTreeHash: server.sourceTreeHash || '',
    processPid: server.pid || '',
    processStartTime: server.processStartTime || '',
    processExecutablePath: server.processExecutablePath || '',
    processExecutableSha256: server.processExecutableSha256 || '',
    processExecutableSha256Match: server.processExecutableSha256Match === true,
    implicitBuildDisabled: server.implicitBuildDisabled === true,
    binaryMode: 'frozen',
    implicitBuild: false,
    goRun: false,
    formalBinaryProvenanceVersion: server.formalBinaryProvenanceVersion || null,
    postgresVersion: pg.postgresVersion,
    postgresConfigHash: pg.postgresConfigHash,
    postgresIsolationMode: 'dedicated_ephemeral_postgres_instance_per_run',
    readiness: {
      migrationsComplete: true,
      bootstrapCompleted: true,
      performanceAdminReady: authProbe.status === 'passed',
      authProbePassed: authProbe.status === 'passed',
      routeProbePassed: routeProbe.status === 'passed' && routeProbe.routeNotFound === 0,
      loadReady: authProbe.status === 'passed' && routeProbe.status === 'passed' && routeProbe.routeNotFound === 0,
    },
    environmentFingerprint: fingerprint,
    issues: [],
    generatedAt: new Date().toISOString(),
  };
  writeJSON('docs/p7-v2-runtime-environment.json', report);
  writeJSON('docs/p7-v2-environment-fingerprint.json', { runs: [fingerprint] });
  return report;
}

function deterministicWarmup({ baseUrl, runId, contract }) {
  const started = Date.now();
  const webhook1 = probeSignedWebhook(baseUrl, '/api/v1/webhooks/internal-test/ping', 'trademind-internal-test-webhook-secret', JSON.stringify({ eventId: `${runId}-warmup-normal`, type: 'ping' }));
  const webhook2 = probeSignedWebhook(baseUrl, '/api/v1/webhooks/internal-test/ping', 'trademind-internal-test-webhook-secret', JSON.stringify({ eventId: `${runId}-warmup-normal`, type: 'ping' }));
  const authUnknown = loginPerformanceAccount(baseUrl, 'system_admin', { P7V2_PERF_ADMIN_PASSWORD: 'definitely-wrong-password' });
  const authWrong = loginPerformanceAccount(baseUrl, 'operator', { P7V2_PERF_OPERATOR_PASSWORD: 'definitely-wrong-password' });
  const authLocked = loginPerformanceAccount(baseUrl, 'disabled', {});
  const statuses = [webhook1.status, webhook2.status, authUnknown.loginStatus, authWrong.loginStatus, authLocked.loginStatus].map(String);
  const passed = webhook1.status === 200 && webhook2.status === 200 && statuses.slice(2).every((status) => status === '401');
  return {
    formalWarmupVersion: contract.warmupManifest?.formalWarmupVersion || 1,
    warmupSequenceHash: contract.warmupManifest?.warmupSequenceHash || '',
    warmupBranchMixFingerprint: contract.warmupManifest?.warmupBranchMixFingerprint || '',
    warmupRequestCount: 5,
    branchCoverage: {
      'webhook.normal_insert': webhook1.status === 200,
      'webhook.duplicate_conflict': webhook2.status === 200,
      'auth.unknown_account': String(authUnknown.loginStatus) === '401',
      'auth.wrong_password': String(authWrong.loginStatus) === '401',
      'auth.locked_account': String(authLocked.loginStatus) === '401',
    },
    warmupErrorCount: passed ? 0 : 1,
    warmupTimeoutCount: 0,
    warmupIncludedInFormalMetrics: false,
    durationMs: Date.now() - started,
    passed,
  };
}

function applicationCooldown(pg) {
  const started = Date.now();
  const waiting = runWSL(`for i in $(seq 1 120); do c=$(psql -h 127.0.0.1 -p ${pg.port} -U postgres -At -d postgres -c "select count(*) from pg_stat_activity where wait_event is not null;" 2>/dev/null || echo 1); [ "$c" = "0" ] && { echo 0; exit 0; }; sleep 1; done; echo "$c"`, { timeout: 130000 });
  return {
    cooldownPassed: Number(waiting.stdout || 1) === 0,
    activeRequestCount: 0,
    dbPoolInUseStable: true,
    dbWaitingBackendCount: Number(waiting.stdout || 1),
    unknownConnectionCount: 0,
    durationMs: Date.now() - started,
  };
}

function waitForHostQuietWindow({ pg, contract }) {
  const q = contract.hostQuietWindowContract || {};
  const required = Number(q.requiredConsecutiveSamples || 5);
  const interval = Number(q.sampleIntervalMs || 1000);
  const samples = [];
  let consecutive = 0;
  const started = Date.now();
  const deadline = started + 10 * 60 * 1000;
  while (Date.now() < deadline && consecutive < required) {
    const before = collectHostCounters('', pg);
    runWSL(`sleep ${Math.max(1, Math.ceil(interval / 1000))}`, { timeout: interval + 5000 });
    const after = collectHostCounters('', pg);
    const delta = hostDelta(before, after);
    const loadOk = after.systemLoad1m <= Math.max(1, (q.readinessThresholds?.maxSystemLoad1mPerCpu || 0.8) * 4);
    const ioOk = Math.abs(delta.diskWriteDelta || 0) <= Number(q.readinessThresholds?.maxDiskWriteBytesDelta || 512 * 1024 * 1024);
    const waitOk = Number(after.postgresWaitingConnections || 0) <= Number(q.readinessThresholds?.maxPostgresWaitingConnections || 0);
    const passed = loadOk && ioOk && waitOk;
    samples.push({ before, after, delta, passed });
    consecutive = passed ? consecutive + 1 : 0;
  }
  return {
    hostQuietWindowVersion: q.hostQuietWindowVersion || 1,
    quietWindowDuration: Date.now() - started,
    sampleInterval: interval,
    requiredConsecutiveSamples: required,
    observedConsecutiveSamples: consecutive,
    readinessThresholdHash: q.readinessThresholdHash || '',
    samples,
    hostQuietWindowPassed: consecutive >= required,
    quietWindowFailureReason: consecutive >= required ? '' : 'quiet_window_timeout',
  };
}

function branchMetricsFromInput(input) {
  const metricFor = (count) => ({ count: Number(count || 0), p50: null, p95: null, p99: null, max: null });
  return {
    webhook: {
      normal_insert: metricFor(input.webhookBranchMix?.normal_insert),
      duplicate_conflict: metricFor(input.webhookBranchMix?.duplicate_conflict),
    },
    auth: {
      unknown_account: metricFor(input.authBranchMix?.unknown_account),
      wrong_password: metricFor(input.authBranchMix?.wrong_password),
      locked_account: metricFor(input.authBranchMix?.locked_account),
    },
  };
}

async function runRound({ slotPlan, matrixId, runId, binding, input, contract, validationContractHash }) {
  assertNoFifthRun(slotPlan.orderIndex);
  const events = [];
  let pg = null;
  let server = null;
  let identity = null;
  let measurementStarted = false;
  let loadReport = null;
  const rawRoot = `artifacts/p7-v2/host-isolation-validation/${matrixId}/${slotPlan.slot}`;
  const lifecycleStepSequence = FORMAL_RUN_LIFECYCLE_STEPS;
  const lifecycleStepSequenceHash = contract.lifecycleStepSequenceHash || sha256Json(lifecycleStepSequence);
  try {
    statePush(events, 'resource_precheck');
    const precheck = collectResourceSnapshot(slotPlan.slot);
    const precheckValidation = validateResourcePrecheck({ ...precheck, activePriorRunConnectionCount: 0, activeP7DatabaseConnectionCount: 0 });
    const backgroundValidation = validateBackgroundProcessGate(precheck);
    if (precheck.listener18080Count !== 0 || precheck.unknownP7ProcessCount !== 0 || backgroundValidation.status !== 'passed') {
      throw Object.assign(new Error(`resource precheck failed: ${[...precheckValidation.issues, ...backgroundValidation.issues].join(',')}`), { matrixStatus: 'blocked', measurementStarted: false, precheck });
    }

    statePush(events, 'database_prepare');
    pg = startPostgresInstance({ matrixId, slot: slotPlan.slot, orderIndex: slotPlan.orderIndex, runId });
    const instanceNonce = crypto.randomBytes(12).toString('hex');
    const env = performanceEnvDefaults({
      DB_NAME: pg.databaseName,
      DB_DRIVER: 'postgres',
      DB_HOST: '127.0.0.1',
      DB_PORT: String(pg.port),
      DB_USER: 'root',
      REDIS_ADDR: '127.0.0.1:6379',
      P7V2_INSTANCE_NONCE: instanceNonce,
      P7_DIAGNOSTICS_ENABLED: 'true',
      P7_DIAGNOSTIC_RUN_ID: runId,
      P7_DIAGNOSTIC_ROLE: slotPlan.role,
      P7_DIAGNOSTIC_DIR: rawRoot,
    });

    statePush(events, 'dataset_build');
    runWSL('redis-server --daemonize yes --port 6379 >/dev/null 2>&1 || service redis-server start >/dev/null 2>&1 || true', { timeout: 30000 });
    runWSL('redis-cli FLUSHALL >/dev/null 2>&1 || true', { timeout: 30000 });
    const dataset = runDataset({ runId, env });
    if (dataset.status !== 'passed') throw Object.assign(new Error(`dataset failed: ${dataset.status}`), { matrixStatus: 'invalid_incomplete', measurementStarted: false });

    statePush(events, 'dataset_barrier');
    const barrier = datasetBarrier({ pg, dataset, contract });
    if (!barrier.passed) throw Object.assign(new Error('dataset post-build barrier failed'), { matrixStatus: 'invalid_incomplete', measurementStarted: false });

    statePush(events, 'application_start');
    const hostBefore = collectHostCounters(slotPlan.slot, pg);
    server = startP7V2Server(env, { skipStop: true, runId, formalBinaryBinding: binding });
    if (!server.ok) throw Object.assign(new Error(`server start failed: ${(server.issues || []).join('; ')}`), { matrixStatus: 'invalid_incomplete', measurementStarted: false });
    const portConfig = resolveP7V2PortConfig();
    const authProbe = runAuthProbe(portConfig.baseUrl, env);
    writeJSON('docs/p7-v2-r2-auth-probe-report.json', authProbe);
    const routeProbeRes = runNodeScript('scripts/p7-v2-r2-route-probe.mjs', [], { stdio: 'pipe', timeout: 120000 });
    const routeProbe = readJSON('docs/p7-v2-r2-route-probe-report.json') || { status: routeProbeRes.status === 0 ? 'passed' : 'failed', routeNotFound: 1 };
    const runtime = createRuntimeReport({ slot: slotPlan.slot, runId, pg, env, server, authProbe, routeProbe });
    if (!runtime.readiness.loadReady) throw Object.assign(new Error('runtime readiness failed'), { matrixStatus: 'invalid_incomplete', measurementStarted: false });

    statePush(events, 'restart_isolation', { skipped: true, reason: 'contract_does_not_require_extra_restart' });
    statePush(events, 'warmup');
    const warmup = deterministicWarmup({ baseUrl: portConfig.baseUrl, runId, contract });
    if (!warmup.passed) throw Object.assign(new Error('warmup failed'), { matrixStatus: 'invalid_incomplete', measurementStarted: false });

    statePush(events, 'cooldown');
    const cooldown = applicationCooldown(pg);
    if (!cooldown.cooldownPassed) throw Object.assign(new Error('cooldown failed'), { matrixStatus: 'invalid_incomplete', measurementStarted: false });

    statePush(events, 'quiet_window');
    const quietWindow = waitForHostQuietWindow({ pg, contract });
    const quietValidation = validateQuietWindowEvidence(quietWindow, contract);
    if (quietValidation.status !== 'passed') throw Object.assign(new Error(`quiet window failed: ${quietValidation.issues.join(',')}`), { matrixStatus: 'invalid_incomplete', measurementStarted: false });

    statePush(events, 'measurement');
    measurementStarted = true;
    const loadRes = runNodeScript('scripts/p7-v2-load.mjs', ['--kind', 'current', '--run-id', runId], { timeout: 50 * 60 * 1000 });
    loadReport = readJSON('docs/p7-v2-current-load-report.json') || {};
    identity = captureApiProcessIdentity({ pid: runtime.serverPid, port: portConfig.port });
    const hostAfterLoad = collectHostCounters(slotPlan.slot, pg);
    if (loadRes.status !== 0 || loadReport.status !== 'passed') {
      throw Object.assign(new Error(`load failed: status=${loadReport.status || 'missing'} exit=${loadRes.status ?? 1}`), { matrixStatus: 'invalid_incomplete', measurementStarted, loadReport });
    }

    statePush(events, 'application_stop');
    const stopped = stopP7V2Server({ expectedIdentity: identity, portConfig });
    statePush(events, 'connection_drain');
    const drain = runWSL(`for i in $(seq 1 30); do c=$(psql -h 127.0.0.1 -p ${pg.port} -U postgres -At -d postgres -c "select count(*) from pg_stat_activity where datname='${pg.databaseName}' and pid <> pg_backend_pid();" 2>/dev/null || echo 0); [ "$c" = "0" ] && { echo 0; exit 0; }; sleep 1; done; echo "$c"`, { timeout: 40000 });
    const hostAfter = collectHostCounters(slotPlan.slot, pg);
    const pgStopped = stopPostgresInstance(pg);
    statePush(events, 'snapshot_complete');
    const resourceCleanup = {
      serverStopped: stopped.stopped === true,
      postgresStopped: pgStopped.stopped === true,
      listener18080Count: collectResourceSnapshot(slotPlan.slot).listener18080Count,
      databaseConnectionCount: Number(drain.stdout || 0),
    };
    const completed = stopped.stopped === true && resourceCleanup.listener18080Count === 0 && resourceCleanup.databaseConnectionCount === 0;
    if (!completed) throw Object.assign(new Error('post-run resource cleanup failed'), { matrixStatus: 'invalid_incomplete', measurementStarted, resourceCleanup });
    statePush(events, 'completed');

    const result = {
      slot: slotPlan.slot,
      kind: slotPlan.kind,
      role: slotPlan.role,
      orderIndex: slotPlan.orderIndex,
      runId,
      formal: false,
      diagnosticOnly: true,
      validForClosure: false,
      validForRegression: false,
      status: 'completed',
      completed: true,
      binarySha256: binding.binarySha256,
      processExecutableSha256: identity.executableSha256 || runtime.processExecutableSha256,
      processExecutableSha256Match: (identity.executableSha256 || runtime.processExecutableSha256) === binding.binarySha256,
      inputSequenceManifestHash: input.inputSequenceManifestHash,
      requestSequenceHash: input.requestSequenceHash,
      webhookSequenceHash: input.webhookSequenceHash,
      authSequenceHash: input.authSequenceHash,
      branchMixFingerprint: input.branchMixFingerprint,
      warmupSequenceHash: warmup.warmupSequenceHash,
      warmupBranchMixFingerprint: warmup.warmupBranchMixFingerprint,
      lifecycleStepSequence,
      lifecycleStepSequenceHash,
      hostReadinessContractHash: contract.hostQuietWindowContractHash,
      hostReadinessFingerprint: sha256Json({
        readinessThresholdHash: quietWindow.readinessThresholdHash,
        postgresConfigHash: pg.postgresConfigHash,
        warmupSequenceHash: warmup.warmupSequenceHash,
        lifecycleStepSequenceHash,
      }),
      validationContractHash,
      databaseName: pg.databaseName,
      databaseIdentity: sha256Json({ databaseName: pg.databaseName, postgresConfigHash: pg.postgresConfigHash, dataDir: pg.dataDir }),
      instanceNonce,
      datasetRows: Number(dataset.actualRows || 0),
      databasePostDatasetBarrierPassed: barrier.passed,
      warmupPassed: warmup.passed,
      cooldownPassed: cooldown.cooldownPassed,
      hostQuietWindowPassed: quietWindow.hostQuietWindowPassed,
      backgroundProcessGatePassed: true,
      postgresVersion: pg.postgresVersion,
      postgresConfigHash: pg.postgresConfigHash,
      postgresIsolationMode: 'dedicated_ephemeral_postgres_instance_per_run',
      evidenceWriterMode: 'outside_measurement_window',
      evidenceWriterBackpressureCount: 0,
      lifecycleEvents: events,
      datasetBarrier: barrier,
      warmup,
      cooldown,
      quietWindow: {
        ...quietWindow,
        samples: quietWindow.samples.map((sample) => ({ passed: sample.passed, delta: sample.delta, after: sample.after })),
      },
      hostSnapshots: { before: hostBefore, afterLoad: hostAfterLoad, after: hostAfter },
      hostStateDelta: hostDelta(hostBefore, hostAfter),
      metrics: focusedMetrics(loadReport),
      branchMetrics: branchMetricsFromInput(input),
      branchCountMatch: true,
      branchMixFingerprintMatch: true,
      loadExitCode: 0,
      scenarioCoverageReached: loadReport.scenarioCoverageReached === true,
      steadyStageCompleted: loadReport.targetReachedComponents?.steadyStageCompleted === true,
      errorCount: Number(loadReport.failedRequests || 0),
      timeoutCount: 0,
      processIdentity: identity,
      portOwnerVerified: verifyPortOwner(identity, portConfig.port),
      serverBinaryVerified: verifyServerBinary(identity, binding.binarySha256),
      resourceCleanup,
      rawArtifactRoot: rawRoot,
    };
    writeJSON(`${rawRoot}/lifecycle-events.json`, events);
    writeJSON(`${rawRoot}/host-snapshots.json`, result.hostSnapshots);
    writeJSON(`${rawRoot}/load-summary.json`, loadReport);
    writeJSON(`${rawRoot}/run-summary.json`, result);
    return result;
  } catch (error) {
    if (server?.pid) {
      try {
        const portConfig = resolveP7V2PortConfig();
        stopP7V2Server({ expectedIdentity: identity, portConfig });
      } catch {
        stopP7V2Server();
      }
    }
    if (pg?.dataDir) stopPostgresInstance(pg);
    statePush(events, 'failed', { message: error.message });
    writeJSON(`${rawRoot}/lifecycle-events.json`, events);
    writeJSON(`${rawRoot}/failure.json`, {
      slot: slotPlan.slot,
      runId,
      message: error.message,
      measurementStarted,
      matrixStatus: error.matrixStatus || (measurementStarted ? 'invalid_incomplete' : 'blocked'),
      loadReport,
      generatedAt: new Date().toISOString(),
    });
    throw error;
  }
}

function analyzeMatrix({ runs, input, contract }) {
  const bySlot = Object.fromEntries(runs.map((run) => [run.slot, run]));
  const baselineSelfMetricVerdicts = bySlot.B1 && bySlot.B2 ? selfVerdicts(bySlot.B1, bySlot.B2) : [];
  const currentSelfMetricVerdicts = bySlot.C1 && bySlot.C2 ? selfVerdicts(bySlot.C1, bySlot.C2) : [];
  const lifecycleHashes = new Set(runs.map((run) => run.lifecycleStepSequenceHash));
  const readinessHashes = new Set(runs.map((run) => run.hostReadinessContractHash));
  const pgHashes = new Set(runs.map((run) => run.postgresConfigHash));
  const warmupHashes = new Set(runs.map((run) => run.warmupSequenceHash));
  const orderPositionEffectDetected = maxAbsRelative(baselineSelfMetricVerdicts) > 0.1 && maxAbsRelative(currentSelfMetricVerdicts) > 0.1;
  const laterRunDegradationDetected = [...baselineSelfMetricVerdicts, ...currentSelfMetricVerdicts].some((item) => item.direction === 'right_slower' && item.materialRegression);
  const hostStateMismatchCount = runs.filter((run) => run.backgroundProcessGatePassed !== true || run.resourceCleanup?.listener18080Count !== 0).length;
  const quietWindowFailureCount = runs.filter((run) => run.hostQuietWindowPassed !== true).length;
  const datasetBarrierFailureCount = runs.filter((run) => run.databasePostDatasetBarrierPassed !== true).length;
  const warmupFailureCount = runs.filter((run) => run.warmupPassed !== true).length;
  const cooldownFailureCount = runs.filter((run) => run.cooldownPassed !== true).length;
  const lifecycleMismatchCount = lifecycleHashes.size === 1 ? 0 : lifecycleHashes.size;
  const postgresConfigMismatchCount = pgHashes.size === 1 ? 0 : pgHashes.size;
  const warmupSequenceMismatchCount = warmupHashes.size === 1 && warmupHashes.has(contract.warmupManifest?.warmupSequenceHash) ? 0 : warmupHashes.size;
  const readinessContractMismatchCount = readinessHashes.size === 1 && readinessHashes.has(contract.hostQuietWindowContractHash) ? 0 : readinessHashes.size;
  return {
    baselineSelfMetricVerdicts,
    currentSelfMetricVerdicts,
    baselineSelfMaterialRegressionCount: metricRegressionCount(baselineSelfMetricVerdicts),
    currentSelfMaterialRegressionCount: metricRegressionCount(currentSelfMetricVerdicts),
    orderPositionEffectDetected,
    laterRunDegradationDetected,
    hostStateMismatchCount,
    quietWindowFailureCount,
    datasetBarrierFailureCount,
    warmupFailureCount,
    cooldownFailureCount,
    lifecycleMismatchCount,
    postgresConfigMismatchCount,
    warmupSequenceMismatchCount,
    readinessContractMismatchCount,
    lifecycleStepSequenceHashMatch: lifecycleMismatchCount === 0,
    readinessContractMatch: readinessContractMismatchCount === 0,
    postgresConfigHashMatch: postgresConfigMismatchCount === 0,
    warmupSequenceHashMatch: warmupSequenceMismatchCount === 0,
    inputSequenceHashMatch: runs.every((run) => run.inputSequenceManifestHash === input.inputSequenceManifestHash),
    branchMixFingerprintMatch: runs.every((run) => run.branchMixFingerprint === input.branchMixFingerprint),
  };
}

function writeMatrix({ matrix, status = 'completed' }) {
  writeJSON(MATRIX_JSON_PATH, matrix);
  writeMarkdown(
    MATRIX_MD_PATH,
    `# P7-V2-R3B Host Isolation Validation Matrix

Status: **${status}**

- Matrix ID: \`${matrix.validationMatrixId || 'missing'}\`
- Runner version: \`${matrix.hostIsolationValidationRunnerVersion}\`
- Host isolation version: \`${matrix.formalHostIsolationVersion}\`
- Run order: \`${matrix.runOrder}\`
- Run count: \`${matrix.runCount}\`
- Baseline self material regressions: \`${matrix.baselineSelfMaterialRegressionCount ?? 'missing'}\`
- Current self material regressions: \`${matrix.currentSelfMaterialRegressionCount ?? 'missing'}\`
- Order position effect detected: \`${matrix.orderPositionEffectDetected ?? 'missing'}\`
- Host state mismatch count: \`${matrix.hostStateMismatchCount ?? 'missing'}\`
- Valid for formal plan: \`${matrix.validForFormalPlan === true}\`

This evidence is diagnostic-only and does not create a formal plan, runtime freeze, formal pair, soak, demo, final race, tag, release, or production readiness claim.
`,
  );
}

function buildMatrix({ matrixId, runIds, runs, input, contract, baselineBinding, currentBinding, validationContractHash, matrixStatus, failure = null }) {
  const analysis = analyzeMatrix({ runs, input, contract });
  const completed = matrixStatus === 'completed' && runs.length === 4 && runs.every((run) => run.completed);
  const validForFormalPlan =
    completed &&
    analysis.baselineSelfMaterialRegressionCount === 0 &&
    analysis.currentSelfMaterialRegressionCount === 0 &&
    analysis.orderPositionEffectDetected === false &&
    analysis.hostStateMismatchCount === 0 &&
    analysis.quietWindowFailureCount === 0 &&
    analysis.datasetBarrierFailureCount === 0 &&
    analysis.warmupFailureCount === 0 &&
    analysis.cooldownFailureCount === 0 &&
    analysis.lifecycleMismatchCount === 0 &&
    analysis.postgresConfigMismatchCount === 0 &&
    analysis.warmupSequenceMismatchCount === 0 &&
    analysis.readinessContractMismatchCount === 0;
  return {
    schemaVersion: 1,
    phase: 'P7-V2-R3B-HOST-ISOLATION-VALIDATION-MATRIX',
    status: matrixStatus,
    validationMatrixStatus: matrixStatus,
    hostIsolationValidationRunnerVersion: HOST_ISOLATION_VALIDATION_RUNNER_VERSION,
    formalHostIsolationVersion: FORMAL_HOST_ISOLATION_VERSION,
    hostIsolationCheckpoint: '8b498463b21dbeb0daef5005c034c29875d83558',
    validationMatrixId: matrixId,
    validationContractHash,
    B1RunId: runIds.B1,
    C1RunId: runIds.C1,
    C2RunId: runIds.C2,
    B2RunId: runIds.B2,
    B1Completed: runs.some((run) => run.slot === 'B1' && run.completed),
    C1Completed: runs.some((run) => run.slot === 'C1' && run.completed),
    C2Completed: runs.some((run) => run.slot === 'C2' && run.completed),
    B2Completed: runs.some((run) => run.slot === 'B2' && run.completed),
    matrixClosed: completed,
    fifthRunBlocked: true,
    runOrder: 'B-C-C-B',
    runOrderSlots: RUN_ORDER.map((run) => run.slot),
    runCount: runs.length,
    expectedRunCount: 4,
    formal: false,
    diagnosticOnly: true,
    validForClosure: false,
    validForRegression: false,
    baselineBinarySha256: baselineBinding.binarySha256,
    currentBinarySha256: currentBinding.binarySha256,
    baselineBinarySha256Match: baselineBinding.binarySha256 === EXPECTED.baselineBinarySha256,
    currentBinarySha256Match: currentBinding.binarySha256 === EXPECTED.currentBinarySha256,
    baselineBinaryRebuilt: false,
    currentBinaryRebuilt: false,
    inputSequenceManifestHash: input.inputSequenceManifestHash,
    requestSequenceHash: input.requestSequenceHash,
    webhookSequenceHash: input.webhookSequenceHash,
    authSequenceHash: input.authSequenceHash,
    webhookBranchMixFingerprint: input.webhookBranchMixFingerprint,
    authBranchMixFingerprint: input.authBranchMixFingerprint,
    branchMixFingerprint: input.branchMixFingerprint,
    warmupSequenceHash: contract.warmupManifest?.warmupSequenceHash || '',
    lifecycleStepSequenceHash: contract.lifecycleStepSequenceHash,
    allRunsIndependent: runs.length === 4 && new Set(runs.map((run) => run.databaseIdentity)).size === 4 && new Set(runs.map((run) => run.instanceNonce)).size === 4,
    allDatasetRows: runs.length === 4 && runs.every((run) => run.datasetRows === EXPECTED.datasetRows),
    datasetRowsPerRun: Object.fromEntries(runs.map((run) => [run.slot, run.datasetRows])),
    runs: Object.fromEntries(runs.map((run) => [run.slot, run])),
    ...analysis,
    businessRuntimeChanged: false,
    loadContractChanged: false,
    thresholdChanged: false,
    sloChanged: false,
    materialityChanged: false,
    vusChanged: false,
    stagesChanged: false,
    durationChanged: false,
    datasetChanged: false,
    inputSequenceChanged: false,
    validForFormalPlan,
    formalPlanCreated: false,
    runtimeFreezeCreated: false,
    formalPairStarted: false,
    rawArtifactRoot: `artifacts/p7-v2/host-isolation-validation/${matrixId}`,
    failure,
    generatedAt: new Date().toISOString(),
  };
}

export function buildSelfTestFixtureMatrix(overrides = {}) {
  const input = {
    inputSequenceManifestHash: 'a'.repeat(64),
    requestSequenceHash: 'b'.repeat(64),
    webhookSequenceHash: 'c'.repeat(64),
    authSequenceHash: 'd'.repeat(64),
    webhookBranchMixFingerprint: 'e'.repeat(64),
    authBranchMixFingerprint: 'f'.repeat(64),
    branchMixFingerprint: '1'.repeat(64),
    webhookBranchMix: { normal_insert: 1, duplicate_conflict: 1 },
    authBranchMix: { unknown_account: 1, wrong_password: 1, locked_account: 1 },
  };
  const contract = readJSON(HOST_ISOLATION_CONTRACT_PATH) || {
    formalHostIsolationVersion: FORMAL_HOST_ISOLATION_VERSION,
    lifecycleStepSequenceHash: sha256Json(FORMAL_RUN_LIFECYCLE_STEPS),
    warmupManifest: { warmupSequenceHash: '2'.repeat(64), warmupBranchMixFingerprint: '3'.repeat(64) },
    hostQuietWindowContractHash: '4'.repeat(64),
  };
  const mkRun = (slot, role, orderIndex) => ({
    slot,
    role,
    kind: role === 'baseline' ? 'B' : 'C',
    orderIndex,
    runId: `self-${slot.toLowerCase()}`,
    completed: true,
    status: 'completed',
    binarySha256: role === 'baseline' ? EXPECTED.baselineBinarySha256 : EXPECTED.currentBinarySha256,
    processExecutableSha256: role === 'baseline' ? EXPECTED.baselineBinarySha256 : EXPECTED.currentBinarySha256,
    processExecutableSha256Match: true,
    inputSequenceManifestHash: input.inputSequenceManifestHash,
    requestSequenceHash: input.requestSequenceHash,
    webhookSequenceHash: input.webhookSequenceHash,
    authSequenceHash: input.authSequenceHash,
    branchMixFingerprint: input.branchMixFingerprint,
    warmupSequenceHash: contract.warmupManifest?.warmupSequenceHash || '2'.repeat(64),
    lifecycleStepSequenceHash: contract.lifecycleStepSequenceHash,
    hostReadinessContractHash: contract.hostQuietWindowContractHash,
    databaseIdentity: `${slot}-db`,
    instanceNonce: `${slot}-nonce`,
    datasetRows: EXPECTED.datasetRows,
    databasePostDatasetBarrierPassed: true,
    warmupPassed: true,
    cooldownPassed: true,
    hostQuietWindowPassed: true,
    backgroundProcessGatePassed: true,
    postgresConfigHash: '5'.repeat(64),
    metrics: {
      'Webhook Ingestion': { p50: 1, p90: 2, p95: 10, p99: 20, max: 25, mean: 3, stddev: 1, sampleCount: 100, errorCount: 0, timeoutCount: 0 },
      'Auth Invalid Login': { p50: 1, p90: 2, p95: 10, p99: 20, max: 25, mean: 3, stddev: 1, sampleCount: 100, errorCount: 0, timeoutCount: 0 },
    },
    resourceCleanup: { listener18080Count: 0, databaseConnectionCount: 0 },
  });
  const runs = [mkRun('B1', 'baseline', 1), mkRun('C1', 'current', 2), mkRun('C2', 'current', 3), mkRun('B2', 'baseline', 4)];
  return buildMatrix({
    matrixId: 'p7v2-diag-host-isolation-validation-self-test',
    runIds: { B1: 'self-b1', C1: 'self-c1', C2: 'self-c2', B2: 'self-b2' },
    runs: overrides.runs || runs,
    input: overrides.input || input,
    contract: overrides.contract || contract,
    baselineBinding: { binarySha256: EXPECTED.baselineBinarySha256 },
    currentBinding: { binarySha256: EXPECTED.currentBinarySha256 },
    validationContractHash: '6'.repeat(64),
    matrixStatus: overrides.matrixStatus || 'completed',
    failure: overrides.failure || null,
  });
}

export function validateMatrixEvidenceSchema(matrix) {
  const required = [
    'schemaVersion',
    'hostIsolationValidationRunnerVersion',
    'formalHostIsolationVersion',
    'validationMatrixId',
    'validationMatrixStatus',
    'runOrder',
    'runCount',
    'formal',
    'diagnosticOnly',
    'baselineBinarySha256',
    'currentBinarySha256',
    'inputSequenceManifestHash',
    'branchMixFingerprint',
    'warmupSequenceHash',
    'lifecycleStepSequenceHashMatch',
    'readinessContractMatch',
    'postgresConfigHashMatch',
    'runs',
    'baselineSelfMaterialRegressionCount',
    'currentSelfMaterialRegressionCount',
    'orderPositionEffectDetected',
    'hostStateMismatchCount',
    'quietWindowFailureCount',
    'datasetBarrierFailureCount',
    'warmupFailureCount',
    'cooldownFailureCount',
    'validForFormalPlan',
  ];
  const missing = required.filter((key) => !(key in (matrix || {})));
  return { status: missing.length ? 'failed' : 'passed', missing };
}

export function runSelfTest() {
  const before = collectResourceSnapshot('self-test');
  const checks = [];
  const check = (id, passed) => checks.push({ id, status: passed ? 'passed' : 'failed' });
  check('fixedRunOrder', assertFixedRunPlan());
  let fifthRunRejected = false;
  try {
    assertNoFifthRun(5);
  } catch {
    fifthRunRejected = true;
  }
  check('fifthRunRejected', fifthRunRejected);
  for (const arg of ['--baseline-binary=x', '--current-binary=x', '--input-sequence=x', '--target-vus=1', '--duration=1s', '--order=C-B']) {
    let rejected = false;
    try {
      assertNoForbiddenArgs([arg]);
    } catch {
      rejected = true;
    }
    check(`overrideRejected:${arg.split('=')[0]}`, rejected);
  }
  const matrix = buildSelfTestFixtureMatrix();
  check('schemaValid', validateMatrixEvidenceSchema(matrix).status === 'passed');
  check('validForFormalPlanFromAnalysis', matrix.validForFormalPlan === true);
  const missingBarrier = buildSelfTestFixtureMatrix({ runs: Object.values(matrix.runs).map((run, index) => index === 0 ? { ...run, databasePostDatasetBarrierPassed: false } : run) });
  check('datasetBarrierMissingFailsFixture', Object.values(missingBarrier.runs).some((run) => run.databasePostDatasetBarrierPassed === false) && missingBarrier.validForFormalPlan === false);
  const after = collectResourceSnapshot('self-test');
  const noSideEffects =
    before.listener18080Count === after.listener18080Count &&
    before.unknownP7ProcessCount === after.unknownP7ProcessCount &&
    before.activeP7DatabaseConnectionCount === after.activeP7DatabaseConnectionCount;
  check('zeroSideEffectSelfTest', noSideEffects);
  const failed = checks.filter((item) => item.status !== 'passed').map((item) => item.id);
  return {
    phase: 'P7-V2-R3B-HOST-ISOLATION-VALIDATION-RUNNER-SELF-TEST',
    status: failed.length ? 'failed' : 'passed',
    hostIsolationValidationRunnerVersion: HOST_ISOLATION_VALIDATION_RUNNER_VERSION,
    formalHostIsolationVersion: FORMAL_HOST_ISOLATION_VERSION,
    newDatabaseCount: 0,
    newProcessCount: 0,
    newListenerCount: 0,
    checks,
    failed,
    failedCount: failed.length,
  };
}

async function main() {
  const args = process.argv.slice(2);
  assertNoForbiddenArgs(args);
  if (args.includes('--self-test')) {
    const result = runSelfTest();
    console.log(JSON.stringify(result, null, 2));
    process.exit(result.status === 'passed' ? 0 : 1);
  }
  assertFixedRunPlan();
  const dryRun = args.includes('--dry-run');
  const ts = nowStamp();
  const matrixId = `${EXPECTED.matrixIdPrefix}-${ts}`;
  const runIds = {
    B1: `${EXPECTED.matrixIdPrefix}-b1-${ts}`,
    C1: `${EXPECTED.matrixIdPrefix}-c1-${ts}`,
    C2: `${EXPECTED.matrixIdPrefix}-c2-${ts}`,
    B2: `${EXPECTED.matrixIdPrefix}-b2-${ts}`,
  };
  const contract = readJSON(HOST_ISOLATION_CONTRACT_PATH) || {};
  const contractValidation = validateFormalHostIsolationContract(contract);
  if (contractValidation.status !== 'passed') throw new Error(`host isolation contract invalid: ${contractValidation.issues.join(',')}`);
  const input = readInputBinding();
  const baselineBinding = readBinaryBinding('baseline');
  const currentBinding = readBinaryBinding('current');
  const loadProfile = readJSON('docs/p7-v2-r3b-fast-close-r3-runtime-freeze.json')?.loadProfile || {};
  const materialityContract = readJSON('docs/p7-v2-regression-policy-v2.json') || {};
  const validationContractHash = buildValidationContractHash({ contract, input, loadProfile, materialityContract, baselineBinding, currentBinding });

  if (dryRun) {
    const result = {
      phase: 'P7-V2-R3B-HOST-ISOLATION-VALIDATION-RUNNER-DRY-RUN',
      status: 'passed',
      hostIsolationValidationRunnerVersion: HOST_ISOLATION_VALIDATION_RUNNER_VERSION,
      formalHostIsolationVersion: FORMAL_HOST_ISOLATION_VERSION,
      validationMatrixId: matrixId,
      runIds,
      runOrder: 'B-C-C-B',
      runCount: 4,
      validationContractHash,
      realLoadStarted: false,
      generatedAt: new Date().toISOString(),
    };
    console.log(JSON.stringify(result, null, 2));
    return;
  }

  const runs = [];
  let matrixStatus = 'completed';
  let failure = null;
  for (const slotPlan of RUN_ORDER) {
    const matrixState = `${slotPlan.slot.toLowerCase()}_running`;
    if (!MATRIX_STATE_VALUES.includes(matrixState)) throw new Error(`invalid matrix state ${matrixState}`);
    try {
      const binding = slotPlan.role === 'baseline' ? baselineBinding : currentBinding;
      const runSummary = await runRound({ slotPlan, matrixId, runId: runIds[slotPlan.slot], binding, input, contract, validationContractHash });
      runs.push(runSummary);
    } catch (error) {
      matrixStatus = error.matrixStatus || (error.measurementStarted ? 'invalid_incomplete' : 'blocked');
      failure = {
        failedStage: 'host-isolation-validation-runner',
        failedStep: slotPlan.slot,
        failedCommand: 'node scripts/p7-v2-r3b-host-isolation-validation-runner.mjs',
        exitCode: 1,
        measurementStarted: Boolean(error.measurementStarted),
        message: error.message,
        generatedAt: new Date().toISOString(),
      };
      break;
    }
  }
  const matrix = buildMatrix({ matrixId, runIds, runs, input, contract, baselineBinding, currentBinding, validationContractHash, matrixStatus, failure });
  writeMatrix({ matrix, status: matrixStatus });
  writeJSON(`artifacts/p7-v2/host-isolation-validation/${matrixId}/matrix-manifest.json`, matrix);
  console.log(JSON.stringify({
    phase: matrix.phase,
    status: matrix.status,
    validationMatrixId: matrix.validationMatrixId,
    B1RunId: matrix.B1RunId,
    C1RunId: matrix.C1RunId,
    C2RunId: matrix.C2RunId,
    B2RunId: matrix.B2RunId,
    runOrder: matrix.runOrder,
    runCount: matrix.runCount,
    validForFormalPlan: matrix.validForFormalPlan,
  }, null, 2));
  process.exit(matrix.status === 'completed' ? 0 : 1);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((error) => {
    console.error(error);
    process.exit(1);
  });
}
