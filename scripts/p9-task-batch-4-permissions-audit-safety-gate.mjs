import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';

export const P9_BATCH_4_PERMISSIONS_AUDIT_SAFETY_JSON = 'docs/p9-task-batch-4-permissions-audit-safety.json';
export const P9_BATCH_4_PERMISSIONS_AUDIT_SAFETY_MD = 'docs/P9_TASK_BATCH_4_PERMISSIONS_AUDIT_SAFETY.md';
export const P9_BATCH_4_PERMISSIONS_AUDIT_SAFETY_GATE_JSON = 'docs/p9-task-batch-4-permissions-audit-safety-gate.json';
export const P9_BATCH_4_PERMISSIONS_AUDIT_SAFETY_GATE_MD = 'docs/P9_TASK_BATCH_4_PERMISSIONS_AUDIT_SAFETY_GATE.md';

const TASK_IDS = ['P9-801', 'P9-802', 'P9-803', 'P9-804'];

const REQUIRED_FILES = [
  'backend/internal/pkg/adminperm/matrix.go',
  'backend/internal/pkg/adminperm/perm_test.go',
  'backend/internal/modules/inventorysyncp9/rbac_authorizers.go',
  'backend/internal/modules/inventorysyncp9/rbac_authorizers_test.go',
  'backend/internal/modules/inventorysyncp9/audit.go',
  'backend/internal/modules/inventorysyncp9/audit_redaction_test.go',
  'backend/internal/modules/inventorysyncp9/redaction.go',
  'backend/internal/modules/inventorysyncp9/inventory_provider.go',
  'backend/internal/modules/inventorysyncp9/sync_orchestration.go',
  'backend/internal/modules/inventorysyncp9/manual_binding.go',
  'backend/internal/modules/operationlog/service.go',
  'backend/internal/config/validate.go',
  'backend/internal/config/validate_test.go',
  P9_BATCH_4_PERMISSIONS_AUDIT_SAFETY_MD,
  P9_BATCH_4_PERMISSIONS_AUDIT_SAFETY_JSON,
];

const FORBIDDEN_PATHS = [
  'admin/src/pages/inventorysyncp9',
  'admin/src/services/inventorysyncp9',
  'backend/internal/api/inventorysyncp9',
  'backend/internal/modules/inventorysyncp9/handler.go',
  'backend/internal/modules/inventorysyncp9/router.go',
  'backend/internal/modules/inventorysyncp9/worker.go',
  'backend/internal/modules/inventorysyncp9/cron.go',
  'backend/internal/providers/douyin/inventorysyncp9',
];

const REQUIRED_PERMISSIONS = [
  'inventory_sync.read',
  'inventory_sync.run',
  'inventory_sync.rerun',
  'inventory_snapshot.read',
  'sku_binding.read',
  'sku_binding.manage',
  'sku_binding.resolve_manual',
  'inventory_sync.audit.read',
];

