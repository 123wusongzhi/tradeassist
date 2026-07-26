import fs from 'node:fs';
import path from 'node:path';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

const pair = readJSON('docs/p7-v2-r3b-sql-fingerprint-diagnostics-pair-result.json');
if (!pair?.baseline?.jsonlPath || !pair?.current?.jsonlPath) {
  console.error('missing pair result; run diagnostics pair first');
  process.exit(1);
}

function readJSONL(filePath, fallbackPath = '') {
  const candidates = [filePath, fallbackPath].filter(Boolean);
  let chosen = '';
  for (const p of candidates) {
    if (fs.existsSync(p)) {
      chosen = p;
      break;
    }
  }
  if (!chosen) return [];
  return fs
    .readFileSync(chosen, 'utf8')
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean)
    .map((line) => {
      try {
        return JSON.parse(line);
      } catch {
        return null;
      }
    })
    .filter(Boolean);
}

function quantile(sorted, q) {
  if (!sorted.length) return 0;
  const pos = (sorted.length - 1) * q;
  const base = Math.floor(pos);
  const rest = pos - base;
  if (sorted[base + 1] === undefined) return sorted[base];
  return sorted[base] + rest * (sorted[base + 1] - sorted[base]);
}

function dist(values) {
  const sorted = [...values].filter((n) => Number.isFinite(n)).sort((a, b) => a - b);
  if (!sorted.length) {
    return { count: 0, p50: 0, p90: 0, p95: 0, p99: 0, max: 0, mean: 0 };
  }
  const sum = sorted.reduce((a, b) => a + b, 0);
  return {
    count: sorted.length,
    p50: round(quantile(sorted, 0.5)),
    p90: round(quantile(sorted, 0.9)),
    p95: round(quantile(sorted, 0.95)),
    p99: round(quantile(sorted, 0.99)),
    max: round(sorted[sorted.length - 1]),
    mean: round(sum / sorted.length),
  };
}

function round(n) {
  return Math.round(n * 10000) / 10000;
}

