import assert from 'node:assert/strict';
import { validateP9Batch5BackendAPIsBundle } from '../../../scripts/p9-task-batch-5-backend-apis-gate.mjs';

const tasks = Object.fromEntries(['P9-901', 'P9-902', 'P9-903', 'P9-904', 'P9-905'].map((id) => [id, { status: 'completed' }]));
const routes = [
  'POST("/runs"', 'GET("/runs"', 'GET("/runs/:runId"', 'POST("/runs/:runId/rerun"',
  'GET("/runs/:runId/snapshots"', 'GET("/snapshots/:snapshotId"', 'GET("/bindings"',
  'GET("/bindings/:bindingId"', 'GET("/manual-binding-requests"',
  'POST("/manual-binding-requests/:requestId/confirm"', 'POST("/manual-binding-requests/:requestId/reject"',
  'GET("/runs/:runId/audit-events"',
];
const permissions = [
  'inventory_sync.read',
  'inventory_sync.run',
  'inventory_sync.rerun',
  'inventory_snapshot.read',
  'sku_binding.read',
  'sku_binding.manage',
  'sku_binding.resolve_manual',
  'inventory_sync.audit.read',
];
const evidence = {
  batchId: 'P9-TASK-BATCH-5', changesCommitted: false, workingTreeDirty: true,
  tasks, apiImplemented: true, httpHandlerImplemented: true, restApiImplemented: true,
  authenticatedRoutesPresent: true, routerRegisteredOnce: true,
  existingRouterReused: true, existingAuthMiddlewareReused: true, existingTenantContextReused: true,
  existingResponseEnvelopeReused: true, existingPaginationReused: true,
  thinHandlersPresent: true, apiServiceFacadePresent: true, handlersDirectRepositoryAccess: false,
  strictJSONBodyLimit: true, unknownFieldsRejected: true, contentTypeValidated: true,
  trustedActorAndTenantIsolation: true, trustedTenantContextUsed: true, trustedActorContextUsed: true,
  callerSuppliedTenantTrusted: false, callerSuppliedActorTrusted: false, callerSuppliedRoleTrusted: false,
  writesRequireIdempotencyKey: true, requestIdPropagated: true,
  keysetPagination: true, offsetPaginationAbsent: true, paginationDuplicates: 0, paginationOmissions: 0,
  explicitSafeDTOs: true, rawDomainModelsNotReturned: true, safeErrorMappingPresent: true,
  rawProviderErrorExposed: false, rawAuditMetadataExposed: false, rawCursorExposed: false,
  rawIdempotencyKeyExposed: false,
  syncRunApisPresent: true, inventorySnapshotApisPresent: true, skuBindingApisPresent: true,
  calibrationApisPresent: true, manualBindingApisPresent: true, syncHistoryApisPresent: true,
  auditTimelineApisPresent: true,
  allowedActionsImplemented: true, allowedActionsServerComputed: true, resourceOperationsReauthorize: true,
  existingRBACReused: true, domainServicesReused: true,
  controlledRecalibrationHistory: true, auditTimelineImplemented: true, readonlySnapshotAndBindingHistory: true,
  realDouyinProviderImplemented: false, productionProviderModesCallable: false, oauthImplemented: false,
  realCredentialsEnabled: false, realCredentialsExecutable: false, realNetworkEnabled: false,
  realPlatformReadEnabled: false, realPlatformWriteEnabled: false, inventoryMutationEnabled: false,
  workerImplemented: false, backgroundSyncWorkerImplemented: false, automaticRetryWorkerImplemented: false,
  adminUiImplemented: false, testsPassed: true, permissionTestsPassed: true, tenantIsolationTestsPassed: true,
  idempotencyTestsPassed: true, paginationTestsPassed: true, apiContractTestsPassed: true,
  concurrencyTestsPassed: true, raceTestsPassed: true, dataRaces: 0, sqliteIntegrationTestsPassed: true,
  postgresIntegrationStatus: 'not_run', postgresIntegrationPassed: false, p9FinalClosureBlocker: true,
  p10BoundaryPreserved: true, productionReady: false,
};
const sources = {
  dto: 'AllowedActions CanRerun CanConfirm CanReject',
  matrix: permissions.join('\n'),
  service: 'Orchestrator.Run Orchestrator.ManualRerun Calibration.recalibrateSnapshotItemWithDB ManualBinding.ConfirmBinding ManualBinding.RejectBinding p9.inventory-sync.recalibrate ResponseSummary inventory_sync.read inventory_sync.run inventory_sync.rerun inventory_snapshot.read sku_binding.read sku_binding.manage sku_binding.resolve_manual inventory_sync.audit.read',
  handler: 'TenantIDFromGin AdminID Idempotency-Key TraceID tenant_id = ? ErrNotFound',
  validation: 'Idempotency-Key IdempotencyKeyHash TraceID',
  repository: 'DecodeCursor ApplyDescKeyset BuildNextCursor Limit(params.Limit + 1) operationlog.OperationLog ListRunAuditEvents',
  router: `func Register ${routes.join(' ')}`,
  strictJSON: 'MaxBytesReader DisallowUnknownFields application/json',
  tests: 'TestInventorySyncAPIRejectsUnknownFieldsAndCredentials TestInventorySyncAPIKeysetAndTenantIsolation TestInventorySyncAPIRoleAndProductionCaps TestInventorySyncAPIAuthRequired',
  applicationRouter: 'authed inventorysyncp9.Register(authed, inventorySyncP9H)',
  packageJSON: 'test:p9-task-batch-5-backend-apis p9:task-batch-5-backend-apis-gate test:p9-task-batch-5 p9:task-batch-5-gate',
};