const REQUIRED_AUDIT_ACTIONS = [
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

const REQUIRED_DANGEROUS_CAPABILITY_FIELDS = [
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

const REQUIRED_DANGEROUS_ENV_KEYS = [
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

const REPO_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

function rootPath(rel) {
  return path.join(REPO_ROOT, rel);
}

function readJSON(rel) {
  try {
    return JSON.parse(fs.readFileSync(rootPath(rel), 'utf8'));
  } catch {
    return null;
  }
}

function writeJSON(rel, data) {
  const full = rootPath(rel);
  fs.mkdirSync(path.dirname(full), { recursive: true });
  fs.writeFileSync(full, `${JSON.stringify(data, null, 2)}\n`, 'utf8');
}

function writeMarkdown(rel, body) {
  const full = rootPath(rel);
  fs.mkdirSync(path.dirname(full), { recursive: true });
  fs.writeFileSync(full, body, 'utf8');
}

function text(rel) {
  try {
    return fs.readFileSync(rootPath(rel), 'utf8');
  } catch {
    return '';
  }
}

function git(args) {
  try {
    return execFileSync('git', args, { cwd: REPO_ROOT, encoding: 'utf8' }).trim();
  } catch {
    return '';
  }
}

function stagedFileCount() {
  const files = git(['diff', '--cached', '--name-only']);
  return files ? files.split('\n').filter(Boolean).length : 0;
}

function workingTreeDirty() {
  return git(['status', '--short']) !== '';
}

function hasAll(value, needles) {
  return needles.every((needle) => value.includes(needle));
}

function arrayIncludes(values, expected) {
  const set = new Set(Array.isArray(values) ? values : []);
  return expected.every((item) => set.has(item));
}

function forbiddenPathsAbsent() {
  return FORBIDDEN_PATHS.every((rel) => !fs.existsSync(rootPath(rel)));
}

function forbiddenProductionSourceAbsent(value) {
  return !/(net\/http|http\.Client|gin\.Default|gin\.New|\.POST\(|\.GET\(|router|cron|ticker|queue|go\s+func|ProviderModeProduction|Authorization:\s*Bearer|Cookie:\s*[^"'`\s]+)/i.test(value);
}

function evidenceNoSecrets(evidence) {
  const secretPattern = /(secret-token|access_token|refresh_token|client_secret|app_secret|authorization:\s*|cookie:\s*|password=|api_key=|Bearer\s+)/i;
  const stack = [evidence];
  while (stack.length) {
    const current = stack.pop();
    if (typeof current === 'string') {
      if (secretPattern.test(current)) {
        return false;
      }
      continue;
    }
    if (Array.isArray(current)) {
      stack.push(...current);
      continue;
    }
    if (current && typeof current === 'object') {
      stack.push(...Object.values(current));
    }
  }
  return true;
}

export function validateP9Batch4PermissionsAuditSafetyBundle({ evidence = {}, sources = {}, gitState = {} } = {}) {
  const matrixText = sources.matrixText ?? text('backend/internal/pkg/adminperm/matrix.go');
  const permTestText = sources.permTestText ?? text('backend/internal/pkg/adminperm/perm_test.go');
  const rbacText = sources.rbacText ?? text('backend/internal/modules/inventorysyncp9/rbac_authorizers.go');
  const rbacTestText = sources.rbacTestText ?? text('backend/internal/modules/inventorysyncp9/rbac_authorizers_test.go');
  const auditText = sources.auditText ?? text('backend/internal/modules/inventorysyncp9/audit.go');
  const redactionText = sources.redactionText ?? text('backend/internal/modules/inventorysyncp9/redaction.go');
  const auditRedactionTestText = sources.auditRedactionTestText ?? text('backend/internal/modules/inventorysyncp9/audit_redaction_test.go');
  const providerText = sources.providerText ?? text('backend/internal/modules/inventorysyncp9/inventory_provider.go');
  const orchestratorText = sources.orchestratorText ?? text('backend/internal/modules/inventorysyncp9/sync_orchestration.go');
  const manualBindingText = sources.manualBindingText ?? text('backend/internal/modules/inventorysyncp9/manual_binding.go');
  const operationLogText = sources.operationLogText ?? text('backend/internal/modules/operationlog/service.go');
  const configText = sources.configText ?? text('backend/internal/config/validate.go');
  const configTestText = sources.configTestText ?? text('backend/internal/config/validate_test.go');
  const packageText = sources.packageText ?? text('package.json');
  const productionSource = [providerText, orchestratorText, manualBindingText, rbacText, auditText, redactionText, configText].join('\n');
  const authorizeRunIndex = orchestratorText.indexOf('o.authorizeRun');
  const providerResolveIndex = orchestratorText.indexOf('o.Registry.Resolve');
  const txCreateRunIndex = Math.max(orchestratorText.indexOf('tx.Create(run)'), orchestratorText.indexOf('Create(run)'));
  const repositoryCreateIndex = txCreateRunIndex > -1 ? txCreateRunIndex : orchestratorText.indexOf('Create(ctx');
  const missingFiles = sources.requiredFilesPresent === true ? [] : REQUIRED_FILES.filter((rel) => !fs.existsSync(rootPath(rel)));
  const branch = gitState.currentBranch ?? git(['branch', '--show-current']);
  const head = gitState.currentHead ?? git(['rev-parse', 'HEAD']);
  const staged = gitState.stagedFileCount ?? stagedFileCount();
  const dirty = gitState.workingTreeDirty ?? workingTreeDirty();

  const checks = [
    ['requiredFilesPresent', missingFiles.length === 0],
    ['batchId', evidence.batchId === 'P9-TASK-BATCH-4'],
    ['currentBranch', branch === 'dev'],
    ['currentHeadPresent', typeof head === 'string' && head.length > 0],
    ['stagedFileCount', staged === 0],
    ['workingTreeDirtyRecorded', evidence.workingTreeDirty === dirty],
    ['changesCommitted', evidence.changesCommitted === false],
    ...TASK_IDS.map((id) => [`${id} status`, String(evidence.tasks?.[id]?.status || '') === 'completed']),
    ['modulePathRecorded', evidence.modulePath === 'backend/internal/modules/inventorysyncp9'],
    ['existingInfrastructureReused', evidence.existingRBACReused === true && evidence.existingAuditInfrastructureReused === true && evidence.existingSecretRedactorReused === true],
    ['duplicateSecurityFrameworkNotCreated', evidence.duplicateSecurityFrameworkCreated === false && !productionSource.includes('type PermissionFramework') && !productionSource.includes('type SecurityFramework')],
    ['permissionMatrixImplemented', evidence.permissionMatrixImplemented === true && arrayIncludes(evidence.permissions, REQUIRED_PERMISSIONS) && hasAll(matrixText, REQUIRED_PERMISSIONS)],
    ['strictPermissionTestsPresent', evidence.strictPermissionTestsPassed === true && hasAll(permTestText, ['StrictHasPermission', 'inventory.run', 'unknown roles'])],
    ['rbacAuthorizerImplemented', evidence.rbacAuthorizerImplemented === true && hasAll(rbacText, ['type RBACAuthorizer struct', 'StrictHasPermission', 'admin.AdminUser', 'StatusActive', 'CanRunInventorySync', 'CanRerunInventorySync', 'CanResolveManualBinding'])],
    ['trustedActorAndTenantIsolation', evidence.trustedActorEnforced === true && evidence.tenantIsolationEnforced === true && hasAll(rbacText + manualBindingText, ['actor.TenantID', 'actor.ActorID', 'tenant_id = ?', 'ErrPermissionDenied'])],
    ['defaultDenyImplemented', evidence.defaultDenyImplemented === true && evidence.nilAuthorizerAllows === false && hasAll(orchestratorText + manualBindingText + rbacText, ['o.Authorizer == nil', 's.Authorizer == nil', 'ErrPermissionDenied'])],
    ['authorizationBeforeProviderAndMutation', evidence.authorizationBeforeProviderCall === true && evidence.authorizationBeforeRepositoryMutation === true && authorizeRunIndex > -1 && providerResolveIndex > -1 && repositoryCreateIndex > -1 && authorizeRunIndex < providerResolveIndex && authorizeRunIndex < repositoryCreateIndex],
    ['idempotencyCannotBypassAuthorization', evidence.idempotencyBypassesAuthorization === false && authorizeRunIndex > -1 && orchestratorText.indexOf('IdempotencyKeyHash') > -1 && authorizeRunIndex < providerResolveIndex],
    ['deniedSideEffectsPrevented', evidence.permissionDeniedProviderCallCount === 0 && evidence.permissionDeniedRepositoryMutationCount === 0 && hasAll(rbacTestText + auditRedactionTestText, ['ErrPermissionDenied', 'CanRunInventorySync', 'inventory_sync.permission_denied'])],
    ['auditServiceImplemented', evidence.auditServiceImplemented === true && hasAll(auditText, ['type InventorySyncAuditService struct', 'operationlog.Service', 'WriteBackground', 'safeAuditMessage'])],
    ['auditActionsImplemented', evidence.auditEventsImplemented === true && arrayIncludes(evidence.auditActions, REQUIRED_AUDIT_ACTIONS) && hasAll(auditText + orchestratorText + manualBindingText, REQUIRED_AUDIT_ACTIONS)],
    ['auditDeliveryReliable', evidence.auditDeliveryMode === 'transactional' && evidence.auditLossPreventionPresent === true && evidence.auditFireAndForgetPresent === false && evidence.auditErrorsIgnored === false && hasAll(orchestratorText + manualBindingText, ['Transaction(func(tx *gorm.DB) error', 'writeAuditWithDB']) && !/go\s+.*WriteBackground|_\s*=\s*.*WriteBackground/.test(productionSource)],
    ['operationLogBackgroundFields', evidence.operationLogBackgroundFieldsPreserved === true && hasAll(operationLogText, ['RequestID', 'ShopID', 'Platform', 'Permission', 'strings.TrimSpace(opts.RequestID)'])],
    ['redactionImplemented', evidence.metadataRedactionImplemented === true && evidence.cursorRawLogged === false && hasAll(redactionText, ['safefields.RedactValue', 'safeInventorySyncMetadataJSON', 'safeProviderMetadataJSON', 'safeManualBindingComment', 'safeCursorHash'])],
    ['metadataAllowlistImplemented', evidence.auditMetadataAllowlistImplemented === true && evidence.arbitraryMetadataPassthrough === false && hasAll(redactionText, ['inventorySyncAllowedMetadataKeys', 'payloadHash', 'cursorHash', 'safeMessage'])],
    ['redactionTestsPresent', evidence.redactionTestsPassed === true && hasAll(auditRedactionTestText, ['TestInventorySyncRedactionAllowlistsAndHashesMetadata', 'secret-token', 'safeCursorHash', 'NotContains'])],
    ['providerCapabilityGuardImplemented', evidence.providerCapabilityGuardImplemented === true && evidence.unsafeProviderCapabilitiesRejected === true && hasAll(providerText, REQUIRED_DANGEROUS_CAPABILITY_FIELDS) && hasAll(providerText, ['ErrProductionCapabilityForbidden', 'ErrProviderCapabilityForbidden'])],
    ['productionModesRejected', evidence.productionProviderModesRejected === true && hasAll(providerText, ['production', 'prod', 'real', 'live', 'online', 'remote', 'oauth'])],
    ['configSafetyValidationImplemented', evidence.configSafetyValidationImplemented === true && hasAll(configText, ['validateP9InventorySyncSafety', 'ErrCodeP9ProductionCapabilityForbidden']) && hasAll(configText, REQUIRED_DANGEROUS_ENV_KEYS) && hasAll(configTestText, ['TestValidateP9InventorySyncSafetyRejectsDangerousEnv'])],
    ['credentialBoundaryEnforced', evidence.realCredentialsEnabled === false && evidence.credentialsInputRejected === true && hasAll(configText + providerText, ['INVENTORY_SYNC_ACCESS_TOKEN', 'AcceptsCredentialPayload', 'RealCredentials'])],
    ['networkAndMutationBoundaryPreserved', evidence.realPlatformNetworkEnabled === false && evidence.realPlatformReadEnabled === false && evidence.realPlatformWriteEnabled === false && evidence.realInventoryReadEnabled === false && evidence.realInventoryWriteEnabled === false && evidence.inventoryMutationEnabled === false && evidence.p9ReachableInventoryMutationCount === 0],
    ['automaticExecutionBoundaryPreserved', evidence.syncWorkerImplemented === false && evidence.cronImplemented === false && evidence.tickerImplemented === false && evidence.queueConsumerImplemented === false && evidence.automaticRetryImplemented === false && evidence.backgroundSyncWorkerPresent === false && evidence.automaticRetryWorkerPresent === false],
    ['testsPresent', evidence.testsPassed === true && hasAll(rbacTestText + auditRedactionTestText + configTestText, ['TestRBACAuthorizer', 'TestInventorySyncAuditPermissionDeniedAndLifecycle', 'TestManualBindingAuditAndCommentRedaction', 'TestValidateP9InventorySyncSafetyRejectsDangerousEnv'])],
    ['raceTestsPassed', evidence.raceTestsPassed === true],
    ['postgresRecordedTruthfully', evidence.postgresIntegrationStatus === 'not_run' && evidence.postgresIntegrationPassed === false && evidence.postgresIntegrationDeferredTo === 'P9_Final_Development_Closure' && evidence.p9FinalClosureBlocker === true],
    ['packageScriptsPresent', hasAll(packageText, ['test:p9-task-batch-4-permissions-audit-safety', 'p9:task-batch-4-permissions-audit-safety-gate'])],
    ['forbiddenPathsAbsent', forbiddenPathsAbsent()],
    ['noForbiddenProductionSource', forbiddenProductionSourceAbsent(productionSource)],
    ['scopeProtectionFlags', evidence.apiImplemented === false && evidence.httpHandlerImplemented === false && evidence.ginRouterImplemented === false && evidence.restApiImplemented === false && evidence.adminUiImplemented === false && evidence.frontendApiClientImplemented === false],
    ['realBoundaryFlags', evidence.realDouyinProviderImplemented === false && evidence.oauthImplemented === false && evidence.realCredentialsEnabled === false && evidence.realPlatformNetworkEnabled === false && evidence.realPlatformReadEnabled === false && evidence.realPlatformWriteEnabled === false && evidence.realInventoryReadEnabled === false && evidence.realInventoryWriteEnabled === false],
    ['evidenceNoSecrets', evidenceNoSecrets(evidence)],
    ['p10BoundaryPreserved', evidence.p10BoundaryPreserved === true],
    ['productionReady', evidence.productionReady === false],
    ['p9Complete', evidence.p9Complete === false],
  ];

  const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failed.length ? 'failed' : 'passed',
    failed,
    failedCount: failed.length,
    missingFiles,
    currentBranch: branch,
    currentHead: head,
    stagedFileCount: staged,
    workingTreeDirty: dirty,
    checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  };
}

export function buildP9Batch4PermissionsAuditSafetyGateReport(bundle = {}) {
  const evidence = bundle.evidence ?? readJSON(P9_BATCH_4_PERMISSIONS_AUDIT_SAFETY_JSON) ?? {};
  const validation = validateP9Batch4PermissionsAuditSafetyBundle({ evidence, sources: bundle.sources, gitState: bundle.gitState });
  return {
    phase: 'P9',
    gate: 'P9-TASK-BATCH-4-PERMISSIONS-AUDIT-SAFETY',
    status: validation.status,
    checkedAt: new Date().toISOString(),
    batchId: evidence.batchId || '',
    currentBranch: validation.currentBranch,
    currentHead: validation.currentHead,
    stagedFileCount: validation.stagedFileCount,
    workingTreeDirty: validation.workingTreeDirty,
    tasks: TASK_IDS,
    modulePath: evidence.modulePath || '',
    existingRBACReused: evidence.existingRBACReused === true,
    existingAuditInfrastructureReused: evidence.existingAuditInfrastructureReused === true,
    existingSecretRedactorReused: evidence.existingSecretRedactorReused === true,
    duplicateSecurityFrameworkCreated: evidence.duplicateSecurityFrameworkCreated === true,
    permissionMatrixImplemented: evidence.permissionMatrixImplemented === true,
    rbacAuthorizerImplemented: evidence.rbacAuthorizerImplemented === true,
    defaultDenyImplemented: evidence.defaultDenyImplemented === true,
    tenantIsolationEnforced: evidence.tenantIsolationEnforced === true,
    auditServiceImplemented: evidence.auditServiceImplemented === true,
    auditDeliveryMode: evidence.auditDeliveryMode || '',
    auditFireAndForgetPresent: evidence.auditFireAndForgetPresent === true,
    metadataRedactionImplemented: evidence.metadataRedactionImplemented === true,
    providerCapabilityGuardImplemented: evidence.providerCapabilityGuardImplemented === true,
    configSafetyValidationImplemented: evidence.configSafetyValidationImplemented === true,
    postgresIntegrationStatus: evidence.postgresIntegrationStatus || '',
    syncWorkerImplemented: evidence.syncWorkerImplemented === true,
    cronImplemented: evidence.cronImplemented === true,
    tickerImplemented: evidence.tickerImplemented === true,
    queueConsumerImplemented: evidence.queueConsumerImplemented === true,
    apiImplemented: evidence.apiImplemented === true,
    adminUiImplemented: evidence.adminUiImplemented === true,
    realDouyinProviderImplemented: evidence.realDouyinProviderImplemented === true,
    realPlatformNetworkEnabled: evidence.realPlatformNetworkEnabled === true,
    inventoryMutationEnabled: evidence.inventoryMutationEnabled === true,
    p10BoundaryPreserved: evidence.p10BoundaryPreserved === true,
    productionReady: evidence.productionReady === true,
    p9Complete: evidence.p9Complete === true,
    failedCount: validation.failedCount,
    failed: validation.failed,
    missingFiles: validation.missingFiles,
    checks: validation.checks,
  };
}

export function writeP9Batch4PermissionsAuditSafetyGateReport(report) {
  writeJSON(P9_BATCH_4_PERMISSIONS_AUDIT_SAFETY_GATE_JSON, report);
  writeMarkdown(
    P9_BATCH_4_PERMISSIONS_AUDIT_SAFETY_GATE_MD,
    `# P9 Batch 4 Permissions, Audit and Safety Gate

Status: **${report.status}**

- Batch id: ${report.batchId}
- Current branch: ${report.currentBranch}
- Current head: ${report.currentHead}
- Staged files: ${report.stagedFileCount}
- Working tree dirty: ${report.workingTreeDirty}
- Tasks: ${report.tasks.join(', ')}
- Module path: ${report.modulePath}
- Existing RBAC reused: ${report.existingRBACReused}
- Existing audit infrastructure reused: ${report.existingAuditInfrastructureReused}
- Existing secret redactor reused: ${report.existingSecretRedactorReused}
- Duplicate security framework created: ${report.duplicateSecurityFrameworkCreated}
- Permission matrix implemented: ${report.permissionMatrixImplemented}
- RBAC authorizer implemented: ${report.rbacAuthorizerImplemented}
- Default deny implemented: ${report.defaultDenyImplemented}
- Tenant isolation enforced: ${report.tenantIsolationEnforced}
- Audit service implemented: ${report.auditServiceImplemented}
- Audit delivery mode: ${report.auditDeliveryMode}
- Audit fire-and-forget present: ${report.auditFireAndForgetPresent}
- Metadata redaction implemented: ${report.metadataRedactionImplemented}
- Provider capability guard implemented: ${report.providerCapabilityGuardImplemented}
- Config safety validation implemented: ${report.configSafetyValidationImplemented}
- PostgreSQL integration status: ${report.postgresIntegrationStatus}
- Sync worker implemented: ${report.syncWorkerImplemented}
- Cron implemented: ${report.cronImplemented}
- Ticker implemented: ${report.tickerImplemented}
- Queue consumer implemented: ${report.queueConsumerImplemented}
- API implemented: ${report.apiImplemented}
- Admin UI implemented: ${report.adminUiImplemented}
- Real Douyin provider implemented: ${report.realDouyinProviderImplemented}
- Real platform network enabled: ${report.realPlatformNetworkEnabled}
- Inventory mutation enabled: ${report.inventoryMutationEnabled}
- P10 boundary preserved: ${report.p10BoundaryPreserved}
- Production Ready: ${report.productionReady}
- P9 complete: ${report.p9Complete}
- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}

This gate validates only P9 Batch 4 inventory sync permissions, audit, redaction, and safety boundaries. It does not authorize Backend APIs, Admin UI, workers, cron/tickers, queues, automatic retry, real Douyin/OAuth/credentials, real platform network, inventory mutation, release, tag, P9 closure, or Production Ready.
`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildP9Batch4PermissionsAuditSafetyGateReport();
  writeP9Batch4PermissionsAuditSafetyGateReport(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}
