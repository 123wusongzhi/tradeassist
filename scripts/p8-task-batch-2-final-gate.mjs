import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

export const P8_TASK_BATCH_2_EVIDENCE_JSON = 'docs/p8-task-batch-2-approval-execution-audit-persistence.json';
export const P8_TASK_BATCH_2_GATE_JSON = 'docs/p8-task-batch-2-final-gate.json';
export const P8_TASK_BATCH_2_GATE_MD = 'docs/P8_TASK_BATCH_2_FINAL_GATE.md';
export const P8_TASK_BATCH_2_PLAN_CHECKPOINT = 'ea356d8077722e2f94c6215fe10c7d4f6e53fde5';
export const P8_TASK_BATCH_1_CHECKPOINT = '73e2ea3ec0b749d607da0e919ad71b29cef73c3d';

const requiredFiles = [
  'backend/internal/modules/operationtask/model.go',
  'backend/internal/modules/operationtask/repository.go',
  'backend/internal/modules/operationtask/migrate.go',
  'backend/internal/modules/operationtask/validation.go',
  'backend/internal/modules/operationtask/repository_test.go',
  'docs/P8_TASK_BATCH_2_APPROVAL_EXECUTION_AUDIT_PERSISTENCE.md',
  'docs/p8-task-batch-2-approval-execution-audit-persistence.json',
];

function rootPath(rel) {
  return path.join(process.cwd(), rel);
}

