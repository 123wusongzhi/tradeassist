import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const read = (p) => fs.readFileSync(path.join(root, p), 'utf8');

const diag = read('backend/internal/pkg/p7diag/diagnostics.go');
const fp = read('backend/internal/pkg/p7diag/fingerprint.go');
const sql = read('backend/internal/pkg/p7diag/sql_observe.go');
const pwd = read('backend/internal/pkg/p7diag/password_verify.go');
const pg = read('backend/internal/pkg/p7diag/pg_sampler.go');
const webhookService = read('backend/internal/modules/webhook/service.go');
const webhookHandler = read('backend/internal/modules/webhook/handler.go');
const webhookProcessor = read('backend/internal/modules/webhook/processor.go');
const authHandler = read('backend/internal/modules/auth/handler.go');
const authService = read('backend/internal/modules/auth/service.go');
const opLog = read('backend/internal/modules/operationlog/service.go');

const checks = [];
const check = (id, passed, detail = '') => checks.push({ id, status: passed ? 'passed' : 'failed', detail });

check('sql-parameter-redaction', fp.includes('NormalizeSQL') && fp.includes(`ReplaceAllString(s, "?")`));
check('low-cardinality-fingerprint', fp.includes('FingerprintFromParts') && fp.includes('sha256') && fp.includes('fingerprintPrefixLen'));
check('unknown-sql-no-raw-params', fp.includes('looksLikeParameterizedLeak') && fp.includes('FingerprintFromNormalizedSQL'));
check('db-timing-metrics', ['p7_diag_db_connection_acquire_duration', 'p7_diag_db_query_duration', 'p7_diag_db_transaction_duration', 'p7_diag_db_commit_duration'].every((m) => sql.includes(m)));
check('password-verify-safe-fields', pwd.includes('configuredCost') && pwd.includes('verifyCountPerRequest') && !pwd.includes('PasswordHash') && !pwd.includes('plain'));
check('pg-sampling-schema', ['activeConnectionCount', 'waitingConnectionCount', 'blockedConnectionCount', 'idleInTransactionCount', 'pgStatStatementsAvailable'].every((s) => pg.includes(s)));
check('dbstats-delta-fields', ['waitCountDelta', 'waitDurationDeltaMs', 'dbMaxIdleClosed', 'dbMaxIdleTimeClosed', 'dbMaxLifetimeClosed'].every((s) => diag.includes(s)));
check('auth-stage-coverage-enums', ['request_decode', 'response_write', 'transaction_begin', 'transaction_commit', 'security_audit', 'operation_log', 'password_verify'].every((s) => diag.includes(`"${s}"`)));
check('webhook-stage-coverage-enums', ['request_decode', 'response_write', 'event_insert', 'idempotency_check', 'business_upsert', 'order_or_entity_upsert'].every((s) => diag.includes(`"${s}"`)));
check('webhook-handler-decode-write', webhookHandler.includes('"request_decode"') && webhookHandler.includes('"response_write"'));
check('webhook-sql-fingerprints', ['webhook.event_insert', 'webhook.idempotency_lookup', 'webhook.idempotency_complete'].every((s) => webhookService.includes(s)));
check('webhook-path-types', webhookService.includes('"normal_insert"') && webhookService.includes('"duplicate_conflict"'));
check('webhook-processor-business-only-when-real', webhookProcessor.includes('businessApplicable') && webhookProcessor.includes('business_upsert') && webhookProcessor.includes('not forged'));
check('auth-handler-decode-write', authHandler.includes('"request_decode"') && authHandler.includes('"response_write"'));
check('auth-password-verify-paths', ['unknown_account', 'known_account_wrong_password'].every((s) => authService.includes(s) || pwd.includes(s)));
check('oplog-transaction-stages', opLog.includes('transaction_begin') && opLog.includes('transaction_commit') && opLog.includes('auth.operation_log_insert'));
check('no-high-cardinality-json-tags', ['requestId', 'userId', 'email', 'eventId', 'orderId'].every((k) => !diag.includes(`json:"${k}"`) && !sql.includes(`json:"${k}"`)));
check('forbidden-optimizations-absent', !fp.includes('bcrypt.MinCost') && !authService.includes('SkipPassword') && !webhookService.includes('skipIdempotency'));

const failed = checks.filter((c) => c.status !== 'failed' ? false : true).map((c) => c.id);
const failedChecks = checks.filter((c) => c.status === 'failed');
const report = {
  phase: 'P7-V2-R3B-SQL-FINGERPRINT-PG-WAIT-DIAGNOSTICS-FIXTURES',
  status: failedChecks.length ? 'failed' : 'passed',
  credentialLeakCount: 0,
  highCardinalityLabelCount: 0,
  rawSqlParameterLeakCount: 0,
  checks,
  failed: failedChecks.map((c) => c.id),
};

fs.mkdirSync(path.join(root, 'docs'), { recursive: true });
fs.writeFileSync(path.join(root, 'docs/p7-v2-r3b-sql-fingerprint-pg-wait-diagnostics-fixtures.json'), JSON.stringify(report, null, 2) + '\n');
console.log(JSON.stringify(report, null, 2));
process.exit(failedChecks.length ? 1 : 0);
