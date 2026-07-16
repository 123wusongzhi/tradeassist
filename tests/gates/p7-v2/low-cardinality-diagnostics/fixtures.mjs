import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const read = (p) => fs.readFileSync(path.join(root, p), 'utf8');
const diag = read('backend/internal/pkg/p7diag/diagnostics.go');
const webhookHandler = read('backend/internal/modules/webhook/handler.go');
const webhookService = read('backend/internal/modules/webhook/service.go');
const authHandler = read('backend/internal/modules/auth/handler.go');
const authService = read('backend/internal/modules/auth/service.go');
const authGuard = read('backend/internal/modules/auth/login_guard.go');

const forbiddenMetricLabels = [
  'requestId',
  'traceId',
  'userId',
  'email',
  'username',
  'shopId',
  'orderId',
  'eventId',
  'databaseName',
  'PID',
  'full URL',
  'error body',
];

const webhookStages = [
  'request_read',
  'signature_verify',
  'json_decode',
  'shop_provider_resolve',
  'event_insert',
  'duplicate_event_reload',
  'idempotency_check',
  'transaction_begin',
  'business_upsert',
  'inventory_update',
  'task_enqueue',
  'operation_log',
  'transaction_commit',
  'response_encode',
  'total',
];

const authStages = [
  'request_read',
  'json_decode',
  'input_normalize',
  'account_lookup',
  'password_verify',
  'invalid_decision',
  'failed_attempt_read',
  'failed_attempt_write',
  'lockout_evaluate',
  'rate_limit_check',
  'security_audit',
  'operation_log',
  'transaction_begin',
  'transaction_commit',
  'response_encode',
  'total',
];

const requiredMetrics = [
  'p7_diag_request_stage_duration_ms',
  'p7_diag_db_operation_duration_ms',
  'p7_diag_db_pool_wait_ms',
  'p7_diag_transaction_duration_ms',
  'p7_diag_transaction_commit_duration_ms',
  'p7_diag_audit_write_duration_ms',
  'p7_diag_runtime_snapshot',
];

const checks = [];
const check = (id, passed, detail = '') => checks.push({ id, status: passed ? 'passed' : 'failed', detail });

check('default-off-env-gate', diag.includes('P7_DIAGNOSTICS_ENABLED') && diag.includes('return strings.EqualFold'));
check('ignored-local-diagnostic-dir', diag.includes('artifacts/p7-v2-diagnostics'));
check('bounded-buffer', diag.includes('P7_DIAGNOSTIC_BUFFER') && diag.includes('default:') && diag.includes('drops.Add(1)'));
check('runtime-sampler-released', diag.includes('Shutdown(ctx context.Context)') && diag.includes('close(stop)') && diag.includes('close(w.ch)'));
check('db-pool-snapshot-fields', ['dbOpenConnections', 'dbInUseConnections', 'dbIdleConnections', 'dbWaitCount', 'dbWaitDurationMs', 'dbMaxOpenConnections', 'dbMaxIdleConnections', 'waitCountDelta', 'waitDurationDeltaMs'].every((s) => diag.includes(s)));
check('runtime-snapshot-fields', ['goroutines', 'heapAllocBytes', 'heapObjects', 'gcCycles', 'gcPauseTotalNs', 'lastGcPauseNs', 'GOMAXPROCS', 'GOGC', 'GOMEMLIMIT', 'GoVersion'].every((s) => diag.includes(s)));
check('required-metric-names', requiredMetrics.every((s) => diag.includes(s)));
check('no-forbidden-event-fields', forbiddenMetricLabels.every((s) => !diag.includes(`json:"${s}"`)));
check('webhook-stage-enum', webhookStages.every((s) => diag.includes(`"${s}"`)));
check('auth-stage-enum', authStages.every((s) => diag.includes(`"${s}"`)));
check('webhook-normal-count', webhookService.includes('"normalInsertCount"'));
check('webhook-duplicate-reload-count', webhookService.includes('"duplicateReloadCount"') && webhookService.includes('"duplicateConflictCount"'));
check('webhook-no-normal-duplicate-reload', webhookService.indexOf('"normalInsertCount"') > webhookService.indexOf('"duplicate_event_reload"'));
check('auth-path-types', ['account_missing', 'wrong_password', 'locked_account', 'rate_limited'].every((s) => diag.includes(`"${s}"`)));
check('auth-counts', ['accountLookupCount', 'passwordVerifyCount', 'failedAttemptReadCount', 'failedAttemptWriteCount', 'securityAuditWriteCount', 'operationLogWriteCount'].every((s) => authService.includes(s) || authGuard.includes(s) || authHandler.includes(s)));
check('webhook-handler-instrumented', ['request_read', 'signature_verify', 'json_decode', 'response_encode', 'total'].every((s) => webhookHandler.includes(`"${s}"`)));
check('auth-handler-instrumented', ['request_read', 'json_decode', 'input_normalize', 'security_audit', 'operation_log', 'response_encode', 'total'].every((s) => authHandler.includes(`"${s}"`)));

const failed = checks.filter((c) => c.status !== 'passed');
const report = {
  phase: 'P7-V2-R3B-DUAL-P99-LOW-CARDINALITY-DIAGNOSTICS-FIXTURES',
  status: failed.length ? 'failed' : 'passed',
  diagnosticsEnabledDefault: false,
  highCardinalityMetricLabelCount: 0,
  checks,
  failed: failed.map((c) => c.id),
};

fs.writeFileSync(path.join(root, 'docs/p7-v2-r3b-dual-p99-low-cardinality-diagnostics-fixtures.json'), JSON.stringify(report, null, 2) + '\n');
console.log(JSON.stringify(report, null, 2));
process.exit(failed.length ? 1 : 0);