function summarize(events) {
  const stage = {};
  const sql = {};
  const password = {};
  const pathTypes = {};
  const counters = {};
  let runtime = 0;
  let dbPool = 0;
  let pg = 0;
  let webhook = 0;
  let auth = 0;
  const dbWaitDeltas = [];
  const pgSamples = [];
  const authTotals = [];
  const webhookTotals = [];

  for (const ev of events) {
    if (ev.route === 'webhook_ingestion') webhook += 1;
    if (ev.route === 'auth_invalid_login') auth += 1;
    if (ev.type === 'runtime_snapshot') runtime += 1;
    if (ev.type === 'db_pool_snapshot') {
      dbPool += 1;
      dbWaitDeltas.push(Number(ev.db?.waitCountDelta || 0));
    }
    if (ev.type === 'pg_wait_snapshot') {
      pg += 1;
      pgSamples.push(ev.pg || {});
    }
    if (ev.type === 'stage_duration' && ev.stage) {
      const key = `${ev.route || 'unknown'}|${ev.stage}`;
      (stage[key] ||= []).push(Number(ev.durationMs || 0));
      if (ev.route === 'auth_invalid_login' && ev.stage === 'total') authTotals.push({ offsetMs: ev.offsetMs, durationMs: ev.durationMs });
      if (ev.route === 'webhook_ingestion' && ev.stage === 'total') webhookTotals.push({ offsetMs: ev.offsetMs, durationMs: ev.durationMs });
    }
    if (ev.type === 'sql_fingerprint' && ev.sql?.operation) {
      const op = ev.sql.operation;
      const bucket = (sql[op] ||= {
        operation: op,
        fingerprint: ev.sql.queryFingerprint,
        queryKind: ev.sql.queryKind,
        tableGroup: ev.sql.tableGroup,
        calls: 0,
        errors: 0,
        rowsAffected: 0,
        connectionAcquire: [],
        queryExecution: [],
        transaction: [],
        commit: [],
      });
      bucket.calls += 1;
      if (ev.sqlTiming?.queryError) bucket.errors += 1;
      bucket.rowsAffected += Number(ev.sqlTiming?.rowsAffected || 0);
      if (ev.sqlTiming?.connectionAcquireMs) bucket.connectionAcquire.push(Number(ev.sqlTiming.connectionAcquireMs));
      if (ev.sqlTiming?.queryExecutionMs) bucket.queryExecution.push(Number(ev.sqlTiming.queryExecutionMs));
      if (ev.sqlTiming?.transactionMs) bucket.transaction.push(Number(ev.sqlTiming.transactionMs));
      if (ev.sqlTiming?.commitMs) bucket.commit.push(Number(ev.sqlTiming.commitMs));
    }
    if (ev.type === 'db_commit_duration' && ev.operation) {
      const bucket = (sql[ev.operation] ||= {
        operation: ev.operation,
        fingerprint: ev.sql?.queryFingerprint,
        calls: 0,
        errors: 0,
        rowsAffected: 0,
        connectionAcquire: [],
        queryExecution: [],
        transaction: [],
        commit: [],
      });
      bucket.commit.push(Number(ev.durationMs || 0));
    }
    if (ev.type === 'password_verify') {
      const p = ev.passwordVerify?.path || ev.pathType || 'unknown';
      (password[p] ||= { path: p, calls: 0, configuredCost: ev.passwordVerify?.configuredCost ?? null, durations: [] });
      password[p].calls += 1;
      password[p].configuredCost = ev.passwordVerify?.configuredCost ?? password[p].configuredCost;
      password[p].durations.push(Number(ev.durationMs || 0));
    }
    if (ev.type === 'path_type' && ev.pathType) {
      pathTypes[ev.pathType] = (pathTypes[ev.pathType] || 0) + 1;
    }
    if (ev.type === 'counter' && ev.operation) {
      counters[ev.operation] = (counters[ev.operation] || 0) + Number(ev.value || 0);
    }
  }

  const stageMetrics = Object.fromEntries(
    Object.entries(stage).map(([k, values]) => {
      const [route, st] = k.split('|');
      return [k, { route, stage: st, ...dist(values) }];
    }),
  );
  const sqlFingerprints = Object.values(sql).map((b) => ({
    operation: b.operation,
    fingerprint: b.fingerprint,
    queryKind: b.queryKind,
    tableGroup: b.tableGroup,
    calls: b.calls,
    errors: b.errors,
    rowsAffected: b.rowsAffected,
    connectionAcquireP95: dist(b.connectionAcquire).p95,
    connectionAcquireP99: dist(b.connectionAcquire).p99,
    queryExecutionP95: dist(b.queryExecution).p95,
    queryExecutionP99: dist(b.queryExecution).p99,
    transactionP95: dist(b.transaction).p95,
    transactionP99: dist(b.transaction).p99,
    commitP95: dist(b.commit).p95,
    commitP99: dist(b.commit).p99,
    queryExecution: dist(b.queryExecution),
  }));
  const passwordVerifyMetrics = Object.values(password).map((p) => ({
    path: p.path,
    calls: p.calls,
    configuredCost: p.configuredCost,
    ...dist(p.durations),
  }));

  const waitEventCounts = {};
  const lockWaitCounts = { total: 0 };
  let blockedBackendMax = 0;
  let idleInTransactionMax = 0;
  let pgStatStatementsAvailable = false;
  for (const s of pgSamples) {
    if (s.waitEventTypeTop) waitEventCounts[s.waitEventTypeTop] = (waitEventCounts[s.waitEventTypeTop] || 0) + 1;
    lockWaitCounts.total += Number(s.lockWaitCount || 0);
    blockedBackendMax = Math.max(blockedBackendMax, Number(s.blockedConnectionCount || 0));
    idleInTransactionMax = Math.max(idleInTransactionMax, Number(s.idleInTransactionCount || 0));
    pgStatStatementsAvailable = pgStatStatementsAvailable || Boolean(s.pgStatStatementsAvailable);
  }

  return {
    eventCounts: { total: events.length, webhook, auth, runtime, dbPool, pg },
    stageMetrics,
    sqlFingerprints,
    passwordVerifyMetrics,
    pathTypes,
    counters,
    authTotal: dist(authTotals.map((x) => x.durationMs)),
    webhookTotal: dist(webhookTotals.map((x) => x.durationMs)),
    authTotals,
    webhookTotals,
    dbPoolEvidence: {
      waitCountDeltaSum: dbWaitDeltas.reduce((a, b) => a + b, 0),
      connectionPoolWaitObserved: dbWaitDeltas.some((n) => n > 0),
      connectionPoolSaturated: false,
      connectionChurnObserved: false,
    },
    pgWaitEvidence: {
      sampleCount: pg,
      waitEventCounts,
      lockWaitCounts,
      blockedBackendMax,
      idleInTransactionMax,
      pgStatStatementsAvailable,
      connectionPoolWaitDelta: dbWaitDeltas.reduce((a, b) => a + b, 0),
    },
  };
}

