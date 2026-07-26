import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

export const P8_TASK_BATCH_7_EVIDENCE_JSON = 'docs/p8-task-batch-7-operation-task-api.json';
export const P8_TASK_BATCH_7_GATE_JSON = 'docs/p8-task-batch-7-final-gate.json';
export const P8_TASK_BATCH_7_GATE_MD = 'docs/P8_TASK_BATCH_7_FINAL_GATE.md';

const requiredFiles = [
  'backend/internal/api/router.go',
  'backend/internal/modules/operationtask/router.go',
  'backend/internal/modules/operationtask/handler.go',
  'backend/internal/modules/operationtask/api_dto.go',
  'backend/internal/modules/operationtask/api_service.go',
  'backend/internal/modules/operationtask/api_errors.go',
  'backend/internal/modules/operationtask/api_validation.go',
  'backend/internal/modules/operationtask/rbac_authorizers.go',
  'backend/internal/modules/operationtask/api_service_test.go',
  'backend/internal/modules/operationtask/api_handler_test.go',
  'docs/P8_OPERATION_TASK_API.md',
  'docs/P8_TASK_BATCH_7_OPERATION_TASK_API.md',
  'docs/p8-task-batch-7-operation-task-api.json',
  'scripts/p8-task-batch-7-final-gate.mjs',
  'tests/gates/p8/task-batch-7.mjs',
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

export function validateP8TaskBatch7Bundle({ evidence = {}, sources = {} } = {}) {
  const apiRouterText = sources.apiRouterText ?? text('backend/internal/api/router.go');
  const moduleRouterText = sources.moduleRouterText ?? text('backend/internal/modules/operationtask/router.go');
  const handlerText = sources.handlerText ?? text('backend/internal/modules/operationtask/handler.go');
  const serviceText = sources.serviceText ?? text('backend/internal/modules/operationtask/api_service.go');
  const validationText = sources.validationText ?? text('backend/internal/modules/operationtask/api_validation.go');
  const errorsText = sources.errorsText ?? text('backend/internal/modules/operationtask/api_errors.go');
  const dtoText = sources.dtoText ?? text('backend/internal/modules/operationtask/api_dto.go');
  const authorizerText = sources.authorizerText ?? text('backend/internal/modules/operationtask/rbac_authorizers.go');
  const testsText = sources.testsText ?? `${text('backend/internal/modules/operationtask/api_service_test.go')}\n${text('backend/internal/modules/operationtask/api_handler_test.go')}`;
  const packageText = sources.packageText ?? text('package.json');
  const docsText = sources.docsText ?? `${text('docs/P8_OPERATION_TASK_API.md')}\n${text('docs/P8_TASK_BATCH_7_OPERATION_TASK_API.md')}`;
  const apiCode = `${apiRouterText}\n${moduleRouterText}\n${handlerText}\n${serviceText}\n${validationText}\n${errorsText}\n${dtoText}\n${authorizerText}`;
  const combined = `${apiCode}\n${testsText}\n${docsText}`;

  const routeNeedles = [
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
  ];

  const checks = [
    ['batchId', evidence.batchId === 'P8-TASK-BATCH-7'],
    ['P8-501 status', evidence.tasks?.['P8-501']?.status === 'completed'],
    ['P8-502 status', evidence.tasks?.['P8-502']?.status === 'completed'],
    ['P8-503 status', evidence.tasks?.['P8-503']?.status === 'completed'],
    ['P8-504 status', evidence.tasks?.['P8-504']?.status === 'completed'],
    ['P8-505 status', evidence.tasks?.['P8-505']?.status === 'completed'],
    ['apiImplemented', evidence.apiImplemented === true && hasAll(apiRouterText, ['BearerAuthWithDB', 'operationtask.Register(authed', 'operationtask.NewAPIService'])],
    ['routesRegistered', evidence.routesRegistered === true && hasAll(moduleRouterText, routeNeedles)],
    ['existingAPIStackReused', evidence.existingAPIStackReused === true && apiRouterText.includes('/api/v1') && !apiCode.includes('http.ListenAndServe')],
    ['trustedTenantActorContext', evidence.trustedTenantActorContext === true && hasAll(handlerText, ['adminperm.TenantIDFromGin', 'apiActorID(c)', 'adminperm.LoadPrincipal'])],
    ['strictJSONBinding', evidence.strictJSONBinding === true && hasAll(validationText, ['DisallowUnknownFields', 'MaxBytesReader'])],
    ['idempotencyRequiredForWrites', evidence.idempotencyRequiredForWrites === true && hasAll(handlerText, ['apiIdempotencyKey(c)', 'requireWrite', 'CreateTask']) && validationText.includes('apiIdempotencyKeyPattern')],
    ['requestIDServerContext', evidence.requestIDServerContext === true && validationText.includes('ctxkey.TraceID') && !combined.includes('json:"requestId"')],
    ['handlerNoDirectRepositoryWrites', evidence.handlerNoDirectRepositoryWrites === true && !handlerText.includes('NewOperationTaskRepository') && !handlerText.includes('.Create(') && handlerText.includes('h.Svc.')],
    ['serviceFacadePresent', evidence.serviceFacadePresent === true && hasAll(serviceText, ['type APIService struct', 'NewAPIService', 'CreateTask', 'Execute', 'RetryExecution'])],
    ['rbacIntegrated', evidence.rbacIntegrated === true && hasAll(authorizerText, ['CanRead', 'CanCreate', 'CanEdit', 'CanCancel', 'PermOperationTaskAuditRead', 'StrictHasPermission'])],
    ['keysetPagination', evidence.keysetPagination === true && hasAll(serviceText, ['NextCursor', 'Cursor']) && !handlerText.includes('offset') && !serviceText.includes('Offset(')],
    ['safeDTOs', evidence.safeDTOs === true && !dtoText.includes('Published') && !dtoText.includes('Listed') && !dtoText.includes('Credential') && !dtoText.includes('Token')],
    ['redactionIntegrated', evidence.redactionIntegrated === true && hasAll(serviceText, ['decodeSafeJSON', 'redactSafeJSON', 'safeKeyHash'])],
    ['dangerousBodyFieldsRejected', evidence.dangerousBodyFieldsRejected === true && testsText.includes('RejectsUnknownDangerousFields') && testsText.includes('tenantId')],
    ['executeResponseDoesNotExposePublished', evidence.executeResponseDoesNotExposePublished === true && testsText.includes('DoesNotExposePublished') && testsText.includes('NotContains')],
    ['apiTestsPassed', evidence.apiTestsPassed === true && hasAll(testsText, ['TestAPIServiceCreateTaskIdempotencyAndTenantActorBoundary', 'TestOperationTaskHandlerRequiresIdempotencyKeyForWrites'])],
    ['adminUiImplemented', evidence.adminUiImplemented === false && !combined.includes('Admin UI route')],
    ['realPlatformWriteEnabled', evidence.realPlatformWriteEnabled === false && !apiCode.includes('realPlatformWriteEnabled = true')],
    ['realCredentialsEnabled', evidence.realCredentialsEnabled === false && !apiCode.includes('realCredentialsEnabled = true')],
    ['automaticPublishEnabled', evidence.automaticPublishEnabled === false && !apiCode.includes('automaticPublishEnabled = true')],
    ['automaticListingEnabled', evidence.automaticListingEnabled === false && !apiCode.includes('automaticListingEnabled = true')],
    ['productionReady', evidence.productionReady === false && !combined.includes('productionReady=true')],
    ['docsPresent', evidence.docsPresent === true && hasAll(docsText, ['P8-501', 'P8-502', 'P8-503', 'P8-504', 'P8-505', 'Production Ready: false'])],
    ['packageScriptsRegistered', hasAll(packageText, ['test:p8-task-batch-7', 'p8:task-batch-7-gate'])],
  ];

  const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failed.length ? 'failed' : 'passed',
    failed,
    failedCount: failed.length,
    checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  };
}

