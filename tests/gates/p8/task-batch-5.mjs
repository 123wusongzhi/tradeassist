import assert from 'node:assert/strict';
import { writeJSON } from '../../../scripts/p7-v2-lib.mjs';
import { validateP8TaskBatch5Bundle } from '../../../scripts/p8-task-batch-5-final-gate.mjs';

const validSources = {
  executionText: 'type DraftExecutionPort interface {\nExecuteDraft(ctx context.Context, input DraftExecutionInput) (DraftExecutionResult, error)\n}',
  adapterText: [
    'type DraftAdapterCapabilities struct { DraftCreation bool Publish bool Listing bool NetworkAccess bool RealCredentials bool AutomaticExecution bool }',
    'type PlatformDraftAdapterRegistry struct {}',
    'func (r *PlatformDraftAdapterRegistry) ExecuteDraft',
    'type LocalDraftAdapter struct {}',
    'func NewLocalDraftAdapter',
    'local_draft',
    'type DouyinDraftFixtureAdapter struct {}',
    'func NewDouyinDraftFixtureAdapter',
    'mock:douyin',
    'sandbox:douyin',
    'func UnsupportedPlatformGuard',
    'func AutomaticPublishGuard',
    'func CredentialAbsenceGuard',
  ].join('\n'),
  testsText: [
    'TestPlatformDraftAdapterContractCapabilitiesAndReferences',
    'TestLocalDraftAdapterIdempotencyAndModeSafety',
    'TestDouyinDraftFixtureAdapterScenariosAndValidation',
    'TestUnsupportedPlatformGuardAndRegistryResolution',
    'TestAutomaticPublishGuardBlocksDangerousConfigAndPayloadBeforeAdapter',
    'TestPlatformDraftAdaptersSourceHasNoNetworkClientDependency',
    'TestPlatformDraftAdapterRegistryConcurrentCallsAreStable',
    'TestExecutionOrchestratorWithSafeRegistryWritesDraftAndRejectsGuardFailure',
    'ErrCodeIdemPayloadConflict',
  ].join('\n'),
  packageText: 'test:p8-task-batch-5\np8:task-batch-5-gate',
  docsText: 'draft_written != published\ndraft_written != listed',
};

function validEvidence(overrides = {}) {
  return {
    batchId: 'P8-TASK-BATCH-5',
    tasks: {
      'P8-301': { status: 'completed' },
      'P8-302': { status: 'completed' },
      'P8-303': { status: 'completed' },
      'P8-304': { status: 'completed' },
      'P8-305': { status: 'completed' },
    },
    draftExecutionPortReused: true,
    parallelAdapterInterfaceAbsent: true,
    platformDraftAdapterRegistryPresent: true,
    draftAdapterCapabilitiesPresent: true,
    localDraftAdapterPresent: true,
    douyinFixtureAdapterPresent: true,
    unsupportedPlatformGuardPresent: true,
    automaticPublishGuardPresent: true,
    credentialAbsenceGuardPresent: true,
    draftCreationCapability: true,
    publishCapability: false,
    listingCapability: false,
    networkAccessCapability: false,
    realCredentialsCapability: false,
    automaticExecutionCapability: false,
    adapterContractTestsPassed: true,
    localAdapterTestsPassed: true,
    douyinMockSandboxTestsPassed: true,
    unsupportedPlatformGuardTestsPassed: true,
    automaticPublishGuardTestsPassed: true,
    networkIsolationTestsPassed: true,
    idempotencyTestsPassed: true,
    concurrencyTestsPassed: true,
    orchestratorIntegrationTestsPassed: true,
    racePassed: true,
    dataRaces: 0,
    draftWrittenNotPublished: true,
    realDouyinApiImplemented: false,
    oauthImplemented: false,
    networkAccessEnabled: false,
    realCredentialsEnabled: false,
    realPlatformWriteImplemented: false,
    automaticPublishImplemented: false,
    automaticListingImplemented: false,
    apiImplemented: false,
    adminUiImplemented: false,
    backgroundWorkerImplemented: false,
    productionPlatformAdapterImplemented: false,
    p7DeferredPerformancePreserved: true,
    p10ProductionBoundaryPreserved: true,
    productionReady: false,
    ...overrides,
  };
}

function assertFails(id, overrides = {}, sourceOverrides = {}) {
  const result = validateP8TaskBatch5Bundle({
    evidence: validEvidence(overrides),
    sources: { ...validSources, ...sourceOverrides },
  });
  assert.equal(result.status, 'failed', id);
  assert.ok(result.failed.includes(id), `${id} should fail, saw ${result.failed.join(', ')}`);
}

assert.equal(validateP8TaskBatch5Bundle({ evidence: validEvidence(), sources: validSources }).status, 'passed');
assertFails('P8-301 status', { tasks: { ...validEvidence().tasks, 'P8-301': { status: 'pending' } } });
assertFails('draftExecutionPortReused', { draftExecutionPortReused: false });
assertFails('parallelAdapterInterfaceAbsent', { parallelAdapterInterfaceAbsent: false }, { adapterText: `${validSources.adapterText}\ntype PlatformDraftAdapter interface {}` });
assertFails('platformDraftAdapterRegistryPresent', { platformDraftAdapterRegistryPresent: false });
assertFails('localDraftAdapterPresent', { localDraftAdapterPresent: false });
assertFails('douyinFixtureAdapterPresent', { douyinFixtureAdapterPresent: false });
assertFails('unsupportedPlatformGuardPresent', { unsupportedPlatformGuardPresent: false });
assertFails('automaticPublishGuardPresent', { automaticPublishGuardPresent: false });
assertFails('credentialAbsenceGuardPresent', { credentialAbsenceGuardPresent: false });
assertFails('capabilitiesSafe', { publishCapability: true });
assertFails('capabilitiesSafe', { listingCapability: true });
assertFails('capabilitiesSafe', { networkAccessCapability: true });
assertFails('capabilitiesSafe', { realCredentialsCapability: true });
assertFails('networkIsolationTestsPassed', { networkIsolationTestsPassed: false });
assertFails('idempotencyTestsPassed', { idempotencyTestsPassed: false });
assertFails('concurrencyTestsPassed', { concurrencyTestsPassed: false });
assertFails('orchestratorIntegrationTestsPassed', { orchestratorIntegrationTestsPassed: false });
assertFails('racePassed', { racePassed: false });
assertFails('draftWrittenNotPublished', { draftWrittenNotPublished: false });
assertFails('realDouyinApiImplemented', { realDouyinApiImplemented: true });
assertFails('oauthImplemented', { oauthImplemented: true });
assertFails('networkAccessEnabled', { networkAccessEnabled: true });
assertFails('realCredentialsEnabled', { realCredentialsEnabled: true });
assertFails('realPlatformWriteImplemented', { realPlatformWriteImplemented: true });
assertFails('automaticPublishImplemented', { automaticPublishImplemented: true });
assertFails('automaticListingImplemented', { automaticListingImplemented: true });
assertFails('apiImplemented', { apiImplemented: true });
assertFails('adminUiImplemented', { adminUiImplemented: true });
assertFails('backgroundWorkerImplemented', { backgroundWorkerImplemented: true });
assertFails('productionReady', { productionReady: true });

const report = {
  phase: 'P8',
  batchId: 'P8-TASK-BATCH-5',
  status: 'passed',
  fixtures: 34,
};
writeJSON('docs/p8-task-batch-5-fixture-report.json', report);
console.log(JSON.stringify(report, null, 2));
