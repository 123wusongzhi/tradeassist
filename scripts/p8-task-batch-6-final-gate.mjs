import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

export const P8_TASK_BATCH_6_EVIDENCE_JSON = 'docs/p8-task-batch-6-permission-audit-secret-foundation.json';
export const P8_TASK_BATCH_6_GATE_JSON = 'docs/p8-task-batch-6-final-gate.json';
export const P8_TASK_BATCH_6_GATE_MD = 'docs/P8_TASK_BATCH_6_FINAL_GATE.md';

const requiredFiles = [
  'backend/internal/pkg/adminperm/matrix.go',
  'backend/internal/pkg/adminperm/principal.go',
  'backend/internal/modules/operationtask/rbac_authorizers.go',
  'backend/internal/modules/operationtask/metadata_redaction.go',
  'backend/internal/modules/operationtask/services.go',
  'backend/internal/modules/operationtask/execution_services.go',
  'backend/internal/modules/operationtask/platform_draft_adapters.go',
  'backend/internal/modules/operationtask/batch6_foundation_test.go',
  'docs/P8_TASK_BATCH_6_PERMISSION_AUDIT_SECRET_FOUNDATION.md',
  'docs/p8-task-batch-6-permission-audit-secret-foundation.json',
  'scripts/p8-task-batch-6-final-gate.mjs',
  'tests/gates/p8/task-batch-6.mjs',
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

export function validateP8TaskBatch6Bundle({ evidence = {}, sources = {} } = {}) {
  const adminpermText = sources.adminpermText ?? `${text('backend/internal/pkg/adminperm/matrix.go')}\n${text('backend/internal/pkg/adminperm/principal.go')}`;
  const authorizerText = sources.authorizerText ?? text('backend/internal/modules/operationtask/rbac_authorizers.go');
  const redactionText = sources.redactionText ?? text('backend/internal/modules/operationtask/metadata_redaction.go');
  const executionText = sources.executionText ?? text('backend/internal/modules/operationtask/execution_services.go');
  const servicesText = sources.servicesText ?? text('backend/internal/modules/operationtask/services.go');
  const adapterText = sources.adapterText ?? text('backend/internal/modules/operationtask/platform_draft_adapters.go');
  const repositoryText = sources.repositoryText ?? text('backend/internal/modules/operationtask/repository.go');
  const testsText = sources.testsText ?? `${text('backend/internal/modules/operationtask/batch6_foundation_test.go')}\n${text('backend/internal/pkg/adminperm/perm_test.go')}`;
  const packageText = sources.packageText ?? text('package.json');
  const docsText = sources.docsText ?? text('docs/P8_TASK_BATCH_6_PERMISSION_AUDIT_SECRET_FOUNDATION.md');
  const combined = `${adminpermText}\n${authorizerText}\n${redactionText}\n${executionText}\n${servicesText}\n${adapterText}\n${repositoryText}\n${testsText}\n${docsText}`;

  const checks = [
    ['batchId', evidence.batchId === 'P8-TASK-BATCH-6'],
    ['P8-401 status', evidence.tasks?.['P8-401']?.status === 'completed'],
    ['P8-402 status', evidence.tasks?.['P8-402']?.status === 'completed'],
    ['P8-403 status', evidence.tasks?.['P8-403']?.status === 'completed'],
    ['P8-404 status', evidence.tasks?.['P8-404']?.status === 'completed'],
    ['existingRBACReused', evidence.existingRBACReused === true && adminpermText.includes('PermOperationTaskReview') && adminpermText.includes('StrictHasPermission')],
    ['duplicateRBACSystemCreated', evidence.duplicateRBACSystemCreated === false && !combined.includes('type OperationTaskRoleMatrix struct')],
    ['approvalAuthorizerIntegrated', evidence.approvalAuthorizerIntegrated === true && hasAll(authorizerText, ['type RBACAuthorizer struct', 'CanReview', 'PermOperationTaskReview'])],
    ['executionAuthorizerIntegrated', evidence.executionAuthorizerIntegrated === true && hasAll(authorizerText, ['CanExecute', 'PermOperationTaskExecute'])],
    ['manualRetryAuthorizerIntegrated', evidence.manualRetryAuthorizerIntegrated === true && hasAll(executionText, ['type ManualRetryAuthorizer interface', 'CanRetry', 'ErrPermissionDenied'])],
    ['authorizationDefaultAllow', evidence.authorizationDefaultAllow === false && testsText.includes('unknown roles must not inherit')],
    ['crossTenantAccessDenied', evidence.crossTenantAccessDenied === true && authorizerText.includes('user.TenantID != tenantID')],
    ['operationTaskAuditServicePresent', evidence.operationTaskAuditServicePresent === true && servicesText.includes('func appendAuditEventTx')],
    ['auditDeliveryMode', evidence.auditDeliveryMode === 'synchronous_db_transaction'],
    ['auditLossPreventionPresent', evidence.auditLossPreventionPresent === true && hasAll(servicesText, ['clause.Locking', 'Sequence = latest.Sequence + 1'])],
    ['auditFireAndForgetPresent', evidence.auditFireAndForgetPresent === false && !combined.includes('go appendAuditEvent')],
    ['secretRedactorPresent', evidence.secretRedactorPresent === true && redactionText.includes('func redactSafeJSON')],
    ['executionErrorRedactionIntegrated', evidence.executionErrorRedactionIntegrated === true && executionText.includes('f.Details = redactSafeJSON(f.Details)')],
    ['auditMetadataRedactionIntegrated', evidence.auditMetadataRedactionIntegrated === true && servicesText.includes('event.Metadata = redactSafeJSON(event.Metadata)')],
    ['adapterMetadataRedactionIntegrated', evidence.adapterMetadataRedactionIntegrated === true && adapterText.includes('redactSafeJSON(datatypes.JSON(data))')],
    ['rawSecretPersistenceDetected', evidence.rawSecretPersistenceDetected === false && testsText.includes('NotContains')],
    ['rawSecretLogDetected', evidence.rawSecretLogDetected === false],
    ['realSecretCount', evidence.realSecretCount === 0],
    ['permissionTestsPassed', evidence.permissionTestsPassed === true && testsText.includes('TestRBACAuthorizerStrictRolesTenantAndActor')],
    ['auditTestsPassed', evidence.auditTestsPassed === true && testsText.includes('TestAuditEventsRecordReasonAndRedactMetadata')],
    ['redactionTestsPassed', evidence.redactionTestsPassed === true && testsText.includes('TestExecutionFailureDetailsAreRedactedNotDropped')],
    ['raceStatus', evidence.raceStatus === 'passed'],
    ['apiImplemented', evidence.apiImplemented === false],
    ['adminUiImplemented', evidence.adminUiImplemented === false],
    ['realPlatformWriteImplemented', evidence.realPlatformWriteImplemented === false],
    ['automaticPublishEnabled', evidence.automaticPublishEnabled === false && !combined.includes('PublishProduct') && !combined.includes('AutoPublish')],
    ['automaticListingEnabled', evidence.automaticListingEnabled === false && !combined.includes('AutoList')],
    ['realCredentialsEnabled', evidence.realCredentialsEnabled === false],
    ['productionReady', evidence.productionReady === false],
    ['packageScriptsRegistered', hasAll(packageText, ['test:p8-task-batch-6', 'p8:task-batch-6-gate'])],
  ];

  const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failed.length ? 'failed' : 'passed',
    failed,
    failedCount: failed.length,
    checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  };
}

