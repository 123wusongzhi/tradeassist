import assert from 'node:assert/strict';
import { writeJSON } from '../../../scripts/p7-v2-lib.mjs';
import { validateP9Batch1DomainPersistenceBundle } from '../../../scripts/p9-task-batch-1-domain-persistence-gate.mjs';

const TASK_IDS = ['P9-501', 'P9-502', 'P9-503', 'P9-504', 'P9-505', 'P9-506'];

const validSources = {
  modelText: `
type InventorySyncRun struct {}
type InventorySnapshotItem struct {}
type SKUBinding struct {}
type SKUBindingCalibration struct {}
type ManualBindingRequest struct {}
func (InventorySnapshotItem) BeforeUpdate() error { return nil }
func (InventorySnapshotItem) BeforeDelete() error { return nil }
func (SKUBindingCalibration) BeforeUpdate() error { return nil }
func (SKUBindingCalibration) BeforeDelete() error { return nil }
`,
  repoText: `
type InventorySyncRunRepository struct {}
type InventorySnapshotRepository struct {}
type SKUBindingRepository struct {}
type SKUBindingCalibrationRepository struct {}
type ManualBindingRequestRepository struct {}
func (r *InventorySyncRunRepository) Create() {}
func (r *InventorySnapshotRepository) CreateBatch() {}
func (r *SKUBindingRepository) TransitionWithRevision() {}
func (r *SKUBindingCalibrationRepository) GetBestCandidate() {}
func (r *ManualBindingRequestRepository) ResolveWithRevision() {}
tenant_id = ? AND id = ?
verifyShopConnection
verifyLocalSKU
localSKUProductID
idempotency_key_hash
InputFingerprint
ErrIdempotencyPayloadConflict
revision + 1
revision = ?
ErrRevisionConflict
Transaction(func(tx *gorm.DB) error
`,
  migrateText: `
p9_inventory_sync_runs
p9_inventory_snapshot_items
p9_sku_bindings
p9_sku_binding_calibrations
p9_manual_binding_requests
ux_p9_inventory_snapshots_tenant_run_external_sku
ux_p9_sku_bindings_current_confirmed
ux_p9_manual_binding_requests_pending
trg_p9_inventory_snapshot_items_no_update
trg_p9_sku_binding_calibrations_no_update
`,
  validationText: 'sensitiveJSONKeys\nnormalizeModelJSON\n',
  errorsText: `
validation_error
not_found
tenant_mismatch
revision_conflict
state_conflict
duplicate_external_sku
binding_conflict
binding_not_confirmed
manual_binding_already_pending
manual_binding_already_resolved
idempotency_payload_conflict
`,
  testText: `
TestInventorySyncRunIdempotencyAndRevision
TestSnapshotUniquenessTenantIsolationAndImmutability
TestBindingConfirmedUniquenessAndRevision
TestCalibrationAtomicityAndImmutability
TestManualBindingIdempotencyPendingUniqueAndResolveConcurrency
Migrate(db)
`,
  scopeTestText: `
TestBatch1ScopeDoesNotImplementLaterWork
TestBatch1ModelsDoNotStoreCredentialFields
`,
  dbMigrateText: 'inventorysyncp9.Migrate(db)',
};

function taskEvidence(status = 'completed') {
  return TASK_IDS.reduce((acc, id) => {
    acc[id] = { status };
    return acc;
  }, {});
}

function validEvidence(overrides = {}) {
  return {
    batchId: 'P9-TASK-BATCH-1',
    baseCheckpoint: '1ac652ac4797eee636bf615f1e9ed272f2b82f84',
    workingBranch: 'dev',
    changesCommitted: false,
    checkpointCreated: false,
    stagedFileCount: 0,
    workingTreeDirty: true,
    tasks: taskEvidence(),
    models: ['InventorySyncRun', 'InventorySnapshotItem', 'SKUBinding', 'SKUBindingCalibration', 'ManualBindingRequest'],
    tables: ['p9_inventory_sync_runs', 'p9_inventory_snapshot_items', 'p9_sku_bindings', 'p9_sku_binding_calibrations', 'p9_manual_binding_requests'],
    autoMigrateRegistered: true,
    repositoryPresent: true,
    stableErrorsPresent: true,
    stableErrors: ['validation_error', 'not_found', 'tenant_mismatch', 'revision_conflict', 'state_conflict', 'duplicate_external_sku', 'binding_conflict', 'binding_not_confirmed', 'manual_binding_already_pending', 'manual_binding_already_resolved', 'idempotency_payload_conflict', 'immutable_record'],
    constraints: Array.from({ length: 10 }, (_, idx) => `constraint-${idx}`),
    repositoryMethods: [
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
    ],
    tenantIsolationPassed: true,
    idempotencyTestsPassed: true,
    optimisticConcurrencyTestsPassed: true,
    immutableHistoryTestsPassed: true,
    batchAtomicityTestsPassed: true,
    repositoryTestsPassed: true,
    migrationTestsPassed: true,
    sqliteIntegrationTestsPassed: true,
    scopeProtectionTestsPassed: true,
    safeMetadataSensitiveKeyRejected: true,
    rawProviderResponseStored: false,
    shopAuthTokenCredentialFieldsRead: false,
    sqlStateExposed: false,
    sqlTextExposed: false,
    connectionStringExposed: false,
    databaseStackTraceExposed: false,
    calibrationServiceImplemented: false,
    matchingAlgorithmServiceImplemented: false,
    automaticConfirmationThresholdImplemented: false,
    syncOrchestratorImplemented: false,
    syncWorkerImplemented: false,
    cronImplemented: false,
    tickerImplemented: false,
    queueConsumerImplemented: false,
    apiImplemented: false,
    httpHandlerImplemented: false,
    ginRouterImplemented: false,
    restApiImplemented: false,
    adminUiImplemented: false,
    frontendApiClientImplemented: false,
    realDouyinProviderImplemented: false,
    oauthImplemented: false,
    realCredentialsEnabled: false,
    realPlatformNetworkEnabled: false,
    realPlatformReadEnabled: false,
    realPlatformWriteEnabled: false,
    realInventoryReadEnabled: false,
    realInventoryWriteEnabled: false,
    automaticPublishEnabled: false,
    automaticListingEnabled: false,
    humanConfirmationRequired: true,
    p10BoundaryPreserved: true,
    productionReady: false,
    tagDeferred: true,
    releaseDeferred: true,
    p9Complete: false,
    nextBatch: {
      taskIds: ['P9-601', 'P9-602', 'P9-603', 'P9-604', 'P9-605', 'P9-606'],
      status: 'notStarted',
    },
    ...overrides,
  };
}

