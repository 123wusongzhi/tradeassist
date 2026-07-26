import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

export const P8_TASK_BATCH_8_EVIDENCE_JSON = 'docs/p8-task-batch-8-admin-operation-task-center.json';
export const P8_TASK_BATCH_8_GATE_JSON = 'docs/p8-task-batch-8-final-gate.json';
export const P8_TASK_BATCH_8_GATE_MD = 'docs/P8_TASK_BATCH_8_FINAL_GATE.md';

const requiredFiles = [
  'admin/config/routes.ts',
  'admin/src/services/request.ts',
  'admin/src/services/operationTasks.ts',
  'admin/src/services/__tests__/operationTasks.test.ts',
  'admin/src/constants/operationTasks.ts',
  'admin/src/pages/TaskCenter/OperationTasks/index.tsx',
  'admin/src/pages/TaskCenter/OperationTasks/Detail.tsx',
  'admin/src/pages/TaskCenter/OperationTasks/components/OperationTaskShared.tsx',
  'admin/src/pages/TaskCenter/OperationTasks/__tests__/OperationTaskShared.test.tsx',
  'docs/P8_TASK_BATCH_8_ADMIN_OPERATION_TASK_CENTER.md',
  'docs/p8-task-batch-8-admin-operation-task-center.json',
  'scripts/p8-task-batch-8-final-gate.mjs',
  'tests/gates/p8/task-batch-8.mjs',
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

export function validateP8TaskBatch8Bundle({ evidence = {}, sources = {} } = {}) {
  const routesText = sources.routesText ?? text('admin/config/routes.ts');
  const requestText = sources.requestText ?? text('admin/src/services/request.ts');
  const serviceText = sources.serviceText ?? text('admin/src/services/operationTasks.ts');
  const listText = sources.listText ?? text('admin/src/pages/TaskCenter/OperationTasks/index.tsx');
  const detailText = sources.detailText ?? text('admin/src/pages/TaskCenter/OperationTasks/Detail.tsx');
  const sharedText = sources.sharedText ?? text('admin/src/pages/TaskCenter/OperationTasks/components/OperationTaskShared.tsx');
  const constantsText = sources.constantsText ?? text('admin/src/constants/operationTasks.ts');
  const permissionsText = sources.permissionsText ?? text('admin/src/utils/permission.ts');
  const menuAccessText = sources.menuAccessText ?? text('admin/src/utils/menuAccess.ts');
  const urlStateText = sources.urlStateText ?? text('admin/src/utils/urlState.ts');
  const testsText = sources.testsText ?? `${text('admin/src/services/__tests__/operationTasks.test.ts')}\n${text('admin/src/pages/TaskCenter/OperationTasks/__tests__/OperationTaskShared.test.tsx')}`;
  const packageText = sources.packageText ?? text('package.json');
  const docsText = sources.docsText ?? `${text('docs/P8_TASK_BATCH_8_ADMIN_OPERATION_TASK_CENTER.md')}\n${text('docs/P8_TASK_BATCH_8_FINAL_GATE.md')}`;
  const adminCode = `${routesText}\n${requestText}\n${serviceText}\n${listText}\n${detailText}\n${sharedText}\n${constantsText}\n${permissionsText}\n${menuAccessText}\n${urlStateText}`;

  const writeMethods = ['createTask', 'createDraft', 'editDraft', 'approveTask', 'rejectTask', 'cancelTask', 'executeTask', 'retryTask'];
  const allWriteMethodsUseIdempotency = serviceText.includes('Idempotency-Key') && writeMethods.every((method) => {
    const idx = serviceText.indexOf(`function ${method}`);
    if (idx < 0) return false;
    const nextExport = serviceText.indexOf('\nexport ', idx + 1);
    const body = serviceText.slice(idx, nextExport > idx ? nextExport : undefined);
    return body.includes('idempotencyKey') && body.includes('idempotencyHeaders(idempotencyKey)');
  });

  const checks = [
    ['batchId', evidence.batchId === 'P8-TASK-BATCH-8'],
    ['P8-601 status', evidence.tasks?.['P8-601']?.status === 'completed'],
    ['P8-602 status', evidence.tasks?.['P8-602']?.status === 'completed'],
    ['P8-603 status', evidence.tasks?.['P8-603']?.status === 'completed'],
    ['P8-604 status', evidence.tasks?.['P8-604']?.status === 'completed'],
    ['P8-605 status', evidence.tasks?.['P8-605']?.status === 'completed'],
    ['P8-606 status', evidence.tasks?.['P8-606']?.status === 'completed'],
    ['adminRoutesPresent', evidence.adminRoutesPresent === true && hasAll(routesText, ['/ops/task-center/operation-tasks', './TaskCenter/OperationTasks/Detail', 'hideInMenu'])],
    ['existingAdminFrameworkReused', evidence.existingAdminFrameworkReused === true && hasAll(listText, ['TmPageContainer', 'TmProTable']) && hasAll(detailText, ['SectionCard', 'TaskJsonBlock'])],
    ['apiServiceLayerPresent', evidence.apiServiceLayerPresent === true && hasAll(serviceText, ['/api/v1/operation-tasks', 'getWithParams', 'postJSON', 'patchJSON'])],
    ['apiClientExtendedSafely', evidence.apiClientExtendedSafely === true && hasAll(requestText, ['RequestOptions', 'ApiRequestError', 'traceId', 'withOptions'])],
    ['noRawFetchOrAxios', evidence.noRawFetchOrAxios === true && !adminCode.includes('fetch(') && !adminCode.includes('axios')],
    ['allowedActionsUsed', evidence.allowedActionsUsed === true && hasAll(detailText, ['allowedActions.canEditDraft', 'allowedActions.canApprove', 'allowedActions.canReject', 'allowedActions.canExecute', 'allowedActions.canRetry', 'allowedActions.canCancel'])],
    ['backendRBACBoundaryPreserved', evidence.backendRBACBoundaryPreserved === true && detailText.includes('后端仍会重新校验权限') && !detailText.includes('normalizeRole(')],
    ['operationTaskPermissionsWired', evidence.operationTaskPermissionsWired === true && hasAll(permissionsText, ['OPERATION_TASK_AUDIT_READ', 'OPERATION_TASK_EXECUTE', 'OPERATION_TASK_REVIEW', 'OPERATION_TASK_RETRY']) && menuAccessText.includes('OPERATION_TASK_AUDIT_READ')],
    ['idempotencyHeadersForWrites', evidence.idempotencyHeadersForWrites === true && allWriteMethodsUseIdempotency && testsText.includes('Idempotency-Key')],
    ['duplicateClickGuardPresent', evidence.duplicateClickGuardPresent === true && detailText.includes('actionLoading') && detailText.includes('confirmLoading={actionLoading}')],
    ['revisionConflictRefresh', evidence.revisionConflictRefresh === true && hasAll(detailText, ['expectedTaskRevision', 'conflict', 'mismatch', 'loadAll'])],
    ['keysetPagination', evidence.keysetPagination === true && hasAll(listText, ['nextCursor', 'hasMore', 'cursorStack', 'pagination={false}']) && !listText.includes('total:')],
    ['eventSequencePagination', evidence.eventSequencePagination === true && hasAll(detailText, ['afterSequence', 'nextSequence', 'loadMoreEvents'])],
    ['loadingEmptyErrorStates', evidence.loadingEmptyErrorStates === true && hasAll(listText, ['loading', 'emptyLocale', 'ErrorAlert', '刷新'])],
    ['safeMetadataRedaction', evidence.safeMetadataRedaction === true && hasAll(sharedText, ['SENSITIVE_KEY', 'redactSensitiveValue', 'safeMetadata', 'OPERATION_METADATA_ALLOWLIST']) && testsText.includes('only exposes allowlisted event metadata')],
    ['payloadNotRenderedAsHTML', evidence.payloadNotRenderedAsHTML === true && !adminCode.includes('dangerouslySetInnerHTML') && sharedText.includes('TaskJsonBlock')],
    ['historicalDraftPayloadNotFabricated', evidence.historicalDraftPayloadNotFabricated === true && detailText.includes('当前后端未返回历史草稿完整 Payload')],
    ['nonProductionBoundaryVisible', evidence.nonProductionBoundaryVisible === true && sharedText.includes('NON_PRODUCTION_BOUNDARY_COPY') && hasAll(`${sharedText}\n${constantsText}`, ['非生产边界', '真实平台写入', '自动发布', '自动上架'])],
    ['safeAdapterModesOnly', evidence.safeAdapterModesOnly === true && hasAll(detailText, ['local_draft_only', 'mock', 'sandbox']) && !detailText.includes('production')],
    ['urlStateCursorKeys', evidence.urlStateCursorKeys === true && hasAll(urlStateText, ['cursor', 'afterSequence'])],
    ['frontendTestsPresent', evidence.frontendTestsPresent === true && hasAll(testsText, ['operation task API service', 'operation task shared helpers', 'validates JSON editor input'])],
    ['docsPresent', evidence.docsPresent === true && hasAll(docsText, ['P8-601', 'P8-602', 'P8-603', 'P8-604', 'P8-605', 'P8-606', 'Production Ready: false'])],
    ['backendBusinessLogicNotDuplicated', evidence.backendBusinessLogicNotDuplicated === true && !adminCode.includes('NewOperationTaskRepository') && !adminCode.includes('TaskStateMachine')],
    ['realPlatformWriteEnabled', evidence.realPlatformWriteEnabled === false && !adminCode.includes('realPlatformWriteEnabled=true')],
    ['realCredentialsEnabled', evidence.realCredentialsEnabled === false && !adminCode.includes('realCredentialsEnabled=true')],
    ['automaticPublishEnabled', evidence.automaticPublishEnabled === false && !adminCode.includes('automaticPublishEnabled=true')],
    ['automaticListingEnabled', evidence.automaticListingEnabled === false && !adminCode.includes('automaticListingEnabled=true')],
    ['backgroundAutoRetryEnabled', evidence.backgroundAutoRetryEnabled === false && !adminCode.includes('setInterval')],
    ['productionReady', evidence.productionReady === false && !adminCode.includes('productionReady=true') && !docsText.includes('Production Ready: true')],
    ['packageScriptsRegistered', hasAll(packageText, ['test:p8-task-batch-8', 'p8:task-batch-8-gate'])],
  ];

  const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failed.length ? 'failed' : 'passed',
    failed,
    failedCount: failed.length,
    checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  };
}

