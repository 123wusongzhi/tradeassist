import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import {
  parseSafeTestDatabaseUrl,
  p9SourceManifest,
  readJSON,
  repoRoot,
  runtimeSummaryPath,
  sha256File,
} from './p9-postgres-contract.mjs';

export const P9_POSTGRES_INTEGRATION_CLOSURE_MD = 'docs/P9_POSTGRESQL_INTEGRATION_CLOSURE.md';
export const P9_POSTGRES_INTEGRATION_CLOSURE_JSON = 'docs/p9-postgresql-integration-closure.json';
export const P9_POSTGRES_INTEGRATION_CLOSURE_GATE_MD = 'docs/P9_POSTGRESQL_INTEGRATION_CLOSURE_GATE.md';
export const P9_POSTGRES_INTEGRATION_CLOSURE_GATE_JSON = 'docs/p9-postgresql-integration-closure-gate.json';

const BATCH_EVIDENCE_SPECS = [
  ['P9-TASK-BATCH-1', 'docs/P9_TASK_BATCH_1_DOMAIN_PERSISTENCE.md', 'docs/p9-task-batch-1-domain-persistence.json'],
  ['P9-TASK-BATCH-2', 'docs/P9_TASK_BATCH_2_SKU_CALIBRATION.md', 'docs/p9-task-batch-2-sku-calibration.json'],
  ['P9-TASK-BATCH-3', 'docs/P9_TASK_BATCH_3_SYNC_ORCHESTRATION.md', 'docs/p9-task-batch-3-sync-orchestration.json'],
  ['P9-TASK-BATCH-4', 'docs/P9_TASK_BATCH_4_PERMISSIONS_AUDIT_SAFETY.md', 'docs/p9-task-batch-4-permissions-audit-safety.json'],
  ['P9-TASK-BATCH-5', 'docs/P9_TASK_BATCH_5_BACKEND_APIS.md', 'docs/p9-task-batch-5-backend-apis.json'],
].map(([batchId, md, json]) => ({ batchId, md, json }));

const HISTORICAL_GATE_PATHS = [
  'docs/p9-entry-gate-report.json',
  'docs/p9-plan-final-gate.json',
  'docs/p9-task-batch-1-scope-gate.json',
  'docs/p9-task-batch-1-domain-persistence-gate.json',
  'docs/p9-task-batch-2-sku-calibration-gate.json',
  'docs/p9-task-batch-3-sync-orchestration-gate.json',
  'docs/p9-task-batch-4-permissions-audit-safety-gate.json',
  'docs/p9-task-batch-5-backend-apis-gate.json',
];

const REQUIRED_FILES = [
  P9_POSTGRES_INTEGRATION_CLOSURE_MD,
  P9_POSTGRES_INTEGRATION_CLOSURE_JSON,
  'docs/P9_EXECUTION_PLAN.md',
  'docs/p9-execution-plan.json',
  'docs/PROGRESS.md',
  'docs/README.md',
  'backend/internal/modules/inventorysyncp9/postgres_integration_test.go',
  'backend/internal/modules/inventorysyncp9/postgres_contract_test.go',
  'backend/internal/testing/integration/p9_postgres_integration_test.go',
  'backend/internal/testing/postgrestest/harness.go',
  'backend/internal/testing/safeenv/safeenv.go',
  'scripts/p9-postgres-contract.mjs',
  'scripts/p9-postgres-runtime.mjs',
  'scripts/p9-postgres-test-db-ensure.mjs',
  'scripts/p9-postgres-integration-gate.mjs',
  'tests/gates/p9/postgres-integration.mjs',
  ...BATCH_EVIDENCE_SPECS.flatMap(({ md, json }) => [md, json]),
];

function readText(relativePath) {
  try {
    return fs.readFileSync(path.join(repoRoot, relativePath), 'utf8');
  } catch {
    return '';
  }
}

function git(args) {
  try {
    return execFileSync('git', args, { cwd: repoRoot, encoding: 'utf8' }).trim();
  } catch {
    return '';
  }
}

function writeJSON(relativePath, data) {
  fs.writeFileSync(path.join(repoRoot, relativePath), `${JSON.stringify(data, null, 2)}\n`, 'utf8');
}

function secretLeakFree(texts) {
  return texts.every((text) => !/postgres(?:ql)?:\/\/[^/\s@]+:[^/\s@]+@/i.test(String(text || '')));
}

