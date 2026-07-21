import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

export const P8_TASK_BATCH_4_EVIDENCE_JSON = 'docs/p8-task-batch-4-execution-retry-idempotency-services.json';
export const P8_TASK_BATCH_4_GATE_JSON = 'docs/p8-task-batch-4-final-gate.json';
export const P8_TASK_BATCH_4_GATE_MD = 'docs/P8_TASK_BATCH_4_FINAL_GATE.md';

const requiredFiles = [
  'backend/internal/modules/operationtask/execution_services.go',
  'backend/internal/modules/operationtask/execution_services_test.go',
  'docs/P8_TASK_BATCH_4_EXECUTION_RETRY_IDEMPOTENCY_SERVICES.md',
  'docs/p8-task-batch-4-execution-retry-idempotency-services.json',
  'scripts/p8-task-batch-4-final-gate.mjs',
  'tests/gates/p8/task-batch-4.mjs',
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

function git(args) {
  try {
    return execFileSync('git', args, { cwd: process.cwd(), encoding: 'utf8' }).trim();
  } catch {
    return '';
  }
}

export function validateP8TaskBatch4Bundle({ evidence = {}, sources = {} } = {}) {
  const executionText = sources.executionText ?? text('backend/internal/modules/operationtask/execution_services.go');
  const modelText = sources.modelText ?? text('backend/internal/modules/operationtask/model.go');
  const repositoryText = sources.repositoryText ?? text('backend/internal/modules/operationtask/repository.go');
  const testsText = sources.testsText ?? text('backend/internal/modules/operationtask/execution_services_test.go');
  const packageText = sources.packageText ?? text('package.json');
  const docsText = sources.docsText ?? text('docs/P8_TASK_BATCH_4_EXECUTION_RETRY_IDEMPOTENCY_SERVICES.md');
  const combined = `${executionText}\n${modelText}\n${repositoryText}\n${testsText}\n${docsText}`;

  const checks = [
    ['batchId', evidence.batchId === 'P8-TASK-BATCH-4'],
    ['P8-204 status', evidence.tasks?.['P8-204']?.status === 'completed'],
    ['P8-205 status', evidence.tasks?.['P8-205']?.status === 'completed'],
    ['P8-206 status', evidence.tasks?.['P8-206']?.status === 'completed'],
    ['executionOrchestratorPresent', evidence.executionOrchestratorPresent === true && hasAll(executionText, ['type ExecutionOrchestrator struct', 'func (s *ExecutionOrchestrator) Execute'])],
    ['executionFailureClassifierPresent', evidence.executionFailureClassifierPresent === true && hasAll(executionText, ['type ExecutionFailureClassifier struct', 'func (c *ExecutionFailureClassifier) Classify'])],
    ['manualRetryServicePresent', evidence.manualRetryServicePresent === true && hasAll(executionText, ['type ManualRetryService struct', 'func (s *ManualRetryService) Retry'])],
    ['executionIdempotencyProtectionPresent', evidence.executionIdempotencyProtectionPresent === true && hasAll(executionText, ['ExecutionIdempotencyStatusInProgress', 'ErrIdemPayloadConflict', 'findAttemptByIdempotencyTx'])],
    ['executionAuthorizerRequired', evidence.executionAuthorizerRequired === true && executionText.includes('Authorizer == nil')],
    ['executionDefaultAllow', evidence.executionDefaultAllow === false],
    ['approvalLatestDraftBindingEnforced', evidence.approvalLatestDraftBindingEnforced === true && executionText.includes('findLatestDraftTx')],
    ['approvalDraftVersionBindingEnforced', evidence.approvalDraftVersionBindingEnforced === true && executionText.includes('approval.DraftVersion != latestDraft.DraftVersion')],
    ['approvalDraftHashBindingEnforced', evidence.approvalDraftHashBindingEnforced === true && executionText.includes('approval.DraftPayloadHash != latestDraft.PayloadHash')],
    ['executionPortCalledOutsideTransaction', evidence.executionPortCalledOutsideTransaction === true && executionText.indexOf('s.prepare') < executionText.indexOf('s.Port.ExecuteDraft') && executionText.indexOf('s.Port.ExecuteDraft') < executionText.indexOf('s.finalize')],
    ['executionPrepareAtomic', evidence.executionPrepareAtomic === true && executionText.includes('func (s *ExecutionOrchestrator) prepare') && executionText.includes('Transaction(func(tx *gorm.DB) error')],
    ['executionSuccessFinalizeAtomic', evidence.executionSuccessFinalizeAtomic === true && executionText.includes('func (s *ExecutionOrchestrator) finalizeSuccess')],
    ['executionFailureFinalizeAtomic', evidence.executionFailureFinalizeAtomic === true && executionText.includes('func (s *ExecutionOrchestrator) finalizeFailure')],
    ['duplicateExecutionPrevented', evidence.duplicateExecutionPrevented === true && testsText.includes('DuplicateReplayInProgressAndPayloadConflict')],
    ['concurrentExecutionPrevented', evidence.concurrentExecutionPrevented === true && executionText.includes('hasActiveAttemptTx')],
    ['idempotencyPayloadConflictDetected', evidence.idempotencyPayloadConflictDetected === true && testsText.includes('ErrIdemPayloadConflict')],
    ['manualRetryOnly', evidence.manualRetryOnly === true && !combined.includes('AutoRetryWorker') && !combined.includes('time.Ticker')],
    ['automaticRetryWorkerPresent', evidence.automaticRetryWorkerPresent === false],
    ['retryLimitPresent', evidence.retryLimitPresent === true && executionText.includes('DefaultMaxManualRetryAttempts')],
    ['nonRetryableErrorsBlocked', evidence.nonRetryableErrorsBlocked === true && testsText.includes('BlocksNonRetryable')],
    ['executionAttemptAppendHistoryPreserved', evidence.executionAttemptAppendHistoryPreserved === true && testsText.includes('AttemptNumber')],
    ['executionErrorAppendOnly', evidence.executionErrorAppendOnly === true && !executionText.includes('Delete(&ExecutionError')],
    ['operationTaskEventAppendOnly', evidence.operationTaskEventAppendOnly === true && !executionText.includes('Delete(&OperationTaskEvent')],
    ['repositoryTestsPassed', evidence.repositoryTestsPassed === true],
    ['transactionTestsPassed', evidence.transactionTestsPassed === true],
    ['rollbackTestsPassed', evidence.rollbackTestsPassed === true],
    ['idempotencyTestsPassed', evidence.idempotencyTestsPassed === true],
    ['concurrencyTestsPassed', evidence.concurrencyTestsPassed === true],
    ['racePassed', evidence.racePassed === true && evidence.dataRaces === 0],
    ['apiImplemented', evidence.apiImplemented === false && !combined.includes('gin.') && !combined.includes('/operation-tasks')],
    ['adminUiImplemented', evidence.adminUiImplemented === false],
    ['productionPlatformAdapterImplemented', evidence.productionPlatformAdapterImplemented === false && !combined.includes('ProductionAdapter')],
    ['realPlatformWriteImplemented', evidence.realPlatformWriteImplemented === false && !combined.includes('RealWrite')],
    ['automaticPublishImplemented', evidence.automaticPublishImplemented === false && !combined.includes('AutoPublish')],
    ['automaticListingImplemented', evidence.automaticListingImplemented === false && !combined.includes('AutoListing')],
    ['realCredentialsEnabled', evidence.realCredentialsEnabled === false],
    ['realPlatformWriteEnabled', evidence.realPlatformWriteEnabled === false],
    ['automaticPublishEnabled', evidence.automaticPublishEnabled === false],
    ['automaticListingEnabled', evidence.automaticListingEnabled === false],
    ['p7DeferredPerformancePreserved', evidence.p7DeferredPerformancePreserved === true],
    ['p10ProductionBoundaryPreserved', evidence.p10ProductionBoundaryPreserved === true],
    ['productionReady', evidence.productionReady === false],
    ['packageScriptsRegistered', hasAll(packageText, ['test:p8-task-batch-4', 'p8:task-batch-4-gate'])],
  ];

  const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failed.length ? 'failed' : 'passed',
    failed,
    failedCount: failed.length,
    checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  };
}

