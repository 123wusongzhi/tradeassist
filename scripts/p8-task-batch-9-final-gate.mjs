import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

export const P8_TASK_BATCH_9_EVIDENCE_JSON = 'docs/p8-task-batch-9-final-integration.json';
export const P8_TASK_BATCH_9_GATE_JSON = 'docs/p8-task-batch-9-final-gate.json';
export const P8_TASK_BATCH_9_GATE_MD = 'docs/P8_TASK_BATCH_9_FINAL_GATE.md';

const requiredFiles = [
  'backend/internal/modules/operationtask/api_service.go',
  'backend/internal/modules/operationtask/api_service_test.go',
  'backend/internal/modules/operationtask/batch6_foundation_test.go',
  'backend/internal/modules/operationtask/platform_draft_adapters.go',
  'backend/internal/modules/operationtask/rbac_authorizers.go',
  'backend/internal/pkg/adminperm/matrix.go',
  'backend/internal/pkg/adminperm/perm_test.go',
  'admin/src/utils/permission.ts',
  'docs/p8-task-batch-9-final-integration.json',
  'scripts/p8-task-batch-9-final-gate.mjs',
  'tests/gates/p8/task-batch-9.mjs',
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

export function validateP8TaskBatch9Bundle({ evidence = {}, sources = {} } = {}) {
  const apiServiceText = sources.apiServiceText ?? text('backend/internal/modules/operationtask/api_service.go');
  const apiServiceTestsText = sources.apiServiceTestsText ?? text('backend/internal/modules/operationtask/api_service_test.go');
  const foundationTestsText = sources.foundationTestsText ?? text('backend/internal/modules/operationtask/batch6_foundation_test.go');
  const adaptersText = sources.adaptersText ?? text('backend/internal/modules/operationtask/platform_draft_adapters.go');
  const rbacText = sources.rbacText ?? text('backend/internal/modules/operationtask/rbac_authorizers.go');
  const backendPermText = sources.backendPermText ?? `${text('backend/internal/pkg/adminperm/matrix.go')}\n${text('backend/internal/pkg/adminperm/perm_test.go')}`;
  const frontendPermText = sources.frontendPermText ?? text('admin/src/utils/permission.ts');
  const packageText = sources.packageText ?? text('package.json');
  const docsText = sources.docsText ?? `${text('docs/P8_EXECUTION_PLAN.md')}\n${text('docs/PROGRESS.md')}\n${text('docs/P8_TASK_BATCH_9_FINAL_GATE.md')}`;
  const combined = `${apiServiceText}\n${apiServiceTestsText}\n${foundationTestsText}\n${adaptersText}\n${rbacText}\n${backendPermText}\n${frontendPermText}\n${docsText}`;

  const checks = [
    ['batchId', evidence.batchId === 'P8-TASK-BATCH-9'],
    ['P8-701 status', evidence.tasks?.['P8-701']?.status === 'completed'],
    ['P8-702 status', evidence.tasks?.['P8-702']?.status === 'completed'],
    ['P8-703 status', evidence.tasks?.['P8-703']?.status === 'completed'],
    ['P8-704 status', evidence.tasks?.['P8-704']?.status === 'completed'],
    ['P8-705 status', evidence.tasks?.['P8-705']?.status === 'completed'],
    ['integrationFixturesPresent', evidence.integrationFixturesPresent === true && hasAll(apiServiceTestsText, ['TestAPIExecuteRejectsStaleExpectedTaskRevision', 'TestAPIRetryValidatesFailedAttemptID', 'TestAPIRetryAcceptsMatchingFailedAttemptID'])],
    ['executeRevisionPrecondition', evidence.executeRevisionPrecondition === true && hasAll(apiServiceText, ['req.ExpectedTaskRevision > 0', 'ErrRevisionConflict', 'CanExecute'])],
    ['retryPreconditions', evidence.retryPreconditions === true && hasAll(apiServiceText, ['req.FailedAttemptID != nil', 'ExecutionAttemptStatusFailed', 'CanRetry', 'req.ExpectedTaskRevision > 0'])],
    ['roleBoundaryAligned', evidence.roleBoundaryAligned === true && hasAll(`${backendPermText}\n${frontendPermText}\n${rbacText}`, ['operationtask.edit', 'PermOperationTaskEdit', 'OPERATION_TASK_EDIT', 'PermOperationTaskExecute', 'PermOperationTaskRetry']) && foundationTestsText.includes('operatorID') && foundationTestsText.includes('reviewerID')],
    ['authenticatedAPIGoldenPathPassed', evidence.authenticatedAPIGoldenPathPassed === true && evidence.authenticatedAPIGoldenPathMode === 'real_backend_bearer_token'],
    ['adminAuthenticatedE2EPassed', evidence.adminAuthenticatedE2EPassed === true && evidence.adminE2EMode === 'real_backend_api'],
    ['unauthRedirectVerified', evidence.unauthRedirectVerified === true],
    ['apiAuthRequiredVerified', evidence.apiAuthRequiredVerified === true],
    ['platformBoundaryGatePassed', evidence.platformBoundaryGatePassed === true && hasAll(adaptersText, ['SafeDraftCreationCapabilities', 'DraftCreation: true', 'capabilities.Publish || capabilities.Listing || capabilities.NetworkAccess || capabilities.RealCredentials || capabilities.AutomaticExecution', 'production_capability_forbidden'])],
    ['safeAdapterModesOnly', evidence.safeAdapterModesOnly === true && hasAll(adaptersText, ['ExecutionPortModeLocalDraftFixture', 'ExecutionPortModeMock', 'ExecutionPortModeSandboxFixture'])],
    ['noProductionPlatformWrite', evidence.realCredentialsEnabled === false && evidence.realPlatformWriteEnabled === false && evidence.automaticPublishEnabled === false && evidence.automaticListingEnabled === false],
    ['productionReadyFalse', evidence.productionReady === false && !combined.includes('productionReady=true') && !docsText.includes('Production Ready: true')],
    ['closureEvidencePresent', evidence.closureEvidencePresent === true && evidence.phaseStatus === 'Development Complete'],
    ['packageScriptsRegistered', hasAll(packageText, ['test:p8-task-batch-9', 'p8:task-batch-9-gate'])],
  ];

  const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failed.length ? 'failed' : 'passed',
    failed,
    failedCount: failed.length,
    checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  };
}