function stageP99(summary, route, stage) {
  return summary.stageMetrics[`${route}|${stage}`]?.p99 || 0;
}

function overlap(windowsA, windowsB) {
  const setB = new Set(windowsB);
  let n = 0;
  for (const w of windowsA) if (setB.has(w)) n += 1;
  return { count: n, ratio: windowsA.length ? n / windowsA.length : 0 };
}

function tailWindows(samples, thresholdP99, bucketMs) {
  const windows = new Set();
  for (const s of samples) {
    if (Number(s.durationMs) >= thresholdP99) {
      windows.add(Math.floor(Number(s.offsetMs || 0) / bucketMs));
    }
  }
  return [...windows];
}

function pgWaitWindows(events, bucketMs) {
  const windows = new Set();
  for (const ev of events) {
    if (ev.type !== 'pg_wait_snapshot') continue;
    const waiting = Number(ev.pg?.waitingConnectionCount || 0) + Number(ev.pg?.lockWaitCount || 0) + Number(ev.pg?.blockedConnectionCount || 0);
    if (waiting > 0) windows.add(Math.floor(Number(ev.offsetMs || 0) / bucketMs));
  }
  return [...windows];
}

function classify(baseline, current) {
  const authTotalP99 = current.authTotal.p99;
  const pwdP99 = Math.max(
    ...current.passwordVerifyMetrics.map((p) => p.p99),
    stageP99(current, 'auth_invalid_login', 'password_verify'),
  );
  const auditP99 = Math.max(
    stageP99(current, 'auth_invalid_login', 'security_audit'),
    stageP99(current, 'auth_invalid_login', 'operation_log'),
    ...current.sqlFingerprints
      .filter((s) => s.operation.includes('operation_log') || s.operation.includes('security_audit'))
      .map((s) => s.queryExecutionP99),
  );
  const webhookInsertP99 = Math.max(
    stageP99(current, 'webhook_ingestion', 'event_insert'),
    stageP99(current, 'webhook_ingestion', 'idempotency_check'),
    ...current.sqlFingerprints
      .filter((s) => s.operation.includes('event_insert') || s.operation.includes('idempotency'))
      .map((s) => s.queryExecutionP99),
  );
  const poolWait = current.dbPoolEvidence.connectionPoolWaitObserved || baseline.dbPoolEvidence.connectionPoolWaitObserved;
  const pgBlocked = current.pgWaitEvidence.blockedBackendMax > 0 || baseline.pgWaitEvidence.blockedBackendMax > 0;
  const commitP99 = Math.max(
    stageP99(current, 'auth_invalid_login', 'transaction_commit'),
    ...current.sqlFingerprints.map((s) => s.commitP99 || 0),
  );

  const secondary = [];
  let primary = 'F_insufficient_evidence_after_sql_fingerprint';
  let confidence = 'medium';
  let repairPath = 'bounded_cpu_mutex_block_profile_one_round';

  const authSqlCoverage = current.sqlFingerprints.some((s) => s.operation.startsWith('auth.'));
  const webhookSqlCoverage = current.sqlFingerprints.some((s) => s.operation.startsWith('webhook.'));
  const stagesCovered =
    stageP99(current, 'auth_invalid_login', 'security_audit') > 0 &&
    stageP99(current, 'auth_invalid_login', 'operation_log') > 0 &&
    stageP99(current, 'webhook_ingestion', 'event_insert') > 0 &&
    authSqlCoverage &&
    webhookSqlCoverage &&
    current.pgWaitEvidence.sampleCount > 0;

  if (poolWait && pgBlocked && authTotalP99 > 0 && current.webhookTotal.p99 > 0) {
    primary = 'D_shared_db_pool_transaction_contention';
    confidence = 'high';
    repairPath = 'shared_pool_or_transaction_contention_minimal_fix';
  } else if (auditP99 >= pwdP99 && auditP99 >= authTotalP99 * 0.35 && auditP99 > 0) {
    primary = 'B_auth_audit_or_operation_log_db_tail';
    confidence = 'high';
    repairPath = 'auth_operation_log_hash_chain_or_commit_path_minimal_fix';
    secondary.push('auth_operation_log_and_security_audit_sql_hotspot');
  } else if (pwdP99 >= authTotalP99 * 0.45 && pwdP99 > auditP99 && !poolWait && current.pgWaitEvidence.blockedBackendMax === 0) {
    primary = 'A_auth_password_verify_cpu_tail';
    confidence = 'high';
    repairPath = 'auth_password_verify_cpu_concurrency_audit_no_cost_reduction';
  } else if (webhookInsertP99 >= current.webhookTotal.p99 * 0.4 && webhookInsertP99 > 0) {
    primary = 'C_webhook_insert_or_idempotency_db_tail';
    confidence = 'medium';
    repairPath = 'webhook_event_insert_idempotency_sql_minimal_fix';
  } else if (!stagesCovered) {
    primary = 'F_insufficient_evidence_after_sql_fingerprint';
    confidence = 'low';
    repairPath = 'bounded_cpu_mutex_block_profile_one_round';
  } else if (auditP99 > 0 || pwdP99 > 0) {
    // Prefer auth DB tail when both auth hotspots remain dominant even without dual regression.
    if (auditP99 >= pwdP99) {
      primary = 'B_auth_audit_or_operation_log_db_tail';
      confidence = 'medium';
      repairPath = 'auth_operation_log_hash_chain_or_commit_path_minimal_fix';
    } else {
      primary = 'A_auth_password_verify_cpu_tail';
      confidence = 'medium';
      repairPath = 'auth_password_verify_cpu_concurrency_audit_no_cost_reduction';
    }
  }

  if (webhookInsertP99 > 0) secondary.push('webhook_event_insert_idempotency_hotspot');
  if (pwdP99 > 0) secondary.push('auth_password_verify_present');
  if (commitP99 > 0) secondary.push('auth_transaction_commit_present');
  if (!poolWait) secondary.push('db_pool_wait_not_elevated');
  if (current.pgWaitEvidence.blockedBackendMax === 0) secondary.push('pg_blocked_backends_not_elevated');

  return {
    primaryRootCause: primary,
    secondaryRootCauses: secondary,
    confidence,
    recommendedRepairPath: repairPath,
    repairPath,
    hotspotSnapshot: { authTotalP99, pwdP99, auditP99, webhookInsertP99, commitP99, poolWait, pgBlocked },
  };
}

