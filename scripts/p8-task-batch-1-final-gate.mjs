import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { readJSON, writeJSON, writeMarkdown } from './p7-v2-lib.mjs';

export const P8_TASK_BATCH_1_EVIDENCE_JSON = 'docs/p8-task-batch-1-domain-persistence-and-repository.json';
export const P8_TASK_BATCH_1_GATE_JSON = 'docs/p8-task-batch-1-final-gate.json';
export const P8_TASK_BATCH_1_GATE_MD = 'docs/P8_TASK_BATCH_1_FINAL_GATE.md';
export const P8_TASK_BATCH_1_PLAN_CHECKPOINT = 'ea356d8077722e2f94c6215fe10c7d4f6e53fde5';

const requiredFiles = [
  'backend/internal/modules/operationtask/model.go',
  'backend/internal/modules/operationtask/repository.go',
  'backend/internal/modules/operationtask/migrate.go',
  'backend/internal/modules/operationtask/repository_test.go',
  'backend/internal/database/migrate.go',
  'docs/P8_TASK_BATCH_1_DOMAIN_PERSISTENCE_AND_REPOSITORY.md',
  'docs/p8-task-batch-1-domain-persistence-and-repository.json',
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

function arrayIncludes(values, expected) {
  const set = new Set(Array.isArray(values) ? values : []);
  return expected.every((item) => set.has(item));
}

export function validateP8TaskBatch1Bundle({ evidence = {}, sources = {} } = {}) {
  const modelText = sources.modelText ?? text('backend/internal/modules/operationtask/model.go');
  const repoText = sources.repoText ?? text('backend/internal/modules/operationtask/repository.go');
  const migrateText = sources.migrateText ?? text('backend/internal/modules/operationtask/migrate.go');
  const validationText = sources.validationText ?? text('backend/internal/modules/operationtask/validation.go');
  const testText = sources.testText ?? text('backend/internal/modules/operationtask/repository_test.go');
  const dbMigrateText = sources.dbMigrateText ?? text('backend/internal/database/migrate.go');
  const checks = [
    ['p8PlanCheckpoint', evidence.baseCheckpoint === P8_TASK_BATCH_1_PLAN_CHECKPOINT],
    ['P8-101 status', evidence.tasks?.['P8-101']?.status === 'completed'],
    ['P8-102 status', evidence.tasks?.['P8-102']?.status === 'completed'],
    ['P8-106 status', evidence.tasks?.['P8-106']?.status === 'completed'],
    ['operationTaskModelPresent', evidence.operationTaskModelPresent === true && modelText.includes('type OperationTask struct')],
    ['platformDraftModelPresent', evidence.platformDraftModelPresent === true && modelText.includes('type PlatformDraft struct')],
    ['operationTaskMigrationPresent', evidence.operationTaskMigrationPresent === true && migrateText.includes('operation_tasks')],
    ['platformDraftMigrationPresent', evidence.platformDraftMigrationPresent === true && migrateText.includes('platform_drafts')],
    ['autoMigrateRegistered', dbMigrateText.includes('operationtask.Migrate(db)')],
    ['tenantIsolationImplemented', evidence.tenantIsolationPassed === true && hasAll(repoText, ['tenant_id = ? AND id = ?', 'tenant_id = ? AND idempotency_key = ?'])],
    ['idempotencyConstraintPresent', evidence.idempotencyTestsPassed === true && migrateText.includes('ux_operation_tasks_tenant_idempotency_key')],
    ['revisionConstraintPresent', evidence.revisionTestsPassed === true && hasAll(repoText, ['revision + 1', 'revision = ?'])],
    ['draftVersionConstraintPresent', evidence.draftVersionTestsPassed === true && (modelText + migrateText).includes('ux_platform_drafts_tenant_task_version')],
    ['payloadHashConstraintPresent', evidence.payloadHashConstraintPresent === true && hasAll(modelText, ['PayloadHash', 'payloadHash']) && validationText.includes('sha256LowerHex')],
    ['adapterModeConstraintPresent', evidence.adapterModeConstraintPresent === true && modelText.includes('AdapterModeLocalDraftOnly')],
    ['repositoryTestsPassed', evidence.repositoryTestsPassed === true && testText.includes('TestOperationTaskRepositoryCreateReadTenantIdempotencyRevisionAndList')],
    ['migrationTestsPassed', evidence.migrationTestsPassed === true && testText.includes('operationtask.Migrate(db)')],
    ['concurrencyTestsPassed', evidence.concurrencyTestsPassed === true && testText.includes('TestConcurrentIdempotencyAndDraftVersionUseDatabaseConstraints')],
    ['requiredRepositoryMethods', arrayIncludes(evidence.repositoryMethods, ['OperationTask.Create', 'OperationTask.GetByID', 'OperationTask.GetByIdempotencyKey', 'OperationTask.List', 'OperationTask.UpdateRevision', 'PlatformDraft.CreateVersion', 'PlatformDraft.GetByID', 'PlatformDraft.GetVersion', 'PlatformDraft.GetLatest', 'PlatformDraft.ListVersions'])],
    ['approvalServiceImplemented', evidence.approvalServiceImplemented === false],
    ['executionServiceImplemented', evidence.executionServiceImplemented === false],
    ['platformWriteImplemented', evidence.platformWriteImplemented === false],
    ['adminUiImplemented', evidence.adminUiImplemented === false],
    ['apiImplemented', evidence.apiImplemented === false],
    ['realCredentialsEnabled', evidence.realCredentialsEnabled === false],
    ['realPlatformWriteEnabled', evidence.realPlatformWriteEnabled === false],
    ['automaticPublishEnabled', evidence.automaticPublishEnabled === false],
    ['automaticListingEnabled', evidence.automaticListingEnabled === false],
    ['p7DeferredPerformancePreserved', evidence.p7DeferredPerformancePreserved === true],
    ['p10ProductionBoundaryPreserved', evidence.p10ProductionBoundaryPreserved === true],
    ['productionReady', evidence.productionReady === false],
    ['forbiddenTablesAbsent', !hasAll(modelText + migrateText, ['approval_records']) && !hasAll(modelText + migrateText, ['execution_attempts']) && !hasAll(modelText + migrateText, ['operation_task_events'])],
    ['forbiddenBusinessServicesAbsent', !repoText.includes('Approve(') && !repoText.includes('Execute(') && !repoText.includes('WriteToPlatform')],
  ];
  const failed = checks.filter(([, ok]) => !ok).map(([id]) => id);
  return {
    status: failed.length ? 'failed' : 'passed',
    failed,
    failedCount: failed.length,
    checks: checks.map(([id, ok]) => ({ id, status: ok ? 'passed' : 'failed' })),
  };
}

export function buildP8TaskBatch1GateReport(bundle = {}) {
  const evidence = bundle.evidence ?? readJSON(P8_TASK_BATCH_1_EVIDENCE_JSON) ?? {};
  const validation = validateP8TaskBatch1Bundle({ evidence, sources: bundle.sources });
  const missingFiles = requiredFiles.filter((rel) => !fs.existsSync(rootPath(rel)));
  const failed = [...validation.failed, ...missingFiles.map((rel) => `missing:${rel}`)];
  return {
    phase: 'P8',
    gate: 'P8-TASK-BATCH-1',
    status: failed.length ? 'failed' : 'passed',
    checkedAt: '2026-07-20T00:00:00.000Z',
    batchId: 'P8-TASK-BATCH-1',
    p8PlanCheckpoint: P8_TASK_BATCH_1_PLAN_CHECKPOINT,
    tasks: ['P8-101', 'P8-102', 'P8-106'],
    operationTaskModelPresent: evidence.operationTaskModelPresent === true,
    platformDraftModelPresent: evidence.platformDraftModelPresent === true,
    operationTaskMigrationPresent: evidence.operationTaskMigrationPresent === true,
    platformDraftMigrationPresent: evidence.platformDraftMigrationPresent === true,
    tenantIsolationImplemented: evidence.tenantIsolationPassed === true,
    idempotencyConstraintPresent: evidence.idempotencyTestsPassed === true,
    revisionConstraintPresent: evidence.revisionTestsPassed === true,
    draftVersionConstraintPresent: evidence.draftVersionTestsPassed === true,
    payloadHashConstraintPresent: evidence.payloadHashConstraintPresent === true,
    adapterModeConstraintPresent: evidence.adapterModeConstraintPresent === true,
    repositoryTestsPassed: evidence.repositoryTestsPassed === true,
    migrationTestsPassed: evidence.migrationTestsPassed === true,
    concurrencyTestsPassed: evidence.concurrencyTestsPassed === true,
    approvalServiceImplemented: evidence.approvalServiceImplemented === true,
    executionServiceImplemented: evidence.executionServiceImplemented === true,
    platformWriteImplemented: evidence.platformWriteImplemented === true,
    adminUiImplemented: evidence.adminUiImplemented === true,
    apiImplemented: evidence.apiImplemented === true,
    realCredentialsEnabled: evidence.realCredentialsEnabled === true,
    realPlatformWriteEnabled: evidence.realPlatformWriteEnabled === true,
    automaticPublishEnabled: evidence.automaticPublishEnabled === true,
    automaticListingEnabled: evidence.automaticListingEnabled === true,
    p7DeferredPerformancePreserved: evidence.p7DeferredPerformancePreserved === true,
    p10ProductionBoundaryPreserved: evidence.p10ProductionBoundaryPreserved === true,
    productionReady: evidence.productionReady === true,
    failedCount: failed.length,
    failed,
    checks: validation.checks,
  };
}

export function writeP8TaskBatch1GateReport(report) {
  writeJSON(P8_TASK_BATCH_1_GATE_JSON, report);
  writeMarkdown(
    P8_TASK_BATCH_1_GATE_MD,
    `# P8 Task Batch 1 Final Gate

Status: **${report.status}**

- Batch: ${report.batchId}
- P8 plan checkpoint: ${report.p8PlanCheckpoint}
- Tasks: ${report.tasks.join(', ')}
- Operation task model present: ${report.operationTaskModelPresent}
- Platform draft model present: ${report.platformDraftModelPresent}
- Operation task migration present: ${report.operationTaskMigrationPresent}
- Platform draft migration present: ${report.platformDraftMigrationPresent}
- Tenant isolation implemented: ${report.tenantIsolationImplemented}
- Idempotency constraint present: ${report.idempotencyConstraintPresent}
- Revision constraint present: ${report.revisionConstraintPresent}
- Draft version constraint present: ${report.draftVersionConstraintPresent}
- Payload hash constraint present: ${report.payloadHashConstraintPresent}
- Adapter mode constraint present: ${report.adapterModeConstraintPresent}
- Repository tests passed: ${report.repositoryTestsPassed}
- Migration tests passed: ${report.migrationTestsPassed}
- Concurrency tests passed: ${report.concurrencyTestsPassed}
- Approval service implemented: ${report.approvalServiceImplemented}
- Execution service implemented: ${report.executionServiceImplemented}
- API implemented: ${report.apiImplemented}
- Admin UI implemented: ${report.adminUiImplemented}
- Platform write implemented: ${report.platformWriteImplemented}
- Real credentials enabled: ${report.realCredentialsEnabled}
- Real platform write enabled: ${report.realPlatformWriteEnabled}
- Automatic publish enabled: ${report.automaticPublishEnabled}
- Automatic listing enabled: ${report.automaticListingEnabled}
- Production Ready: ${report.productionReady}
- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}

This gate validates only P8 Batch 1 domain persistence and repository work. It does not authorize approval services, execution orchestration, API, Admin UI, real credentials, real platform writes, automatic publish, automatic listing, production tag, production release, or Production Ready.
`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildP8TaskBatch1GateReport();
  writeP8TaskBatch1GateReport(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}