export function buildP8TaskBatch6GateReport(bundle = {}) {
  const evidence = bundle.evidence ?? readJSON(P8_TASK_BATCH_6_EVIDENCE_JSON) ?? {};
  const validation = validateP8TaskBatch6Bundle({ evidence, sources: bundle.sources });
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
    gate: 'P8-TASK-BATCH-6',
    status: failed.length ? 'failed' : 'passed',
    checkedAt: '2026-07-26T00:00:00.000Z',
    batchId: 'P8-TASK-BATCH-6',
    tasks: ['P8-401', 'P8-402', 'P8-403', 'P8-404'],
    currentBranch,
    stagedFileCount: stagedFiles === '' ? 0 : stagedFiles.split('\n').length,
    existingRBACReused: evidence.existingRBACReused === true,
    duplicateRBACSystemCreated: evidence.duplicateRBACSystemCreated === true,
    approvalAuthorizerIntegrated: evidence.approvalAuthorizerIntegrated === true,
    executionAuthorizerIntegrated: evidence.executionAuthorizerIntegrated === true,
    manualRetryAuthorizerIntegrated: evidence.manualRetryAuthorizerIntegrated === true,
    operationTaskAuditServicePresent: evidence.operationTaskAuditServicePresent === true,
    auditDeliveryMode: evidence.auditDeliveryMode ?? null,
    secretRedactorPresent: evidence.secretRedactorPresent === true,
    rawSecretPersistenceDetected: evidence.rawSecretPersistenceDetected === true,
    rawSecretLogDetected: evidence.rawSecretLogDetected === true,
    realSecretCount: evidence.realSecretCount ?? null,
    raceStatus: evidence.raceStatus ?? null,
    apiImplemented: evidence.apiImplemented === true,
    adminUiImplemented: evidence.adminUiImplemented === true,
    realPlatformWriteImplemented: evidence.realPlatformWriteImplemented === true,
    automaticPublishEnabled: evidence.automaticPublishEnabled === true,
    automaticListingEnabled: evidence.automaticListingEnabled === true,
    productionReady: evidence.productionReady === true,
    failedCount: failed.length,
    failed,
    checks: validation.checks,
  };
}

