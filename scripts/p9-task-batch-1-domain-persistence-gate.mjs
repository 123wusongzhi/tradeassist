import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';

export const P9_BATCH_1_DOMAIN_PERSISTENCE_JSON = 'docs/p9-task-batch-1-domain-persistence.json';
export const P9_BATCH_1_DOMAIN_PERSISTENCE_MD = 'docs/P9_TASK_BATCH_1_DOMAIN_PERSISTENCE.md';
export const P9_BATCH_1_DOMAIN_PERSISTENCE_GATE_JSON = 'docs/p9-task-batch-1-domain-persistence-gate.json';
export const P9_BATCH_1_DOMAIN_PERSISTENCE_GATE_MD = 'docs/P9_TASK_BATCH_1_DOMAIN_PERSISTENCE_GATE.md';
export const P9_BATCH_1_BASE_CHECKPOINT = '1ac652ac4797eee636bf615f1e9ed272f2b82f84';

const REPO_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

function readJSON(rel) {
  try {
    return JSON.parse(fs.readFileSync(path.join(REPO_ROOT, rel), 'utf8'));
  } catch {
    return null;
  }
}

function writeJSON(rel, data) {
  const full = path.join(REPO_ROOT, rel);
  fs.mkdirSync(path.dirname(full), { recursive: true });
  fs.writeFileSync(full, `${JSON.stringify(data, null, 2)}\n`, 'utf8');
}

function writeMarkdown(rel, body) {
  const full = path.join(REPO_ROOT, rel);
  fs.mkdirSync(path.dirname(full), { recursive: true });
  fs.writeFileSync(full, body, 'utf8');
}

const TASK_IDS = ['P9-501', 'P9-502', 'P9-503', 'P9-504', 'P9-505', 'P9-506'];

const REQUIRED_FILES = [
  'backend/internal/modules/inventorysyncp9/errors.go',
  'backend/internal/modules/inventorysyncp9/model.go',
  'backend/internal/modules/inventorysyncp9/validate.go',
  'backend/internal/modules/inventorysyncp9/repository.go',
  'backend/internal/modules/inventorysyncp9/migrate.go',
  'backend/internal/modules/inventorysyncp9/repository_test.go',
  'backend/internal/modules/inventorysyncp9/scope_test.go',
  'backend/internal/database/migrate.go',
  P9_BATCH_1_DOMAIN_PERSISTENCE_MD,
  P9_BATCH_1_DOMAIN_PERSISTENCE_JSON,
];

const FORBIDDEN_PATHS = [
  'admin/src/pages/inventorysyncp9',
  'admin/src/services/inventorysyncp9',
  'backend/internal/api/inventorysyncp9',
  'backend/internal/modules/inventorysyncp9/handler.go',
  'backend/internal/modules/inventorysyncp9/service.go',
  'backend/internal/modules/inventorysyncp9/worker.go',
  'backend/internal/providers/douyin/inventorysyncp9',
];

const REQUIRED_MODELS = [
  'type InventorySyncRun struct',
  'type InventorySnapshotItem struct',
  'type SKUBinding struct',
  'type SKUBindingCalibration struct',
  'type ManualBindingRequest struct',
];

const REQUIRED_TABLES = [
  'p9_inventory_sync_runs',
  'p9_inventory_snapshot_items',
  'p9_sku_bindings',
  'p9_sku_binding_calibrations',
  'p9_manual_binding_requests',
];

const REQUIRED_REPOSITORY_METHODS = [
  'InventorySyncRun.Create',
  'InventorySyncRun.GetByID',
  'InventorySyncRun.GetByIdempotency',
  'InventorySyncRun.UpdateStatusWithRevision',
  'InventorySnapshot.CreateBatch',
  'InventorySnapshot.ListByRun',
  'InventorySnapshot.GetByRunAndExternalSKU',
  'InventorySnapshot.CountByRun',
  'SKUBinding.CreateProposed',
  'SKUBinding.GetCurrentConfirmed',
  'SKUBinding.ListByExternalSKU',
  'SKUBinding.ListByLocalSKU',
  'SKUBinding.TransitionWithRevision',
  'SKUBindingCalibration.CreateBatch',
  'SKUBindingCalibration.ListBySnapshot',
  'SKUBindingCalibration.ListByRun',
  'SKUBindingCalibration.GetBestCandidate',
  'ManualBindingRequest.Create',
  'ManualBindingRequest.GetByID',
  'ManualBindingRequest.GetPendingByExternalSKU',
  'ManualBindingRequest.ListPending',
  'ManualBindingRequest.ResolveWithRevision',
];