function validate(overrides = {}, sourceOverrides = {}, gitOverrides = {}) {
  return validateP9Batch5BackendAPIsBundle({
    evidence: { ...evidence, ...overrides },
    sources: { ...sources, ...sourceOverrides },
    gitState: {
      currentBranch: 'dev',
      currentHead: 'abc123',
      stagedFileCount: 0,
      workingTreeDirty: true,
      ...gitOverrides,
    },
  });
}

function expectFailed(check, overrides = {}, sourceOverrides = {}, gitOverrides = {}) {
  const result = validate(overrides, sourceOverrides, gitOverrides);
  assert.equal(result.status, 'failed');
  assert.ok(result.failed.includes(check), `${check} should fail; actual failures: ${result.failed.join(', ')}`);
}

const valid = validate();
assert.equal(valid.status, 'passed');

// B5-02 through B5-21 are negative fixtures; B5-01 is the valid bundle above.
expectFailed('thinHandlers', { handlersDirectRepositoryAccess: true });
expectFailed('authenticatedRoutes', { authenticatedRoutesPresent: false });
expectFailed('callerSuppliedContextRejected', { callerSuppliedTenantTrusted: true });
expectFailed('callerSuppliedContextRejected', { callerSuppliedActorTrusted: true, callerSuppliedRoleTrusted: true });
expectFailed('idempotencyAndRequestId', { writesRequireIdempotencyKey: false });
expectFailed('strictJSONContract', { unknownFieldsRejected: false });
expectFailed('strictJSONContract', { strictJSONBodyLimit: false });
expectFailed('keysetPagination', { keysetPagination: false, offsetPaginationAbsent: false });
expectFailed('safeDTO', { rawDomainModelsNotReturned: false });
expectFailed('safeOutputRedaction', { rawCursorExposed: true });
expectFailed('safeOutputRedaction', { rawIdempotencyKeyExposed: true });
expectFailed('safeOutputRedaction', { rawAuditMetadataExposed: true });
expectFailed('safeOutputRedaction', { rawProviderErrorExposed: true });
expectFailed('productionProviderGuard', { productionProviderModesCallable: true });
expectFailed('noRealCredentialsOrOAuth', { realCredentialsExecutable: true });
expectFailed('noMutationRoutes', { inventoryMutationEnabled: true });
expectFailed('noRealProviderWorkerUI', { adminUiImplemented: true });
expectFailed('noRealProviderWorkerUI', { workerImplemented: true, backgroundSyncWorkerImplemented: true });
expectFailed('p10BoundaryPreserved', { productionReady: true });
expectFailed('stagedFileCount', {}, {}, { stagedFileCount: 1 });

// Additional regression guards retained from the implementation gate.
expectFailed('changesCommitted', { changesCommitted: true });
expectFailed('strictJSONContract', {}, { strictJSON: 'MaxBytesReader' });
expectFailed('noMutationRoutes', {}, { router: `${sources.router} .PATCH("/snapshots/:snapshotId"` });
console.log('p9 task batch 5 backend api fixtures passed');
