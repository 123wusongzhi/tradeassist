import assert from 'node:assert/strict';
import { writeJSON } from '../../../scripts/p7-v2-lib.mjs';
import { validateP8TaskBatch7Bundle } from '../../../scripts/p8-task-batch-7-final-gate.mjs';

const validSources = {
  apiRouterText: 'BearerAuthWithDB\n/api/v1\noperationtask.NewAPIService\noperationtask.Register(authed',
  moduleRouterText: [
    'POST("", h.CreateTask)',
    'GET("", h.ListTasks)',
    'GET("/:taskId", h.GetTask)',
    'POST("/:taskId/cancel", h.CancelTask)',
    'POST("/:taskId/drafts", h.CreateDraft)',
    'PATCH("/:taskId/drafts/latest", h.EditLatestDraft)',
    'GET("/:taskId/drafts", h.ListDrafts)',
    'POST("/:taskId/approve", h.Approve)',
    'POST("/:taskId/reject", h.Reject)',
    'POST("/:taskId/execute", h.Execute)',
    'POST("/:taskId/retry", h.Retry)',
    'GET("/:taskId/attempts", h.ListAttempts)',
    'GET("/:taskId/events", h.ListEvents)',
  ].join('\n'),
  handlerText: [
    'adminperm.TenantIDFromGin',
    'apiActorID(c)',
    'adminperm.LoadPrincipal',
    'apiIdempotencyKey(c)',
    'requireWrite',
    'CreateTask',
    'h.Svc.',
  ].join('\n'),
  serviceText: [
    'type APIService struct',
    'NewAPIService',
    'CreateTask',
    'Execute',
    'RetryExecution',
    'NextCursor',
    'Cursor',
    'decodeSafeJSON',
    'redactSafeJSON',
    'safeKeyHash',
  ].join('\n'),
  validationText: 'DisallowUnknownFields\nMaxBytesReader\napiIdempotencyKeyPattern\nctxkey.TraceID',
  errorsText: 'apiRespondError\npermission_denied\nvalidation_error',
  dtoText: 'type ExecutionResponse struct\ntype OperationTaskDetailResponse struct',
  authorizerText: 'CanRead\nCanCreate\nCanEdit\nCanCancel\nPermOperationTaskAuditRead\nStrictHasPermission',
  testsText: [
    'TestAPIServiceCreateTaskIdempotencyAndTenantActorBoundary',
    'TestOperationTaskHandlerRequiresIdempotencyKeyForWrites',
    'RejectsUnknownDangerousFields',
    'tenantId',
    'DoesNotExposePublished',
    'NotContains',
  ].join('\n'),
  packageText: 'test:p8-task-batch-7\np8:task-batch-7-gate',
  docsText: 'P8-501\nP8-502\nP8-503\nP8-504\nP8-505\nProduction Ready: false',
};

function validEvidence(overrides = {}) {
  return {
    batchId: 'P8-TASK-BATCH-7',
    tasks: {
      'P8-501': { status: 'completed' },
      'P8-502': { status: 'completed' },
      'P8-503': { status: 'completed' },
      'P8-504': { status: 'completed' },
      'P8-505': { status: 'completed' },
    },
    apiImplemented: true,
    routesRegistered: true,
    existingAPIStackReused: true,
    trustedTenantActorContext: true,
    strictJSONBinding: true,
    idempotencyRequiredForWrites: true,
    requestIDServerContext: true,
    handlerNoDirectRepositoryWrites: true,
    serviceFacadePresent: true,
    rbacIntegrated: true,
    keysetPagination: true,
    safeDTOs: true,
    redactionIntegrated: true,
    dangerousBodyFieldsRejected: true,
    executeResponseDoesNotExposePublished: true,
    apiTestsPassed: true,
    adminUiImplemented: false,
    realPlatformWriteEnabled: false,
    realCredentialsEnabled: false,
    automaticPublishEnabled: false,
    automaticListingEnabled: false,
    productionReady: false,
    docsPresent: true,
    ...overrides,
  };
}

function assertFails(id, overrides = {}, sourceOverrides = {}) {
  const result = validateP8TaskBatch7Bundle({
    evidence: validEvidence(overrides),
    sources: { ...validSources, ...sourceOverrides },
  });
  assert.equal(result.status, 'failed', id);
  assert.ok(result.failed.includes(id), `${id} should fail, saw ${result.failed.join(', ')}`);
}

assert.equal(validateP8TaskBatch7Bundle({ evidence: validEvidence(), sources: validSources }).status, 'passed');
assertFails('P8-501 status', { tasks: { ...validEvidence().tasks, 'P8-501': { status: 'pending' } } });
assertFails('P8-502 status', { tasks: { ...validEvidence().tasks, 'P8-502': { status: 'pending' } } });
assertFails('P8-503 status', { tasks: { ...validEvidence().tasks, 'P8-503': { status: 'pending' } } });
assertFails('P8-504 status', { tasks: { ...validEvidence().tasks, 'P8-504': { status: 'pending' } } });
assertFails('P8-505 status', { tasks: { ...validEvidence().tasks, 'P8-505': { status: 'pending' } } });
assertFails('apiImplemented', { apiImplemented: false });
assertFails('routesRegistered', { routesRegistered: false });
assertFails('existingAPIStackReused', { existingAPIStackReused: false });
assertFails('trustedTenantActorContext', { trustedTenantActorContext: false });
assertFails('strictJSONBinding', { strictJSONBinding: false });
assertFails('idempotencyRequiredForWrites', { idempotencyRequiredForWrites: false });
assertFails('requestIDServerContext', { requestIDServerContext: false });
assertFails('handlerNoDirectRepositoryWrites', { handlerNoDirectRepositoryWrites: false }, { handlerText: `${validSources.handlerText}\nNewOperationTaskRepository` });
assertFails('serviceFacadePresent', { serviceFacadePresent: false });
assertFails('rbacIntegrated', { rbacIntegrated: false });
assertFails('keysetPagination', { keysetPagination: false }, { handlerText: `${validSources.handlerText}\noffset` });
assertFails('safeDTOs', { safeDTOs: false }, { dtoText: `${validSources.dtoText}\nPublished bool` });
assertFails('redactionIntegrated', { redactionIntegrated: false });
assertFails('dangerousBodyFieldsRejected', { dangerousBodyFieldsRejected: false });
assertFails('executeResponseDoesNotExposePublished', { executeResponseDoesNotExposePublished: false });
assertFails('apiTestsPassed', { apiTestsPassed: false });
assertFails('adminUiImplemented', { adminUiImplemented: true });
assertFails('realPlatformWriteEnabled', { realPlatformWriteEnabled: true });
assertFails('realCredentialsEnabled', { realCredentialsEnabled: true });
assertFails('automaticPublishEnabled', { automaticPublishEnabled: true });
assertFails('automaticListingEnabled', { automaticListingEnabled: true });
assertFails('productionReady', { productionReady: true });
assertFails('docsPresent', { docsPresent: false });
assertFails('packageScriptsRegistered', {}, { packageText: 'test:p8-task-batch-6' });

const report = {
  phase: 'P8',
  batchId: 'P8-TASK-BATCH-7',
  status: 'passed',
  fixtures: 31,
};
writeJSON('docs/p8-task-batch-7-fixture-report.json', report);
console.log(JSON.stringify(report, null, 2));