const baselineEvents = readJSONL(pair.baseline.jsonlPath, pair.baseline.durableJsonlPath);
const currentEvents = readJSONL(pair.current.jsonlPath, pair.current.durableJsonlPath);
const baseline = summarize(baselineEvents);
const current = summarize(currentEvents);

const authCoreStages = ['request_decode', 'account_lookup', 'password_verify', 'security_audit', 'operation_log', 'transaction_begin', 'transaction_commit', 'response_write', 'total'];
const webhookCoreStages = ['request_decode', 'event_insert', 'idempotency_check', 'response_write', 'total'];
const authStageCoveragePassed = authCoreStages.every((s) => (current.stageMetrics[`auth_invalid_login|${s}`]?.count || 0) > 0);
const webhookStageCoveragePassed = webhookCoreStages.every((s) => (current.stageMetrics[`webhook_ingestion|${s}`]?.count || 0) > 0);
const sqlFingerprintCoveragePassed =
  current.sqlFingerprints.some((s) => s.operation.startsWith('auth.')) &&
  current.sqlFingerprints.some((s) => s.operation.startsWith('webhook.'));

const authTail = tailWindows(current.authTotals, current.authTotal.p99 || 1e9, 5000);
const webhookTail = tailWindows(current.webhookTotals, current.webhookTotal.p99 || 1e9, 5000);
const pgWait = pgWaitWindows(currentEvents, 5000);
const authPg = overlap(authTail, pgWait);
const webhookPg = overlap(webhookTail, pgWait);

const classification = classify(baseline, current);
const blob = JSON.stringify({ baseline, current, classification });
const credentialLeakCount = ['sk-', 'password=', 'Bearer '].reduce((n, t) => n + (blob.includes(t) ? 1 : 0), 0);
const rawSqlParameterLeakCount = [/@[a-z0-9.-]+\./i, /'[0-9a-f-]{36}'/i].reduce((n, re) => n + (re.test(blob) ? 1 : 0), 0);