function validate(overrides = {}, sourceOverrides = {}, gitOverrides = {}) {
  return validateP9Batch1DomainPersistenceBundle({
    evidence: validEvidence(overrides),
    sources: { ...validSources, ...sourceOverrides },
    gitState: { currentBranch: 'dev', currentHead: 'abc123', stagedFileCount: 0, workingTreeDirty: true, ...gitOverrides },
  });
}

function assertFails(id, overrides = {}, sourceOverrides = {}, gitOverrides = {}) {
  const result = validate(overrides, sourceOverrides, gitOverrides);
  assert.equal(result.status, 'failed', id);
  assert.ok(result.failed.includes(id), `${id} should fail, saw ${result.failed.join(', ')}`);
}

assert.equal(validate().status, 'passed');
assertFails('batchId', { batchId: 'P9-TASK-BATCH-2' });
assertFails('baseCheckpoint', { baseCheckpoint: 'bad' });
assertFails('currentBranch', {}, {}, { currentBranch: 'main' });
assertFails('stagedFileCount', {}, {}, { stagedFileCount: 1 });
assertFails('workingTreeDirtyRecorded', { workingTreeDirty: false });
assertFails('changesCommitted', { changesCommitted: true });
assertFails('checkpointCreated', { checkpointCreated: true });
assertFails('P9-501 status', { tasks: { ...taskEvidence(), 'P9-501': { status: 'pending' } } });
assertFails('allModelsPresent', { models: ['InventorySyncRun'] });
assertFails('allTablesPresent', { tables: ['p9_inventory_sync_runs'] });
assertFails('autoMigrateRegistered', { autoMigrateRegistered: false });
assertFails('repositoryMethods', { repositoryMethods: [] });
assertFails('stableErrorsPresent', { stableErrorsPresent: false });
assertFails('tenantIsolationImplemented', { tenantIsolationPassed: false });
assertFails('idempotencyImplemented', { idempotencyTestsPassed: false });
assertFails('optimisticConcurrencyImplemented', { optimisticConcurrencyTestsPassed: false });
assertFails('immutableHistoryImplemented', { immutableHistoryTestsPassed: false });
assertFails('batchAtomicityImplemented', { batchAtomicityTestsPassed: false });
assertFails('constraintsPresent', { constraints: [] });
assertFails('safeJSONValidationPresent', { safeMetadataSensitiveKeyRejected: false });
assertFails('rawProviderResponseNotStored', { rawProviderResponseStored: true });
assertFails('shopAuthTokenCredentialFieldsNotRead', { shopAuthTokenCredentialFieldsRead: true });
assertFails('sqlLeakageDisabled', { sqlStateExposed: true });
assertFails('repositoryTestsPresent', { repositoryTestsPassed: false });
assertFails('migrationTestsPresent', { migrationTestsPassed: false });
assertFails('scopeProtectionTestsPresent', { scopeProtectionTestsPassed: false });
assertFails('modelCredentialFieldsAbsent', {}, { modelText: `${validSources.modelText}\nAccessToken string` });
assertFails('forbiddenImplementationFlags', { syncWorkerImplemented: true });
assertFails('p10BoundaryPreserved', { p10BoundaryPreserved: false });
assertFails('humanConfirmationRequired', { humanConfirmationRequired: false });
assertFails('tagDeferred', { tagDeferred: false });
assertFails('releaseDeferred', { releaseDeferred: false });
assertFails('nextBatchNotStarted', { nextBatch: { taskIds: ['P9-601'], status: 'started' } });

const report = {
  phase: 'P9',
  batchId: 'P9-TASK-BATCH-1',
  gate: 'P9-TASK-BATCH-1-DOMAIN-PERSISTENCE-FIXTURE',
  status: 'passed',
  fixtureAssertions: 35,
};
writeJSON('docs/p9-task-batch-1-domain-persistence-fixture-report.json', report);
console.log(JSON.stringify(report, null, 2));
