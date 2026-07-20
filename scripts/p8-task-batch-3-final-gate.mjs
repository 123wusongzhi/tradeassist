import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

export const P8_TASK_BATCH_3_EVIDENCE_JSON = 'docs/p8-task-batch-3-state-draft-approval-services.json';
export const P8_TASK_BATCH_3_GATE_JSON = 'docs/p8-task-batch-3-final-gate.json';
export const P8_TASK_BATCH_3_GATE_MD = 'docs/P8_TASK_BATCH_3_FINAL_GATE.md';

const requiredFiles = [
  'backend/internal/modules/operationtask/state_machine.go',
  'backend/internal/modules/operationtask/canonical_json.go',
  'backend/internal/modules/operationtask/services.go',
  'backend/internal/modules/operationtask/state_machine_test.go',
  'backend/internal/modules/operationtask/canonical_json_test.go',
  'backend/internal/modules/operationtask/services_test.go',
  'docs/P8_TASK_BATCH_3_STATE_DRAFT_APPROVAL_SERVICES.md',
  'docs/p8-task-batch-3-state-draft-approval-services.json',
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

export function validateP8TaskBatch3Bundle({ evidence = {}, sources = {} } = {}) {
  const stateMachineText = sources.stateMachineText ?? text('backend/internal/modules/operationtask/state_machine.go');
  const canonicalText = sources.canonicalText ?? text('backend/internal/modules/operationtask/canonical_json.go');
  const servicesText = sources.servicesText ?? text('backend/internal/modules/operationtask/services.go');
  const modelText = sources.modelText ?? text('backend/internal/modules/operationtask/model.go');
  const testsText = sources.testsText ?? [
    text('backend/internal/modules/operationtask/state_machine_test.go'),
    text('backend/internal/modules/operationtask/canonical_json_test.go'),
    text('backend/internal/modules/operationtask/services_test.go'),
  ].join('\n');
  const packageText = sources.packageText ?? text('package.json');
  const docsText = sources.docsText ?? text('docs/P8_TASK_BATCH_3_STATE_DRAFT_APPROVAL_SERVICES.md');
  const combined = `${stateMachineText}\n${canonicalText}\n${servicesText}\n${modelText}\n${testsText}\n${docsText}`;

  const checks = [
    ['batchId', evidence.batchId === 'P8-TASK-BATCH-3'],
    ['P8-201 status', evidence.tasks?.['P8-201']?.status === 'completed'],
    ['P8-202 status', evidence.tasks?.['P8-202']?.status === 'completed'],
    ['P8-203 status', evidence.tasks?.['P8-203']?.status === 'completed'],
    ['taskStateMachinePresent', evidence.taskStateMachinePresent === true && hasAll(stateMachineText, ['type TaskStateMachine struct', 'ValidateTransition', 'ErrInvalidTransition'])],
    ['taskTransitionServicePresent', evidence.taskTransitionServicePresent === true && hasAll(servicesText, ['type TaskTransitionService struct', 'func (s *TaskTransitionService) Transition'])],
    ['draftVersionServicePresent', evidence.draftVersionServicePresent === true && hasAll(servicesText, ['type DraftVersionService struct', 'CreateInitialDraft', 'CreateNextVersion', 'ComputePayloadHash'])],
    ['approvalServicePresent', evidence.approvalServicePresent === true && hasAll(servicesText, ['type ApprovalService struct', 'Approve', 'Reject'])],
    ['stateTransitionMatrixTested', evidence.stateTransitionMatrixTested === true && testsText.includes('TestTaskStateMachineCanonicalTransitions')],
    ['invalidTransitionsRejected', evidence.invalidTransitionsRejected === true && testsText.includes('RejectsUndeclaredAndSameStatusTransitions')],
    ['canonicalJsonHashVersionPresent', evidence.canonicalJsonHashVersion === 1 && canonicalText.includes('CanonicalJSONHashVersion = 1')],
    ['draftPayloadHashComputedByService', evidence.draftPayloadHashComputedByService === true && hasAll(servicesText, ['ComputePayloadHash([]byte(in.Payload))'])],
    ['draftVersionsAppendOnly', evidence.draftVersionsAppendOnly === true && testsText.includes('AppendOnly') && !servicesText.includes('Delete(&PlatformDraft')],
    ['approvedDraftEditRequiresReapproval', evidence.approvedDraftEditRequiresReapproval === true && testsText.includes('ApprovedEditRequiresReapproval')],
    ['approvalLatestDraftBindingPresent', evidence.approvalLatestDraftBindingPresent === true && servicesText.includes('findLatestDraftTx')],
    ['approvalDraftVersionBindingPresent', evidence.approvalDraftVersionBindingPresent === true && servicesText.includes('latest.DraftVersion != in.DraftVersion')],
    ['approvalDraftHashBindingPresent', evidence.approvalDraftHashBindingPresent === true && servicesText.includes('latest.PayloadHash != in.DraftPayloadHash')],
    ['approvalAuthorizerRequired', evidence.approvalAuthorizerRequired === true && servicesText.includes('Authorizer == nil')],
    ['approvalDefaultAllow', evidence.approvalDefaultAllow === false],
    ['taskEventWrittenAtomically', evidence.taskEventWrittenAtomically === true && testsText.includes('RollbackWhenEventAppendFails')],
    ['approvalWrittenAtomically', evidence.approvalWrittenAtomically === true && servicesText.includes('tx.Create(&out).Error')],
    ['draftVersionWrittenAtomically', evidence.draftVersionWrittenAtomically === true && servicesText.includes('func (s *DraftVersionService) createDraftVersion')],
    ['idempotencyTestsPassed', evidence.idempotencyTestsPassed === true && testsText.includes('IdempotencyAndCrossTenant')],
    ['concurrencyTestsPassed', evidence.concurrencyTestsPassed === true && testsText.includes('ConcurrentRevisionConflict') && testsText.includes('Concurrency')],
    ['rollbackTestsPassed', evidence.rollbackTestsPassed === true && testsText.includes('RollbackWhenEventAppendFails')],
    ['racePassed', evidence.racePassed === true && evidence.dataRaces === 0],
    ['taskStatusValues', hasAll(modelText, ['draft_preparing', 'pending_review', 'execution_queued', 'draft_written', 'execution_failed'])],
    ['executionOrchestratorImplemented', evidence.executionOrchestratorImplemented === false && !combined.includes('type ExecutionOrchestrator')],
    ['retryServiceImplemented', evidence.retryServiceImplemented === false && !combined.includes('type RetryService')],
    ['apiImplemented', evidence.apiImplemented === false && !combined.includes('/operation-tasks')],
    ['adminUiImplemented', evidence.adminUiImplemented === false],
    ['platformAdapterImplemented', evidence.platformAdapterImplemented === false],
    ['platformWriteImplemented', evidence.platformWriteImplemented === false && !combined.includes('WriteToPlatform')],
    ['realCredentialsEnabled', evidence.realCredentialsEnabled === false],
    ['realPlatformWriteEnabled', evidence.realPlatformWriteEnabled === false],
    ['automaticPublishEnabled', evidence.automaticPublishEnabled === false],
    ['automaticListingEnabled', evidence.automaticListingEnabled === false],
    ['humanConfirmationRequired', evidence.humanConfirmationRequired === true],
    ['p7DeferredPerformancePreserved', evidence.p7DeferredPerformancePreserved === true],
    ['p10ProductionBoundaryPreserved', evidence.p10ProductionBoundaryPreserved === true],
    ['productionReady', evidence.productionReady === false],
    ['packageScriptsRegistered', hasAll(packageText, ['test:p8-task-batch-3', 'p8:task-batch-3-gate'])],
  ];

  const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failed.length ? 'failed' : 'passed',
    failed,
    failedCount: failed.length,
    checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  };
}