const REQUIRED_ERROR_CODES = [
  'validation_error',
  'not_found',
  'tenant_mismatch',
  'revision_conflict',
  'state_conflict',
  'duplicate_external_sku',
  'binding_conflict',
  'binding_not_confirmed',
  'manual_binding_already_pending',
  'manual_binding_already_resolved',
  'idempotency_payload_conflict',
];

const FORBIDDEN_IMPLEMENTATION_FLAGS = [
  'calibrationServiceImplemented',
  'matchingAlgorithmServiceImplemented',
  'automaticConfirmationThresholdImplemented',
  'syncOrchestratorImplemented',
  'syncWorkerImplemented',
  'cronImplemented',
  'tickerImplemented',
  'queueConsumerImplemented',
  'apiImplemented',
  'httpHandlerImplemented',
  'ginRouterImplemented',
  'restApiImplemented',
  'adminUiImplemented',
  'frontendApiClientImplemented',
  'realDouyinProviderImplemented',
  'oauthImplemented',
  'realCredentialsEnabled',
  'realPlatformNetworkEnabled',
  'realPlatformReadEnabled',
  'realPlatformWriteEnabled',
  'realInventoryReadEnabled',
  'realInventoryWriteEnabled',
  'automaticPublishEnabled',
  'automaticListingEnabled',
  'productionReady',
  'p9Complete',
  'rawProviderResponseStored',
  'shopAuthTokenCredentialFieldsRead',
  'sqlStateExposed',
  'sqlTextExposed',
  'connectionStringExposed',
  'databaseStackTraceExposed',
];

function rootPath(rel) {
  return path.join(REPO_ROOT, rel);
}

function text(rel) {
  try {
    return fs.readFileSync(rootPath(rel), 'utf8');
  } catch {
    return '';
  }
}

function git(args) {
  try {
    return execFileSync('git', args, { cwd: REPO_ROOT, encoding: 'utf8' }).trim();
  } catch {
    return '';
  }
}

function hasAll(value, needles) {
  return needles.every((needle) => value.includes(needle));
}

function arrayIncludes(values, expected) {
  const set = new Set(Array.isArray(values) ? values : []);
  return expected.every((item) => set.has(item));
}

function stagedFileCount() {
  const files = git(['diff', '--cached', '--name-only']);
  return files ? files.split('\n').filter(Boolean).length : 0;
}

function workingTreeDirty() {
  return git(['status', '--short']) !== '';
}

function forbiddenPathsAbsent() {
  return FORBIDDEN_PATHS.every((rel) => !fs.existsSync(rootPath(rel)));
}

function sourceHasNoForbiddenCredentialFields(modelText) {
  const source = modelText.toLowerCase();
  return ['accesstoken', 'refresh_token', 'refreshtoken', 'appsecret', 'cookie', 'oauth'].every((needle) => !source.includes(needle));
}

