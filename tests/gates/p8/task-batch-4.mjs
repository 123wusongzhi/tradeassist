import assert from 'node:assert/strict';
import { writeJSON } from '../../../scripts/p7-v2-lib.mjs';
import { validateP8TaskBatch4Bundle } from '../../../scripts/p8-task-batch-4-final-gate.mjs';

const validSources = {
  executionText: [
    'type ExecutionOrchestrator struct {}',
    'func (s *ExecutionOrchestrator) Execute',
    'type ExecutionFailureClassifier struct {}',
    'func (c *ExecutionFailureClassifier) Classify',
    'type ManualRetryService struct {}',
    'func (s *ManualRetryService) Retry',
    'ExecutionIdempotencyStatusInProgress',
    'ErrIdemPayloadConflict',
    'findAttemptByIdempotencyTx',
    'Authorizer == nil',
    'findLatestDraftTx',
    'approval.DraftVersion != latestDraft.DraftVersion',
    'approval.DraftPayloadHash != latestDraft.PayloadHash',
    's.prepare',
    's.Port.ExecuteDraft',
    's.finalize',
    'func (s *ExecutionOrchestrator) prepare',
    'Transaction(func(tx *gorm.DB) error',
    'func (s *ExecutionOrchestrator) finalizeSuccess',
    'func (s *ExecutionOrchestrator) finalizeFailure',
    'hasActiveAttemptTx',
    'DefaultMaxManualRetryAttempts',
  ].join('\n'),
  modelText: 'ExecutionAttempt\nResultType\nSafeMetadata',
  repositoryText: 'ExecutionAttemptLifecyclePatch',
  testsText: [
    'DuplicateReplayInProgressAndPayloadConflict',
    'ErrIdemPayloadConflict',
    'BlocksNonRetryable',
    'AttemptNumber',
  ].join('\n'),
  packageText: 'test:p8-task-batch-4\np8:task-batch-4-gate',
  docsText: 'rbacConcreteIntegrationDeferredTo=P8-401/P8-402',
};

function validEvidence(overrides = {}) {
  return {
    batchId: 'P8-TASK-BATCH-4',
    tasks: {
      'P8-204': { status: 'completed' },
      'P8-205': { status: 'completed' },
      'P8-206': { status: 'completed' },
    },
    executionOrchestratorPresent: true,
    executionFailureClassifierPresent: true,
    manualRetryServicePresent: true,
    executionIdempotencyProtectionPresent: true,
    executionAuthorizerRequired: true,
    executionDefaultAllow: false,
    approvalLatestDraftBindingEnforced: true,
    approvalDraftVersionBindingEnforced: true,
    approvalDraftHashBindingEnforced: true,
    executionPortCalledOutsideTransaction: true,
    executionPrepareAtomic: true,
    executionSuccessFinalizeAtomic: true,
    executionFailureFinalizeAtomic: true,
    duplicateExecutionPrevented: true,
    concurrentExecutionPrevented: true,
    idempotencyPayloadConflictDetected: true,
    manualRetryOnly: true,
    automaticRetryWorkerPresent: false,
    retryLimitPresent: true,
    nonRetryableErrorsBlocked: true,
    executionAttemptAppendHistoryPreserved: true,
    executionErrorAppendOnly: true,
    operationTaskEventAppendOnly: true,
    repositoryTestsPassed: true,
    transactionTestsPassed: true,
    rollbackTestsPassed: true,
    idempotencyTestsPassed: true,
    concurrencyTestsPassed: true,
    racePassed: true,
    dataRaces: 0,
    apiImplemented: false,
    adminUiImplemented: false,
    productionPlatformAdapterImplemented: false,
    realPlatformWriteImplemented: false,
    automaticPublishImplemented: false,
    automaticListingImplemented: false,
    realCredentialsEnabled: false,
    realPlatformWriteEnabled: false,
    automaticPublishEnabled: false,
    automaticListingEnabled: false,
    p7DeferredPerformancePreserved: true,
    p10ProductionBoundaryPreserved: true,
    productionReady: false,
    ...overrides,
  };
}

function assertFails(id, overrides = {}, sourceOverrides = {}) {
  const result = validateP8TaskBatch4Bundle({
    evidence: validEvidence(overrides),
    sources: { ...validSources, ...sourceOverrides },
  });
  assert.equal(result.status, 'failed', id);
  assert.ok(result.failed.includes(id), `${id} should fail, saw ${result.failed.join(', ')}`);
}

assert.equal(validateP8TaskBatch4Bundle({ evidence: validEvidence(), sources: validSources }).status, 'passed');
assertFails('P8-204 status', { tasks: { ...validEvidence().tasks, 'P8-204': { status: 'pending' } } });
assertFails('executionOrchestratorPresent', { executionOrchestratorPresent: false });
assertFails('executionFailureClassifierPresent', { executionFailureClassifierPresent: false });
assertFails('manualRetryServicePresent', { manualRetryServicePresent: false });
assertFails('executionIdempotencyProtectionPresent', { executionIdempotencyProtectionPresent: false });
assertFails('executionAuthorizerRequired', { executionAuthorizerRequired: false });
assertFails('executionDefaultAllow', { executionDefaultAllow: true });
assertFails('approvalDraftVersionBindingEnforced', { approvalDraftVersionBindingEnforced: false });
assertFails('executionPortCalledOutsideTransaction', { executionPortCalledOutsideTransaction: false });
assertFails('duplicateExecutionPrevented', { duplicateExecutionPrevented: false });
assertFails('concurrentExecutionPrevented', { concurrentExecutionPrevented: false });
assertFails('idempotencyPayloadConflictDetected', { idempotencyPayloadConflictDetected: false });
assertFails('manualRetryOnly', { manualRetryOnly: false });
assertFails('automaticRetryWorkerPresent', { automaticRetryWorkerPresent: true });
assertFails('retryLimitPresent', { retryLimitPresent: false });
assertFails('nonRetryableErrorsBlocked', { nonRetryableErrorsBlocked: false });
assertFails('racePassed', { racePassed: false });
assertFails('apiImplemented', { apiImplemented: true });
assertFails('adminUiImplemented', { adminUiImplemented: true });
assertFails('productionPlatformAdapterImplemented', { productionPlatformAdapterImplemented: true });
assertFails('realPlatformWriteImplemented', { realPlatformWriteImplemented: true });
assertFails('automaticPublishImplemented', { automaticPublishImplemented: true });
assertFails('automaticListingImplemented', { automaticListingImplemented: true });
assertFails('productionReady', { productionReady: true });

const report = {
  phase: 'P8',
  batchId: 'P8-TASK-BATCH-4',
  status: 'passed',
  fixtures: 25,
};
writeJSON('docs/p8-task-batch-4-fixture-report.json', report);
console.log(JSON.stringify(report, null, 2));
