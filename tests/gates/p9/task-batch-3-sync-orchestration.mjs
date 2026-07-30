import assert from 'node:assert/strict';
import { writeJSON } from '../../../scripts/p7-v2-lib.mjs';
import { validateP9Batch3SyncOrchestrationBundle } from '../../../scripts/p9-task-batch-3-sync-orchestration-gate.mjs';

const TASK_IDS = ['P9-701', 'P9-702', 'P9-703', 'P9-704', 'P9-705', 'P9-706'];
const fixtureScenarios = [
  'success_single_page',
  'success_multi_page',
  'empty_inventory',
  'low_confidence_binding',
  'binding_conflict',
  'unmatched_sku',
  'provider_timeout',
  'provider_rejected',
  'malformed_item',
  'duplicate_external_sku',
  'cursor_loop',
  'cancelled_context',
];
const stableErrors = [
  'provider_not_registered',
  'provider_capability_forbidden',
  'provider_cursor_invalid',
  'provider_cursor_loop',
  'provider_page_limit_exceeded',
  'provider_page_invalid',
  'provider_rejected',
  'provider_timeout',
  'sync_run_already_running',
  'sync_cancelled',
];

const validSources = {
  requiredFilesPresent: true,
  providerText: `
type InventoryProvider interface { FetchInventoryPage() }
type InventoryProviderCapabilities struct { NetworkAccess bool OAuth bool Credentials bool RealPlatformRead bool RealInventoryWrite bool InventoryMutation bool }
type InventoryProviderRegistry struct {}
ProviderModeMock ProviderModeSandbox ProviderModeLocalDraftOnly
inventoryFixtureCursor FixtureHash PageIndex ErrProviderCursorInvalid ErrProviderCursorLoop
${fixtureScenarios.join('\n')}
ErrProviderCapabilityForbidden
`,
  orchestratorText: `
type InventorySyncOrchestrator struct {}
func (o *InventorySyncOrchestrator) Run() {}
provider.FetchInventoryPage()
func (o *InventorySyncOrchestrator) commitPage() {
Transaction(func(tx *gorm.DB) error
CreateBatch
ResolvePageWithDB
updateRunStatusWithDB
}
finishWithError
ManualRerun
InventorySyncAuthorizer
ErrPermissionDenied
IdempotencyKeyHash
InputFingerprint
inventorySyncLockRegistry
provider_not_registered provider_capability_forbidden provider_cursor_invalid provider_cursor_loop provider_page_limit_exceeded provider_page_invalid provider_rejected provider_timeout sync_run_already_running sync_cancelled
`,
  pipelineText: `
type BindingResolutionPipeline struct {}
getCurrentConfirmedWithDB
calibrateSnapshotWithDB
manual_review_required
`,
  calibrationText: `
calibrateSnapshotWithDB
`,
  errorsText: stableErrors.join('\n'),
  testText: `
TestInventoryProviderRegistryAndFixtureProviderSafety
TestInventorySyncOrchestratorProcessesFixturePages
TestBindingResolutionPipelinePrioritizesConfirmedBinding
TestInventorySyncOrchestratorFailureCancellationIdempotencyAndRerun
TestInventorySyncOrchestratorConcurrentSameRequestCreatesOneRun
`,
  packageText: `
test:p9-task-batch-3-sync-orchestration
p9:task-batch-3-sync-orchestration-gate
`,
};

function taskEvidence(status = 'completed') {
  return TASK_IDS.reduce((acc, id) => {
    acc[id] = { status };
    return acc;
  }, {});
}

function validEvidence(overrides = {}) {
  return {
    batchId: 'P9-TASK-BATCH-3',
    batchName: 'Fixture Inventory Sync Orchestration',
    workingBranch: 'dev',
    changesCommitted: false,
    stagedFileCount: 0,
    workingTreeDirty: true,
    tasks: taskEvidence(),
    providerPortImplemented: true,
    fixtureProviderImplemented: true,
    fixtureScenariosImplemented: true,
    fixtureScenarios,
    unsafeProviderCapabilitiesRejected: true,
    providerModeContractPreserved: true,
    cursorContractImplemented: true,
    syncOrchestratorImplemented: true,
    providerFetchOutsideTransaction: true,
    pageTransactionImplemented: true,
    bindingResolutionPipelineImplemented: true,
    autoConfirmationEnabled: false,
    automaticConfirmedBindingCreatedByBatch3: false,
    manualFallbackImplemented: true,
    manualRerunImplemented: true,
    defaultRerunAllow: false,
    failureClassificationImplemented: true,
    stableErrors,
    idempotencyTestsPassed: true,
    concurrencyTestsPassed: true,
    testsPassed: true,
    raceTestsPassed: true,
    sqliteIntegrationTestsPassed: true,
    postgresIntegrationStatus: 'not_run',
    postgresIntegrationPassed: false,
    postgresIntegrationDeferredTo: 'P9_Final_Development_Closure',
    p9FinalClosureBlocker: true,
    syncWorkerImplemented: false,
    cronImplemented: false,
    tickerImplemented: false,
    queueConsumerImplemented: false,
    automaticRetryImplemented: false,
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
    inventoryMutationEnabled: false,
    p10BoundaryPreserved: true,
    productionReady: false,
    p9Complete: false,
    ...overrides,
  };
}

