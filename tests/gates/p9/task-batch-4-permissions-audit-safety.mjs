import assert from 'node:assert/strict';
import { writeJSON } from '../../../scripts/p7-v2-lib.mjs';
import { validateP9Batch4PermissionsAuditSafetyBundle } from '../../../scripts/p9-task-batch-4-permissions-audit-safety-gate.mjs';

const TASK_IDS = ['P9-801', 'P9-802', 'P9-803', 'P9-804'];
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
const auditActions = [
  'inventory_sync.run_created',
  'inventory_sync.started',
  'inventory_sync.page_processed',
  'inventory_sync.completed',
  'inventory_sync.failed',
  'inventory_sync.permission_denied',
  'inventory_sync.production_capability_blocked',
  'sku_binding.manual_confirmed',
  'sku_binding.manual_rejected',
];
const dangerousCapabilityFields = [
  'NetworkAccess',
  'OAuth',
  'Credentials',
  'RealCredentials',
  'RealPlatformRead',
  'RealPlatformWrite',
  'RealInventoryRead',
  'RealInventoryWrite',
  'InventoryMutation',
  'AutomaticExecution',
  'AutomaticRetry',
  'BackgroundWorker',
  'AcceptsCredentialPayload',
];
const dangerousEnvKeys = [
  'REAL_DOUYIN_ENABLED',
  'REAL_PLATFORM_READ',
  'REAL_PLATFORM_WRITE',
  'REAL_INVENTORY_READ',
  'REAL_INVENTORY_WRITE',
  'INVENTORY_MUTATION_ENABLED',
  'AUTO_INVENTORY_SYNC',
  'AUTO_RETRY',
  'INVENTORY_SYNC_AUTO_RETRY',
  'INVENTORY_SYNC_BACKGROUND_WORKER_ENABLED',
  'INVENTORY_SYNC_NETWORK_ACCESS',
  'INVENTORY_SYNC_ACCESS_TOKEN',
  'INVENTORY_SYNC_REFRESH_TOKEN',
  'INVENTORY_SYNC_OAUTH_CODE',
  'INVENTORY_SYNC_AUTHORIZATION',
  'INVENTORY_SYNC_COOKIE',
  'INVENTORY_SYNC_CLIENT_SECRET',
  'INVENTORY_SYNC_APP_SECRET',
  'INVENTORY_SYNC_PASSWORD',
  'INVENTORY_SYNC_API_KEY',
  'DOUYIN_INVENTORY_ACCESS_TOKEN',
  'DOUYIN_INVENTORY_REFRESH_TOKEN',
  'DOUYIN_INVENTORY_CLIENT_SECRET',
  'DOUYIN_INVENTORY_APP_SECRET',
  'INVENTORY_SYNC_PROVIDER_MODE',
  'DOUYIN_INVENTORY_PROVIDER_MODE',
];

const validSources = {
  requiredFilesPresent: true,
  matrixText: `${permissions.join('\n')}\n`,
  permTestText: 'StrictHasPermission inventory.run unknown roles',
  rbacText: `
    type RBACAuthorizer struct {}
    StrictHasPermission admin.AdminUser StatusActive
    CanRunInventorySync CanRerunInventorySync CanResolveManualBinding
    tenant_id = ? ErrPermissionDenied
  `,
  rbacTestText: 'TestRBACAuthorizer CanRunInventorySync ErrPermissionDenied',
  auditText: `
    type InventorySyncAuditService struct {}
    operationlog.Service WriteBackground safeAuditMessage
    ${auditActions.join('\n')}
  `,
  redactionText: `
    safefields.RedactValue safeInventorySyncMetadataJSON safeProviderMetadataJSON safeManualBindingComment safeCursorHash
    inventorySyncAllowedMetadataKeys payloadHash cursorHash safeMessage
  `,
  auditRedactionTestText: `
    TestInventorySyncRedactionAllowlistsAndHashesMetadata
    TestInventorySyncAuditPermissionDeniedAndLifecycle
    TestManualBindingAuditAndCommentRedaction
    secret-token safeCursorHash NotContains inventory_sync.permission_denied ErrPermissionDenied
  `,
  providerText: `
    ${dangerousCapabilityFields.join('\n')}
    ErrProductionCapabilityForbidden ErrProviderCapabilityForbidden
    production prod real live online remote oauth
    INVENTORY_SYNC_ACCESS_TOKEN
  `,
  orchestratorText: `
    o.Authorizer == nil
    o.authorizeRun()
    o.Registry.Resolve()
    Create(ctx)
    IdempotencyKeyHash
    Transaction(func(tx *gorm.DB) error
    writeAuditWithDB
    inventory_sync.run_created inventory_sync.started inventory_sync.page_processed inventory_sync.completed inventory_sync.failed inventory_sync.permission_denied inventory_sync.production_capability_blocked
  `,
  manualBindingText: `
    s.Authorizer == nil
    actor.TenantID actor.ActorID tenant_id = ? ErrPermissionDenied
    Transaction(func(tx *gorm.DB) error writeAuditWithDB
    sku_binding.manual_confirmed sku_binding.manual_rejected
  `,
  operationLogText: 'RequestID string ShopID Platform Permission strings.TrimSpace(opts.RequestID)',
  configText: `validateP9InventorySyncSafety ErrCodeP9ProductionCapabilityForbidden ${dangerousEnvKeys.join(' ')}`,
  configTestText: 'TestValidateP9InventorySyncSafetyRejectsDangerousEnv',
  packageText: 'test:p9-task-batch-4-permissions-audit-safety p9:task-batch-4-permissions-audit-safety-gate',
};