export function buildP8TaskBatch3GateReport(bundle = {}) {
  const evidence = bundle.evidence ?? readJSON(P8_TASK_BATCH_3_EVIDENCE_JSON) ?? {};
  const validation = validateP8TaskBatch3Bundle({ evidence, sources: bundle.sources });
  const missingFiles = requiredFiles.filter((rel) => !fs.existsSync(rootPath(rel)));
  const failed = [...validation.failed, ...missingFiles.map((rel) => `missing:${rel}`)];
  return {
    phase: 'P8',
    gate: 'P8-TASK-BATCH-3',
    status: failed.length ? 'failed' : 'passed',
    checkedAt: '2026-07-20T00:00:00.000Z',
    batchId: 'P8-TASK-BATCH-3',
    tasks: ['P8-201', 'P8-202', 'P8-203'],
    taskStateMachinePresent: evidence.taskStateMachinePresent === true,
    taskTransitionServicePresent: evidence.taskTransitionServicePresent === true,
    draftVersionServicePresent: evidence.draftVersionServicePresent === true,
    approvalServicePresent: evidence.approvalServicePresent === true,
    invalidTransitionsRejected: evidence.invalidTransitionsRejected === true,
    taskEventWrittenAtomically: evidence.taskEventWrittenAtomically === true,
    canonicalJsonHashVersion: evidence.canonicalJsonHashVersion,
    draftVersionsAppendOnly: evidence.draftVersionsAppendOnly === true,
    approvedDraftEditRequiresReapproval: evidence.approvedDraftEditRequiresReapproval === true,
    approvalLatestDraftBindingPresent: evidence.approvalLatestDraftBindingPresent === true,
    approvalDraftVersionBindingPresent: evidence.approvalDraftVersionBindingPresent === true,
    approvalDraftHashBindingPresent: evidence.approvalDraftHashBindingPresent === true,
    approvalAuthorizerRequired: evidence.approvalAuthorizerRequired === true,
    approvalDefaultAllow: evidence.approvalDefaultAllow === true,
    idempotencyTestsPassed: evidence.idempotencyTestsPassed === true,
    concurrencyTestsPassed: evidence.concurrencyTestsPassed === true,
    rollbackTestsPassed: evidence.rollbackTestsPassed === true,
    racePassed: evidence.racePassed === true,
    dataRaces: evidence.dataRaces ?? null,
    executionOrchestratorImplemented: evidence.executionOrchestratorImplemented === true,
    retryServiceImplemented: evidence.retryServiceImplemented === true,
    apiImplemented: evidence.apiImplemented === true,
    adminUiImplemented: evidence.adminUiImplemented === true,
    platformAdapterImplemented: evidence.platformAdapterImplemented === true,
    platformWriteImplemented: evidence.platformWriteImplemented === true,
    realCredentialsEnabled: evidence.realCredentialsEnabled === true,
    realPlatformWriteEnabled: evidence.realPlatformWriteEnabled === true,
    automaticPublishEnabled: evidence.automaticPublishEnabled === true,
    automaticListingEnabled: evidence.automaticListingEnabled === true,
    humanConfirmationRequired: evidence.humanConfirmationRequired === true,
    p7DeferredPerformancePreserved: evidence.p7DeferredPerformancePreserved === true,
    p10ProductionBoundaryPreserved: evidence.p10ProductionBoundaryPreserved === true,
    productionReady: evidence.productionReady === true,
    failedCount: failed.length,
    failed,
    checks: validation.checks,
  };
}