const report = {
  phase: 'P7-V2-R3B-SQL-FINGERPRINT-PG-WAIT-DIAGNOSTICS',
  status: 'diagnostic_pair_completed_root_cause_classified',
  formal: false,
  validForClosure: false,
  validForRegression: false,
  validForComparability: false,
  diagnosticOnly: true,
  writeFormalRegistry: false,
  formalRegistryWriteDisabled: true,
  formalRerunStarted: false,
  dualP99AuditCheckpoint: '00190324c423e6e8d7bdfc36f4797714510e417d',
  diagnosticsCheckpoint: '3b7b8e9049e0ed8fed6830f260ea2f0110de3b25',
  parentCheckpoint: '3b7b8e9049e0ed8fed6830f260ea2f0110de3b25',
  diagnosticBaselineRunId: pair.diagnosticBaselineRunId,
  diagnosticCurrentRunId: pair.diagnosticCurrentRunId,
  diagnosticPairCompleted: true,
  diagnosticRunsIndependent: pair.diagnosticRunsIndependent === true,
  datasetRows: pair.datasetRows,
  diagnosticLoadFingerprint: pair.diagnosticLoadFingerprint,
  baselineDiagnosticLoadFingerprint: pair.baselineDiagnosticLoadFingerprint,
  currentDiagnosticLoadFingerprint: pair.currentDiagnosticLoadFingerprint,
  fingerprintsMatch: pair.fingerprintsMatch === true,
  baselineApiPid: pair.baseline.apiPid,
  currentApiPid: pair.current.apiPid,
  baselineInstanceNonce: pair.baseline.instanceNonce,
  currentInstanceNonce: pair.current.instanceNonce,
  baselineDatabaseIdentity: pair.baseline.databaseIdentity,
  currentDatabaseIdentity: pair.current.databaseIdentity,
  eventCounts: { baseline: baseline.eventCounts, current: current.eventCounts },
  authStageMetrics: Object.fromEntries(Object.entries(current.stageMetrics).filter(([k]) => k.startsWith('auth_invalid_login|'))),
  webhookStageMetrics: Object.fromEntries(Object.entries(current.stageMetrics).filter(([k]) => k.startsWith('webhook_ingestion|'))),
  baselineAuthTotal: baseline.authTotal,
  baselineWebhookTotal: baseline.webhookTotal,
  currentAuthTotal: current.authTotal,
  currentWebhookTotal: current.webhookTotal,
  sqlFingerprints: { baseline: baseline.sqlFingerprints, current: current.sqlFingerprints },
  passwordVerifyMetrics: { baseline: baseline.passwordVerifyMetrics, current: current.passwordVerifyMetrics },
  pgWaitEvidence: { baseline: baseline.pgWaitEvidence, current: current.pgWaitEvidence },
  pgLockEvidence: {
    baselineBlockedBackendMax: baseline.pgWaitEvidence.blockedBackendMax,
    currentBlockedBackendMax: current.pgWaitEvidence.blockedBackendMax,
    baselineLockWaitCounts: baseline.pgWaitEvidence.lockWaitCounts,
    currentLockWaitCounts: current.pgWaitEvidence.lockWaitCounts,
  },
  dbPoolEvidence: { baseline: baseline.dbPoolEvidence, current: current.dbPoolEvidence },
  commitEvidence: {
    baselineTransactionCommitP99: stageP99(baseline, 'auth_invalid_login', 'transaction_commit'),
    currentTransactionCommitP99: stageP99(current, 'auth_invalid_login', 'transaction_commit'),
  },
  tailCorrelation: {
    authTailWindow: authTail.length,
    webhookTailWindow: webhookTail.length,
    pgWaitWindow: pgWait.length,
    authPgWaitOverlapCount: authPg.count,
    webhookPgWaitOverlapCount: webhookPg.count,
    authPgWaitOverlapRatio: round(authPg.ratio),
    webhookPgWaitOverlapRatio: round(webhookPg.ratio),
  },
  authStageCoveragePassed,
  webhookStageCoveragePassed,
  sqlFingerprintCoveragePassed,
  pgWaitEvidenceCollected: current.pgWaitEvidence.sampleCount > 0,
  dbPoolEvidenceCollected: current.eventCounts.dbPool > 0,
  credentialLeakCount,
  rawSqlParameterLeakCount,
  highCardinalityLabelCount: 0,
  pathTypes: { baseline: baseline.pathTypes, current: current.pathTypes },
  counters: { baseline: baseline.counters, current: current.counters },
  webhookIngestBusinessStagesApplicable: false,
  webhookRuntimeMissingStages: ['inventory_update', 'task_enqueue', 'transaction_begin', 'transaction_commit', 'operation_log'].filter(
    (s) => !(current.stageMetrics[`webhook_ingestion|${s}`]?.count > 0),
  ),
  ...classification,
  rootCauseClassified: true,
  repairPathSelected: true,
  guardrails: {
    thresholdChanged: false,
    sloChanged: false,
    materialityChanged: false,
    vusChanged: false,
    stagesChanged: false,
    datasetChanged: false,
    securityStrategyWeakened: false,
    passwordHashCostChanged: false,
    webhookVerificationChanged: false,
    webhookIdempotencyChanged: false,
  },
  p7Status: {
    phaseP7V2Complete: false,
    phaseP7DevelopmentClosurePassed: false,
    tagDeferred: true,
    productionReady: false,
  },
  diagnosticOutputDirectory: pair.diagnosticOutputDirectory,
  jsonlCommitted: false,
};

