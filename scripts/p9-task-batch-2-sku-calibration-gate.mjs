import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';

export const P9_BATCH_2_SKU_CALIBRATION_JSON = 'docs/p9-task-batch-2-sku-calibration.json';
export const P9_BATCH_2_SKU_CALIBRATION_MD = 'docs/P9_TASK_BATCH_2_SKU_CALIBRATION.md';
export const P9_BATCH_2_SKU_CALIBRATION_GATE_JSON = 'docs/p9-task-batch-2-sku-calibration-gate.json';
export const P9_BATCH_2_SKU_CALIBRATION_GATE_MD = 'docs/P9_TASK_BATCH_2_SKU_CALIBRATION_GATE.md';

const TASK_IDS = ['P9-601', 'P9-602', 'P9-603', 'P9-604', 'P9-605', 'P9-606'];

const REQUIRED_FILES = [
  'backend/internal/modules/inventorysyncp9/normalization.go',
  'backend/internal/modules/inventorysyncp9/calibration.go',
  'backend/internal/modules/inventorysyncp9/manual_binding.go',
  'backend/internal/modules/inventorysyncp9/calibration_service_test.go',
  P9_BATCH_2_SKU_CALIBRATION_MD,
  P9_BATCH_2_SKU_CALIBRATION_JSON,
];

const FORBIDDEN_PATHS = [
  'admin/src/pages/inventorysyncp9',
  'admin/src/services/inventorysyncp9',
  'backend/internal/api/inventorysyncp9',
  'backend/internal/modules/inventorysyncp9/handler.go',
  'backend/internal/modules/inventorysyncp9/worker.go',
  'backend/internal/providers/douyin/inventorysyncp9',
];

const REPO_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

function readJSON(rel) {
  try {
    return JSON.parse(fs.readFileSync(path.join(REPO_ROOT, rel), 'utf8'));
  } catch {
    return null;
  }
}

function writeJSON(rel, data) {
  const full = path.join(REPO_ROOT, rel);
  fs.mkdirSync(path.dirname(full), { recursive: true });
  fs.writeFileSync(full, `${JSON.stringify(data, null, 2)}\n`, 'utf8');
}

function writeMarkdown(rel, body) {
  const full = path.join(REPO_ROOT, rel);
  fs.mkdirSync(path.dirname(full), { recursive: true });
  fs.writeFileSync(full, body, 'utf8');
}

const REQUIRED_ERROR_CODES = [
  'invalid_identifier',
  'normalization_failed',
  'no_binding_candidate',
  'multiple_binding_candidates',
  'binding_conflict',
  'calibration_policy_invalid',
  'calibration_threshold_not_met',
  'manual_review_required',
  'manual_binding_already_pending',
  'manual_binding_already_resolved',
  'candidate_local_sku_not_found',
  'candidate_local_sku_tenant_mismatch',
  'permission_denied',
  'revision_conflict',
  'idempotency_payload_conflict',
];

