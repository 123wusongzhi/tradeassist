import assert from 'node:assert/strict';
import { writeJSON } from '../../../scripts/p7-v2-lib.mjs';
import { validateP8TaskBatch3Bundle } from '../../../scripts/p8-task-batch-3-final-gate.mjs';

const validSources = {
  stateMachineText: [
    'type TaskStateMachine struct {}',
    'ValidateTransition',
    'ErrInvalidTransition',
  ].join('\n'),
  canonicalText: [
    'CanonicalJSONHashVersion = 1',
    'sha256.Sum256',
    'sort.Strings(keys)',
  ].join('\n'),
  servicesText: [
    'type TaskTransitionService struct {}',
    'func (s *TaskTransitionService) Transition',
    'type DraftVersionService struct {}',
    'CreateInitialDraft',
    'CreateNextVersion',
    'ComputePayloadHash',
    'ComputePayloadHash([]byte(in.Payload))',
    'type ApprovalService struct {}',
    'Approve',
    'Reject',
    'findLatestDraftTx',
    'latest.DraftVersion != in.DraftVersion',
    'latest.PayloadHash != in.DraftPayloadHash',
    'Authorizer == nil',
    'tx.Create(&out).Error',
    'func (s *DraftVersionService) createDraftVersion',
  ].join('\n'),
  modelText: 'draft_preparing\npending_review\nexecution_queued\ndraft_written\nexecution_failed',
  testsText: [
    'TestTaskStateMachineCanonicalTransitions',
    'RejectsUndeclaredAndSameStatusTransitions',
    'AppendOnly',
    'ApprovedEditRequiresReapproval',
    'RollbackWhenEventAppendFails',
    'IdempotencyAndCrossTenant',
    'ConcurrentRevisionConflict',
    'Concurrency',
  ].join('\n'),
  packageText: 'test:p8-task-batch-3\np8:task-batch-3-gate',
  docsText: 'rbacConcreteIntegrationDeferredTo=P8-401',
};

function validEvidence(overrides = {}) {
  return {
    batchId: 'P8-TASK-BATCH-3',
    tasks: {
      'P8-201': { status: 'completed' },
      'P8-202': { status: 'completed' },
      'P8-203': { status: 'completed' },
    },
    taskStateMachinePresent: true,
    taskTransitionServicePresent: true,
    draftVersionServicePresent: true,
    approvalServicePresent: true,
    stateTransitionMatrixTested: true,
    invalidTransitionsRejected: true,
    canonicalJsonHashVersion: 1,
    draftPayloadHashComputedByService: true,
    draftVersionsAppendOnly: true,
    approvedDraftEditRequiresReapproval: true,
    approvalLatestDraftBindingPresent: true,
    approvalDraftVersionBindingPresent: true,
    approvalDraftHashBindingPresent: true,
    approvalAuthorizerRequired: true,
    approvalDefaultAllow: false,
    taskEventWrittenAtomically: true,
    approvalWrittenAtomically: true,
    draftVersionWrittenAtomically: true,
    idempotencyTestsPassed: true,
    concurrencyTestsPassed: true,
    rollbackTestsPassed: true,
    racePassed: true,
    dataRaces: 0,
    executionOrchestratorImplemented: false,
    retryServiceImplemented: false,
    apiImplemented: false,
    adminUiImplemented: false,
    platformAdapterImplemented: false,
    platformWriteImplemented: false,
    realCredentialsEnabled: false,
    realPlatformWriteEnabled: false,
    automaticPublishEnabled: false,
    automaticListingEnabled: false,
    humanConfirmationRequired: true,
    p7DeferredPerformancePreserved: true,
    p10ProductionBoundaryPreserved: true,
    productionReady: false,
    ...overrides,
  };
}

function assertFails(id, overrides = {}, sourceOverrides = {}) {
  const result = validateP8TaskBatch3Bundle({
    evidence: validEvidence(overrides),
    sources: { ...validSources, ...sourceOverrides },
  });
  assert.equal(result.status, 'failed', id);
  assert.ok(result.failed.includes(id), `${id} should fail, saw ${result.failed.join(', ')}`);
}

assert.equal(validateP8TaskBatch3Bundle({ evidence: validEvidence(), sources: validSources }).status, 'passed');
assertFails('P8-201 status', { tasks: { ...validEvidence().tasks, 'P8-201': { status: 'pending' } } });
assertFails('taskStateMachinePresent', { taskStateMachinePresent: false });
assertFails('taskTransitionServicePresent', { taskTransitionServicePresent: false });
assertFails('draftVersionServicePresent', { draftVersionServicePresent: false });
assertFails('approvalServicePresent', { approvalServicePresent: false });
assertFails('invalidTransitionsRejected', { invalidTransitionsRejected: false });
assertFails('canonicalJsonHashVersionPresent', { canonicalJsonHashVersion: 0 });
assertFails('draftPayloadHashComputedByService', {}, { servicesText: validSources.servicesText.replace('ComputePayloadHash([]byte(in.Payload))', '') });
assertFails('approvedDraftEditRequiresReapproval', { approvedDraftEditRequiresReapproval: false });
assertFails('approvalLatestDraftBindingPresent', {}, { servicesText: validSources.servicesText.replace('findLatestDraftTx', '') });
assertFails('approvalDraftHashBindingPresent', { approvalDraftHashBindingPresent: false });
assertFails('approvalAuthorizerRequired', { approvalAuthorizerRequired: false });
assertFails('approvalDefaultAllow', { approvalDefaultAllow: true });
assertFails('taskEventWrittenAtomically', { taskEventWrittenAtomically: false });
assertFails('idempotencyTestsPassed', { idempotencyTestsPassed: false });
assertFails('concurrencyTestsPassed', { concurrencyTestsPassed: false });
assertFails('rollbackTestsPassed', { rollbackTestsPassed: false });
assertFails('racePassed', { racePassed: false });
assertFails('executionOrchestratorImplemented', { executionOrchestratorImplemented: true });
assertFails('retryServiceImplemented', { retryServiceImplemented: true });
assertFails('apiImplemented', { apiImplemented: true });
assertFails('adminUiImplemented', { adminUiImplemented: true });
assertFails('platformWriteImplemented', { platformWriteImplemented: true });
assertFails('automaticPublishEnabled', { automaticPublishEnabled: true });
assertFails('productionReady', { productionReady: true });

const report = {
  phase: 'P8',
  batchId: 'P8-TASK-BATCH-3',
  status: 'passed',
  fixtures: 27,
};
writeJSON('docs/p8-task-batch-3-fixture-report.json', report);
console.log(JSON.stringify(report, null, 2));