function validate(overrides = {}, sourceOverrides = {}, gitOverrides = {}) {
  return validateP9Batch3SyncOrchestrationBundle({
    evidence: validEvidence(overrides),
    sources: { ...validSources, ...sourceOverrides },
    gitState: { currentBranch: 'dev', currentHead: 'abc123', stagedFileCount: 0, workingTreeDirty: true, ...gitOverrides },
  });
}

function assertFails(fixtureId, id, overrides = {}, sourceOverrides = {}, gitOverrides = {}) {
  const result = validate(overrides, sourceOverrides, gitOverrides);
  assert.equal(result.status, 'failed', fixtureId);
  assert.ok(result.failed.includes(id), `${fixtureId} expected ${id}, saw ${result.failed.join(', ')}`);
}

assert.equal(validate().status, 'passed', 'B3-01 complete Batch 3 passes');
assertFails('B3-02 provider port is required', 'providerPortImplemented', { providerPortImplemented: false });
assertFails('B3-03 fixture scenarios are required', 'fixtureScenariosImplemented', { fixtureScenarios: fixtureScenarios.filter((id) => id !== 'cursor_loop') });
assertFails('B3-04 unsafe capabilities must be rejected', 'unsafeProviderCapabilitiesRejected', { unsafeProviderCapabilitiesRejected: false });
assertFails('B3-05 provider mode contract must be preserved', 'providerModeContractPreserved', { providerModeContractPreserved: false });
assertFails('B3-06 cursor contract is required', 'cursorContractImplemented', { cursorContractImplemented: false });
assertFails('B3-07 orchestrator is required', 'orchestratorImplemented', { syncOrchestratorImplemented: false });
assertFails('B3-08 provider fetch stays outside transaction', 'providerFetchOutsideTransaction', { providerFetchOutsideTransaction: false });
assertFails('B3-09 page transaction is required', 'pageTransactionImplemented', { pageTransactionImplemented: false });
assertFails('B3-10 binding resolution is required', 'bindingResolutionImplemented', { bindingResolutionPipelineImplemented: false });
assertFails('B3-11 auto confirmation remains disabled', 'autoConfirmationRespected', { automaticConfirmedBindingCreatedByBatch3: true });
assertFails('B3-12 manual rerun default deny is required', 'manualRerunImplemented', { defaultRerunAllow: true });
assertFails('B3-13 failure classification is required', 'failureClassificationImplemented', { stableErrors: stableErrors.filter((code) => code !== 'provider_timeout') });
assertFails('B3-14 idempotency and concurrency are required', 'idempotencyConcurrencyImplemented', { concurrencyTestsPassed: false });
assertFails('B3-15 tests are required', 'testsPresent', { testsPassed: false });
assertFails('B3-16 race tests are required', 'raceTestsPassed', { raceTestsPassed: false });
assertFails('B3-17 PostgreSQL must be recorded truthfully', 'postgresRecordedTruthfully', { postgresIntegrationStatus: 'passed', postgresIntegrationPassed: true });
assertFails('B3-18 package scripts are required', 'packageScriptsPresent', {}, { packageText: '' });
assertFails('B3-19 worker remains forbidden', 'scopeProtectionFlags', { syncWorkerImplemented: true });
assertFails('B3-20 real platform network remains forbidden', 'realBoundaryFlags', { realPlatformNetworkEnabled: true });
assertFails('B3-21 staged files fail gate', 'stagedFileCount', {}, {}, { stagedFileCount: 1 });
assertFails('Batch 3 task IDs cannot be renamed', 'P9-701 status', { tasks: { ...taskEvidence(), 'P9-701': { status: 'renamed' } } });
assertFails('P10 boundary must stay preserved', 'p10BoundaryPreserved', { p10BoundaryPreserved: false });
assertFails('Production Ready must stay false', 'productionReady', { productionReady: true });
assertFails('P9 complete must stay false', 'p9Complete', { p9Complete: true });

const report = {
  phase: 'P9',
  batchId: 'P9-TASK-BATCH-3',
  gate: 'P9-TASK-BATCH-3-SYNC-ORCHESTRATION-FIXTURE',
  status: 'passed',
  fixtureAssertions: 26,
};
writeJSON('docs/p9-task-batch-3-sync-orchestration-fixture-report.json', report);
console.log(JSON.stringify(report, null, 2));