export function writeP8TaskBatch3GateReport(report) {
  writeJSON(P8_TASK_BATCH_3_GATE_JSON, report);
  writeMarkdown(
    P8_TASK_BATCH_3_GATE_MD,
    `# P8 Task Batch 3 Final Gate

Status: **${report.status}**

- Batch: ${report.batchId}
- Tasks: ${report.tasks.join(', ')}
- Task state machine present: ${report.taskStateMachinePresent}
- Task transition service present: ${report.taskTransitionServicePresent}
- Draft version service present: ${report.draftVersionServicePresent}
- Approval service present: ${report.approvalServicePresent}
- Invalid transitions rejected: ${report.invalidTransitionsRejected}
- Task events written atomically: ${report.taskEventWrittenAtomically}
- Canonical JSON hash version: ${report.canonicalJsonHashVersion}
- Draft versions append-only: ${report.draftVersionsAppendOnly}
- Approved draft edit requires reapproval: ${report.approvedDraftEditRequiresReapproval}
- Approval latest draft binding present: ${report.approvalLatestDraftBindingPresent}
- Approval draft version binding present: ${report.approvalDraftVersionBindingPresent}
- Approval draft hash binding present: ${report.approvalDraftHashBindingPresent}
- Approval authorizer required: ${report.approvalAuthorizerRequired}
- Approval default allow: ${report.approvalDefaultAllow}
- Idempotency tests passed: ${report.idempotencyTestsPassed}
- Concurrency tests passed: ${report.concurrencyTestsPassed}
- Rollback tests passed: ${report.rollbackTestsPassed}
- Race passed: ${report.racePassed}
- Data races: ${report.dataRaces}
- Execution orchestrator implemented: ${report.executionOrchestratorImplemented}
- Retry service implemented: ${report.retryServiceImplemented}
- API implemented: ${report.apiImplemented}
- Admin UI implemented: ${report.adminUiImplemented}
- Platform adapter implemented: ${report.platformAdapterImplemented}
- Platform write implemented: ${report.platformWriteImplemented}
- Real credentials enabled: ${report.realCredentialsEnabled}
- Real platform write enabled: ${report.realPlatformWriteEnabled}
- Automatic publish enabled: ${report.automaticPublishEnabled}
- Automatic listing enabled: ${report.automaticListingEnabled}
- Human confirmation required: ${report.humanConfirmationRequired}
- Production Ready: ${report.productionReady}
- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}

This gate validates only P8 Batch 3 state-machine, draft-version, and human approval services. It does not authorize execution orchestration, retry service, API, Admin UI, platform adapters, real platform writes, automatic publish, automatic listing, production tag, production release, or Production Ready.
`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildP8TaskBatch3GateReport();
  writeP8TaskBatch3GateReport(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}

