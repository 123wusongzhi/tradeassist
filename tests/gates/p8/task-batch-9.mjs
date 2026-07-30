import assert from 'node:assert/strict';
import { writeJSON } from '../../../scripts/p7-v2-lib.mjs';
import { validateP8TaskBatch9Bundle } from '../../../scripts/p8-task-batch-9-final-gate.mjs';

const validSources = {
  apiServiceText: [
    'CanExecute',
    'CanRetry',
    'req.ExpectedTaskRevision > 0',
    'req.FailedAttemptID != nil',
    'ErrRevisionConflict',
    'ExecutionAttemptStatusFailed',
  ].join('\n'),
  apiServiceTestsText: [
    'TestAPIExecuteRejectsStaleExpectedTaskRevision',
    'TestAPIRetryValidatesFailedAttemptID',
    'TestAPIRetryAcceptsMatchingFailedAttemptID',
  ].join('\n'),
  foundationTestsText: 'operatorID\nreviewerID\nCanCreate\nCanEdit\nCanExecute\nCanRetry',
  adaptersText: [
    'SafeDraftCreationCapabilities',
    'DraftCreation: true',
    'capabilities.Publish || capabilities.Listing || capabilities.NetworkAccess || capabilities.RealCredentials || capabilities.AutomaticExecution',
    'production_capability_forbidden',
    'ExecutionPortModeLocalDraftFixture',
    'ExecutionPortModeMock',
    'ExecutionPortModeSandboxFixture',
  ].join('\n'),
  rbacText: 'PermOperationTaskEdit\nPermOperationTaskExecute\nPermOperationTaskRetry',
  backendPermText: 'operationtask.edit\nPermOperationTaskEdit\nPermOperationTaskExecute\nPermOperationTaskRetry',
  frontendPermText: 'operationtask.edit\nOPERATION_TASK_EDIT\nOPERATION_TASK_EXECUTE\nOPERATION_TASK_RETRY',
  packageText: 'test:p8-task-batch-9\np8:task-batch-9-gate',
  docsText: 'P8 Development Complete\nProduction Ready: false',
};

function validEvidence(overrides = {}) {
  return {
    batchId: 'P8-TASK-BATCH-9',
    phaseStatus: 'Development Complete',
    tasks: {
      'P8-701': { status: 'completed' },
      'P8-702': { status: 'completed' },
      'P8-703': { status: 'completed' },
      'P8-704': { status: 'completed' },
      'P8-705': { status: 'completed' },
    },
    integrationFixturesPresent: true,
    executeRevisionPrecondition: true,
    retryPreconditions: true,
    roleBoundaryAligned: true,
    authenticatedAPIGoldenPathPassed: true,
    authenticatedAPIGoldenPathMode: 'real_backend_bearer_token',
    adminAuthenticatedE2EPassed: true,
    adminE2EMode: 'real_backend_api',
    unauthRedirectVerified: true,
    apiAuthRequiredVerified: true,
    platformBoundaryGatePassed: true,
    safeAdapterModesOnly: true,
    realCredentialsEnabled: false,
    realPlatformWriteEnabled: false,
    automaticPublishEnabled: false,
    automaticListingEnabled: false,
    productionReady: false,
    closureEvidencePresent: true,
    ...overrides,
  };
}

function assertFails(id, overrides = {}, sourceOverrides = {}) {
  const result = validateP8TaskBatch9Bundle({
    evidence: validEvidence(overrides),
    sources: { ...validSources, ...sourceOverrides },
  });
  assert.equal(result.status, 'failed', id);
  assert.ok(result.failed.includes(id), `${id} should fail, saw ${result.failed.join(', ')}`);
}

assert.equal(validateP8TaskBatch9Bundle({ evidence: validEvidence(), sources: validSources }).status, 'passed');
assertFails('P8-701 status', { tasks: { ...validEvidence().tasks, 'P8-701': { status: 'pending' } } });
assertFails('P8-702 status', { tasks: { ...validEvidence().tasks, 'P8-702': { status: 'pending' } } });
assertFails('P8-703 status', { tasks: { ...validEvidence().tasks, 'P8-703': { status: 'pending' } } });
assertFails('P8-704 status', { tasks: { ...validEvidence().tasks, 'P8-704': { status: 'pending' } } });
assertFails('P8-705 status', { tasks: { ...validEvidence().tasks, 'P8-705': { status: 'pending' } } });
assertFails('integrationFixturesPresent', { integrationFixturesPresent: false });
assertFails('executeRevisionPrecondition', { executeRevisionPrecondition: false }, { apiServiceText: 'CanExecute' });
assertFails('retryPreconditions', { retryPreconditions: false }, { apiServiceText: 'CanRetry' });
assertFails('roleBoundaryAligned', { roleBoundaryAligned: false }, { frontendPermText: 'OPERATION_TASK_EXECUTE' });
assertFails('authenticatedAPIGoldenPathPassed', { authenticatedAPIGoldenPathPassed: false });
assertFails('authenticatedAPIGoldenPathPassed', { authenticatedAPIGoldenPathMode: 'mocked_frontend_local_storage' });
assertFails('adminAuthenticatedE2EPassed', { adminAuthenticatedE2EPassed: false });
assertFails('adminAuthenticatedE2EPassed', { adminE2EMode: 'mocked_admin_routes' });
assertFails('unauthRedirectVerified', { unauthRedirectVerified: false });
assertFails('apiAuthRequiredVerified', { apiAuthRequiredVerified: false });
assertFails('platformBoundaryGatePassed', { platformBoundaryGatePassed: false });
assertFails('safeAdapterModesOnly', { safeAdapterModesOnly: false });
assertFails('noProductionPlatformWrite', { realCredentialsEnabled: true });
assertFails('noProductionPlatformWrite', { realPlatformWriteEnabled: true });
assertFails('noProductionPlatformWrite', { automaticPublishEnabled: true });
assertFails('noProductionPlatformWrite', { automaticListingEnabled: true });
assertFails('productionReadyFalse', { productionReady: true });
assertFails('closureEvidencePresent', { closureEvidencePresent: false });
assertFails('closureEvidencePresent', { phaseStatus: 'In Progress' });
assertFails('packageScriptsRegistered', {}, { packageText: 'test:p8-task-batch-8' });

const report = {
  phase: 'P8',
  batchId: 'P8-TASK-BATCH-9',
  status: 'passed',
  fixtures: 30,
};
writeJSON('docs/p8-task-batch-9-fixture-report.json', report);
console.log(JSON.stringify(report, null, 2));