export function buildP8TaskBatch9GateReport(bundle = {}) {
  const evidence = bundle.evidence ?? readJSON(P8_TASK_BATCH_9_EVIDENCE_JSON) ?? {};
  const validation = validateP8TaskBatch9Bundle({ evidence, sources: bundle.sources });
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
  if (head !== '2635f2438ca6c83bb28fa5016d9bb1cd16e3c40e') failed.push('headChanged');
  return {
    phase: 'P8',
    gate: 'P8-TASK-BATCH-9',
    status: failed.length ? 'failed' : 'passed',
    checkedAt: '2026-07-26T00:00:00.000Z',
    batchId: 'P8-TASK-BATCH-9',
    tasks: ['P8-701', 'P8-702', 'P8-703', 'P8-704', 'P8-705'],
    currentBranch,
    headDetached,
    head,
    stagedFileCount: stagedFiles === '' ? 0 : stagedFiles.split('\n').length,
    authenticatedAPIGoldenPathPassed: evidence.authenticatedAPIGoldenPathPassed === true,
    adminAuthenticatedE2EPassed: evidence.adminAuthenticatedE2EPassed === true,
    platformBoundaryGatePassed: evidence.platformBoundaryGatePassed === true,
    realCredentialsEnabled: evidence.realCredentialsEnabled === true,
    realPlatformWriteEnabled: evidence.realPlatformWriteEnabled === true,
    automaticPublishEnabled: evidence.automaticPublishEnabled === true,
    automaticListingEnabled: evidence.automaticListingEnabled === true,
    productionReady: evidence.productionReady === true,
    failedCount: failed.length,
    failed,
    checks: validation.checks,
  };
}

export function writeP8TaskBatch9GateReport(report) {
  writeJSON(P8_TASK_BATCH_9_GATE_JSON, report);
  writeMarkdown(
    P8_TASK_BATCH_9_GATE_MD,
    `# P8 Task Batch 9 Final Gate\n\nStatus: **${report.status}**\n\n- Batch: ${report.batchId}\n- Tasks: ${report.tasks.join(', ')}\n- Current branch: ${report.currentBranch}\n- Head detached: ${report.headDetached}\n- HEAD: ${report.head}\n- Staged files: ${report.stagedFileCount}\n- Authenticated API golden path passed: ${report.authenticatedAPIGoldenPathPassed}\n- Admin authenticated E2E passed: ${report.adminAuthenticatedE2EPassed}\n- Platform boundary gate passed: ${report.platformBoundaryGatePassed}\n- Real credentials enabled: ${report.realCredentialsEnabled}\n- Real platform write enabled: ${report.realPlatformWriteEnabled}\n- Automatic publish enabled: ${report.automaticPublishEnabled}\n- Automatic listing enabled: ${report.automaticListingEnabled}\n- Production Ready: ${report.productionReady}\n- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}\n\nThis gate validates only P8 final development closure. Passing this gate does not authorize real credentials, real platform writes, automatic publish/listing, production gray, tag, release, or Production Ready. Authenticated Admin/API golden path evidence must come from a real local backend Bearer-token login flow; static frontend mocks do not satisfy Batch 9.\n`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildP8TaskBatch9GateReport();
  writeP8TaskBatch9GateReport(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}