export function buildP8TaskBatch8GateReport(bundle = {}) {
  const evidence = bundle.evidence ?? readJSON(P8_TASK_BATCH_8_EVIDENCE_JSON) ?? {};
  const validation = validateP8TaskBatch8Bundle({ evidence, sources: bundle.sources });
  const missingFiles = requiredFiles.filter((rel) => !fs.existsSync(rootPath(rel)));
  const stagedFiles = git(['diff', '--cached', '--name-only']);
  const currentBranch = git(['branch', '--show-current']);
  const headDetached = git(['rev-parse', '--abbrev-ref', 'HEAD']) === 'HEAD';
  const head = git(['rev-parse', 'HEAD']);
  const failed = [
    ...validation.failed,
    ...missingFiles.map((rel) => `missing:${rel}`),
  ];
  if (currentBranch !== 'dev') failed.push('currentBranch');
  if (headDetached) failed.push('headDetached');
  if (stagedFiles !== '') failed.push('stagedFileCount');
  if (head !== '73c12f12b0503aed654d0adbcc5cc8bc5be2073d') failed.push('headChanged');
  return {
    phase: 'P8',
    gate: 'P8-TASK-BATCH-8',
    status: failed.length ? 'failed' : 'passed',
    checkedAt: '2026-07-26T00:00:00.000Z',
    batchId: 'P8-TASK-BATCH-8',
    tasks: ['P8-601', 'P8-602', 'P8-603', 'P8-604', 'P8-605', 'P8-606'],
    currentBranch,
    headDetached,
    head,
    stagedFileCount: stagedFiles === '' ? 0 : stagedFiles.split('\n').length,
    adminRoutesPresent: evidence.adminRoutesPresent === true,
    existingAdminFrameworkReused: evidence.existingAdminFrameworkReused === true,
    apiServiceLayerPresent: evidence.apiServiceLayerPresent === true,
    allowedActionsUsed: evidence.allowedActionsUsed === true,
    idempotencyHeadersForWrites: evidence.idempotencyHeadersForWrites === true,
    keysetPagination: evidence.keysetPagination === true,
    safeMetadataRedaction: evidence.safeMetadataRedaction === true,
    nonProductionBoundaryVisible: evidence.nonProductionBoundaryVisible === true,
    backendBusinessLogicNotDuplicated: evidence.backendBusinessLogicNotDuplicated === true,
    realPlatformWriteEnabled: evidence.realPlatformWriteEnabled === true,
    realCredentialsEnabled: evidence.realCredentialsEnabled === true,
    automaticPublishEnabled: evidence.automaticPublishEnabled === true,
    automaticListingEnabled: evidence.automaticListingEnabled === true,
    backgroundAutoRetryEnabled: evidence.backgroundAutoRetryEnabled === true,
    productionReady: evidence.productionReady === true,
    failedCount: failed.length,
    failed,
    checks: validation.checks,
  };
}