export function validateP9Batch1DomainPersistenceBundle({ evidence = {}, sources = {}, gitState = {} } = {}) {
  const modelText = sources.modelText ?? text('backend/internal/modules/inventorysyncp9/model.go');
  const repoText = sources.repoText ?? text('backend/internal/modules/inventorysyncp9/repository.go');
  const migrateText = sources.migrateText ?? text('backend/internal/modules/inventorysyncp9/migrate.go');
  const validationText = sources.validationText ?? text('backend/internal/modules/inventorysyncp9/validate.go');
  const errorsText = sources.errorsText ?? text('backend/internal/modules/inventorysyncp9/errors.go');
  const testText = sources.testText ?? text('backend/internal/modules/inventorysyncp9/repository_test.go');
  const scopeTestText = sources.scopeTestText ?? text('backend/internal/modules/inventorysyncp9/scope_test.go');
  const dbMigrateText = sources.dbMigrateText ?? text('backend/internal/database/migrate.go');
  const missingFiles = REQUIRED_FILES.filter((rel) => !fs.existsSync(rootPath(rel)));
  const branch = gitState.currentBranch ?? git(['branch', '--show-current']);
  const head = gitState.currentHead ?? git(['rev-parse', 'HEAD']);
  const staged = gitState.stagedFileCount ?? stagedFileCount();
  const dirty = gitState.workingTreeDirty ?? workingTreeDirty();

  const checks = [
    ['requiredFilesPresent', missingFiles.length === 0],
    ['batchId', evidence.batchId === 'P9-TASK-BATCH-1'],
    ['baseCheckpoint', evidence.baseCheckpoint === P9_BATCH_1_BASE_CHECKPOINT],
    ['currentBranch', branch === 'dev'],
    ['currentHeadPresent', typeof head === 'string' && head.length > 0],
    ['stagedFileCount', staged === 0],
    ['workingTreeDirtyRecorded', evidence.workingTreeDirty === dirty],
    ['changesCommitted', evidence.changesCommitted === false],
    ['checkpointCreated', evidence.checkpointCreated === false],
    ...TASK_IDS.map((id) => [`${id} status`, String(evidence.tasks?.[id]?.status || '').startsWith('completed')]),
    ['allModelsPresent', hasAll(modelText, REQUIRED_MODELS) && arrayIncludes(evidence.models, ['InventorySyncRun', 'InventorySnapshotItem', 'SKUBinding', 'SKUBindingCalibration', 'ManualBindingRequest'])],
    ['allTablesPresent', hasAll(migrateText, REQUIRED_TABLES) && arrayIncludes(evidence.tables, REQUIRED_TABLES)],
    ['autoMigrateRegistered', evidence.autoMigrateRegistered === true && dbMigrateText.includes('inventorysyncp9.Migrate(db)')],
    ['repositoryPresent', evidence.repositoryPresent === true && hasAll(repoText, ['type InventorySyncRunRepository struct', 'type InventorySnapshotRepository struct', 'type SKUBindingRepository struct', 'type SKUBindingCalibrationRepository struct', 'type ManualBindingRequestRepository struct'])],
    ['repositoryMethods', arrayIncludes(evidence.repositoryMethods, REQUIRED_REPOSITORY_METHODS) && hasAll(repoText, ['func (r *InventorySyncRunRepository) Create', 'func (r *InventorySnapshotRepository) CreateBatch', 'func (r *SKUBindingRepository) TransitionWithRevision', 'func (r *SKUBindingCalibrationRepository) GetBestCandidate', 'func (r *ManualBindingRequestRepository) ResolveWithRevision'])],
    ['stableErrorsPresent', evidence.stableErrorsPresent === true && arrayIncludes(evidence.stableErrors, REQUIRED_ERROR_CODES) && hasAll(errorsText, REQUIRED_ERROR_CODES)],
    ['tenantIsolationImplemented', evidence.tenantIsolationPassed === true && hasAll(repoText, ['tenant_id = ? AND id = ?', 'verifyShopConnection', 'verifyLocalSKU', 'localSKUProductID'])],
    ['idempotencyImplemented', evidence.idempotencyTestsPassed === true && repoText.includes('idempotency_key_hash') && repoText.includes('InputFingerprint') && repoText.includes('ErrIdempotencyPayloadConflict')],
    ['optimisticConcurrencyImplemented', evidence.optimisticConcurrencyTestsPassed === true && hasAll(repoText, ['revision + 1', 'revision = ?', 'ErrRevisionConflict'])],
    ['immutableHistoryImplemented', evidence.immutableHistoryTestsPassed === true && hasAll(modelText, ['BeforeUpdate', 'BeforeDelete']) && hasAll(migrateText, ['trg_p9_inventory_snapshot_items_no_update', 'trg_p9_sku_binding_calibrations_no_update'])],
    ['batchAtomicityImplemented', evidence.batchAtomicityTestsPassed === true && repoText.includes('Transaction(func(tx *gorm.DB) error')],
    ['constraintsPresent', Array.isArray(evidence.constraints) && evidence.constraints.length >= 10 && hasAll(migrateText, ['ux_p9_inventory_snapshots_tenant_run_external_sku', 'ux_p9_sku_bindings_current_confirmed', 'ux_p9_manual_binding_requests_pending'])],
    ['safeJSONValidationPresent', evidence.safeMetadataSensitiveKeyRejected === true && validationText.includes('sensitiveJSONKeys') && validationText.includes('normalizeModelJSON')],
    ['rawProviderResponseNotStored', evidence.rawProviderResponseStored === false],
    ['shopAuthTokenCredentialFieldsNotRead', evidence.shopAuthTokenCredentialFieldsRead === false && !repoText.includes('ShopAuthToken')],
    ['sqlLeakageDisabled', evidence.sqlStateExposed === false && evidence.sqlTextExposed === false && evidence.connectionStringExposed === false && evidence.databaseStackTraceExposed === false],
    ['repositoryTestsPresent', evidence.repositoryTestsPassed === true && hasAll(testText, ['TestInventorySyncRunIdempotencyAndRevision', 'TestSnapshotUniquenessTenantIsolationAndImmutability', 'TestBindingConfirmedUniquenessAndRevision', 'TestCalibrationAtomicityAndImmutability', 'TestManualBindingIdempotencyPendingUniqueAndResolveConcurrency'])],
    ['migrationTestsPresent', evidence.migrationTestsPassed === true && testText.includes('Migrate(db)')],
    ['scopeProtectionTestsPresent', evidence.scopeProtectionTestsPassed === true && hasAll(scopeTestText, ['TestBatch1ScopeDoesNotImplementLaterWork', 'TestBatch1ModelsDoNotStoreCredentialFields'])],
    ['forbiddenPathsAbsent', forbiddenPathsAbsent()],
    ['modelCredentialFieldsAbsent', sourceHasNoForbiddenCredentialFields(modelText)],
    ['forbiddenImplementationFlags', FORBIDDEN_IMPLEMENTATION_FLAGS.every((flag) => evidence[flag] === false)],
    ['p10BoundaryPreserved', evidence.p10BoundaryPreserved === true],
    ['humanConfirmationRequired', evidence.humanConfirmationRequired === true],
    ['tagDeferred', evidence.tagDeferred === true],
    ['releaseDeferred', evidence.releaseDeferred === true],
    ['nextBatchNotStarted', evidence.nextBatch?.status === 'notStarted' && arrayIncludes(evidence.nextBatch?.taskIds, ['P9-601', 'P9-602', 'P9-603', 'P9-604', 'P9-605', 'P9-606'])],
  ];

  const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failed.length ? 'failed' : 'passed',
    failed,
    failedCount: failed.length,
    missingFiles,
    currentBranch: branch,
    currentHead: head,
    stagedFileCount: staged,
    workingTreeDirty: dirty,
    checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  };
}