export function buildP8TaskBatch4GateReport(bundle = {}) {
  const evidence = bundle.evidence ?? readJSON(P8_TASK_BATCH_4_EVIDENCE_JSON) ?? {};
  const validation = validateP8TaskBatch4Bundle({ evidence, sources: bundle.sources });
  const missingFiles = requiredFiles.filter((rel) => !fs.existsSync(rootPath(rel)));
  const stagedFiles = git(['diff', '--cached', '--name-only']);
  const currentBranch = git(['branch', '--show-current']);
  const failed = [
    ...validation.failed,
    ...missingFiles.map((rel) => `missing:${rel}`),
  ];
  if (currentBranch !== 'dev') failed.push('currentBranch');
  if (stagedFiles !== '') failed.push('stagedFileCount');
  return {
    phase: 'P8',
    gate: 'P8-TASK-BATCH-4',
    status: failed.length ? 'failed' : 'passed',
    checkedAt: '2026-07-21T00:00:00.000Z',
    batchId: 'P8-TASK-BATCH-4',
    tasks: ['P8-204', 'P8-205', 'P8-206'],
    currentBranch,
    stagedFileCount: stagedFiles === '' ? 0 : stagedFiles.split('\n').length,
    executionOrchestratorPresent: evidence.executionOrchestratorPresent === true,
    executionFailureClassifierPresent: evidence.executionFailureClassifierPresent === true,
    manualRetryServicePresent: evidence.manualRetryServicePresent === true,
    executionIdempotencyProtectionPresent: evidence.executionIdempotencyProtectionPresent === true,
    executionAuthorizerRequired: evidence.executionAuthorizerRequired === true,
    executionDefaultAllow: evidence.executionDefaultAllow === true,
    executionPortCalledOutsideTransaction: evidence.executionPortCalledOutsideTransaction === true,
    duplicateExecutionPrevented: evidence.duplicateExecutionPrevented === true,
    concurrentExecutionPrevented: evidence.concurrentExecutionPrevented === true,
    idempotencyPayloadConflictDetected: evidence.idempotencyPayloadConflictDetected === true,
    manualRetryOnly: evidence.manualRetryOnly === true,
    automaticRetryWorkerPresent: evidence.automaticRetryWorkerPresent === true,
    retryLimitPresent: evidence.retryLimitPresent === true,
    nonRetryableErrorsBlocked: evidence.nonRetryableErrorsBlocked === true,
    racePassed: evidence.racePassed === true,
    dataRaces: evidence.dataRaces ?? null,
    apiImplemented: evidence.apiImplemented === true,
    adminUiImplemented: evidence.adminUiImplemented === true,
    productionPlatformAdapterImplemented: evidence.productionPlatformAdapterImplemented === true,
    realPlatformWriteImplemented: evidence.realPlatformWriteImplemented === true,
    automaticPublishImplemented: evidence.automaticPublishImplemented === true,
    automaticListingImplemented: evidence.automaticListingImplemented === true,
    productionReady: evidence.productionReady === true,
    failedCount: failed.length,
    failed,
    checks: validation.checks,
  };
}