function text(rel) {
  try {
    return fs.readFileSync(rootPath(rel), 'utf8');
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

export function validateP8TaskBatch2Bundle({ evidence = {}, sources = {} } = {}) {
  const modelText = sources.modelText ?? text('backend/internal/modules/operationtask/model.go');
  const repoText = sources.repoText ?? text('backend/internal/modules/operationtask/repository.go');
  const migrateText = sources.migrateText ?? text('backend/internal/modules/operationtask/migrate.go');
  const validationText = sources.validationText ?? text('backend/internal/modules/operationtask/validation.go');
  const testText = sources.testText ?? text('backend/internal/modules/operationtask/repository_test.go');
  const docsText = sources.docsText ?? text('docs/P8_TASK_BATCH_2_APPROVAL_EXECUTION_AUDIT_PERSISTENCE.md');
  const combined = `${modelText}\n${repoText}\n${migrateText}\n${validationText}\n${testText}\n${docsText}`;

  const checks = [
    ['p8PlanCheckpoint', evidence.p8PlanCheckpoint === P8_TASK_BATCH_2_PLAN_CHECKPOINT],
    ['p8TaskBatch1Checkpoint', evidence.baseCheckpoint === P8_TASK_BATCH_1_CHECKPOINT],
    ['P8-103 status', evidence.tasks?.['P8-103']?.status === 'completed'],
    ['P8-104 status', evidence.tasks?.['P8-104']?.status === 'completed'],
    ['P8-105 status', evidence.tasks?.['P8-105']?.status === 'completed'],
    ['approvalRecordModelPresent', evidence.approvalRecordModelPresent === true && modelText.includes('type ApprovalRecord struct')],
    ['executionAttemptModelPresent', evidence.executionAttemptModelPresent === true && modelText.includes('type ExecutionAttempt struct')],
    ['executionErrorModelPresent', evidence.executionErrorModelPresent === true && modelText.includes('type ExecutionError struct')],
    ['operationTaskEventModelPresent', evidence.operationTaskEventModelPresent === true && modelText.includes('type OperationTaskEvent struct')],
    ['approvalRecordAppendOnly', evidence.approvalImmutable === true && hasAll(modelText + migrateText, ['ApprovalRecord) BeforeUpdate', 'trg_approval_records_no_update']) && !repoText.includes('UpdateApproval')],
    ['executionErrorAppendOnly', evidence.executionErrorImmutable === true && hasAll(modelText + migrateText, ['ExecutionError) BeforeUpdate', 'trg_execution_errors_no_update']) && !repoText.includes('RetryAttempt')],
    ['operationTaskEventAppendOnly', evidence.taskEventImmutable === true && hasAll(modelText + migrateText, ['OperationTaskEvent) BeforeUpdate', 'trg_operation_task_events_no_update']) && !repoText.includes('RewriteHistory')],
    ['tenantIsolationImplemented', evidence.tenantIsolationPassed === true && hasAll(repoText, ['tenant_id = ? AND id = ?', 'tenant_id = ? AND operation_task_id = ?'])],
    ['approvalIdempotencyConstraintPresent', evidence.approvalIdempotencyPassed === true && migrateText.includes('ux_approval_records_task_idempotency')],
    ['executionIdempotencyConstraintPresent', evidence.executionIdempotencyPassed === true && migrateText.includes('ux_execution_attempts_task_idempotency')],
    ['attemptNumberConstraintPresent', evidence.attemptNumberConcurrencyPassed === true && migrateText.includes('ux_execution_attempts_task_attempt')],
    ['executionErrorSequenceConstraintPresent', evidence.errorSequenceConcurrencyPassed === true && migrateText.includes('ux_execution_errors_attempt_sequence')],
    ['taskEventSequenceConstraintPresent', evidence.eventSequenceConcurrencyPassed === true && migrateText.includes('ux_operation_task_events_task_sequence')],
    ['approvalDraftVersionBindingPresent', hasAll(modelText + repoText, ['DraftVersion', 'draft.DraftVersion != draftVersion'])],
    ['approvalDraftHashBindingPresent', hasAll(modelText + repoText, ['DraftPayloadHash', 'draft.PayloadHash != payloadHash'])],
    ['executionApprovalReferencePresent', hasAll(modelText + repoText, ['ApprovalRecordID', 'approval.OperationTaskID != attempt.OperationTaskID'])],
    ['repositoryTestsPassed', evidence.repositoryTestsPassed === true && testText.includes('TestApprovalRecordRepositoryValidationIdempotencyTenantAndImmutable')],
    ['migrationTestsPassed', evidence.migrationTestsPassed === true && testText.includes('operationtask.Migrate(db)')],
    ['concurrencyTestsPassed', evidence.concurrencyTestsPassed === true && testText.includes('TestBatch2ConcurrentConstraints')],
    ['racePassed', evidence.racePassed === true && evidence.currentBatchRaceStatus === 'passed'],
    ['requiredRepositoryMethods', arrayIncludes(evidence.repositoryMethods, [
      'ApprovalRecord.CreateDecision',
      'ExecutionAttempt.CreateAttempt',
      'ExecutionAttempt.UpdateLifecycle',
      'ExecutionError.AppendError',
      'OperationTaskEvent.AppendEvent',
      'OperationTaskEvent.ListByTask',
    ])],
    ['stateMachineServiceImplemented', evidence.stateMachineServiceImplemented === false && evidence.taskStateMachineImplemented === false && !combined.includes('type TaskStateMachine')],
    ['approvalServiceImplemented', evidence.approvalServiceImplemented === false && !combined.includes('type ApprovalService')],
    ['executionOrchestratorImplemented', evidence.executionOrchestratorImplemented === false && !combined.includes('type ExecutionOrchestrator')],
    ['retryServiceImplemented', evidence.retryServiceImplemented === false && !combined.includes('AutomaticBackoff')],
    ['apiImplemented', evidence.apiImplemented === false && !combined.includes('/operation-tasks')],
    ['adminUiImplemented', evidence.adminUiImplemented === false],
    ['platformWriteImplemented', evidence.platformWriteImplemented === false && !combined.includes('WriteToPlatform')],
    ['realCredentialsEnabled', evidence.realCredentialsEnabled === false],
    ['realPlatformWriteEnabled', evidence.realPlatformWriteEnabled === false],
    ['automaticPublishEnabled', evidence.automaticPublishEnabled === false],
    ['automaticListingEnabled', evidence.automaticListingEnabled === false],
    ['p7DeferredPerformancePreserved', evidence.p7DeferredPerformancePreserved === true],
    ['p10ProductionBoundaryPreserved', evidence.p10ProductionBoundaryPreserved === true],
    ['productionReady', evidence.productionReady === false],
  ];

  const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failed.length ? 'failed' : 'passed',
    failed,
    failedCount: failed.length,
    checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  };
}

export function buildP8TaskBatch2GateReport(bundle = {}) {
  const evidence = bundle.evidence ?? readJSON(P8_TASK_BATCH_2_EVIDENCE_JSON) ?? {};
  const validation = validateP8TaskBatch2Bundle({ evidence, sources: bundle.sources });
  const missingFiles = requiredFiles.filter((rel) => !fs.existsSync(rootPath(rel)));
  const failed = [...validation.failed, ...missingFiles.map((rel) => `missing:${rel}`)];
  return {
    phase: 'P8',
    gate: 'P8-TASK-BATCH-2',
    status: failed.length ? 'failed' : 'passed',
    checkedAt: '2026-07-20T00:00:00.000Z',
    batchId: 'P8-TASK-BATCH-2',
    p8PlanCheckpoint: P8_TASK_BATCH_2_PLAN_CHECKPOINT,
    p8TaskBatch1Checkpoint: P8_TASK_BATCH_1_CHECKPOINT,
    tasks: ['P8-103', 'P8-104', 'P8-105'],
    approvalRecordModelPresent: evidence.approvalRecordModelPresent === true,
    executionAttemptModelPresent: evidence.executionAttemptModelPresent === true,
    executionErrorModelPresent: evidence.executionErrorModelPresent === true,
    operationTaskEventModelPresent: evidence.operationTaskEventModelPresent === true,
    approvalRecordAppendOnly: evidence.approvalImmutable === true,
    executionErrorAppendOnly: evidence.executionErrorImmutable === true,
    operationTaskEventAppendOnly: evidence.taskEventImmutable === true,
    tenantIsolationImplemented: evidence.tenantIsolationPassed === true,
    approvalIdempotencyConstraintPresent: evidence.approvalIdempotencyPassed === true,
    executionIdempotencyConstraintPresent: evidence.executionIdempotencyPassed === true,
    attemptNumberConstraintPresent: evidence.attemptNumberConcurrencyPassed === true,
    executionErrorSequenceConstraintPresent: evidence.errorSequenceConcurrencyPassed === true,
    taskEventSequenceConstraintPresent: evidence.eventSequenceConcurrencyPassed === true,
    approvalDraftVersionBindingPresent: evidence.approvalDraftVersionBindingPresent === true,
    approvalDraftHashBindingPresent: evidence.approvalDraftHashBindingPresent === true,
    executionApprovalReferencePresent: evidence.executionApprovalReferencePresent === true,
    repositoryTestsPassed: evidence.repositoryTestsPassed === true,
    migrationTestsPassed: evidence.migrationTestsPassed === true,
    concurrencyTestsPassed: evidence.concurrencyTestsPassed === true,
    racePassed: evidence.racePassed === true,
    stateMachineServiceImplemented: evidence.stateMachineServiceImplemented === true,
    approvalServiceImplemented: evidence.approvalServiceImplemented === true,
    executionOrchestratorImplemented: evidence.executionOrchestratorImplemented === true,
    retryServiceImplemented: evidence.retryServiceImplemented === true,
    apiImplemented: evidence.apiImplemented === true,
    adminUiImplemented: evidence.adminUiImplemented === true,
    platformWriteImplemented: evidence.platformWriteImplemented === true,
    realCredentialsEnabled: evidence.realCredentialsEnabled === true,
    realPlatformWriteEnabled: evidence.realPlatformWriteEnabled === true,
    automaticPublishEnabled: evidence.automaticPublishEnabled === true,
    automaticListingEnabled: evidence.automaticListingEnabled === true,
    p7DeferredPerformancePreserved: evidence.p7DeferredPerformancePreserved === true,
    p10ProductionBoundaryPreserved: evidence.p10ProductionBoundaryPreserved === true,
    productionReady: evidence.productionReady === true,
    failedCount: failed.length,
    failed,
    checks: validation.checks,
  };
}

export function writeP8TaskBatch2GateReport(report) {
  writeJSON(P8_TASK_BATCH_2_GATE_JSON, report);
  writeMarkdown(
    P8_TASK_BATCH_2_GATE_MD,
    `# P8 Task Batch 2 Final Gate

Status: **${report.status}**

- Batch: ${report.batchId}
- P8 plan checkpoint: ${report.p8PlanCheckpoint}
- P8 task batch 1 checkpoint: ${report.p8TaskBatch1Checkpoint}
- Tasks: ${report.tasks.join(', ')}
- Approval record model present: ${report.approvalRecordModelPresent}
- Execution attempt model present: ${report.executionAttemptModelPresent}
- Execution error model present: ${report.executionErrorModelPresent}
- Operation task event model present: ${report.operationTaskEventModelPresent}
- Approval record append-only: ${report.approvalRecordAppendOnly}
- Execution error append-only: ${report.executionErrorAppendOnly}
- Operation task event append-only: ${report.operationTaskEventAppendOnly}
- Tenant isolation implemented: ${report.tenantIsolationImplemented}
- Approval idempotency constraint present: ${report.approvalIdempotencyConstraintPresent}
- Execution idempotency constraint present: ${report.executionIdempotencyConstraintPresent}
- Attempt number constraint present: ${report.attemptNumberConstraintPresent}
- Execution error sequence constraint present: ${report.executionErrorSequenceConstraintPresent}
- Task event sequence constraint present: ${report.taskEventSequenceConstraintPresent}
- Repository tests passed: ${report.repositoryTestsPassed}
- Migration tests passed: ${report.migrationTestsPassed}
- Concurrency tests passed: ${report.concurrencyTestsPassed}
- Race passed: ${report.racePassed}
- State machine service implemented: ${report.stateMachineServiceImplemented}
- Approval service implemented: ${report.approvalServiceImplemented}
- Execution orchestrator implemented: ${report.executionOrchestratorImplemented}
- Retry service implemented: ${report.retryServiceImplemented}
- API implemented: ${report.apiImplemented}
- Admin UI implemented: ${report.adminUiImplemented}
- Platform write implemented: ${report.platformWriteImplemented}
- Real credentials enabled: ${report.realCredentialsEnabled}
- Real platform write enabled: ${report.realPlatformWriteEnabled}
- Automatic publish enabled: ${report.automaticPublishEnabled}
- Automatic listing enabled: ${report.automaticListingEnabled}
- Production Ready: ${report.productionReady}
- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}

This gate validates only P8 Batch 2 approval, execution history, error, and task-event persistence. It does not authorize state-machine services, approval services, execution orchestration, API, Admin UI, real credentials, real platform writes, automatic publish, automatic listing, production tag, production release, or Production Ready.
`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildP8TaskBatch2GateReport();
  writeP8TaskBatch2GateReport(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}