export function buildP9Batch1DomainPersistenceGateReport(bundle = {}) {
  const evidence = bundle.evidence ?? readJSON(P9_BATCH_1_DOMAIN_PERSISTENCE_JSON) ?? {};
  const validation = validateP9Batch1DomainPersistenceBundle({ evidence, sources: bundle.sources, gitState: bundle.gitState });
  return {
    phase: 'P9',
    gate: 'P9-TASK-BATCH-1-DOMAIN-PERSISTENCE',
    status: validation.status,
    checkedAt: new Date().toISOString(),
    batchId: evidence.batchId || '',
    batchName: evidence.batchName || '',
    baseCheckpoint: evidence.baseCheckpoint || '',
    currentBranch: validation.currentBranch,
    currentHead: validation.currentHead,
    stagedFileCount: validation.stagedFileCount,
    workingTreeDirty: validation.workingTreeDirty,
    tasks: TASK_IDS,
    models: evidence.models || [],
    tables: evidence.tables || [],
    tenantIsolationImplemented: evidence.tenantIsolationPassed === true,
    idempotencyImplemented: evidence.idempotencyTestsPassed === true,
    optimisticConcurrencyImplemented: evidence.optimisticConcurrencyTestsPassed === true,
    immutableHistoryImplemented: evidence.immutableHistoryTestsPassed === true,
    batchAtomicityImplemented: evidence.batchAtomicityTestsPassed === true,
    repositoryTestsPassed: evidence.repositoryTestsPassed === true,
    migrationTestsPassed: evidence.migrationTestsPassed === true,
    sqliteIntegrationTestsPassed: evidence.sqliteIntegrationTestsPassed === true,
    scopeProtectionTestsPassed: evidence.scopeProtectionTestsPassed === true,
    currentBatchRaceStatus: evidence.currentBatchRaceStatus || '',
    postgresIntegrationStatus: evidence.postgresIntegrationStatus || '',
    fullVerificationStatus: evidence.fullVerificationStatus || '',
    calibrationServiceImplemented: evidence.calibrationServiceImplemented === true,
    syncOrchestratorImplemented: evidence.syncOrchestratorImplemented === true,
    syncWorkerImplemented: evidence.syncWorkerImplemented === true,
    apiImplemented: evidence.apiImplemented === true,
    adminUiImplemented: evidence.adminUiImplemented === true,
    realDouyinProviderImplemented: evidence.realDouyinProviderImplemented === true,
    oauthImplemented: evidence.oauthImplemented === true,
    realCredentialsEnabled: evidence.realCredentialsEnabled === true,
    realPlatformNetworkEnabled: evidence.realPlatformNetworkEnabled === true,
    realPlatformReadEnabled: evidence.realPlatformReadEnabled === true,
    realPlatformWriteEnabled: evidence.realPlatformWriteEnabled === true,
    productionReady: evidence.productionReady === true,
    p10BoundaryPreserved: evidence.p10BoundaryPreserved === true,
    failedCount: validation.failedCount,
    failed: validation.failed,
    missingFiles: validation.missingFiles,
    checks: validation.checks,
  };
}