function batchValidation(batch, spec) {
  const evidence = batch.evidence || {};
  const md = batch.md || '';
  const initial = evidence.initialPostgresVerification || {};
  const revalidation = evidence.postgresRevalidation || {};
  const historical = ['not_run', 'not_executed_test_database_url_not_set'].includes(String(evidence.postgresIntegrationStatus || '').toLowerCase());
  const rootBoundary = (evidence.postgresIntegrationPassed === undefined || evidence.postgresIntegrationPassed === false)
    && (evidence.postgresIntegrationDeferredTo === undefined || evidence.postgresIntegrationDeferredTo === 'P9_Final_Development_Closure')
    && (evidence.p9FinalClosureBlocker === undefined || evidence.p9FinalClosureBlocker === true);
  const passed = batch.batchId === spec.batchId
    && initial.status === 'not_run'
    && initial.reason === 'TEST_DATABASE_URL_not_set'
    && revalidation.status === 'passed'
    && revalidation.evidencePath === P9_POSTGRES_INTEGRATION_CLOSURE_MD
    && revalidation.gateReportPath === P9_POSTGRES_INTEGRATION_CLOSURE_GATE_MD
    && Boolean(revalidation.verifiedAt)
    && evidence.currentPostgresIntegrationStatus === 'passed'
    && historical
    && rootBoundary
    && md.includes('PostgreSQL Revalidation')
    && md.includes('currentPostgresIntegrationStatus=passed')
    && secretLeakFree([md, JSON.stringify(evidence)]);
  return { batchId: spec.batchId, status: passed ? 'passed' : 'failed' };
}

function liveRuntimeIntegrity(runtime) {
  const summaryExists = Boolean(runtime && fs.existsSync(path.join(repoRoot, runtimeSummaryPath)));
  const summarySha256 = summaryExists ? sha256File(runtimeSummaryPath) : '';
  const rawArtifactHashesVerified = Boolean(runtime?.commands?.length >= 2 && runtime.commands.every((command) => {
    const actual = sha256File(command.rawArtifactPath || '');
    return /^[a-f0-9]{64}$/.test(actual) && actual === command.rawArtifactSha256;
  }));
  const currentManifest = p9SourceManifest();
  const sourceManifestVerified = Boolean(runtime?.sourceManifest?.stable === true
    && runtime.sourceManifest.beforeSha256 === runtime.sourceManifest.afterSha256
    && runtime.sourceManifest.afterSha256 === currentManifest.sha256);
  return { summaryHashVerified: summaryExists, summarySha256, rawArtifactHashesVerified, sourceManifestVerified };
}

function historicalStatus() {
  const rows = HISTORICAL_GATE_PATHS.map((relativePath) => ({ path: relativePath, status: readJSON(relativePath)?.status || 'missing' }));
  const failed = rows.filter((row) => row.status !== 'passed' && !(row.path === 'docs/p9-entry-gate-report.json' && row.status === 'allowed'));
  return { rows, failureCount: failed.length, passed: failed.length === 0 };
}