function taskEvidence(status = 'completed') {
  return TASK_IDS.reduce((acc, id) => {
    acc[id] = { status };
    return acc;
  }, {});
}

function validEvidence(overrides = {}) {
  return {
    batchId: 'P9-TASK-BATCH-4',
    batchName: 'Permissions, Audit and Safety',
    workingBranch: 'dev',
    changesCommitted: false,
    stagedFileCount: 0,
    workingTreeDirty: true,
    modulePath: 'backend/internal/modules/inventorysyncp9',
    tasks: taskEvidence(),
    existingRBACReused: true,
    existingAuditInfrastructureReused: true,
    existingSecretRedactorReused: true,
    duplicateSecurityFrameworkCreated: false,
    permissionMatrixImplemented: true,
    permissions,
    strictPermissionTestsPassed: true,
    rbacAuthorizerImplemented: true,
    trustedActorEnforced: true,
    tenantIsolationEnforced: true,
    defaultDenyImplemented: true,
    nilAuthorizerAllows: false,
    authorizationBeforeProviderCall: true,
    authorizationBeforeRepositoryMutation: true,
    idempotencyBypassesAuthorization: false,
    permissionDeniedProviderCallCount: 0,
    permissionDeniedRepositoryMutationCount: 0,
    auditServiceImplemented: true,
    auditEventsImplemented: true,
    auditActions,
    auditDeliveryMode: 'transactional',
    auditLossPreventionPresent: true,
    auditFireAndForgetPresent: false,
    auditErrorsIgnored: false,
    operationLogBackgroundFieldsPreserved: true,
    metadataRedactionImplemented: true,
    auditMetadataAllowlistImplemented: true,
    arbitraryMetadataPassthrough: false,
    cursorRawLogged: false,
    redactionTestsPassed: true,
    providerCapabilityGuardImplemented: true,
    unsafeProviderCapabilitiesRejected: true,
    productionProviderModesRejected: true,
    configSafetyValidationImplemented: true,
    credentialsInputRejected: true,
    testsPassed: true,
    raceTestsPassed: true,
    sqliteIntegrationTestsPassed: true,
    postgresIntegrationStatus: 'not_run',
    postgresIntegrationPassed: false,
    postgresIntegrationDeferredTo: 'P9_Final_Development_Closure',
    p9FinalClosureBlocker: true,
    syncWorkerImplemented: false,
    cronImplemented: false,
    tickerImplemented: false,
    queueConsumerImplemented: false,
    automaticRetryImplemented: false,
    backgroundSyncWorkerPresent: false,
    automaticRetryWorkerPresent: false,
    apiImplemented: false,
    httpHandlerImplemented: false,
    ginRouterImplemented: false,
    restApiImplemented: false,
    adminUiImplemented: false,
    frontendApiClientImplemented: false,
    realDouyinProviderImplemented: false,
    oauthImplemented: false,
    realCredentialsEnabled: false,
    realPlatformNetworkEnabled: false,
    realPlatformReadEnabled: false,
    realPlatformWriteEnabled: false,
    realInventoryReadEnabled: false,
    realInventoryWriteEnabled: false,
    inventoryMutationEnabled: false,
    p9ReachableInventoryMutationCount: 0,
    p10BoundaryPreserved: true,
    productionReady: false,
    p9Complete: false,
    ...overrides,
  };
}

function validate(overrides = {}, sourceOverrides = {}, gitOverrides = {}) {
  return validateP9Batch4PermissionsAuditSafetyBundle({
    evidence: validEvidence(overrides),
    sources: { ...validSources, ...sourceOverrides },
    gitState: { currentBranch: 'dev', currentHead: 'abc123', stagedFileCount: 0, workingTreeDirty: true, ...gitOverrides },
  });
}

