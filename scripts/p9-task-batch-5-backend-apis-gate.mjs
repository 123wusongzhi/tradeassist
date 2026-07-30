import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

export const P9_BATCH_5_JSON = 'docs/p9-task-batch-5-backend-apis.json';
export const P9_BATCH_5_MD = 'docs/P9_TASK_BATCH_5_BACKEND_APIS.md';
export const P9_BATCH_5_GATE_JSON = 'docs/p9-task-batch-5-backend-apis-gate.json';
export const P9_BATCH_5_GATE_MD = 'docs/P9_TASK_BATCH_5_BACKEND_APIS_GATE.md';

const TASK_IDS = ['P9-901', 'P9-902', 'P9-903', 'P9-904', 'P9-905'];
const REQUIRED_FILES = [
  'backend/internal/pkg/httpapi/strict_json.go',
  'backend/internal/modules/inventorysyncp9/api_dto.go',
  'backend/internal/modules/inventorysyncp9/api_errors.go',
  'backend/internal/modules/inventorysyncp9/api_validation.go',
  'backend/internal/modules/inventorysyncp9/api_repository.go',
  'backend/internal/modules/inventorysyncp9/api_service.go',
  'backend/internal/modules/inventorysyncp9/api_handler.go',
  'backend/internal/modules/inventorysyncp9/api_router.go',
  'backend/internal/modules/inventorysyncp9/api_handler_test.go',
  'backend/internal/pkg/httpapi/strict_json_test.go',
  'backend/internal/api/router.go',
  P9_BATCH_5_MD,
  P9_BATCH_5_JSON,
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
const ROUTES = [
  'POST("/runs"',
  'GET("/runs"',
  'GET("/runs/:runId"',
  'POST("/runs/:runId/rerun"',
  'GET("/runs/:runId/snapshots"',
  'GET("/snapshots/:snapshotId"',
  'GET("/bindings"',
  'GET("/bindings/:bindingId"',
  'GET("/manual-binding-requests"',
  'POST("/manual-binding-requests/:requestId/confirm"',
  'POST("/manual-binding-requests/:requestId/reject"',
  'GET("/runs/:runId/audit-events"',
];
const PERMISSIONS = [
  'inventory_sync.read',
  'inventory_sync.run',
  'inventory_sync.rerun',
  'inventory_snapshot.read',
  'sku_binding.read',
  'sku_binding.manage',
  'sku_binding.resolve_manual',
  'inventory_sync.audit.read',
];
const SECRET_PATTERN = /(access_token|refresh_token|client_secret|app_secret|authorization:\s*bearer|cookie:\s*|password=|api_key=|secret-token)/i;
const REPO_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

function rootPath(rel) { return path.join(REPO_ROOT, rel); }
function read(rel) { try { return fs.readFileSync(rootPath(rel), 'utf8'); } catch { return ''; } }
function readJSON(rel) { try { return JSON.parse(read(rel)); } catch { return null; } }
function write(rel, value) {
  fs.mkdirSync(path.dirname(rootPath(rel)), { recursive: true });
  fs.writeFileSync(rootPath(rel), `${typeof value === 'string' ? value : JSON.stringify(value, null, 2)}\n`, 'utf8');
}
function git(args) {
  try { return execFileSync('git', args, { cwd: REPO_ROOT, encoding: 'utf8' }).trim(); } catch { return ''; }
}
function hasAll(text, values) { return values.every((value) => text.includes(value)); }
function anyForbidden(paths) { return paths.some((rel) => fs.existsSync(rootPath(rel))); }
function noSecrets(value) {
  const stack = [value];
  while (stack.length) {
    const item = stack.pop();
    if (typeof item === 'string') {
      if (SECRET_PATTERN.test(item)) return false;
    } else if (Array.isArray(item)) stack.push(...item);
    else if (item && typeof item === 'object') stack.push(...Object.values(item));
  }
  return true;
}

export function validateP9Batch5BackendAPIsBundle({ evidence = {}, sources = {}, gitState = {} } = {}) {
  const dto = sources.dto ?? read('backend/internal/modules/inventorysyncp9/api_dto.go');
  const service = sources.service ?? read('backend/internal/modules/inventorysyncp9/api_service.go');
  const handler = sources.handler ?? read('backend/internal/modules/inventorysyncp9/api_handler.go');
  const validation = sources.validation ?? read('backend/internal/modules/inventorysyncp9/api_validation.go');
  const repository = sources.repository ?? read('backend/internal/modules/inventorysyncp9/api_repository.go');
  const router = sources.router ?? read('backend/internal/modules/inventorysyncp9/api_router.go');
  const strictJSON = sources.strictJSON ?? read('backend/internal/pkg/httpapi/strict_json.go');
  const matrix = sources.matrix ?? read('backend/internal/pkg/adminperm/matrix.go');
  const tests = sources.tests ?? read('backend/internal/modules/inventorysyncp9/api_handler_test.go');
  const packageJSON = sources.packageJSON ?? read('package.json');
  const applicationRouter = sources.applicationRouter ?? read('backend/internal/api/router.go');
  const currentBranch = gitState.currentBranch ?? git(['branch', '--show-current']);
  const currentHead = gitState.currentHead ?? git(['rev-parse', 'HEAD']);
  const stagedFileCount = gitState.stagedFileCount ?? (git(['diff', '--cached', '--name-only']) ? git(['diff', '--cached', '--name-only']).split('\n').filter(Boolean).length : 0);
  const dirty = gitState.workingTreeDirty ?? Boolean(git(['status', '--short']));
  const apiText = [dto, service, handler, validation, repository, router, strictJSON, tests].join('\n');
  const productionAPIText = [dto, service, handler, validation, repository, router, strictJSON].join('\n');
  const forbiddenMethods = /\.\s*(PATCH|DELETE)\s*\(/i.test(router);
  const handlerHasDirectRepositoryAccess = /gorm\.io\/gorm|\.(Where|Create|Save|Updates?|Delete|First|Find)\s*\(/.test(handler);
  const checks = [
    ['requiredFilesPresent', REQUIRED_FILES.every((rel) => fs.existsSync(rootPath(rel)))],
    ['forbiddenPathsAbsent', !anyForbidden(FORBIDDEN_PATHS)],
    ['batchId', evidence.batchId === 'P9-TASK-BATCH-5'],
    ['currentBranch', currentBranch === 'dev'],
    ['currentHeadPresent', typeof currentHead === 'string' && currentHead.length > 0],
    ['stagedFileCount', stagedFileCount === 0],
    ['workingTreeDirtyRecorded', evidence.workingTreeDirty === dirty],
    ['changesCommitted', evidence.changesCommitted === false],
    ...TASK_IDS.map((id) => [`${id} status`, evidence.tasks?.[id]?.status === 'completed']),
    ['apiImplemented', evidence.apiImplemented === true && evidence.httpHandlerImplemented === true && evidence.restApiImplemented === true],
    ['authenticatedRoutes', evidence.authenticatedRoutesPresent === true && applicationRouter.includes('inventorysyncp9.Register(authed, inventorySyncP9H)')],
    ['singleRegisteredRouter', evidence.routerRegisteredOnce === true && router.includes('func Register') && hasAll(router, ROUTES)],
    ['existingConventionsReused', evidence.existingRouterReused === true
      && evidence.existingAuthMiddlewareReused === true
      && evidence.existingTenantContextReused === true
      && evidence.existingResponseEnvelopeReused === true
      && evidence.existingPaginationReused === true],
    ['thinHandlers', evidence.thinHandlersPresent === true
      && evidence.apiServiceFacadePresent === true
      && evidence.handlersDirectRepositoryAccess === false
      && handlerHasDirectRepositoryAccess === false],
    ['strictJSONContract', evidence.strictJSONBodyLimit === true && evidence.unknownFieldsRejected === true && evidence.contentTypeValidated === true && hasAll(strictJSON, ['MaxBytesReader', 'DisallowUnknownFields', 'application/json'])],
    ['trustedActorTenantScope', evidence.trustedActorAndTenantIsolation === true
      && evidence.trustedTenantContextUsed === true
      && evidence.trustedActorContextUsed === true
      && hasAll(handler + service + repository + validation, ['TenantIDFromGin', 'AdminID', 'tenant_id = ?', 'ErrNotFound'])],
    ['callerSuppliedContextRejected', evidence.callerSuppliedTenantTrusted === false
      && evidence.callerSuppliedActorTrusted === false
      && evidence.callerSuppliedRoleTrusted === false],
    ['idempotencyAndRequestId', evidence.writesRequireIdempotencyKey === true && evidence.requestIdPropagated === true && hasAll(handler + service + validation, ['Idempotency-Key', 'IdempotencyKeyHash', 'TraceID'])],
    ['keysetPagination', evidence.keysetPagination === true
      && evidence.offsetPaginationAbsent === true
      && evidence.paginationDuplicates === 0
      && evidence.paginationOmissions === 0
      && hasAll(repository, ['DecodeCursor', 'ApplyDescKeyset', 'BuildNextCursor', 'Limit(params.Limit + 1)'])],
    ['safeDTO', evidence.explicitSafeDTOs === true && evidence.rawDomainModelsNotReturned === true && !apiText.includes('json:"accessToken"') && !apiText.includes('json:"refreshToken"')],
    ['safeOutputRedaction', evidence.safeErrorMappingPresent === true
      && evidence.rawProviderErrorExposed === false
      && evidence.rawAuditMetadataExposed === false
      && evidence.rawCursorExposed === false
      && evidence.rawIdempotencyKeyExposed === false],
    ['apiCoverage', evidence.syncRunApisPresent === true
      && evidence.inventorySnapshotApisPresent === true
      && evidence.skuBindingApisPresent === true
      && evidence.calibrationApisPresent === true
      && evidence.manualBindingApisPresent === true
      && evidence.syncHistoryApisPresent === true
      && evidence.auditTimelineApisPresent === true],
    ['allowedActions', evidence.allowedActionsImplemented === true
      && evidence.allowedActionsServerComputed === true
      && evidence.resourceOperationsReauthorize === true
      && hasAll(dto + service, ['AllowedActions', 'CanRerun', 'CanConfirm', 'CanReject'])],
    ['rbacReused', evidence.existingRBACReused === true && hasAll(matrix, PERMISSIONS)],
    ['writesUseDomainServices', evidence.domainServicesReused === true && hasAll(service, ['Orchestrator.Run', 'Orchestrator.ManualRerun', 'Calibration.recalibrateSnapshotItemWithDB', 'ManualBinding.ConfirmBinding', 'ManualBinding.RejectBinding'])],
    ['recalibrationHistory', evidence.controlledRecalibrationHistory === true && hasAll(service, ['p9.inventory-sync.recalibrate', 'ResponseSummary'])],
    ['auditHistory', evidence.auditTimelineImplemented === true && hasAll(repository + service, ['operationlog.OperationLog', 'ListRunAuditEvents'])],
    ['verificationCoverage', evidence.testsPassed === true
      && evidence.permissionTestsPassed === true
      && evidence.tenantIsolationTestsPassed === true
      && evidence.idempotencyTestsPassed === true
      && evidence.paginationTestsPassed === true
      && evidence.apiContractTestsPassed === true
      && evidence.concurrencyTestsPassed === true
      && evidence.raceTestsPassed === true
      && evidence.dataRaces === 0],
    ['noRealProviderWorkerUI', evidence.realDouyinProviderImplemented === false
      && evidence.realNetworkEnabled === false
      && evidence.workerImplemented === false
      && evidence.backgroundSyncWorkerImplemented === false
      && evidence.automaticRetryWorkerImplemented === false
      && evidence.adminUiImplemented === false
      && !/(http\.Client|time\.NewTicker|go\s+func|queue|accessToken|refreshToken)/i.test(productionAPIText)],
    ['productionProviderGuard', evidence.productionProviderModesCallable === false],
    ['noRealCredentialsOrOAuth', evidence.oauthImplemented === false
      && evidence.realCredentialsEnabled === false
      && evidence.realCredentialsExecutable === false],
    ['noMutationRoutes', evidence.readonlySnapshotAndBindingHistory === true
      && evidence.realPlatformReadEnabled === false
      && evidence.realPlatformWriteEnabled === false
      && evidence.inventoryMutationEnabled === false
      && forbiddenMethods === false],
    ['secretFreeEvidence', noSecrets(evidence)],
    ['testsPresent', evidence.testsPassed === true && hasAll(tests, ['TestInventorySyncAPIRejectsUnknownFieldsAndCredentials', 'TestInventorySyncAPIKeysetAndTenantIsolation', 'TestInventorySyncAPIRoleAndProductionCaps', 'TestInventorySyncAPIAuthRequired'])],
    ['sqliteIntegration', evidence.sqliteIntegrationTestsPassed === true],
    ['postgresDeferred', evidence.postgresIntegrationStatus === 'not_run' && evidence.postgresIntegrationPassed === false && evidence.p9FinalClosureBlocker === true],
    ['p10BoundaryPreserved', evidence.p10BoundaryPreserved === true && evidence.productionReady === false],
    ['packageScripts', hasAll(packageJSON, [
      'test:p9-task-batch-5-backend-apis',
      'p9:task-batch-5-backend-apis-gate',
      'test:p9-task-batch-5',
      'p9:task-batch-5-gate',
    ])],
  ];
  const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failed.length === 0 ? 'passed' : 'failed',
    failed,
    failedCount: failed.length,
    currentBranch,
    currentHead,
    stagedFileCount,
    workingTreeDirty: dirty,
    requiredFiles: REQUIRED_FILES,
    forbiddenPaths: FORBIDDEN_PATHS,
    checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  };
}

export function renderP9Batch5BackendAPIsGateMarkdown(report) {
  return `# P9 Task Batch 5 Backend APIs Gate

Status: **${report.status}**

- Current branch: ${report.currentBranch}
- Current HEAD: ${report.currentHead}
- Staged files: ${report.stagedFileCount}
- Working tree dirty: ${report.workingTreeDirty}
- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}

${report.checks.map((check) => `- ${check.status === 'passed' ? 'PASS' : 'FAIL'} \`${check.id}\``).join('\n')}
`;
}

if (process.argv[1] && path.resolve(process.argv[1]) === path.resolve(fileURLToPath(import.meta.url))) {
  const evidence = readJSON(P9_BATCH_5_JSON) ?? {};
  const report = validateP9Batch5BackendAPIsBundle({ evidence });
  write(P9_BATCH_5_GATE_JSON, report);
  write(P9_BATCH_5_GATE_MD, renderP9Batch5BackendAPIsGateMarkdown(report));
  console.log(JSON.stringify(report, null, 2));
  if (report.status !== 'passed') process.exitCode = 1;
}