export function buildP8TaskBatch7GateReport(bundle = {}) {
  const evidence = bundle.evidence ?? readJSON(P8_TASK_BATCH_7_EVIDENCE_JSON) ?? {};
  const validation = validateP8TaskBatch7Bundle({ evidence, sources: bundle.sources });
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
    gate: 'P8-TASK-BATCH-7',
    status: failed.length ? 'failed' : 'passed',
    checkedAt: '2026-07-26T00:00:00.000Z',
    batchId: 'P8-TASK-BATCH-7',
    tasks: ['P8-501', 'P8-502', 'P8-503', 'P8-504', 'P8-505'],
    currentBranch,
    stagedFileCount: stagedFiles === '' ? 0 : stagedFiles.split('\n').length,
    apiImplemented: evidence.apiImplemented === true,
    routesRegistered: evidence.routesRegistered === true,
    existingAPIStackReused: evidence.existingAPIStackReused === true,
    trustedTenantActorContext: evidence.trustedTenantActorContext === true,
    idempotencyRequiredForWrites: evidence.idempotencyRequiredForWrites === true,
    handlerNoDirectRepositoryWrites: evidence.handlerNoDirectRepositoryWrites === true,
    keysetPagination: evidence.keysetPagination === true,
    safeDTOs: evidence.safeDTOs === true,
    apiTestsPassed: evidence.apiTestsPassed === true,
    adminUiImplemented: evidence.adminUiImplemented === true,
    realPlatformWriteEnabled: evidence.realPlatformWriteEnabled === true,
    realCredentialsEnabled: evidence.realCredentialsEnabled === true,
    automaticPublishEnabled: evidence.automaticPublishEnabled === true,
    automaticListingEnabled: evidence.automaticListingEnabled === true,
    productionReady: evidence.productionReady === true,
    failedCount: failed.length,
    failed,
    checks: validation.checks,
  };
}