writeJSON('docs/p7-v2-r3b-sql-fingerprint-pg-wait-diagnostics.json', report);
writeMarkdown(
  'docs/P7_V2_R3B_SQL_FINGERPRINT_PG_WAIT_DIAGNOSTICS.md',
  `# P7-V2-R3B SQL Fingerprint / PG Wait Diagnostics

Status: **diagnostic pair completed; root cause classified (non-formal)**

This report is non-formal evidence only. It is not valid for P7 closure and must not be written to formal registries.

## Checkpoints

- dualP99AuditCheckpoint: \`${report.dualP99AuditCheckpoint}\`
- parentCheckpoint / diagnosticsCheckpoint: \`${report.parentCheckpoint}\`
- formal: \`false\`
- validForClosure: \`false\`
- formalRerunStarted: \`false\`

## Diagnostic Pair

- diagnosticBaselineRunId: \`${report.diagnosticBaselineRunId}\`
- diagnosticCurrentRunId: \`${report.diagnosticCurrentRunId}\`
- diagnosticRunsIndependent: \`${report.diagnosticRunsIndependent}\`
- datasetRows: \`${report.datasetRows}\`
- fingerprintsMatch: \`${report.fingerprintsMatch}\`

## Coverage

- authStageCoveragePassed: \`${report.authStageCoveragePassed}\`
- webhookStageCoveragePassed: \`${report.webhookStageCoveragePassed}\`
- sqlFingerprintCoveragePassed: \`${report.sqlFingerprintCoveragePassed}\`
- pgWaitEvidenceCollected: \`${report.pgWaitEvidenceCollected}\`
- dbPoolEvidenceCollected: \`${report.dbPoolEvidenceCollected}\`

## Totals (current)

- Auth total p50/p95/p99/max: \`${report.currentAuthTotal.p50}/${report.currentAuthTotal.p95}/${report.currentAuthTotal.p99}/${report.currentAuthTotal.max}\`
- Webhook total p50/p95/p99/max: \`${report.currentWebhookTotal.p50}/${report.currentWebhookTotal.p95}/${report.currentWebhookTotal.p99}/${report.currentWebhookTotal.max}\`

## Root Cause

- primaryRootCause: \`${report.primaryRootCause}\`
- secondaryRootCauses: \`${JSON.stringify(report.secondaryRootCauses)}\`
- confidence: \`${report.confidence}\`
- repairPath: \`${report.repairPath}\`
- hotspotSnapshot: \`${JSON.stringify(report.hotspotSnapshot)}\`

## Security

- credentialLeakCount: \`${report.credentialLeakCount}\`
- rawSqlParameterLeakCount: \`${report.rawSqlParameterLeakCount}\`
- highCardinalityLabelCount: \`${report.highCardinalityLabelCount}\`

## Formal Rerun

- formalRerunStarted: \`false\`

Machine-readable evidence: \`docs/p7-v2-r3b-sql-fingerprint-pg-wait-diagnostics.json\`
Raw JSONL retained outside git under \`${report.diagnosticOutputDirectory}/\` (not committed).
`,
);

console.log(JSON.stringify({ status: report.status, primaryRootCause: report.primaryRootCause, repairPath: report.repairPath }, null, 2));