export function writeP9Batch1DomainPersistenceGateReport(report) {
  writeJSON(P9_BATCH_1_DOMAIN_PERSISTENCE_GATE_JSON, report);
  writeMarkdown(
    P9_BATCH_1_DOMAIN_PERSISTENCE_GATE_MD,
    `# P9 Batch 1 Domain Persistence Gate

Status: **${report.status}**

- Batch id: ${report.batchId}
- Batch name: ${report.batchName}
- Base checkpoint: ${report.baseCheckpoint}
- Current branch: ${report.currentBranch}
- Current head: ${report.currentHead}
- Staged files: ${report.stagedFileCount}
- Working tree dirty: ${report.workingTreeDirty}
- Tasks: ${report.tasks.join(', ')}
- Models: ${report.models.join(', ')}
- Tables: ${report.tables.join(', ')}
- Tenant isolation implemented: ${report.tenantIsolationImplemented}
- Idempotency implemented: ${report.idempotencyImplemented}
- Optimistic concurrency implemented: ${report.optimisticConcurrencyImplemented}
- Immutable history implemented: ${report.immutableHistoryImplemented}
- Batch atomicity implemented: ${report.batchAtomicityImplemented}
- Repository tests passed: ${report.repositoryTestsPassed}
- Migration tests passed: ${report.migrationTestsPassed}
- SQLite integration tests passed: ${report.sqliteIntegrationTestsPassed}
- Scope protection tests passed: ${report.scopeProtectionTestsPassed}
- Current batch race status: ${report.currentBatchRaceStatus}
- Postgres integration status: ${report.postgresIntegrationStatus}
- Full verification status: ${report.fullVerificationStatus}
- Calibration service implemented: ${report.calibrationServiceImplemented}
- Sync orchestrator implemented: ${report.syncOrchestratorImplemented}
- Sync worker implemented: ${report.syncWorkerImplemented}
- API implemented: ${report.apiImplemented}
- Admin UI implemented: ${report.adminUiImplemented}
- Real Douyin provider implemented: ${report.realDouyinProviderImplemented}
- OAuth implemented: ${report.oauthImplemented}
- Real credentials enabled: ${report.realCredentialsEnabled}
- Real platform network enabled: ${report.realPlatformNetworkEnabled}
- Real platform read enabled: ${report.realPlatformReadEnabled}
- Real platform write enabled: ${report.realPlatformWriteEnabled}
- P10 boundary preserved: ${report.p10BoundaryPreserved}
- Production Ready: ${report.productionReady}
- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}

This gate validates only P9 Batch 1 domain and persistence work. It does not authorize calibration services, sync orchestration, workers, API, Admin UI, real Douyin/OAuth/credentials, real platform network, real inventory reads/writes, release, tag, P9 completion, or Production Ready.
`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildP9Batch1DomainPersistenceGateReport();
  writeP9Batch1DomainPersistenceGateReport(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}