function assertFails(fixtureId, id, overrides = {}, sourceOverrides = {}, gitOverrides = {}) {
  const result = validate(overrides, sourceOverrides, gitOverrides);
  assert.equal(result.status, 'failed', fixtureId);
  assert.ok(result.failed.includes(id), `${fixtureId} expected ${id}, saw ${result.failed.join(', ')}`);
}

assert.equal(validate().status, 'passed', 'B4-01 complete Batch 4 passes');
assertFails('B4-02 duplicate RBAC framework fails', 'duplicateSecurityFrameworkNotCreated', { duplicateSecurityFrameworkCreated: true });
assertFails('B4-03 missing permission matrix fails', 'permissionMatrixImplemented', { permissionMatrixImplemented: false });
assertFails('B4-04 missing authorizer default allow fails', 'defaultDenyImplemented', { nilAuthorizerAllows: true });
assertFails('B4-05 caller role trust fails', 'trustedActorAndTenantIsolation', { trustedActorEnforced: false });
assertFails('B4-06 cross-tenant admin operation fails', 'trustedActorAndTenantIsolation', { tenantIsolationEnforced: false });
assertFails('B4-07 permission denied provider call fails', 'deniedSideEffectsPrevented', { permissionDeniedProviderCallCount: 1 });
assertFails('B4-08 permission denied repository write fails', 'deniedSideEffectsPrevented', { permissionDeniedRepositoryMutationCount: 1 });
assertFails('B4-09 audit fire-and-forget fails', 'auditDeliveryReliable', { auditFireAndForgetPresent: true });
assertFails('B4-10 audit silently lost fails', 'auditDeliveryReliable', { auditLossPreventionPresent: false });
assertFails('B4-11 arbitrary audit metadata passthrough fails', 'metadataAllowlistImplemented', { arbitraryMetadataPassthrough: true });
assertFails('B4-12 provider metadata secret leak fails', 'redactionImplemented', { metadataRedactionImplemented: false });
assertFails('B4-13 raw cursor logging fails', 'redactionImplemented', { cursorRawLogged: true });
assertFails('B4-14 capability guard missing fails', 'providerCapabilityGuardImplemented', { providerCapabilityGuardImplemented: false });
assertFails('B4-15 NetworkAccess runnable fails', 'networkAndMutationBoundaryPreserved', { realPlatformNetworkEnabled: true });
assertFails('B4-16 RealPlatformRead runnable fails', 'networkAndMutationBoundaryPreserved', { realPlatformReadEnabled: true });
assertFails('B4-17 InventoryMutation runnable fails', 'networkAndMutationBoundaryPreserved', { inventoryMutationEnabled: true });
assertFails('B4-18 credentials input still executes fails', 'credentialBoundaryEnforced', { credentialsInputRejected: false });
assertFails('B4-19 background sync worker exists fails', 'automaticExecutionBoundaryPreserved', { backgroundSyncWorkerPresent: true });
assertFails('B4-20 API/Admin UI premature implementation fails', 'scopeProtectionFlags', { apiImplemented: true });
assertFails('B4-21 production mode fallback fails', 'productionModesRejected', { productionProviderModesRejected: false });
assertFails('B4-22 config safety validation missing fails', 'configSafetyValidationImplemented', { configSafetyValidationImplemented: false });
assertFails('B4-23 audit action missing fails', 'auditActionsImplemented', { auditActions: auditActions.filter((action) => action !== 'inventory_sync.permission_denied') });
assertFails('B4-24 idempotency bypass authorization fails', 'idempotencyCannotBypassAuthorization', { idempotencyBypassesAuthorization: true });
assertFails('B4-25 PostgreSQL must be recorded truthfully', 'postgresRecordedTruthfully', { postgresIntegrationStatus: 'passed', postgresIntegrationPassed: true });
assertFails('B4-26 evidence cannot contain secrets', 'evidenceNoSecrets', { safeMarker: 'Bearer secret-token' });
assertFails('Batch 4 task IDs cannot be renamed', 'P9-801 status', { tasks: { ...taskEvidence(), 'P9-801': { status: 'renamed' } } });
assertFails('Package scripts are required', 'packageScriptsPresent', {}, { packageText: '' });
assertFails('P10 boundary must stay preserved', 'p10BoundaryPreserved', { p10BoundaryPreserved: false });
assertFails('Production Ready must stay false', 'productionReady', { productionReady: true });
assertFails('P9 complete must stay false', 'p9Complete', { p9Complete: true });

const report = {
  phase: 'P9',
  batchId: 'P9-TASK-BATCH-4',
  gate: 'P9-TASK-BATCH-4-PERMISSIONS-AUDIT-SAFETY-FIXTURE',
  status: 'passed',
  fixtureAssertions: 31,
};
writeJSON('docs/p9-task-batch-4-permissions-audit-safety-fixture-report.json', report);
console.log(JSON.stringify(report, null, 2));