export function writeP8TaskBatch8GateReport(report) {
  writeJSON(P8_TASK_BATCH_8_GATE_JSON, report);
  writeMarkdown(
    P8_TASK_BATCH_8_GATE_MD,
    `# P8 Task Batch 8 Final Gate\n\nStatus: **${report.status}**\n\n- Batch: ${report.batchId}\n- Tasks: ${report.tasks.join(', ')}\n- Current branch: ${report.currentBranch}\n- Head detached: ${report.headDetached}\n- HEAD: ${report.head}\n- Staged files: ${report.stagedFileCount}\n- Admin routes present: ${report.adminRoutesPresent}\n- Existing Admin framework reused: ${report.existingAdminFrameworkReused}\n- API service layer present: ${report.apiServiceLayerPresent}\n- Backend allowedActions used: ${report.allowedActionsUsed}\n- Idempotency headers for writes: ${report.idempotencyHeadersForWrites}\n- Keyset pagination: ${report.keysetPagination}\n- Safe metadata redaction: ${report.safeMetadataRedaction}\n- Non-production boundary visible: ${report.nonProductionBoundaryVisible}\n- Backend business logic duplicated: ${!report.backendBusinessLogicNotDuplicated}\n- Real platform write enabled: ${report.realPlatformWriteEnabled}\n- Real credentials enabled: ${report.realCredentialsEnabled}\n- Automatic publish enabled: ${report.automaticPublishEnabled}\n- Automatic listing enabled: ${report.automaticListingEnabled}\n- Background auto retry enabled: ${report.backgroundAutoRetryEnabled}\n- Production Ready: ${report.productionReady}\n- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}\n\nThis gate validates only P8 Batch 8 Admin operation-task center closure. It does not authorize real Douyin/OAuth, real credentials, real platform writes, automatic publish/listing, background automatic retry, scheduled execution, production queue workers, production gray, tag, release, or Production Ready. Final Production Acceptance remains deferred to P10.\n`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildP8TaskBatch8GateReport();
  writeP8TaskBatch8GateReport(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}
