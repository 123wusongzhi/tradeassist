import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';

export const P9_BATCH_3_SYNC_ORCHESTRATION_JSON = 'docs/p9-task-batch-3-sync-orchestration.json';
export const P9_BATCH_3_SYNC_ORCHESTRATION_MD = 'docs/P9_TASK_BATCH_3_SYNC_ORCHESTRATION.md';
export const P9_BATCH_3_SYNC_ORCHESTRATION_GATE_JSON = 'docs/p9-task-batch-3-sync-orchestration-gate.json';
export const P9_BATCH_3_SYNC_ORCHESTRATION_GATE_MD = 'docs/P9_TASK_BATCH_3_SYNC_ORCHESTRATION_GATE.md';

const TASK_IDS = ['P9-701', 'P9-702', 'P9-703', 'P9-704', 'P9-705', 'P9-706'];

const REQUIRED_FILES = [
  'backend/internal/modules/inventorysyncp9/inventory_provider.go',
  'backend/internal/modules/inventorysyncp9/binding_resolution_pipeline.go',
  'backend/internal/modules/inventorysyncp9/sync_orchestration.go',
  'backend/internal/modules/inventorysyncp9/sync_orchestration_test.go',
  P9_BATCH_3_SYNC_ORCHESTRATION_MD,
  P9_BATCH_3_SYNC_ORCHESTRATION_JSON,
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

const REQUIRED_SCENARIOS = [
  'success_single_page',
  'success_multi_page',
  'empty_inventory',
  'low_confidence_binding',
  'binding_conflict',
  'unmatched_sku',
  'provider_timeout',
  'provider_rejected',
  'malformed_item',
  'duplicate_external_sku',
  'cursor_loop',
  'cancelled_context',
];

const REQUIRED_ERROR_CODES = [
  'provider_not_registered',
  'provider_capability_forbidden',
  'provider_cursor_invalid',
  'provider_cursor_loop',
  'provider_page_limit_exceeded',
  'provider_page_invalid',
  'provider_rejected',
  'provider_timeout',
  'sync_run_already_running',
  'sync_cancelled',
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

function forbiddenSourceAbsent(value) {
  return !/(net\/http|http\.Client|access_token|refresh_token|cron|ticker|queue|gin\.|router|InventoryMutation:\s*true|NetworkAccess:\s*true|OAuth:\s*true|Credentials:\s*true|RealPlatformRead:\s*true|RealPlatformWrite:\s*true|RealInventoryRead:\s*true|RealInventoryWrite:\s*true)/i.test(value);
}

export function validateP9Batch3SyncOrchestrationBundle({ evidence = {}, sources = {}, gitState = {} } = {}) {
  const providerText = sources.providerText ?? text('backend/internal/modules/inventorysyncp9/inventory_provider.go');
  const orchestratorText = sources.orchestratorText ?? text('backend/internal/modules/inventorysyncp9/sync_orchestration.go');
  const pipelineText = sources.pipelineText ?? text('backend/internal/modules/inventorysyncp9/binding_resolution_pipeline.go');
  const calibrationText = sources.calibrationText ?? text('backend/internal/modules/inventorysyncp9/calibration.go');
  const errorsText = sources.errorsText ?? text('backend/internal/modules/inventorysyncp9/errors.go');
  const testText = sources.testText ?? text('backend/internal/modules/inventorysyncp9/sync_orchestration_test.go');
  const packageText = sources.packageText ?? text('package.json');
  const missingFiles = sources.requiredFilesPresent === true ? [] : REQUIRED_FILES.filter((rel) => !fs.existsSync(rootPath(rel)));
  const branch = gitState.currentBranch ?? git(['branch', '--show-current']);
  const head = gitState.currentHead ?? git(['rev-parse', 'HEAD']);
  const staged = gitState.stagedFileCount ?? stagedFileCount();
  const dirty = gitState.workingTreeDirty ?? workingTreeDirty();

  const checks = [
    ['requiredFilesPresent', missingFiles.length === 0],
    ['batchId', evidence.batchId === 'P9-TASK-BATCH-3'],
    ['currentBranch', branch === 'dev'],
    ['currentHeadPresent', typeof head === 'string' && head.length > 0],
    ['stagedFileCount', staged === 0],
    ['workingTreeDirtyRecorded', evidence.workingTreeDirty === dirty],
    ['changesCommitted', evidence.changesCommitted === false],
    ...TASK_IDS.map((id) => [`${id} status`, String(evidence.tasks?.[id]?.status || '') === 'completed']),
    ['providerPortImplemented', evidence.providerPortImplemented === true && hasAll(providerText, ['type InventoryProvider interface', 'FetchInventoryPage', 'InventoryProviderCapabilities', 'InventoryProviderRegistry'])],
    ['fixtureScenariosImplemented', evidence.fixtureScenariosImplemented === true && arrayIncludes(evidence.fixtureScenarios, REQUIRED_SCENARIOS) && hasAll(providerText, REQUIRED_SCENARIOS)],
    ['unsafeProviderCapabilitiesRejected', evidence.unsafeProviderCapabilitiesRejected === true && hasAll(providerText, ['NetworkAccess', 'OAuth', 'Credentials', 'RealPlatformRead', 'RealInventoryWrite', 'InventoryMutation', 'ErrProviderCapabilityForbidden'])],
    ['providerModeContractPreserved', evidence.providerModeContractPreserved === true && hasAll(providerText, ['ProviderModeMock', 'ProviderModeSandbox', 'ProviderModeLocalDraftOnly']) && !providerText.includes('ProviderModeProduction')],
    ['cursorContractImplemented', evidence.cursorContractImplemented === true && hasAll(providerText + orchestratorText, ['inventoryFixtureCursor', 'FixtureHash', 'PageIndex', 'ErrProviderCursorInvalid', 'ErrProviderCursorLoop'])],
    ['orchestratorImplemented', evidence.syncOrchestratorImplemented === true && hasAll(orchestratorText, ['type InventorySyncOrchestrator struct', 'func (o *InventorySyncOrchestrator) Run', 'commitPage', 'finishWithError'])],
    ['providerFetchOutsideTransaction', evidence.providerFetchOutsideTransaction === true && orchestratorText.indexOf('provider.FetchInventoryPage') > -1 && orchestratorText.indexOf('func (o *InventorySyncOrchestrator) commitPage') > orchestratorText.indexOf('provider.FetchInventoryPage')],
    ['pageTransactionImplemented', evidence.pageTransactionImplemented === true && hasAll(orchestratorText, ['Transaction(func(tx *gorm.DB) error', 'CreateBatch', 'ResolvePageWithDB', 'updateRunStatusWithDB'])],
    ['bindingResolutionImplemented', evidence.bindingResolutionPipelineImplemented === true && hasAll(pipelineText, ['type BindingResolutionPipeline', 'getCurrentConfirmedWithDB', 'calibrateSnapshotWithDB', 'manual_review_required'])],
    ['autoConfirmationRespected', evidence.autoConfirmationEnabled === false && evidence.automaticConfirmedBindingCreatedByBatch3 === false && !orchestratorText.includes('CreateConfirmed')],
    ['manualRerunImplemented', evidence.manualRerunImplemented === true && evidence.defaultRerunAllow === false && hasAll(orchestratorText, ['ManualRerun', 'InventorySyncAuthorizer', 'ErrPermissionDenied'])],
    ['failureClassificationImplemented', evidence.failureClassificationImplemented === true && arrayIncludes(evidence.stableErrors, REQUIRED_ERROR_CODES) && hasAll(errorsText + providerText + orchestratorText, REQUIRED_ERROR_CODES)],
    ['idempotencyConcurrencyImplemented', evidence.idempotencyTestsPassed === true && evidence.concurrencyTestsPassed === true && hasAll(orchestratorText, ['IdempotencyKeyHash', 'InputFingerprint', 'inventorySyncLockRegistry'])],
    ['testsPresent', evidence.testsPassed === true && hasAll(testText, ['TestInventoryProviderRegistryAndFixtureProviderSafety', 'TestInventorySyncOrchestratorProcessesFixturePages', 'TestBindingResolutionPipelinePrioritizesConfirmedBinding', 'TestInventorySyncOrchestratorFailureCancellationIdempotencyAndRerun', 'TestInventorySyncOrchestratorConcurrentSameRequestCreatesOneRun'])],
    ['raceTestsPassed', evidence.raceTestsPassed === true],
    ['postgresRecordedTruthfully', evidence.postgresIntegrationStatus === 'not_run' && evidence.postgresIntegrationPassed === false && evidence.postgresIntegrationDeferredTo === 'P9_Final_Development_Closure' && evidence.p9FinalClosureBlocker === true],
    ['packageScriptsPresent', hasAll(packageText, ['test:p9-task-batch-3-sync-orchestration', 'p9:task-batch-3-sync-orchestration-gate'])],
    ['forbiddenPathsAbsent', forbiddenPathsAbsent()],
    ['noForbiddenSource', forbiddenSourceAbsent(providerText + orchestratorText + pipelineText + calibrationText)],
    ['scopeProtectionFlags', evidence.syncWorkerImplemented === false && evidence.cronImplemented === false && evidence.tickerImplemented === false && evidence.queueConsumerImplemented === false && evidence.apiImplemented === false && evidence.adminUiImplemented === false],
    ['realBoundaryFlags', evidence.realDouyinProviderImplemented === false && evidence.oauthImplemented === false && evidence.realCredentialsEnabled === false && evidence.realPlatformNetworkEnabled === false && evidence.realPlatformReadEnabled === false && evidence.realPlatformWriteEnabled === false && evidence.realInventoryReadEnabled === false && evidence.realInventoryWriteEnabled === false],
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

export function buildP9Batch3SyncOrchestrationGateReport(bundle = {}) {
  const evidence = bundle.evidence ?? readJSON(P9_BATCH_3_SYNC_ORCHESTRATION_JSON) ?? {};
  const validation = validateP9Batch3SyncOrchestrationBundle({ evidence, sources: bundle.sources, gitState: bundle.gitState });
  return {
    phase: 'P9',
    gate: 'P9-TASK-BATCH-3-SYNC-ORCHESTRATION',
    status: validation.status,
    checkedAt: new Date().toISOString(),
    batchId: evidence.batchId || '',
    currentBranch: validation.currentBranch,
    currentHead: validation.currentHead,
    stagedFileCount: validation.stagedFileCount,
    workingTreeDirty: validation.workingTreeDirty,
    tasks: TASK_IDS,
    providerPortImplemented: evidence.providerPortImplemented === true,
    fixtureScenariosImplemented: evidence.fixtureScenariosImplemented === true,
    cursorContractImplemented: evidence.cursorContractImplemented === true,
    syncOrchestratorImplemented: evidence.syncOrchestratorImplemented === true,
    bindingResolutionPipelineImplemented: evidence.bindingResolutionPipelineImplemented === true,
    manualRerunImplemented: evidence.manualRerunImplemented === true,
    postgresIntegrationStatus: evidence.postgresIntegrationStatus || '',
    syncWorkerImplemented: evidence.syncWorkerImplemented === true,
    cronImplemented: evidence.cronImplemented === true,
    tickerImplemented: evidence.tickerImplemented === true,
    queueConsumerImplemented: evidence.queueConsumerImplemented === true,
    apiImplemented: evidence.apiImplemented === true,
    adminUiImplemented: evidence.adminUiImplemented === true,
    realDouyinProviderImplemented: evidence.realDouyinProviderImplemented === true,
    realPlatformNetworkEnabled: evidence.realPlatformNetworkEnabled === true,
    p10BoundaryPreserved: evidence.p10BoundaryPreserved === true,
    productionReady: evidence.productionReady === true,
    p9Complete: evidence.p9Complete === true,
    failedCount: validation.failedCount,
    failed: validation.failed,
    missingFiles: validation.missingFiles,
    checks: validation.checks,
  };
}

export function writeP9Batch3SyncOrchestrationGateReport(report) {
  writeJSON(P9_BATCH_3_SYNC_ORCHESTRATION_GATE_JSON, report);
  writeMarkdown(
    P9_BATCH_3_SYNC_ORCHESTRATION_GATE_MD,
    `# P9 Batch 3 Sync Orchestration Gate

Status: **${report.status}**

- Batch id: ${report.batchId}
- Current branch: ${report.currentBranch}
- Current head: ${report.currentHead}
- Staged files: ${report.stagedFileCount}
- Working tree dirty: ${report.workingTreeDirty}
- Tasks: ${report.tasks.join(', ')}
- Provider port implemented: ${report.providerPortImplemented}
- Fixture scenarios implemented: ${report.fixtureScenariosImplemented}
- Cursor contract implemented: ${report.cursorContractImplemented}
- Sync orchestrator implemented: ${report.syncOrchestratorImplemented}
- Binding resolution pipeline implemented: ${report.bindingResolutionPipelineImplemented}
- Manual rerun implemented: ${report.manualRerunImplemented}
- PostgreSQL integration status: ${report.postgresIntegrationStatus}
- Sync worker implemented: ${report.syncWorkerImplemented}
- Cron implemented: ${report.cronImplemented}
- Ticker implemented: ${report.tickerImplemented}
- Queue consumer implemented: ${report.queueConsumerImplemented}
- API implemented: ${report.apiImplemented}
- Admin UI implemented: ${report.adminUiImplemented}
- Real Douyin provider implemented: ${report.realDouyinProviderImplemented}
- Real platform network enabled: ${report.realPlatformNetworkEnabled}
- P10 boundary preserved: ${report.p10BoundaryPreserved}
- Production Ready: ${report.productionReady}
- P9 complete: ${report.p9Complete}
- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}

This gate validates only P9 Batch 3 fixture inventory sync orchestration. It does not authorize background workers, automatic retries, API, Admin UI, real Douyin/OAuth/credentials, real platform network, real inventory reads/writes, release, tag, P9 closure, or Production Ready.
`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildP9Batch3SyncOrchestrationGateReport();
  writeP9Batch3SyncOrchestrationGateReport(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}