export function validateP9PostgresIntegrationClosure(bundle = {}) {
  const evidence = bundle.evidence || {};
  const runtime = bundle.runtime || {};
  const branch = bundle.currentBranch ?? git(['branch', '--show-current']);
  const head = bundle.currentHead ?? git(['rev-parse', 'HEAD']);
  const detached = bundle.headDetached ?? git(['rev-parse', '--abbrev-ref', 'HEAD']) === 'HEAD';
  const staged = bundle.stagedFileCount ?? (git(['diff', '--cached', '--name-only']) ? git(['diff', '--cached', '--name-only']).split(/\r?\n/).filter(Boolean).length : 0);
  const appEnv = bundle.appEnv ?? process.env.APP_ENV;
  const db = parseSafeTestDatabaseUrl(bundle.testDatabaseUrl ?? process.env.TEST_DATABASE_URL, { ...process.env, APP_ENV: appEnv });
  const runtimeIntegrity = bundle.runtimeIntegrity || liveRuntimeIntegrity(runtime);
  const historical = bundle.historicalGatesPassed === undefined ? historicalStatus() : {
    rows: [],
    passed: bundle.historicalGatesPassed,
    failureCount: bundle.historicalGateFailureCount ?? (bundle.historicalGatesPassed ? 0 : 1),
  };
  const specs = BATCH_EVIDENCE_SPECS;
  const batches = bundle.batchEvidence || specs.map((spec) => ({ batchId: spec.batchId, evidence: readJSON(spec.json) || {}, md: readText(spec.md) }));
  const batchResults = specs.map((spec, index) => batchValidation(batches[index] || {}, spec));
  const contracts = runtime.contracts || {};
  const boundary = runtime.platformBoundary || {};
  const requiredFilesPresent = bundle.requiredFilesPresent ?? REQUIRED_FILES.every((file) => fs.existsSync(path.join(repoRoot, file)));
  const scripts = bundle.packageScriptsPresent ?? (() => {
    const packageJSON = readJSON('package.json') || {};
    return ['test:p9-postgres-gate-fixtures', 'test:p9-postgres-integration', 'p9:postgres-integration-gate'].every((name) => typeof packageJSON.scripts?.[name] === 'string');
  })();
  const checks = [
    ['requiredFilesPresent', requiredFilesPresent],
    ['packageScriptsPresent', scripts],
    ['currentBranch', branch === 'dev'],
    ['headDetached', detached === false],
    ['stagedFileCount', staged === 0],
    ['runtimeGitBinding', runtime.git?.startBranch === branch && runtime.git?.endBranch === branch && runtime.git?.startHead === head && runtime.git?.endHead === head && runtime.git?.stable === true],
    ['runtimeSummaryHashVerified', runtimeIntegrity.summaryHashVerified === true],
    ['runtimeRawArtifactHashesVerified', runtimeIntegrity.rawArtifactHashesVerified === true],
    ['runtimeSourceManifestVerified', runtimeIntegrity.sourceManifestVerified === true],
    ['runtimeCompleted', runtime.completed === true && runtime.status === 'passed'],
    ['testDatabaseUrlPresent', db.present === true],
    ['testDatabaseUrlValid', db.valid === true],
    ['testDatabaseDriver', db.valid === true && runtime.testDatabase?.driver === 'postgresql'],
    ['testDatabaseNameSafe', db.nameSafe === true && runtime.testDatabase?.nameSafe === true],
    ['productionDatabaseRejected', db.productionRejected === true && runtime.testDatabase?.productionRejected === true],
    ['actualDatabaseMatched', runtime.testDatabase?.actualDatabaseMatched === true],
    ['sqliteFallbackUsed', runtime.testDatabase?.sqliteFallbackUsed === false && contracts.sqliteFallbackUsed === false],
    ['postgresConnectionPassed', contracts.postgresConnectionPassed === true],
    ['migrationUpPassed', contracts.migrationUpPassed === true],
    ['migrationIdempotencyPassed', contracts.migrationIdempotencyPassed === true],
    ['schemaVerificationPassed', contracts.schemaVerificationPassed === true],
    ['foreignKeysPresent', contracts.foreignKeysPresent === true],
    ['checkConstraintsPresent', contracts.checkConstraintsPresent === true],
    ['partialUniqueIndexesPresent', contracts.partialUniqueIndexesPresent === true],
    ['snapshotUniquenessPassed', contracts.snapshotUniquenessPassed === true],
    ['confirmedBindingUniquenessPassed', contracts.confirmedBindingUniquenessPassed === true],
    ['pendingManualRequestUniquenessPassed', contracts.pendingManualRequestUniquenessPassed === true],
    ['snapshotImmutabilityPassed', contracts.snapshotImmutabilityPassed === true],
    ['calibrationImmutabilityPassed', contracts.calibrationImmutabilityPassed === true],
    ['decisionHistoryImmutabilityPassed', contracts.decisionHistoryImmutabilityPassed === true],
    ['auditHistoryImmutabilityPassed', contracts.auditHistoryImmutabilityPassed === true],
    ['repositoryTestsPassed', contracts.repositoryTestsPassed === true],
    ['tenantIsolationPassed', contracts.tenantIsolationPassed === true],
    ['idempotencyTestsPassed', contracts.idempotencyTestsPassed === true],
    ['optimisticConcurrencyPassed', contracts.optimisticConcurrencyPassed === true],
    ['transactionAtomicityPassed', contracts.transactionAtomicityPassed === true],
    ['concurrencyTestsPassed', contracts.concurrencyTestsPassed === true],
    ['keysetPaginationPassed', contracts.keysetPaginationPassed === true],
    ['jsonContractPassed', contracts.jsonContractPassed === true],
    ['timestampContractPassed', contracts.timestampContractPassed === true],
    ['postgresApiIntegrationPassed', contracts.postgresApiIntegrationPassed === true],
    ['postgresFixtureGoldenPathPassed', contracts.postgresFixtureGoldenPathPassed === true],
    ['racePassed', runtime.racePassed === true && runtime.dataRaces === 0],
    ['fixtureProviderNetworkCalls', boundary.fixtureProviderNetworkCalls === 0],
    ['realPlatformNetworkCalls', boundary.realPlatformNetworkCalls === 0],
    ['realCredentialsUsed', boundary.realCredentialsUsed === false],
    ['inventoryMutationCalls', boundary.inventoryMutationCalls === 0],
    ['historicalGatesPassed', historical.passed === true && historical.failureCount === 0],
    ['batchRevalidations', batchResults.length === 5 && batchResults.every((batch) => batch.status === 'passed')],
    ['runtimeEvidenceBinding', evidence.runtimeEvidence?.runId === runtime.runId
      && evidence.runtimeEvidence?.sourceManifestSha256 === runtime.sourceManifest?.afterSha256
      && evidence.runtimeEvidence?.finishedAt === runtime.finishedAt
      && (!evidence.runtimeEvidence?.runtimeSummarySha256 || evidence.runtimeEvidence.runtimeSummarySha256 === runtimeIntegrity.summarySha256)],
    ['closureStatus', evidence.status === 'passed' && evidence.postgresIntegrationStatus === 'passed' && evidence.postgresIntegrationPassed === true],
    ['p9FinalClosureBlocker', evidence.p9FinalClosureBlocker === false],
    ['productionReady', evidence.productionReady === false],
    ['p10BoundaryPreserved', evidence.p10BoundaryPreserved === true],
    ['batch6ReadyToStart', evidence.batch6ReadyToStart === true],
    ['p9Complete', evidence.p9Complete === false],
    ['scopeBoundary', evidence.adminUiImplemented === false && evidence.backgroundSyncWorkerImplemented === false && evidence.automaticRetryWorkerImplemented === false && evidence.realDouyinProviderImplemented === false && evidence.oauthImplemented === false && evidence.realPlatformReadEnabled === false && evidence.realPlatformWriteEnabled === false],
    ['secretLeakFree', secretLeakFree([JSON.stringify(evidence), JSON.stringify(runtime), ...batches.flatMap((batch) => [batch.md || '', JSON.stringify(batch.evidence || {})])])],
  ];
  const failed = checks.filter(([, passed]) => !passed).map(([name]) => name);
  return {
    status: failed.length === 0 ? 'passed' : 'blocked',
    failed,
    failedCount: failed.length,
    currentBranch: branch,
    currentHead: head,
    headDetached: detached,
    stagedFileCount: staged,
    runtimeRunId: runtime.runId || '',
    runtimeSummarySha256: runtimeIntegrity.summarySha256 || '',
    testDatabase: { driver: runtime.testDatabase?.driver || '', purpose: 'test', urlRecorded: false, hostCategory: runtime.testDatabase?.hostCategory || '', nameSafe: db.nameSafe === true, productionRejected: db.productionRejected === true, serverVersion: runtime.testDatabase?.serverVersion || '' },
    contracts,
    racePassed: runtime.racePassed === true,
    dataRaces: runtime.dataRaces ?? null,
    platformBoundary: boundary,
    historicalGateFailureCount: historical.failureCount,
    historicalGates: historical.rows,
    batchRevalidations: batchResults,
    p9FinalClosureBlocker: evidence.p9FinalClosureBlocker !== false,
    productionReady: evidence.productionReady === true,
    batch6ReadyToStart: evidence.batch6ReadyToStart === true,
    checks: checks.map(([id, passed]) => ({ id, status: passed ? 'passed' : 'failed' })),
  };
}