function rootPath(rel) {
  return path.join(REPO_ROOT, rel);
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

export function validateP9Batch2SKUCalibrationBundle({ evidence = {}, sources = {}, gitState = {} } = {}) {
  const normalizationText = sources.normalizationText ?? text('backend/internal/modules/inventorysyncp9/normalization.go');
  const calibrationText = sources.calibrationText ?? text('backend/internal/modules/inventorysyncp9/calibration.go');
  const manualText = sources.manualText ?? text('backend/internal/modules/inventorysyncp9/manual_binding.go');
  const modelText = sources.modelText ?? text('backend/internal/modules/inventorysyncp9/model.go');
  const migrateText = sources.migrateText ?? text('backend/internal/modules/inventorysyncp9/migrate.go');
  const errorsText = sources.errorsText ?? text('backend/internal/modules/inventorysyncp9/errors.go');
  const testText = sources.testText ?? text('backend/internal/modules/inventorysyncp9/calibration_service_test.go');
  const missingFiles = sources.requiredFilesPresent === true ? [] : REQUIRED_FILES.filter((rel) => !fs.existsSync(rootPath(rel)));
  const branch = gitState.currentBranch ?? git(['branch', '--show-current']);
  const head = gitState.currentHead ?? git(['rev-parse', 'HEAD']);
  const staged = gitState.stagedFileCount ?? stagedFileCount();
  const dirty = gitState.workingTreeDirty ?? workingTreeDirty();

  const checks = [
    ['requiredFilesPresent', missingFiles.length === 0],
    ['batchId', evidence.batchId === 'P9-TASK-BATCH-2'],
    ['currentBranch', branch === 'dev'],
    ['currentHeadPresent', typeof head === 'string' && head.length > 0],
    ['stagedFileCount', staged === 0],
    ['workingTreeDirtyRecorded', evidence.workingTreeDirty === dirty],
    ['changesCommitted', evidence.changesCommitted === false],
    ...TASK_IDS.map((id) => [`${id} status`, String(evidence.tasks?.[id]?.status || '') === 'completed']),
    ['normalizerImplemented', evidence.normalizerImplemented === true && hasAll(normalizationText, ['type SKUIdentifierNormalizer interface', 'NormalizeSKUCode', 'NormalizeBarcode', 'NormalizationVersionV1'])],
    ['barcodeLeadingZeroPreserved', evidence.barcodeLeadingZeroPreserved === true && testText.includes('0012345') && !normalizationText.includes('Atoi') && !normalizationText.includes('ParseFloat')],
    ['exactMatcherImplemented', evidence.exactMatcherImplemented === true && hasAll(calibrationText, ['type ExactIdentifierMatcher', 'MatchStrategyExactBarcode', 'MatchStrategyNormalizedSKUCode', 'MatchResultConflict'])],
    ['scoringImplemented', evidence.scoringImplemented === true && hasAll(calibrationText, ['type CandidateScoringService', 'ScoreBreakdownItem', 'ReasonCodes', 'Confidence'])],
    ['thresholdPolicyImplemented', evidence.thresholdPolicyImplemented === true && hasAll(calibrationText, ['type CalibrationThresholdPolicy', 'AutoConfirmationEnabled', 'ManualReviewRequired'])],
    ['manualBindingServiceImplemented', evidence.manualBindingServiceImplemented === true && hasAll(manualText, ['type ManualBindingService', 'type ManualBindingAuthorizer interface', 'ConfirmBinding', 'RejectBinding'])],
    ['authorizerDefaultDeny', evidence.authorizerRequired === true && evidence.defaultAllow === false && manualText.includes('Authorizer == nil') && manualText.includes('ErrPermissionDenied')],
    ['trustedActorImplemented', evidence.trustedActorRequired === true && manualText.includes('ManualBindingActor') && manualText.includes('ResolvedBy:             input.Actor.ActorID')],
    ['idempotencyImplemented', evidence.idempotencyTestsPassed === true && hasAll(manualText + calibrationText, ['IdempotencyKeyHash', 'PayloadFingerprint', 'ErrIdempotencyPayloadConflict'])],
    ['optimisticConcurrencyImplemented', evidence.optimisticConcurrencyTestsPassed === true && hasAll(manualText, ['ExpectedRevision', 'ErrRevisionConflict'])],
    ['transactionAtomicityImplemented', evidence.transactionAtomicityTestsPassed === true && hasAll(manualText + calibrationText, ['Transaction(func(tx *gorm.DB) error'])],
    ['manualDecisionHistoryImplemented', evidence.manualDecisionHistoryImplemented === true && hasAll(modelText + migrateText, ['ManualBindingDecision', 'p9_manual_binding_decisions', 'trg_p9_manual_binding_decisions_no_update'])],
    ['stableErrorsPresent', evidence.stableErrorsPresent === true && arrayIncludes(evidence.stableErrors, REQUIRED_ERROR_CODES) && hasAll(errorsText, REQUIRED_ERROR_CODES)],
    ['testsPresent', evidence.testsPassed === true && hasAll(testText, ['TestSKUIdentifierNormalizationRules', 'TestExactMatchingScoringAndPolicy', 'TestCalibrationServicePersistsCandidatesAndManualRequestIdempotently', 'TestManualBindingServiceAuthorizationIdempotencyAndConcurrency', 'TestManualBindingRejectPreservesCandidatesAndRequest'])],
    ['noAIOrVectorMatching', evidence.aiMatchingImplemented === false && !calibrationText.includes('llm') && !calibrationText.includes('vector')],
    ['forbiddenPathsAbsent', forbiddenPathsAbsent()],
    ['scopeProtectionFlags', evidence.syncOrchestratorImplemented === false && evidence.syncWorkerImplemented === false && evidence.apiImplemented === false && evidence.adminUiImplemented === false],
    ['realBoundaryFlags', evidence.realDouyinProviderImplemented === false && evidence.realCredentialsEnabled === false && evidence.realPlatformNetworkEnabled === false && evidence.realPlatformReadEnabled === false && evidence.realPlatformWriteEnabled === false],
    ['p10BoundaryPreserved', evidence.p10BoundaryPreserved === true],
    ['productionReady', evidence.productionReady === false],
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

export function buildP9Batch2SKUCalibrationGateReport(bundle = {}) {
  const evidence = bundle.evidence ?? readJSON(P9_BATCH_2_SKU_CALIBRATION_JSON) ?? {};
  const validation = validateP9Batch2SKUCalibrationBundle({ evidence, sources: bundle.sources, gitState: bundle.gitState });
  return {
    phase: 'P9',
    gate: 'P9-TASK-BATCH-2-SKU-CALIBRATION',
    status: validation.status,
    checkedAt: new Date().toISOString(),
    batchId: evidence.batchId || '',
    currentBranch: validation.currentBranch,
    currentHead: validation.currentHead,
    stagedFileCount: validation.stagedFileCount,
    workingTreeDirty: validation.workingTreeDirty,
    tasks: TASK_IDS,
    normalizerImplemented: evidence.normalizerImplemented === true,
    exactMatcherImplemented: evidence.exactMatcherImplemented === true,
    scoringImplemented: evidence.scoringImplemented === true,
    thresholdPolicyImplemented: evidence.thresholdPolicyImplemented === true,
    manualBindingServiceImplemented: evidence.manualBindingServiceImplemented === true,
    authorizerRequired: evidence.authorizerRequired === true,
    defaultAllow: evidence.defaultAllow === true,
    postgresIntegrationStatus: evidence.postgresIntegrationStatus || '',
    syncOrchestratorImplemented: evidence.syncOrchestratorImplemented === true,
    syncWorkerImplemented: evidence.syncWorkerImplemented === true,
    apiImplemented: evidence.apiImplemented === true,
    adminUiImplemented: evidence.adminUiImplemented === true,
    realDouyinProviderImplemented: evidence.realDouyinProviderImplemented === true,
    realPlatformNetworkEnabled: evidence.realPlatformNetworkEnabled === true,
    productionReady: evidence.productionReady === true,
    p10BoundaryPreserved: evidence.p10BoundaryPreserved === true,
    failedCount: validation.failedCount,
    failed: validation.failed,
    missingFiles: validation.missingFiles,
    checks: validation.checks,
  };
}

export function writeP9Batch2SKUCalibrationGateReport(report) {
  writeJSON(P9_BATCH_2_SKU_CALIBRATION_GATE_JSON, report);
  writeMarkdown(
    P9_BATCH_2_SKU_CALIBRATION_GATE_MD,
    `# P9 Batch 2 SKU Calibration Gate

Status: **${report.status}**

- Batch id: ${report.batchId}
- Current branch: ${report.currentBranch}
- Current head: ${report.currentHead}
- Staged files: ${report.stagedFileCount}
- Working tree dirty: ${report.workingTreeDirty}
- Tasks: ${report.tasks.join(', ')}
- Normalizer implemented: ${report.normalizerImplemented}
- Exact matcher implemented: ${report.exactMatcherImplemented}
- Scoring implemented: ${report.scoringImplemented}
- Threshold policy implemented: ${report.thresholdPolicyImplemented}
- Manual binding service implemented: ${report.manualBindingServiceImplemented}
- Authorizer required: ${report.authorizerRequired}
- Default allow: ${report.defaultAllow}
- PostgreSQL integration status: ${report.postgresIntegrationStatus}
- Sync orchestrator implemented: ${report.syncOrchestratorImplemented}
- Sync worker implemented: ${report.syncWorkerImplemented}
- API implemented: ${report.apiImplemented}
- Admin UI implemented: ${report.adminUiImplemented}
- Real Douyin provider implemented: ${report.realDouyinProviderImplemented}
- Real platform network enabled: ${report.realPlatformNetworkEnabled}
- P10 boundary preserved: ${report.p10BoundaryPreserved}
- Production Ready: ${report.productionReady}
- Failed checks: ${report.failedCount ? report.failed.join(', ') : 'none'}

This gate validates only P9 Batch 2 SKU calibration and manual binding domain services. It does not authorize sync orchestration, workers, API, Admin UI, real Douyin/OAuth/credentials, real platform network, real inventory reads/writes, release, tag, P9 completion, or Production Ready.
`,
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const report = buildP9Batch2SKUCalibrationGateReport();
  writeP9Batch2SKUCalibrationGateReport(report);
  console.log(JSON.stringify(report, null, 2));
  process.exit(report.status === 'passed' ? 0 : 1);
}
