import assert from 'node:assert/strict';
import { writeJSON } from '../../../scripts/p7-v2-lib.mjs';
import { validateP8TaskBatch8Bundle } from '../../../scripts/p8-task-batch-8-final-gate.mjs';

const validSources = {
  routesText: '/ops/task-center/operation-tasks\n./TaskCenter/OperationTasks/Detail\nhideInMenu',
  requestText: 'RequestOptions\nApiRequestError\ntraceId\nwithOptions',
  serviceText: [
    '/api/v1/operation-tasks',
    'getWithParams',
    'postJSON',
    'patchJSON',
    'Idempotency-Key',
    'function createTask(body, idempotencyKey) { idempotencyHeaders(idempotencyKey) }',
    'function createDraft(taskId, body, idempotencyKey) { idempotencyHeaders(idempotencyKey) }',
    'function editDraft(taskId, body, idempotencyKey) { idempotencyHeaders(idempotencyKey) }',
    'function approveTask(taskId, body, idempotencyKey) { idempotencyHeaders(idempotencyKey) }',
    'function rejectTask(taskId, body, idempotencyKey) { idempotencyHeaders(idempotencyKey) }',
    'function cancelTask(taskId, body, idempotencyKey) { idempotencyHeaders(idempotencyKey) }',
    'function executeTask(taskId, body, idempotencyKey) { idempotencyHeaders(idempotencyKey) }',
    'function retryTask(taskId, body, idempotencyKey) { idempotencyHeaders(idempotencyKey) }',
  ].join('\n'),
  listText: 'TmPageContainer\nTmProTable\nnextCursor\nhasMore\ncursorStack\npagination={false}\nloading\nemptyLocale\nErrorAlert\n刷新',
  detailText: [
    'SectionCard',
    'TaskJsonBlock',
    'allowedActions.canEditDraft',
    'allowedActions.canApprove',
    'allowedActions.canReject',
    'allowedActions.canExecute',
    'allowedActions.canRetry',
    'allowedActions.canCancel',
    '后端仍会重新校验权限',
    'actionLoading',
    'confirmLoading={actionLoading}',
    'expectedTaskRevision',
    'conflict',
    'mismatch',
    'loadAll',
    'afterSequence',
    'nextSequence',
    'loadMoreEvents',
    '当前后端未返回历史草稿完整 Payload',
    'local_draft_only',
    'mock',
    'sandbox',
  ].join('\n'),
  sharedText: 'SENSITIVE_KEY\nredactSensitiveValue\nsafeMetadata\nOPERATION_METADATA_ALLOWLIST\nTaskJsonBlock\nNON_PRODUCTION_BOUNDARY_COPY',
  constantsText: 'OPERATION_TASK_STATUS_LABELS\nOPERATION_EVENT_TYPE_LABELS\nNON_PRODUCTION_BOUNDARY_COPY\n非生产边界\n真实平台写入\n自动发布\n自动上架',
  permissionsText: 'OPERATION_TASK_AUDIT_READ\nOPERATION_TASK_EXECUTE\nOPERATION_TASK_REVIEW\nOPERATION_TASK_RETRY',
  menuAccessText: 'OPERATION_TASK_AUDIT_READ',
  urlStateText: 'cursor\nafterSequence',
  testsText: 'operation task API service\noperation task shared helpers\nvalidates JSON editor input\nIdempotency-Key\nonly exposes allowlisted event metadata',
  packageText: 'test:p8-task-batch-8\np8:task-batch-8-gate',
  docsText: 'P8-601\nP8-602\nP8-603\nP8-604\nP8-605\nP8-606\nProduction Ready: false',
};

function validEvidence(overrides = {}) {
  return {
    batchId: 'P8-TASK-BATCH-8',
    tasks: {
      'P8-601': { status: 'completed' },
      'P8-602': { status: 'completed' },
      'P8-603': { status: 'completed' },
      'P8-604': { status: 'completed' },
      'P8-605': { status: 'completed' },
      'P8-606': { status: 'completed' },
    },
    adminRoutesPresent: true,
    existingAdminFrameworkReused: true,
    apiServiceLayerPresent: true,
    apiClientExtendedSafely: true,
    noRawFetchOrAxios: true,
    allowedActionsUsed: true,
    backendRBACBoundaryPreserved: true,
    operationTaskPermissionsWired: true,
    idempotencyHeadersForWrites: true,
    duplicateClickGuardPresent: true,
    revisionConflictRefresh: true,
    keysetPagination: true,
    eventSequencePagination: true,
    loadingEmptyErrorStates: true,
    safeMetadataRedaction: true,
    payloadNotRenderedAsHTML: true,
    historicalDraftPayloadNotFabricated: true,
    nonProductionBoundaryVisible: true,
    safeAdapterModesOnly: true,
    urlStateCursorKeys: true,
    frontendTestsPresent: true,
    docsPresent: true,
    backendBusinessLogicNotDuplicated: true,
    realPlatformWriteEnabled: false,
    realCredentialsEnabled: false,
    automaticPublishEnabled: false,
    automaticListingEnabled: false,
    backgroundAutoRetryEnabled: false,
    productionReady: false,
    ...overrides,
  };
}

