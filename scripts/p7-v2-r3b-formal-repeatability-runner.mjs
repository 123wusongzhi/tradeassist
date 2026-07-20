import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { pathToFileURL } from 'node:url';
import {
  collectEnvironmentFingerprint,
  configFingerprint,
  performanceEnvDefaults,
  readJSON,
  resolveP7V2PortConfig,
  root,
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

export const RUN_ORDER = ['B1', 'C1', 'C2', 'B2'];
export const ROLE_BY_SLOT = { B1: 'baseline', C1: 'current', C2: 'current', B2: 'baseline' };
export const KIND_BY_SLOT = { B1: 'B', C1: 'C', C2: 'C', B2: 'B' };
export const TRACKED_METRICS = [
  ['Webhook Ingestion', 'p95'],
  ['Webhook Ingestion', 'p99'],
  ['Auth Invalid Login', 'p95'],
  ['Auth Invalid Login', 'p99'],
];

const EXPECTED = {
  baselineRunId: 'p7v2-baseline-r3b-recovery6-20260718054823',
  currentRunId: 'p7v2-current-r3b-recovery6-20260718054823',
  runtimeFreezeId: '4712cb17a71e5a33c98627cf595d25e2c3d25137287c9b2254c46635e5dde082',
  baselineBinarySha256: '4e0408ac29b777beac7598872dfd8cac6430542eae37b5ba304c3dbdf7bd79f1',
  currentBinarySha256: 'c6564cfe47a9f5cc60ce6b8c7c29accf7460e213318ba62a11c0d480f4f47766',
  datasetRows: 1900150,
};

function sha256Json(value) {
  return crypto.createHash('sha256').update(JSON.stringify(value)).digest('hex');
}

function valueOf(args, name) {
  const index = args.indexOf(name);
  if (index >= 0 && args[index + 1]) return args[index + 1];
  const prefix = `${name}=`;
  const hit = args.find((arg) => arg.startsWith(prefix));
  return hit ? hit.slice(prefix.length) : '';
}

export function assertFixedOrder(order = RUN_ORDER) {
  const actual = order.join('-');
  if (actual !== 'B1-C1-C2-B2') throw new Error(`fixed B-C-C-B order required, got ${actual}`);
  return true;
}

export function assertNoFifthRound(nextIndex) {
  if (Number(nextIndex) > 4) throw new Error('fifth repeatability round is forbidden');
  return true;
}

function asNumber(value) {
  const n = Number(value);
  return Number.isFinite(n) ? n : null;
}

function pct(delta, base) {
  const denominator = Math.abs(Number(base));
  if (!Number.isFinite(delta) || !denominator) return null;
  return (delta / denominator) * 100;
}

export function compareMetric(left, right) {
  const a = asNumber(left);
  const b = asNumber(right);
  if (a === null || b === null) return { absoluteDelta: null, relativeDeltaPct: null, direction: 'not_comparable' };
  const absoluteDelta = b - a;
  return {
    absoluteDelta,
    relativeDeltaPct: pct(absoluteDelta, a),
    direction: absoluteDelta > 0 ? 'right_slower' : absoluteDelta < 0 ? 'right_faster' : 'equal',
  };
}

function compareRuns(leftRun, rightRun) {
  const out = {};
  for (const [scenario, aggregation] of TRACKED_METRICS) {
    const key = `${scenario} ${aggregation}`;
    out[key] = compareMetric(leftRun?.focusedMetrics?.[scenario]?.[aggregation], rightRun?.focusedMetrics?.[scenario]?.[aggregation]);
  }
  return out;
}

function maxAbsRelative(comparison) {
  return Math.max(
    0,
    ...Object.values(comparison || {})
      .map((item) => Math.abs(Number(item.relativeDeltaPct)))
      .filter(Number.isFinite),
  );
}

export function classifyRootCause({ runsBySlot, inputSequenceHashMatch, branchMixFingerprintMatch, binaryProvenancePassed, hostExplainsTail = false }) {
  if (!inputSequenceHashMatch || !branchMixFingerprintMatch) return 'E_input_sequence_or_branch_mix_binding_defect';
  if (!binaryProvenancePassed) return 'F_process_executable_or_binary_binding_defect';

  const baselineSelf = compareRuns(runsBySlot.B1, runsBySlot.B2);
  const currentSelf = compareRuns(runsBySlot.C1, runsBySlot.C2);
  const selfCeiling = Math.max(maxAbsRelative(baselineSelf), maxAbsRelative(currentSelf));
  const crossPairs = [
    compareRuns(runsBySlot.B1, runsBySlot.C1),
    compareRuns(runsBySlot.B1, runsBySlot.C2),
    compareRuns(runsBySlot.B2, runsBySlot.C1),
    compareRuns(runsBySlot.B2, runsBySlot.C2),
  ];
  const crossRelatives = crossPairs.flatMap((pair) => Object.values(pair).map((item) => Math.abs(Number(item.relativeDeltaPct))).filter(Number.isFinite));
  const minCross = Math.min(...crossRelatives);
  if (selfCeiling >= 10 && selfCeiling >= minCross) return 'A_formal_harness_repeatability_or_order_bias_defect';

  const stableForScenario = (scenario) =>
    ['p95', 'p99'].every((aggregation) => {
      const key = `${scenario} ${aggregation}`;
      const deltas = crossPairs.map((pair) => pair[key]);
      const allCurrentSlower = deltas.every((item) => item.direction === 'right_slower');
      const scenarioMinCross = Math.min(...deltas.map((item) => Math.abs(Number(item.relativeDeltaPct))).filter(Number.isFinite));
      return allCurrentSlower && scenarioMinCross > Math.max(selfCeiling * 1.2, 5) && !hostExplainsTail;
    });

  const webhook = stableForScenario('Webhook Ingestion');
  const auth = stableForScenario('Auth Invalid Login');
  if (webhook && auth) return 'D_deterministic_multi_path_regression';
  if (webhook) return 'B_deterministic_webhook_tail_regression';
  if (auth) return 'C_deterministic_auth_tail_regression';
  return 'G_insufficient_evidence_after_binary_bound_bounded_matrix';
}

function readRequired(rel) {
  const value = readJSON(rel);
  if (!value) throw new Error(`required evidence missing: ${rel}`);
  return value;
}

function readBinaryBinding(role) {
  const provenance = readRequired('docs/p7-v2-r3b-formal-binary-provenance-manifest.json');
  const binding = provenance.binaryProvenance?.[role] || {};
  const expectedSha = role === 'baseline' ? EXPECTED.baselineBinarySha256 : EXPECTED.currentBinarySha256;
  const binaryPath = binding.binaryPath || provenance[`${role}BinaryPath`] || '';
  const receiptPath = binding.receiptPath || provenance[`${role}BinaryReceiptPath`] || '';
  const binarySha256 = binding.binarySha256 || provenance[`${role}BinarySha256`] || '';
  const runtimeCommit = binding.runtimeCommit || provenance[`${role}RuntimeCommit`] || '';
  const issues = [];
  if (binarySha256 !== expectedSha) issues.push(`${role}_binary_sha256_not_expected`);
  if (!binaryPath || !fs.existsSync(path.join(root, binaryPath))) issues.push(`${role}_binary_missing`);
  if (binaryPath && fs.existsSync(path.join(root, binaryPath)) && sha256File(path.join(root, binaryPath)) !== expectedSha) issues.push(`${role}_binary_content_sha256_mismatch`);
  if (receiptPath) {
    const receipt = verifyBinaryReceipt(path.join(root, receiptPath), { role, runtimeCommit });
    if (!receipt.valid) issues.push(...receipt.issues.map((issue) => `${role}_receipt_${issue}`));
  } else {
    issues.push(`${role}_receipt_missing`);
  }
  return {
    role,
    binaryPath,
    binarySha256,
    runtimeCommit,
    receiptPath,
    sourceTreeHash: binding.sourceTreeHash || '',
    status: issues.length ? 'failed' : 'passed',
    issues,
  };
}

function readInputBinding() {
  const manifest = readRequired('docs/p7-v2-r3b-formal-input-sequence-manifest.json');
  const runManifest = readJSON('docs/p7-v2-r3b-run-manifest.json') || {};
  const runtimeFreeze = readJSON('docs/p7-v2-r3b-fast-close-r3-runtime-freeze.json') || {};
  const keys = [
    'inputSequenceManifestHash',
    'requestSequenceHash',
    'webhookSequenceHash',
    'authSequenceHash',
    'webhookBranchMixFingerprint',
    'authBranchMixFingerprint',
    'branchMixFingerprint',
  ];
  const values = Object.fromEntries(keys.map((key) => [key, manifest[key] || runManifest[key] || runtimeFreeze[key] || '']));
  return { manifest, ...values };
}

function collectHostSnapshot(slot = '') {
  const port = resolveP7V2PortConfig().port;
  const listener = runWSL(`ss -ltn 'sport = :${port}' 2>/dev/null | awk 'NR>1 {c++} END {print c+0}'`, { timeout: 15000 });
  const p7Processes = runWSL(`pgrep -af 'artifacts/p7-v2/(formal-binaries|server)|trademind_p7v2_|P7_DIAGNOSTIC_RUN_ID|P7_V2_RUN_ID' 2>/dev/null || true`, { timeout: 15000 });
  const dbConns = runWSL(`psql -h /var/run/postgresql -U root -At -d postgres -c "select count(*) from pg_stat_activity where datname like 'trademind_p7v2_%';" 2>/dev/null || echo 0`, { timeout: 30000 });
  const loadAvg = runWSL(`cat /proc/loadavg 2>/dev/null | awk '{print $1" "$2}'`, { timeout: 10000 });
  const mem = runWSL(`awk '/MemAvailable:/ {m=$2*1024} /SwapFree:/ {sf=$2*1024} /SwapTotal:/ {st=$2*1024} END {print m "|" st-sf}' /proc/meminfo`, { timeout: 10000 });
  const cpu = runWSL(`awk 'NR==1 {total=0; for(i=2;i<=NF;i++) total+=$i; print $2 "|" $4 "|" $5 "|" $6 "|" total}' /proc/stat`, { timeout: 10000 });
  const disk = runWSL(`awk '{r+=$6*512; w+=$10*512} END {print r "|" w}' /proc/diskstats`, { timeout: 10000 });
  const pg = runWSL(`psql -h /var/run/postgresql -U root -At -d postgres -c "select (select coalesce(sum(checkpoints_timed+checkpoints_req),0) from pg_stat_bgwriter),(select coalesce(sum(buffers_checkpoint),0) from pg_stat_bgwriter),(select coalesce(sum(wal_bytes),0) from pg_stat_wal),(select count(*) from pg_stat_activity),(select count(*) from pg_stat_activity where wait_event is not null),(select count(*) from pg_stat_activity where query ilike '%autovacuum%');" 2>/dev/null || echo '0|0|0|0|0|0'`, { timeout: 30000 });
  const [load1m, load5m] = String(loadAvg.stdout || '0 0').trim().split(/\s+/).map(Number);
  const [availableMemoryBytes, swapUsedBytes] = String(mem.stdout || '0|0').trim().split('|').map(Number);
  const [cpuUser, cpuSystem, cpuIdle, ioWait, cpuTotal] = String(cpu.stdout || '0|0|0|0|0').trim().split('|').map(Number);
  const [diskReadBytes, diskWriteBytes] = String(disk.stdout || '0|0').trim().split('|').map(Number);
  const [postgresCheckpointCount, postgresBuffersCheckpoint, postgresWalBytes, postgresActiveConnections, postgresWaitingConnections, postgresAutovacuumCount] =
    String(pg.stdout || '0|0|0|0|0|0').trim().split('|').map(Number);
  const activeP7ProcessCount = String(p7Processes.stdout || '').split(/\r?\n/).filter((line) => line.trim()).length;
  return {
    slot,
    timestamp: new Date().toISOString(),
    listener18080Count: Number(listener.stdout || 0),
    activeP7ProcessCount,
    unknownProcessCount: Number(listener.stdout || 0) > 0 && activeP7ProcessCount === 0 ? 1 : 0,
    activeP7DatabaseConnectionCount: Number(dbConns.stdout || 0),
    systemLoad1m: load1m || 0,
    systemLoad5m: load5m || 0,
    availableMemoryBytes: availableMemoryBytes || 0,
    swapUsedBytes: swapUsedBytes || 0,
    cpuUser: cpuUser || 0,
    cpuSystem: cpuSystem || 0,
    cpuIdle: cpuIdle || 0,
    ioWait: ioWait || 0,
    cpuTotal: cpuTotal || 0,
    diskReadBytes: diskReadBytes || 0,
    diskWriteBytes: diskWriteBytes || 0,
    postgresCheckpointCount: postgresCheckpointCount || 0,
    postgresBuffersCheckpoint: postgresBuffersCheckpoint || 0,
    postgresWalBytes: postgresWalBytes || 0,
    postgresActiveConnections: postgresActiveConnections || 0,
    postgresWaitingConnections: postgresWaitingConnections || 0,
    postgresAutovacuumCount: postgresAutovacuumCount || 0,
  };
}

function hostDelta(before, after) {
  return {
    checkpointDelta: Number(after.postgresCheckpointCount || 0) - Number(before.postgresCheckpointCount || 0),
    walBytesDelta: Number(after.postgresWalBytes || 0) - Number(before.postgresWalBytes || 0),
    autovacuumDelta: Number(after.postgresAutovacuumCount || 0) - Number(before.postgresAutovacuumCount || 0),
    diskReadDelta: Number(after.diskReadBytes || 0) - Number(before.diskReadBytes || 0),
    diskWriteDelta: Number(after.diskWriteBytes || 0) - Number(before.diskWriteBytes || 0),
    cpuUsage: Math.max(0, 1 - ((Number(after.cpuIdle || 0) - Number(before.cpuIdle || 0)) / Math.max(1, Number(after.cpuTotal || 0) - Number(before.cpuTotal || 0)))),
    ioWait: Number(after.ioWait || 0) - Number(before.ioWait || 0),
    dbPoolWaitCountDelta: null,
    dbPoolWaitDurationDelta: null,
    gcCycleDelta: null,
    gcPauseDelta: null,
  };
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

function scenarioMetrics(report, scenario) {
  const row = (report.scenarios || []).find((item) => item.scenario === scenario) || {};
  return {
    sampleCount: row.sampleCount ?? row.requestCount ?? row.requests ?? null,
    errorCount: report.failedRequests ?? null,
    timeoutCount: row.timeoutCount ?? row.timeouts ?? 0,
    min: row.min ?? null,
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

function createRuntimeReport({ slot, runId, dbName, env, server, authProbe, routeProbe }) {
  const portConfig = resolveP7V2PortConfig();
  const fingerprint = collectEnvironmentFingerprint('binary-repeatability', runId, {
    databaseNameHash: crypto.createHash('sha256').update(dbName).digest('hex').slice(0, 16),
    configFingerprint: configFingerprint(env),
    datasetProfile: 'medium',
  });
  const redactedEnv = Object.fromEntries(Object.entries(env).map(([key, value]) =>
    /(PASSWORD|SECRET|TOKEN|COOKIE|JWT)/i.test(key) ? [key, '[redacted]'] : [key, value],
  ));
  const report = {
    phase: 'P7-V2-R3B-BINARY-BOUND-B-C-C-B-REPEATABILITY-MATRIX',
    component: 'diagnostic-runtime-environment',
    status: 'passed',
    slot,
    runId,
    dbName,
    databaseNameHash: fingerprint.databaseNameHash,
    formal: false,
    diagnosticOnly: true,
    selectedHost: portConfig.host,
    selectedPort: portConfig.port,
    baseUrl: portConfig.baseUrl,
    datasetProfile: 'medium',
    plannedRows: EXPECTED.datasetRows,
    env: redactedEnv,
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

function writeAuditIndex({ input, baselineBinding, currentBinding }) {
  const regression = readJSON('docs/p7-v2-r3b-fast-close-r3-regression-v2-report.json') || {};
  const index = {
    phase: 'P7-V2-R3B-BINARY-BOUND-FAILED-FORMAL-PAIR-AUDIT-INDEX',
    status: 'passed',
    baselineRunId: EXPECTED.baselineRunId,
    currentRunId: EXPECTED.currentRunId,
    runtimeFreezeId: EXPECTED.runtimeFreezeId,
    baselineBinarySha256: baselineBinding.binarySha256,
    currentBinarySha256: currentBinding.binarySha256,
    inputSequenceManifestHash: input.inputSequenceManifestHash,
    requestSequenceHash: input.requestSequenceHash,
    webhookSequenceHash: input.webhookSequenceHash,
    authSequenceHash: input.authSequenceHash,
    branchMixFingerprint: input.branchMixFingerprint,
    baselineFrozenArtifactPath: 'docs/baselines/frozen/p7v2-baseline-r3b-recovery6-20260718054823/raw-summary.json',
    currentFrozenArtifactPath: 'docs/currents/frozen/p7v2-current-r3b-recovery6-20260718054823/raw-summary.json',
    comparabilityReportPath: 'docs/p7-v2-r3b-fast-close-r3-comparability-report.json',
    regressionReportPath: 'docs/p7-v2-r3b-fast-close-r3-regression-v2-report.json',
    validForRepeatabilityMatrix: true,
    validForClosure: false,
    regressionStatus: regression.status || '',
    generatedAt: new Date().toISOString(),
  };
  writeJSON('docs/p7-v2-r3b-binary-bound-failed-formal-pair-audit-index.json', index);
  writeMarkdown(
    'docs/P7_V2_R3B_BINARY_BOUND_FAILED_FORMAL_PAIR_AUDIT_INDEX.md',
    `# P7-V2 R3B Binary-Bound Failed Formal Pair Audit Index

Status: ${index.status}

- Baseline run: \`${index.baselineRunId}\`
- Current run: \`${index.currentRunId}\`
- Runtime freeze: \`${index.runtimeFreezeId}\`
- Baseline binary SHA-256: \`${index.baselineBinarySha256}\`
- Current binary SHA-256: \`${index.currentBinarySha256}\`
- Input sequence manifest hash: \`${index.inputSequenceManifestHash}\`
- Branch mix fingerprint: \`${index.branchMixFingerprint}\`
- Valid for repeatability matrix: ${index.validForRepeatabilityMatrix}
- Valid for closure: ${index.validForClosure}
`,
  );
  return index;
}

async function runRound({ slot, matrixId, runId, binding, input }) {
  assertNoFifthRound(RUN_ORDER.indexOf(slot) + 1);
  const role = ROLE_BY_SLOT[slot];
  const dbName = safeDbName(runId);
  const instanceNonce = crypto.randomBytes(12).toString('hex');
  const before = collectHostSnapshot(slot);
  if (before.listener18080Count !== 0 || before.unknownProcessCount !== 0) {
    throw Object.assign(new Error('host preflight blocked round start'), { before, loadStarted: false });
  }

  runWSL('service postgresql start >/dev/null 2>&1 || /etc/init.d/postgresql start >/dev/null 2>&1 || true', { timeout: 30000 });
  const createDb = runWSL(`sudo -u postgres psql -v ON_ERROR_STOP=1 -d postgres -c "CREATE DATABASE ${dbName};"`, { timeout: 120000 });
  if (createDb.status !== 0) throw Object.assign(new Error(`database create failed: ${createDb.stderr || createDb.stdout}`), { before, loadStarted: false });
  runWSL('redis-server --daemonize yes --port 6379 >/dev/null 2>&1 || service redis-server start >/dev/null 2>&1 || true', { timeout: 30000 });
  runWSL('redis-cli FLUSHALL >/dev/null 2>&1 || true', { timeout: 30000 });

  const env = performanceEnvDefaults({
    DB_NAME: dbName,
    DB_DRIVER: 'postgres',
    DB_HOST: '/var/run/postgresql',
    DB_PORT: '5432',
    DB_USER: 'root',
    REDIS_ADDR: '127.0.0.1:6379',
    P7V2_INSTANCE_NONCE: instanceNonce,
    P7_DIAGNOSTICS_ENABLED: 'true',
    P7_DIAGNOSTIC_RUN_ID: runId,
    P7_DIAGNOSTIC_ROLE: role,
    P7_DIAGNOSTIC_DIR: `artifacts/p7-v2/repeatability/${matrixId}/${slot}`,
  });
  const server = startP7V2Server(env, { skipStop: true, runId, formalBinaryBinding: binding });
  if (!server.ok) throw Object.assign(new Error(`server start failed: ${(server.issues || []).join('; ')}`), { before, loadStarted: false });

  const portConfig = resolveP7V2PortConfig();
  const authProbe = runAuthProbe(portConfig.baseUrl, env);
  writeJSON('docs/p7-v2-r2-auth-probe-report.json', authProbe);
  const routeProbeRes = runNodeScript('scripts/p7-v2-r2-route-probe.mjs', [], { stdio: 'pipe', timeout: 120000 });
  const routeProbe = readJSON('docs/p7-v2-r2-route-probe-report.json') || { status: routeProbeRes.status === 0 ? 'passed' : 'failed', routeNotFound: 1 };
  const runtime = createRuntimeReport({ slot, runId, dbName, env, server, authProbe, routeProbe });
  if (!runtime.readiness.loadReady) throw Object.assign(new Error('runtime readiness failed before dataset/load'), { before, loadStarted: false });

  const datasetRes = runNodeScript('scripts/p7-v2-dataset.mjs', ['--run-id', runId, '--execute'], { timeout: 6 * 60 * 60 * 1000 });
  const dataset = readJSON('docs/p7-v2-dataset-report.json') || {};
  if (datasetRes.status !== 0 || dataset.status !== 'passed') throw Object.assign(new Error('dataset execution failed'), { before, loadStarted: false });

  const loadRes = runNodeScript('scripts/p7-v2-load.mjs', ['--kind', 'current', '--run-id', runId], { timeout: 50 * 60 * 1000 });
  const loadReport = readJSON('docs/p7-v2-current-load-report.json') || {};
  const identity = captureApiProcessIdentity({ pid: runtime.serverPid, port: portConfig.port });
  const afterLoad = collectHostSnapshot(slot);
  const stopped = stopP7V2Server({ expectedIdentity: identity, portConfig });
  const afterStop = collectHostSnapshot(slot);
  const connectionDrain = runWSL(`for i in $(seq 1 20); do c=$(psql -h /var/run/postgresql -U root -At -d postgres -c "select count(*) from pg_stat_activity where datname='${dbName}';" 2>/dev/null || echo 0); [ "$c" = "0" ] && { echo 0; exit 0; }; sleep 1; done; echo "$c"`, { timeout: 30000 });
  const dbConnectionCountAfterStop = Number(String(connectionDrain.stdout || '0').trim()) || 0;
  const roundStatus = loadRes.status === 0 && loadReport.status === 'passed' && stopped.stopped === true && dbConnectionCountAfterStop === 0 ? 'completed' : 'failed';
  const runSummary = {
    slot,
    kind: KIND_BY_SLOT[slot],
    role,
    orderIndex: RUN_ORDER.indexOf(slot) + 1,
    runId,
    status: roundStatus,
    completed: roundStatus === 'completed',
    independent: true,
    pid: runtime.serverPid,
    instanceNonce,
    databaseName: dbName,
    databaseIdentity: runtime.databaseNameHash,
    datasetRows: Number(dataset.actualRows || 0),
    binarySha256: runtime.serverBinarySha256,
    resultSha256: sha256Json({ runId, focusedMetrics: focusedMetrics(loadReport), datasetFingerprint: dataset.fullDatasetFingerprint || dataset.datasetFingerprint || '' }),
    inputSequenceHash: input.inputSequenceManifestHash,
    requestSequenceHash: input.requestSequenceHash,
    webhookSequenceHash: input.webhookSequenceHash,
    authSequenceHash: input.authSequenceHash,
    branchMixFingerprint: input.branchMixFingerprint,
    focusedMetrics: focusedMetrics(loadReport),
    branchMetrics: {
      webhook: input.manifest?.webhookBranchMix || {},
      auth: input.manifest?.authBranchMix || {},
    },
    hostSnapshotBefore: before,
    hostSnapshotAfterLoad: afterLoad,
    hostSnapshotAfter: afterStop,
    hostStateDelta: hostDelta(before, afterStop),
    processIdentity: identity,
    processIdentityProbeVersion: identity.processIdentityProbeVersion || 2,
    probeMethod: identity.probeMethod || 'linux_procfs',
    externalShimUsed: identity.externalShimUsed === true,
    processExecutableSha256Match: identity.executableSha256 === binding.binarySha256,
    portOwnerVerified: verifyPortOwner(identity, portConfig.port),
    serverBinaryVerified: verifyServerBinary(identity, binding.binarySha256),
    cleanup: {
      serverStopped: stopped.stopped === true,
      dbConnectionCountAfterStop,
      listener18080CountAfterStop: afterStop.listener18080Count,
    },
    reportPath: `artifacts/p7-v2/repeatability/${matrixId}/${slot}/load-report.json`,
    runtimeReportPath: `artifacts/p7-v2/repeatability/${matrixId}/${slot}/runtime-environment.json`,
    datasetReportPath: `artifacts/p7-v2/repeatability/${matrixId}/${slot}/dataset-report.json`,
    loadExitCode: loadRes.status ?? 1,
    issues: [
      ...(loadRes.status === 0 ? [] : ['load command failed']),
      ...(loadReport.status === 'passed' ? [] : [`load report status=${loadReport.status || 'missing'}`]),
      ...(stopped.stopped === true ? [] : ['server did not stop cleanly']),
      ...(dbConnectionCountAfterStop === 0 ? [] : ['database connections did not drain']),
    ],
  };
  writeJSON(runSummary.reportPath, loadReport);
  writeJSON(runSummary.runtimeReportPath, runtime);
  writeJSON(runSummary.datasetReportPath, dataset);
  if (roundStatus !== 'completed') throw Object.assign(new Error(`round ${slot} failed`), { before, runSummary, loadStarted: true });
  return runSummary;
}

function buildReport({ matrixId, runIds, runs, baselineBinding, currentBinding, input, auditIndex, matrixStatus = 'completed', failure = null }) {
  const runsBySlot = Object.fromEntries(runs.map((run) => [run.slot, run]));
  const baselineSelfVariance = runsBySlot.B1 && runsBySlot.B2 ? compareRuns(runsBySlot.B1, runsBySlot.B2) : null;
  const currentSelfVariance = runsBySlot.C1 && runsBySlot.C2 ? compareRuns(runsBySlot.C1, runsBySlot.C2) : null;
  const crossVersionVariance = runs.length === 4
    ? {
        'B1 vs C1': compareRuns(runsBySlot.B1, runsBySlot.C1),
        'B2 vs C2': compareRuns(runsBySlot.B2, runsBySlot.C2),
        'B1 vs C2': compareRuns(runsBySlot.B1, runsBySlot.C2),
        'B2 vs C1': compareRuns(runsBySlot.B2, runsBySlot.C1),
      }
    : null;
  const binaryProvenancePassed =
    baselineBinding.status === 'passed' &&
    currentBinding.status === 'passed' &&
    runs.every((run) => run.processExecutableSha256Match && run.serverBinaryVerified);
  const inputSequenceHashMatch = runs.every((run) => run.inputSequenceHash === input.inputSequenceManifestHash);
  const webhookSequenceHashMatch = runs.every((run) => run.webhookSequenceHash === input.webhookSequenceHash);
  const authSequenceHashMatch = runs.every((run) => run.authSequenceHash === input.authSequenceHash);
  const branchMixFingerprintMatch = runs.every((run) => run.branchMixFingerprint === input.branchMixFingerprint);
  const allRunsIndependent = runs.length === 4 && new Set(runs.map((run) => run.databaseName)).size === 4 && new Set(runs.map((run) => run.instanceNonce)).size === 4;
  const allDatasetRows = runs.length === 4 && runs.every((run) => run.datasetRows === EXPECTED.datasetRows);
  const orderPositionEffectDetected =
    runs.length === 4 &&
    maxAbsRelative(baselineSelfVariance) >= 10 &&
    maxAbsRelative(currentSelfVariance) >= 10;
  const orderDependentHostStateDetected = runs.some((run) => Number(run.hostStateDelta?.checkpointDelta || 0) > 0 || Number(run.hostStateDelta?.autovacuumDelta || 0) > 0);
  const primaryRootCause = runs.length === 4
    ? classifyRootCause({ runsBySlot, inputSequenceHashMatch, branchMixFingerprintMatch, binaryProvenancePassed, hostExplainsTail: orderDependentHostStateDetected })
    : failure?.primaryRootCause || 'G_insufficient_evidence_after_binary_bound_bounded_matrix';
  const repairPath = {
    A_formal_harness_repeatability_or_order_bias_defect: 'repair_harness_order_or_host_state_isolation_then_create_new_formal_plan_freeze_pair',
    B_deterministic_webhook_tail_regression: 'repair_webhook_tail_stage_only_then_create_new_runtime_checkpoint_plan_freeze_pair',
    C_deterministic_auth_tail_regression: 'repair_auth_tail_stage_only_then_create_new_runtime_checkpoint_plan_freeze_pair',
    D_deterministic_multi_path_regression: 'repair_verified_webhook_and_auth_tail_stages_then_create_new_runtime_checkpoint_plan_freeze_pair',
    E_input_sequence_or_branch_mix_binding_defect: 'repair_input_sequence_or_branch_mix_binding_only',
    F_process_executable_or_binary_binding_defect: 'repair_binary_process_binding_only',
    G_insufficient_evidence_after_binary_bound_bounded_matrix: 'block_phase_without_fifth_round',
    formal_repeatability_runner_preload_blocked: 'repair_repeatability_harness_preload_blocker_then_start_new_matrix_from_B1',
  }[primaryRootCause];
  const report = {
    phase: 'P7-V2-R3B-BINARY-BOUND-B-C-C-B-REPEATABILITY-MATRIX',
    status: matrixStatus,
    rootCauseClassified: Boolean(primaryRootCause),
    failedFormalBaselineRunId: EXPECTED.baselineRunId,
    failedFormalCurrentRunId: EXPECTED.currentRunId,
    failedFormalRuntimeFreezeId: EXPECTED.runtimeFreezeId,
    formalPlanCheckpoint: readJSON('docs/p7-v2-r3b-run-manifest.json')?.planCheckpoint || '',
    runtimeFreezeId: EXPECTED.runtimeFreezeId,
    baselineBinarySha256: baselineBinding.binarySha256,
    currentBinarySha256: currentBinding.binarySha256,
    inputSequenceManifestHash: input.inputSequenceManifestHash,
    requestSequenceHash: input.requestSequenceHash,
    webhookSequenceHash: input.webhookSequenceHash,
    authSequenceHash: input.authSequenceHash,
    branchMixFingerprint: input.branchMixFingerprint,
    matrixId,
    B1RunId: runIds.B1,
    C1RunId: runIds.C1,
    C2RunId: runIds.C2,
    B2RunId: runIds.B2,
    runOrder: 'B-C-C-B',
    runCount: runs.length,
    allRunsIndependent,
    datasetRowsPerRun: Object.fromEntries(runs.map((run) => [run.slot, run.datasetRows])),
    B1Metrics: runsBySlot.B1?.focusedMetrics || null,
    C1Metrics: runsBySlot.C1?.focusedMetrics || null,
    C2Metrics: runsBySlot.C2?.focusedMetrics || null,
    B2Metrics: runsBySlot.B2?.focusedMetrics || null,
    runs,
    baselineSelfVariance,
    currentSelfVariance,
    crossVersionVariance,
    orderPositionEffectDetected,
    orderDependentHostStateDetected,
    primaryRootCause,
    secondaryRootCauses: failure?.secondaryRootCauses || [],
    confidence: runs.length === 4 && primaryRootCause !== 'G_insufficient_evidence_after_binary_bound_bounded_matrix' ? 'medium' : 'low',
    repairPath,
    formal: false,
    diagnosticOnly: true,
    validForClosure: false,
    validForRegression: false,
    formalRerunStarted: false,
    binaryProvenancePassed,
    baselineBinarySha256Match: baselineBinding.binarySha256 === EXPECTED.baselineBinarySha256,
    currentBinarySha256Match: currentBinding.binarySha256 === EXPECTED.currentBinarySha256,
    inputSequenceHashMatch,
    webhookSequenceHashMatch,
    authSequenceHashMatch,
    branchMixFingerprintMatch,
    processIdentityProbeVersion: 2,
    probeMethod: 'linux_procfs',
    externalShimUsed: runs.some((run) => run.externalShimUsed) ? true : false,
    hostSnapshotsPresent: runs.length === 4 && runs.every((run) => run.hostSnapshotBefore && run.hostSnapshotAfter),
    allDatasetRows,
    guardrails: {
      thresholdChanged: false,
      sloChanged: false,
      materialityChanged: false,
      vusChanged: false,
      stagesChanged: false,
      datasetChanged: false,
      durationChanged: false,
      businessCodeChanged: false,
    },
    auditIndexPath: 'docs/p7-v2-r3b-binary-bound-failed-formal-pair-audit-index.json',
    auditIndex,
    failure,
    generatedAt: new Date().toISOString(),
  };
  writeJSON('docs/p7-v2-r3b-binary-bound-repeatability-matrix.json', report);
  writeMarkdown(
    'docs/P7_V2_R3B_BINARY_BOUND_REPEATABILITY_MATRIX.md',
    `# P7-V2 R3B Binary-Bound B-C-C-B Repeatability Matrix

Status: ${report.status}

- Matrix ID: \`${matrixId}\`
- Run order: \`${report.runOrder}\`
- Run count: ${report.runCount}
- All runs independent: ${report.allRunsIndependent}
- All dataset rows: ${report.allDatasetRows}
- Binary provenance passed: ${report.binaryProvenancePassed}
- Input sequence hash match: ${report.inputSequenceHashMatch}
- Branch mix fingerprint match: ${report.branchMixFingerprintMatch}
- Primary root cause: \`${report.primaryRootCause}\`
- Confidence: \`${report.confidence}\`
- Repair path: \`${report.repairPath}\`
- Formal rerun started: ${report.formalRerunStarted}

This matrix is diagnostic-only evidence and is not valid for P7 closure, soak, demo, final gates, tag, release, or production readiness.
`,
  );
  writeJSON(`artifacts/p7-v2/repeatability/${matrixId}/matrix-manifest.json`, report);
  return report;
}

export function runSelfTest() {
  assertFixedOrder(['B1', 'C1', 'C2', 'B2']);
  let fifthBlocked = false;
  try {
    assertNoFifthRound(5);
  } catch {
    fifthBlocked = true;
  }
  const runsBySlot = {
    B1: { focusedMetrics: { 'Webhook Ingestion': { p95: 20, p99: 40 }, 'Auth Invalid Login': { p95: 10, p99: 20 } } },
    B2: { focusedMetrics: { 'Webhook Ingestion': { p95: 21, p99: 42 }, 'Auth Invalid Login': { p95: 11, p99: 21 } } },
    C1: { focusedMetrics: { 'Webhook Ingestion': { p95: 45, p99: 90 }, 'Auth Invalid Login': { p95: 24, p99: 46 } } },
    C2: { focusedMetrics: { 'Webhook Ingestion': { p95: 46, p99: 91 }, 'Auth Invalid Login': { p95: 23, p99: 47 } } },
  };
  const rootCause = classifyRootCause({
    runsBySlot,
    inputSequenceHashMatch: true,
    branchMixFingerprintMatch: true,
    binaryProvenancePassed: true,
    hostExplainsTail: false,
  });
  return {
    status: fifthBlocked && rootCause === 'D_deterministic_multi_path_regression' ? 'passed' : 'failed',
    fixedOrderAccepted: true,
    fifthBlocked,
    rootCause,
  };
}

async function main() {
  const args = process.argv.slice(2);
  if (args.includes('--self-test')) {
    const result = runSelfTest();
    console.log(JSON.stringify(result, null, 2));
    process.exit(result.status === 'passed' ? 0 : 1);
  }
  if (args.some((arg) => arg.startsWith('--order'))) throw new Error('custom order is forbidden; runner is fixed to B-C-C-B');
  assertFixedOrder();
  const ts = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
  const matrixId = valueOf(args, '--matrix-id') || `p7v2-diag-binary-repeatability-${ts}`;
  const runIds = {
    B1: `p7v2-diag-binary-repeatability-b1-${ts}`,
    C1: `p7v2-diag-binary-repeatability-c1-${ts}`,
    C2: `p7v2-diag-binary-repeatability-c2-${ts}`,
    B2: `p7v2-diag-binary-repeatability-b2-${ts}`,
  };
  const baselineBinding = readBinaryBinding('baseline');
  const currentBinding = readBinaryBinding('current');
  const input = readInputBinding();
  const auditIndex = writeAuditIndex({ input, baselineBinding, currentBinding });
  const runs = [];
  let matrixStatus = 'completed';
  let failure = null;
  for (const slot of RUN_ORDER) {
    try {
      const binding = ROLE_BY_SLOT[slot] === 'baseline' ? baselineBinding : currentBinding;
      const runSummary = await runRound({ slot, matrixId, runId: runIds[slot], binding, input });
      runs.push(runSummary);
    } catch (error) {
      matrixStatus = error.loadStarted ? 'invalid_incomplete' : 'blocked';
      failure = {
        failedStage: 'repeatability-runner',
        failedStep: slot,
        failedCommand: 'node scripts/p7-v2-r3b-formal-repeatability-runner.mjs',
        exitCode: 1,
        message: error.message,
        primaryRootCause: error.loadStarted ? 'G_insufficient_evidence_after_binary_bound_bounded_matrix' : 'formal_repeatability_runner_preload_blocked',
        secondaryRootCauses: [error.message],
        runSummary: error.runSummary || null,
        hostSnapshotBefore: error.before || null,
      };
      break;
    }
  }
  const report = buildReport({ matrixId, runIds, runs, baselineBinding, currentBinding, input, auditIndex, matrixStatus, failure });
  console.log(JSON.stringify({
    phase: report.phase,
    status: report.status,
    matrixId,
    runOrder: report.runOrder,
    runCount: report.runCount,
    primaryRootCause: report.primaryRootCause,
    repairPath: report.repairPath,
  }, null, 2));
  process.exit(report.status === 'completed' ? 0 : 1);
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((error) => {
    console.error(error);
    process.exit(1);
  });
}