export function buildP9PostgresIntegrationGateReport(bundle = {}) {
  const evidence = bundle.evidence || readJSON(P9_POSTGRES_INTEGRATION_CLOSURE_JSON) || {};
  const runtime = bundle.runtime || readJSON(runtimeSummaryPath) || {};
  const validation = validateP9PostgresIntegrationClosure({ ...bundle, evidence, runtime });
  return { phase: 'P9', gate: 'P9-POSTGRES-INTEGRATION', checkedAt: new Date().toISOString(), ...validation };
}

export function writeP9PostgresIntegrationGateReport(report) {
  writeJSON(P9_POSTGRES_INTEGRATION_CLOSURE_GATE_JSON, report);
  fs.writeFileSync(path.join(repoRoot, P9_POSTGRES_INTEGRATION_CLOSURE_GATE_MD), `# P9 PostgreSQL Integration Closure Gate\n\nStatus: **${report.status}**\n\n- Runtime run ID: ${report.runtimeRunId || 'missing'}\n- Runtime summary SHA-256: ${report.runtimeSummarySha256 || 'missing'}\n- Current branch: ${report.currentBranch}\n- Current HEAD: ${report.currentHead}\n- Staged files: ${report.stagedFileCount}\n- PostgreSQL driver: ${report.testDatabase.driver || 'missing'}\n- PostgreSQL server version: ${report.testDatabase.serverVersion || 'missing'}\n- SQLite fallback used: ${report.contracts?.sqliteFallbackUsed !== false}\n- Race passed: ${report.racePassed}\n- Data races: ${report.dataRaces ?? 'unknown'}\n- Historical gate failures: ${report.historicalGateFailureCount}\n- P9 final closure blocker: ${report.p9FinalClosureBlocker}\n- Production ready: ${report.productionReady}\n- Batch 6 ready to start: ${report.batch6ReadyToStart}\n- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}\n\nThis gate accepts only a fresh, hash-bound PostgreSQL runtime artifact. It does not enable production capabilities or close P9.\n`, 'utf8');
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildP9PostgresIntegrationGateReport();
  const checkOnly = process.argv.includes('--check');
  if (!checkOnly) writeP9PostgresIntegrationGateReport(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}
