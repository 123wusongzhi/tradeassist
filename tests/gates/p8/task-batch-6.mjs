import assert from 'node:assert/strict';
import { writeJSON } from '../../../scripts/p7-v2-lib.mjs';
import { validateP8TaskBatch6Bundle } from '../../../scripts/p8-task-batch-6-final-gate.mjs';

const validSources = {
  adminpermText: [
    'PermOperationTaskReview',
    'PermOperationTaskExecute',
    'PermOperationTaskRetry',
    'PermOperationTaskAuditRead',
    'RoleReviewer',
    'StrictHasPermission',
  ].join('\n'),
  authorizerText: [
    'type RBACAuthorizer struct',
    'CanReview',
    'PermOperationTaskReview',
    'CanExecute',
    'PermOperationTaskExecute',
    'CanRetry',
    'PermOperationTaskRetry',
    'user.TenantID != tenantID',
  ].join('\n'),
  redactionText: 'func redactSafeJSON\nredactedSensitiveField',
  executionText: [
    'type ManualRetryAuthorizer interface',
    'CanRetry',
    'ErrPermissionDenied',
    'result.SafeMetadata = redactSafeJSON(result.SafeMetadata)',
    'f.Details = redactSafeJSON(f.Details)',
  ].join('\n'),
  servicesText: [
    'func appendAuditEventTx',
    'event.Metadata = redactSafeJSON(event.Metadata)',
    'clause.Locking',
    'Sequence = latest.Sequence + 1',
  ].join('\n'),
  adapterText: 'redactSafeJSON(datatypes.JSON(data))',
  repositoryText: 'redactSafeJSON',
  testsText: [
    'TestRBACAuthorizerStrictRolesTenantAndActor',
    'TestManualRetryAuthorizerDeniesBeforeRetryExecution',
    'TestAuditEventsRecordReasonAndRedactMetadata',
    'TestExecutionFailureDetailsAreRedactedNotDropped',
    'unknown roles must not inherit',
    'NotContains',
  ].join('\n'),
  packageText: 'test:p8-task-batch-6\np8:task-batch-6-gate',
  docsText: 'Tag deferred\n非 Production Ready\nFinal Production Acceptance Deferred to P10',
};

function validEvidence(overrides = {}) {
  return {
    batchId: 'P8-TASK-BATCH-6',
    tasks: {
      'P8-401': { status: 'completed' },
      'P8-402': { status: 'completed' },
      'P8-403': { status: 'completed' },
      'P8-404': { status: 'completed' },
    },
    existingRBACReused: true,
    duplicateRBACSystemCreated: false,
    approvalAuthorizerIntegrated: true,
    executionAuthorizerIntegrated: true,
    manualRetryAuthorizerIntegrated: true,
    authorizationDefaultAllow: false,
    crossTenantAccessDenied: true,
    operationTaskAuditServicePresent: true,
    auditDeliveryMode: 'synchronous_db_transaction',
    auditLossPreventionPresent: true,
    auditFireAndForgetPresent: false,
    secretRedactorPresent: true,
    executionErrorRedactionIntegrated: true,
    auditMetadataRedactionIntegrated: true,
    adapterMetadataRedactionIntegrated: true,
    rawSecretPersistenceDetected: false,
    rawSecretLogDetected: false,
    realSecretCount: 0,
    permissionTestsPassed: true,
    auditTestsPassed: true,
    redactionTestsPassed: true,
    raceStatus: 'passed',
    apiImplemented: false,
    adminUiImplemented: false,
    realPlatformWriteImplemented: false,
    automaticPublishEnabled: false,
    automaticListingEnabled: false,
    realCredentialsEnabled: false,
    productionReady: false,
    ...overrides,
  };
}

function assertFails(id, overrides = {}, sourceOverrides = {}) {
  const result = validateP8TaskBatch6Bundle({
    evidence: validEvidence(overrides),
    sources: { ...validSources, ...sourceOverrides },
  });
  assert.equal(result.status, 'failed', id);
  assert.ok(result.failed.includes(id), `${id} should fail, saw ${result.failed.join(', ')}`);
}

assert.equal(validateP8TaskBatch6Bundle({ evidence: validEvidence(), sources: validSources }).status, 'passed');
assertFails('P8-401 status', { tasks: { ...validEvidence().tasks, 'P8-401': { status: 'pending' } } });
assertFails('P8-402 status', { tasks: { ...validEvidence().tasks, 'P8-402': { status: 'pending' } } });
assertFails('P8-403 status', { tasks: { ...validEvidence().tasks, 'P8-403': { status: 'pending' } } });
assertFails('P8-404 status', { tasks: { ...validEvidence().tasks, 'P8-404': { status: 'pending' } } });
assertFails('existingRBACReused', { existingRBACReused: false });
assertFails('duplicateRBACSystemCreated', { duplicateRBACSystemCreated: true }, { authorizerText: `${validSources.authorizerText}\ntype OperationTaskRoleMatrix struct {}` });
assertFails('approvalAuthorizerIntegrated', { approvalAuthorizerIntegrated: false });
assertFails('executionAuthorizerIntegrated', { executionAuthorizerIntegrated: false });
assertFails('manualRetryAuthorizerIntegrated', { manualRetryAuthorizerIntegrated: false });
assertFails('authorizationDefaultAllow', { authorizationDefaultAllow: true });
assertFails('crossTenantAccessDenied', { crossTenantAccessDenied: false });
assertFails('operationTaskAuditServicePresent', { operationTaskAuditServicePresent: false });
assertFails('auditDeliveryMode', { auditDeliveryMode: 'fire_and_forget' });
assertFails('auditLossPreventionPresent', { auditLossPreventionPresent: false });
assertFails('auditFireAndForgetPresent', { auditFireAndForgetPresent: true });
assertFails('secretRedactorPresent', { secretRedactorPresent: false });
assertFails('executionErrorRedactionIntegrated', { executionErrorRedactionIntegrated: false });
assertFails('auditMetadataRedactionIntegrated', { auditMetadataRedactionIntegrated: false });
assertFails('adapterMetadataRedactionIntegrated', { adapterMetadataRedactionIntegrated: false });
assertFails('rawSecretPersistenceDetected', { rawSecretPersistenceDetected: true });
assertFails('rawSecretLogDetected', { rawSecretLogDetected: true });
assertFails('realSecretCount', { realSecretCount: 1 });
assertFails('permissionTestsPassed', { permissionTestsPassed: false });
assertFails('auditTestsPassed', { auditTestsPassed: false });
assertFails('redactionTestsPassed', { redactionTestsPassed: false });
assertFails('raceStatus', { raceStatus: 'not_run' });
assertFails('apiImplemented', { apiImplemented: true });
assertFails('adminUiImplemented', { adminUiImplemented: true });
assertFails('realPlatformWriteImplemented', { realPlatformWriteImplemented: true });
assertFails('automaticPublishEnabled', { automaticPublishEnabled: true });
assertFails('automaticListingEnabled', { automaticListingEnabled: true });
assertFails('realCredentialsEnabled', { realCredentialsEnabled: true });
assertFails('productionReady', { productionReady: true });
assertFails('packageScriptsRegistered', {}, { packageText: 'test:p8-task-batch-5' });

const report = {
  phase: 'P8',
  batchId: 'P8-TASK-BATCH-6',
  status: 'passed',
  fixtures: 35,
};
writeJSON('docs/p8-task-batch-6-fixture-report.json', report);
console.log(JSON.stringify(report, null, 2));