export function writeP8TaskBatch6GateReport(report) {
  writeJSON(P8_TASK_BATCH_6_GATE_JSON, report);
  writeMarkdown(
    P8_TASK_BATCH_6_GATE_MD,
    `# P8 Task Batch 6 Final Gate\n\nStatus: **${report.status}**\n\n- Batch: ${report.batchId}\n- Tasks: ${report.tasks.join(', ')}\n- Current branch: ${report.currentBranch}\n- Staged files: ${report.stagedFileCount}\n- Existing RBAC reused: ${report.existingRBACReused}\n- Duplicate RBAC system created: ${report.duplicateRBACSystemCreated}\n- Approval authorizer integrated: ${report.approvalAuthorizerIntegrated}\n- Execution authorizer integrated: ${report.executionAuthorizerIntegrated}\n- Manual retry authorizer integrated: ${report.manualRetryAuthorizerIntegrated}\n- Operation task audit service present: ${report.operationTaskAuditServicePresent}\n- Audit delivery mode: ${report.auditDeliveryMode}\n- Secret redactor present: ${report.secretRedactorPresent}\n- Raw secret persistence detected: ${report.rawSecretPersistenceDetected}\n- Raw secret log detected: ${report.rawSecretLogDetected}\n- Real secret count: ${report.realSecretCount}\n- Race status: ${report.raceStatus}\n- API implemented: ${report.apiImplemented}\n- Admin UI implemented: ${report.adminUiImplemented}\n- Real platform write implemented: ${report.realPlatformWriteImplemented}\n- Automatic publish enabled: ${report.automaticPublishEnabled}\n- Automatic listing enabled: ${report.automaticListingEnabled}\n- Production Ready: ${report.productionReady}\n- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}\n\nThis gate validates only P8 Batch 6 permission, audit, and sensitive-information protection foundation. It does not authorize real credentials, real platform writes, automatic publish, automatic listing, API, Admin UI, production gray, production tag, production release, or Production Ready. Final Production Acceptance remains deferred to P10.\n`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildP8TaskBatch6GateReport();
  writeP8TaskBatch6GateReport(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}