export function writeP8TaskBatch7GateReport(report) {
  writeJSON(P8_TASK_BATCH_7_GATE_JSON, report);
  writeMarkdown(
    P8_TASK_BATCH_7_GATE_MD,
    `# P8 Task Batch 7 Final Gate\n\nStatus: **${report.status}**\n\n- Batch: ${report.batchId}\n- Tasks: ${report.tasks.join(', ')}\n- Current branch: ${report.currentBranch}\n- Staged files: ${report.stagedFileCount}\n- API implemented: ${report.apiImplemented}\n- Routes registered: ${report.routesRegistered}\n- Existing API stack reused: ${report.existingAPIStackReused}\n- Trusted tenant/actor context: ${report.trustedTenantActorContext}\n- Idempotency required for writes: ${report.idempotencyRequiredForWrites}\n- Handler direct repository writes: ${!report.handlerNoDirectRepositoryWrites}\n- Keyset pagination: ${report.keysetPagination}\n- Safe DTOs: ${report.safeDTOs}\n- API tests passed: ${report.apiTestsPassed}\n- Admin UI implemented: ${report.adminUiImplemented}\n- Real platform write enabled: ${report.realPlatformWriteEnabled}\n- Real credentials enabled: ${report.realCredentialsEnabled}\n- Automatic publish enabled: ${report.automaticPublishEnabled}\n- Automatic listing enabled: ${report.automaticListingEnabled}\n- Production Ready: ${report.productionReady}\n- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}\n\nThis gate validates only P8 Batch 7 backend operation-task API closure. It does not authorize Admin UI, real Douyin/OAuth, real credentials, real platform writes, automatic publish/listing, production gray, tag, release, or Production Ready. Final Production Acceptance remains deferred to P10.\n`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildP8TaskBatch7GateReport();
  writeP8TaskBatch7GateReport(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}