export function writeP8TaskBatch4GateReport(report) {
  writeJSON(P8_TASK_BATCH_4_GATE_JSON, report);
  writeMarkdown(
    P8_TASK_BATCH_4_GATE_MD,
    `# P8 Task Batch 4 Final Gate

Status: **${report.status}**

- Batch: ${report.batchId}
- Tasks: ${report.tasks.join(', ')}
- Current branch: ${report.currentBranch}
- Staged files: ${report.stagedFileCount}
- Execution orchestrator present: ${report.executionOrchestratorPresent}
- Failure classifier present: ${report.executionFailureClassifierPresent}
- Manual retry service present: ${report.manualRetryServicePresent}
- Execution idempotency protection present: ${report.executionIdempotencyProtectionPresent}
- Execution authorizer required: ${report.executionAuthorizerRequired}
- Execution default allow: ${report.executionDefaultAllow}
- Execution port called outside transaction: ${report.executionPortCalledOutsideTransaction}
- Duplicate execution prevented: ${report.duplicateExecutionPrevented}
- Concurrent execution prevented: ${report.concurrentExecutionPrevented}
- Idempotency payload conflict detected: ${report.idempotencyPayloadConflictDetected}
- Manual retry only: ${report.manualRetryOnly}
- Automatic retry worker present: ${report.automaticRetryWorkerPresent}
- Retry limit present: ${report.retryLimitPresent}
- Non-retryable errors blocked: ${report.nonRetryableErrorsBlocked}
- Race passed: ${report.racePassed}
- Data races: ${report.dataRaces}
- API implemented: ${report.apiImplemented}
- Admin UI implemented: ${report.adminUiImplemented}
- Production platform adapter implemented: ${report.productionPlatformAdapterImplemented}
- Real platform write implemented: ${report.realPlatformWriteImplemented}
- Automatic publish implemented: ${report.automaticPublishImplemented}
- Automatic listing implemented: ${report.automaticListingImplemented}
- Production Ready: ${report.productionReady}
- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}

This gate validates only P8 Batch 4 execution orchestration, failure classification, manual retry, and idempotency protection. It does not authorize API, Admin UI, production platform adapters, real platform writes, automatic publish, automatic listing, production tag, production release, or Production Ready.
`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildP8TaskBatch4GateReport();
  writeP8TaskBatch4GateReport(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}