function assertFails(id, overrides = {}, sourceOverrides = {}) {
  const result = validateP8TaskBatch8Bundle({
    evidence: validEvidence(overrides),
    sources: { ...validSources, ...sourceOverrides },
  });
  assert.equal(result.status, 'failed', id);
  assert.ok(result.failed.includes(id), `${id} should fail, saw ${result.failed.join(', ')}`);
}

assert.equal(validateP8TaskBatch8Bundle({ evidence: validEvidence(), sources: validSources }).status, 'passed');
assertFails('P8-601 status', { tasks: { ...validEvidence().tasks, 'P8-601': { status: 'pending' } } });
assertFails('P8-602 status', { tasks: { ...validEvidence().tasks, 'P8-602': { status: 'pending' } } });
assertFails('P8-603 status', { tasks: { ...validEvidence().tasks, 'P8-603': { status: 'pending' } } });
assertFails('P8-604 status', { tasks: { ...validEvidence().tasks, 'P8-604': { status: 'pending' } } });
assertFails('P8-605 status', { tasks: { ...validEvidence().tasks, 'P8-605': { status: 'pending' } } });
assertFails('P8-606 status', { tasks: { ...validEvidence().tasks, 'P8-606': { status: 'pending' } } });
assertFails('adminRoutesPresent', { adminRoutesPresent: false });
assertFails('existingAdminFrameworkReused', { existingAdminFrameworkReused: false });
assertFails('apiServiceLayerPresent', { apiServiceLayerPresent: false });
assertFails('apiClientExtendedSafely', { apiClientExtendedSafely: false });
assertFails('noRawFetchOrAxios', { noRawFetchOrAxios: false }, { detailText: `${validSources.detailText}\nfetch('/api/v1/operation-tasks')` });
assertFails('allowedActionsUsed', { allowedActionsUsed: false });
assertFails('backendRBACBoundaryPreserved', { backendRBACBoundaryPreserved: false }, { detailText: `${validSources.detailText}\nnormalizeRole(role)` });
assertFails('operationTaskPermissionsWired', { operationTaskPermissionsWired: false });
assertFails('idempotencyHeadersForWrites', { idempotencyHeadersForWrites: false });
assertFails('duplicateClickGuardPresent', { duplicateClickGuardPresent: false });
assertFails('revisionConflictRefresh', { revisionConflictRefresh: false });
assertFails('keysetPagination', { keysetPagination: false }, { listText: `${validSources.listText}\ntotal:` });
assertFails('eventSequencePagination', { eventSequencePagination: false });
assertFails('loadingEmptyErrorStates', { loadingEmptyErrorStates: false });
assertFails('safeMetadataRedaction', { safeMetadataRedaction: false });
assertFails('payloadNotRenderedAsHTML', { payloadNotRenderedAsHTML: false }, { sharedText: `${validSources.sharedText}\ndangerouslySetInnerHTML` });
assertFails('historicalDraftPayloadNotFabricated', { historicalDraftPayloadNotFabricated: false });
assertFails('nonProductionBoundaryVisible', { nonProductionBoundaryVisible: false });
assertFails('safeAdapterModesOnly', { safeAdapterModesOnly: false });
assertFails('urlStateCursorKeys', { urlStateCursorKeys: false });
assertFails('frontendTestsPresent', { frontendTestsPresent: false });
assertFails('docsPresent', { docsPresent: false });
assertFails('backendBusinessLogicNotDuplicated', { backendBusinessLogicNotDuplicated: false }, { detailText: `${validSources.detailText}\nTaskStateMachine` });
assertFails('realPlatformWriteEnabled', { realPlatformWriteEnabled: true });
assertFails('realCredentialsEnabled', { realCredentialsEnabled: true });
assertFails('automaticPublishEnabled', { automaticPublishEnabled: true });
assertFails('automaticListingEnabled', { automaticListingEnabled: true });
assertFails('backgroundAutoRetryEnabled', { backgroundAutoRetryEnabled: true });
assertFails('productionReady', { productionReady: true });
assertFails('packageScriptsRegistered', {}, { packageText: 'test:p8-task-batch-7' });

const report = {
  phase: 'P8',
  batchId: 'P8-TASK-BATCH-8',
  status: 'passed',
  fixtures: 39,
};
writeJSON('docs/p8-task-batch-8-fixture-report.json', report);
console.log(JSON.stringify(report, null, 2));
