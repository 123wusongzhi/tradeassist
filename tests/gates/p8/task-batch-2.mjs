import assert from 'node:assert/strict';
import { writeJSON } from '../../../scripts/p7-v2-lib.mjs';
import { validateP8TaskBatch2Bundle } from '../../../scripts/p8-task-batch-2-final-gate.mjs';

const validSources = {
  modelText: [
    'type ApprovalRecord struct {}',
    'type ExecutionAttempt struct {}',
    'type ExecutionError struct {}',
    'type OperationTaskEvent struct {}',
    'ApprovalRecord) BeforeUpdate',
    'ExecutionError) BeforeUpdate',
    'OperationTaskEvent) BeforeUpdate',
    'DraftVersion',
    'DraftPayloadHash',
    'ApprovalRecordID',
  ].join('\n'),
  repoText: [
    'tenant_id = ? AND id = ?',
    'tenant_id = ? AND operation_task_id = ?',
    'draft.DraftVersion != draftVersion',
    'draft.PayloadHash != payloadHash',
    'approval.OperationTaskID != attempt.OperationTaskID',
  ].join('\n'),
  migrateText: [
    'trg_approval_records_no_update',
    'trg_execution_errors_no_update',
    'trg_operation_task_events_no_update',
    'ux_approval_records_task_idempotency',
    'ux_execution_attempts_task_idempotency',
    'ux_execution_attempts_task_attempt',
    'ux_execution_errors_attempt_sequence',
    'ux_operation_task_events_task_sequence',
  ].join('\n'),
  validationText: 'AdapterModeLocalDraftOnly\nExecutionErrorCategoryProviderTimeout\nOperationTaskEventActorUser',
  testText: [
    'operationtask.Migrate(db)',
    'TestApprovalRecordRepositoryValidationIdempotencyTenantAndImmutable',
    'TestBatch2ConcurrentConstraints',
  ].join('\n'),
  docsText: 'fullSecretRedactionDeferredTo=P8-404',
};

function validEvidence(overrides = {}) {
  return {
    batchId: 'P8-TASK-BATCH-2',
    p8PlanCheckpoint: 'ea356d8077722e2f94c6215fe10c7d4f6e53fde5',
    baseCheckpoint: '73e2ea3ec0b749d607da0e919ad71b29cef73c3d',
    tasks: {
      'P8-103': { status: 'completed' },
      'P8-104': { status: 'completed' },
      'P8-105': { status: 'completed' },
    },
    approvalRecordModelPresent: true,
    executionAttemptModelPresent: true,
    executionErrorModelPresent: true,
    operationTaskEventModelPresent: true,
    approvalImmutable: true,
    executionErrorImmutable: true,
    taskEventImmutable: true,
    tenantIsolationPassed: true,
    approvalIdempotencyPassed: true,
    executionIdempotencyPassed: true,
    attemptNumberConcurrencyPassed: true,
    errorSequenceConcurrencyPassed: true,
    eventSequenceConcurrencyPassed: true,
    approvalDraftVersionBindingPresent: true,
    approvalDraftHashBindingPresent: true,
    executionApprovalReferencePresent: true,
    repositoryTestsPassed: true,
    migrationTestsPassed: true,
    concurrencyTestsPassed: true,
    racePassed: true,
    currentBatchRaceStatus: 'passed',
    repositoryMethods: [
      'ApprovalRecord.CreateDecision',
      'ExecutionAttempt.CreateAttempt',
      'ExecutionAttempt.UpdateLifecycle',
      'ExecutionError.AppendError',
      'OperationTaskEvent.AppendEvent',
      'OperationTaskEvent.ListByTask',
    ],
    stateMachineServiceImplemented: false,
    taskStateMachineImplemented: false,
    approvalServiceImplemented: false,
    executionOrchestratorImplemented: false,
    retryServiceImplemented: false,
    apiImplemented: false,
    adminUiImplemented: false,
    platformWriteImplemented: false,
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
  const result = validateP8TaskBatch2Bundle({
    evidence: validEvidence(overrides),
    sources: { ...validSources, ...sourceOverrides },
  });
  assert.equal(result.status, 'failed', id);
  assert.ok(result.failed.includes(id), `${id} should fail, saw ${result.failed.join(', ')}`);
}

assert.equal(validateP8TaskBatch2Bundle({ evidence: validEvidence(), sources: validSources }).status, 'passed');
assertFails('approvalRecordModelPresent', { approvalRecordModelPresent: false });
assertFails('operationTaskEventAppendOnly', { taskEventImmutable: false });
assertFails('approvalDraftVersionBindingPresent', {}, { repoText: validSources.repoText.replace('draft.DraftVersion != draftVersion', '') });
assertFails('executionApprovalReferencePresent', {}, { repoText: validSources.repoText.replace('approval.OperationTaskID != attempt.OperationTaskID', '') });
assertFails('attemptNumberConstraintPresent', { attemptNumberConcurrencyPassed: false });
assertFails('taskEventSequenceConstraintPresent', { eventSequenceConcurrencyPassed: false });
assertFails('automaticPublishEnabled', { automaticPublishEnabled: true });
assertFails('retryServiceImplemented', { retryServiceImplemented: true });
assertFails('platformWriteImplemented', { platformWriteImplemented: true });
assertFails('migrationTestsPassed', { migrationTestsPassed: false });
assertFails('concurrencyTestsPassed', { concurrencyTestsPassed: false });
assertFails('racePassed', { racePassed: false });
assertFails('p7DeferredPerformancePreserved', { p7DeferredPerformancePreserved: false });
assertFails('p10ProductionBoundaryPreserved', { p10ProductionBoundaryPreserved: false });
assertFails('productionReady', { productionReady: true });

const report = {
  phase: 'P8',
  batchId: 'P8-TASK-BATCH-2',
  status: 'passed',
  fixtures: 15,
};
writeJSON('docs/p8-task-batch-2-fixture-report.json', report);
console.log(JSON.stringify(report, null, 2));
