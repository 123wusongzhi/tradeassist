import assert from 'node:assert/strict';
import { writeJSON } from '../../../scripts/p7-v2-lib.mjs';
import { validateP8TaskBatch1Bundle } from '../../../scripts/p8-task-batch-1-final-gate.mjs';

const validSources = {
  modelText: 'type OperationTask struct {}\ntype PlatformDraft struct {}\nPayloadHash string `json:"payloadHash"`\nAdapterModeLocalDraftOnly = "local_draft_only"',
  repoText: 'tenant_id = ? AND id = ?\ntenant_id = ? AND idempotency_key = ?\nrevision + 1\nrevision = ?\nsha256LowerHex\n',
  migrateText: 'operation_tasks\nplatform_drafts\nux_operation_tasks_tenant_idempotency_key\nux_platform_drafts_tenant_task_version\n',
  testText: 'TestOperationTaskRepositoryCreateReadTenantIdempotencyRevisionAndList\noperationtask.Migrate(db)\nTestConcurrentIdempotencyAndDraftVersionUseDatabaseConstraints\n',
  dbMigrateText: 'operationtask.Migrate(db)',
};

function validEvidence(overrides = {}) {
  return {
    batchId: 'P8-TASK-BATCH-1',
    baseCheckpoint: 'ea356d8077722e2f94c6215fe10c7d4f6e53fde5',
    tasks: {
      'P8-101': { status: 'completed' },
      'P8-102': { status: 'completed' },
      'P8-106': { status: 'completed' },
    },
    operationTaskModelPresent: true,
    platformDraftModelPresent: true,
    operationTaskMigrationPresent: true,
    platformDraftMigrationPresent: true,
    tenantIsolationPassed: true,
    idempotencyTestsPassed: true,
    revisionTestsPassed: true,
    draftVersionTestsPassed: true,
    payloadHashConstraintPresent: true,
    adapterModeConstraintPresent: true,
    repositoryTestsPassed: true,
    migrationTestsPassed: true,
    concurrencyTestsPassed: true,
    repositoryMethods: [
      'OperationTask.Create',
      'OperationTask.GetByID',
      'OperationTask.GetByIdempotencyKey',
      'OperationTask.List',
      'OperationTask.UpdateRevision',
      'PlatformDraft.CreateVersion',
      'PlatformDraft.GetByID',
      'PlatformDraft.GetVersion',
      'PlatformDraft.GetLatest',
      'PlatformDraft.ListVersions',
    ],
    approvalServiceImplemented: false,
    executionServiceImplemented: false,
    platformWriteImplemented: false,
    adminUiImplemented: false,
    apiImplemented: false,
    realCredentialsEnabled: false,
    realPlatformWriteEnabled: false,
    automaticPublishEnabled: false,
    automaticListingEnabled: false,
    p7DeferredPerformancePreserved: true,
    p10ProductionBoundaryPreserved: true,
    productionReady: false,
    ...overrides,
  };
}

function assertFails(id, overrides = {}, sourceOverrides = {}) {
  const result = validateP8TaskBatch1Bundle({
    evidence: validEvidence(overrides),
    sources: { ...validSources, ...sourceOverrides },
  });
  assert.equal(result.status, 'failed', id);
  assert.ok(result.failed.includes(id), `${id} should fail, saw ${result.failed.join(', ')}`);
}

assert.equal(validateP8TaskBatch1Bundle({ evidence: validEvidence(), sources: validSources }).status, 'passed');
assertFails('P8-101 status', { tasks: { ...validEvidence().tasks, 'P8-101': { status: 'pending' } } });
assertFails('tenantIsolationImplemented', { tenantIsolationPassed: false });
assertFails('idempotencyConstraintPresent', { idempotencyTestsPassed: false });
assertFails('draftVersionConstraintPresent', { draftVersionTestsPassed: false });
assertFails('adapterModeConstraintPresent', { adapterModeConstraintPresent: false });
assertFails('repositoryTestsPassed', { repositoryTestsPassed: false });
assertFails('approvalServiceImplemented', { approvalServiceImplemented: true });
assertFails('executionServiceImplemented', { executionServiceImplemented: true });
assertFails('platformWriteImplemented', { platformWriteImplemented: true });
assertFails('automaticPublishEnabled', { automaticPublishEnabled: true });
assertFails('automaticListingEnabled', { automaticListingEnabled: true });
assertFails('p7DeferredPerformancePreserved', { p7DeferredPerformancePreserved: false });

const report = {
  phase: 'P8',
  batchId: 'P8-TASK-BATCH-1',
  status: 'passed',
  fixtures: 12,
};
writeJSON('docs/p8-task-batch-1-fixture-report.json', report);
console.log(JSON.stringify(report, null, 2));

