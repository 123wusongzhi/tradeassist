import assert from 'node:assert/strict';
import { writeJSON } from '../../../scripts/p7-v2-lib.mjs';
import { validateP9Batch2SKUCalibrationBundle } from '../../../scripts/p9-task-batch-2-sku-calibration-gate.mjs';

const TASK_IDS = ['P9-601', 'P9-602', 'P9-603', 'P9-604', 'P9-605', 'P9-606'];

const validSources = {
  requiredFilesPresent: true,
  normalizationText: `
type SKUIdentifierNormalizer interface {}
func NormalizeSKUCode() {}
func NormalizeBarcode() {}
const NormalizationVersionV1 = "sku-normalization-v1"
`,
  calibrationText: `
type ExactIdentifierMatcher struct {}
const MatchStrategyExactBarcode = "exact_barcode"
const MatchStrategyNormalizedSKUCode = "normalized_sku_code"
const MatchResultConflict = "conflict"
type CandidateScoringService struct {}
type ScoreBreakdownItem struct {}
ReasonCodes
Confidence
type CalibrationThresholdPolicy struct {}
AutoConfirmationEnabled
ManualReviewRequired
Transaction(func(tx *gorm.DB) error
IdempotencyKeyHash
PayloadFingerprint
ErrIdempotencyPayloadConflict
`,
  manualText: `
type ManualBindingService struct {}
type ManualBindingAuthorizer interface {}
func ConfirmBinding() {}
func RejectBinding() {}
Authorizer == nil
ErrPermissionDenied
ManualBindingActor
ResolvedBy:             input.Actor.ActorID
ExpectedRevision
ErrRevisionConflict
Transaction(func(tx *gorm.DB) error
IdempotencyKeyHash
PayloadFingerprint
ErrIdempotencyPayloadConflict
`,
  modelText: `
type ManualBindingDecision struct {}
`,
  migrateText: `
p9_manual_binding_decisions
trg_p9_manual_binding_decisions_no_update
`,
  errorsText: `
invalid_identifier
normalization_failed
no_binding_candidate
multiple_binding_candidates
binding_conflict
calibration_policy_invalid
calibration_threshold_not_met
manual_review_required
manual_binding_already_pending
manual_binding_already_resolved
candidate_local_sku_not_found
candidate_local_sku_tenant_mismatch
permission_denied
revision_conflict
idempotency_payload_conflict
`,
  testText: `
0012345
TestSKUIdentifierNormalizationRules
TestExactMatchingScoringAndPolicy
TestCalibrationServicePersistsCandidatesAndManualRequestIdempotently
TestManualBindingServiceAuthorizationIdempotencyAndConcurrency
TestManualBindingRejectPreservesCandidatesAndRequest
`,
};

const stableErrors = [
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

function taskEvidence(status = 'completed') {
  return TASK_IDS.reduce((acc, id) => {
    acc[id] = { status };
    return acc;
  }, {});
}

function validEvidence(overrides = {}) {
  return {
    batchId: 'P9-TASK-BATCH-2',
    batchName: 'SKU Calibration and Manual Binding Services',
    workingBranch: 'dev',
    changesCommitted: false,
    stagedFileCount: 0,
    workingTreeDirty: true,
    tasks: taskEvidence(),
    normalizerImplemented: true,
    normalizationVersion: 'sku-normalization-v1',
    barcodeLeadingZeroPreserved: true,
    exactMatcherImplemented: true,
    scoringImplemented: true,
    thresholdPolicyImplemented: true,
    manualBindingServiceImplemented: true,
    authorizerRequired: true,
    defaultAllow: false,
    trustedActorRequired: true,
    idempotencyTestsPassed: true,
    optimisticConcurrencyTestsPassed: true,
    transactionAtomicityTestsPassed: true,
    manualDecisionHistoryImplemented: true,
    stableErrorsPresent: true,
    stableErrors,
    testsPassed: true,
    raceTestsPassed: true,
    sqliteIntegrationTestsPassed: true,
    postgresIntegrationStatus: 'not_run',
    postgresIntegrationPassed: false,
    postgresIntegrationDeferredTo: 'P9_Final_Development_Closure',
    p9FinalClosureBlocker: true,
    aiMatchingImplemented: false,
    vectorMatchingImplemented: false,
    llmMatchingImplemented: false,
    syncOrchestratorImplemented: false,
    syncWorkerImplemented: false,
    cronImplemented: false,
    tickerImplemented: false,
    queueConsumerImplemented: false,
    apiImplemented: false,
    httpHandlerImplemented: false,
    ginRouterImplemented: false,
    restApiImplemented: false,
    adminUiImplemented: false,
    frontendApiClientImplemented: false,
    realDouyinProviderImplemented: false,
    oauthImplemented: false,
    realCredentialsEnabled: false,
    realPlatformNetworkEnabled: false,
    realPlatformReadEnabled: false,
    realPlatformWriteEnabled: false,
    realInventoryReadEnabled: false,
    realInventoryWriteEnabled: false,
    automaticPublishEnabled: false,
    automaticListingEnabled: false,
    p10BoundaryPreserved: true,
    productionReady: false,
    p9Complete: false,
    ...overrides,
  };
}

function validate(overrides = {}, sourceOverrides = {}, gitOverrides = {}) {
  return validateP9Batch2SKUCalibrationBundle({
    evidence: validEvidence(overrides),
    sources: { ...validSources, ...sourceOverrides },
    gitState: { currentBranch: 'dev', currentHead: 'abc123', stagedFileCount: 0, workingTreeDirty: true, ...gitOverrides },
  });
}

function assertFails(fixtureId, id, overrides = {}, sourceOverrides = {}, gitOverrides = {}) {
  const result = validate(overrides, sourceOverrides, gitOverrides);
  assert.equal(result.status, 'failed', fixtureId);
  assert.ok(result.failed.includes(id), `${fixtureId} expected ${id}, saw ${result.failed.join(', ')}`);
}

assert.equal(validate().status, 'passed', 'B2-01 complete Batch 2 passes');
assertFails('B2-02 normalizer is required', 'normalizerImplemented', { normalizerImplemented: false });
assertFails('B2-03 barcode leading zeros are preserved', 'barcodeLeadingZeroPreserved', { barcodeLeadingZeroPreserved: false });
assertFails('B2-04 exact matcher is required', 'exactMatcherImplemented', { exactMatcherImplemented: false });
assertFails('B2-05 scoring explainability is required', 'scoringImplemented', { scoringImplemented: false });
assertFails('B2-06 threshold policy is required', 'thresholdPolicyImplemented', { thresholdPolicyImplemented: false });
assertFails('B2-07 manual binding service is required', 'manualBindingServiceImplemented', { manualBindingServiceImplemented: false });
assertFails('B2-08 missing authorizer must deny', 'authorizerDefaultDeny', { defaultAllow: true });
assertFails('B2-09 trusted actor context is required', 'trustedActorImplemented', { trustedActorRequired: false });
assertFails('B2-10 idempotency is required', 'idempotencyImplemented', { idempotencyTestsPassed: false });
assertFails('B2-11 optimistic concurrency is required', 'optimisticConcurrencyImplemented', { optimisticConcurrencyTestsPassed: false });
assertFails('B2-12 transaction atomicity is required', 'transactionAtomicityImplemented', { transactionAtomicityTestsPassed: false });
assertFails('B2-13 immutable manual decision history is required', 'manualDecisionHistoryImplemented', { manualDecisionHistoryImplemented: false });
assertFails('B2-14 stable errors are required', 'stableErrorsPresent', { stableErrors: stableErrors.filter((code) => code !== 'permission_denied') });
assertFails('B2-15 AI matching remains forbidden', 'noAIOrVectorMatching', { aiMatchingImplemented: true });
assertFails('B2-16 sync orchestration remains forbidden', 'scopeProtectionFlags', { syncWorkerImplemented: true });
assertFails('B2-17 API and Admin UI remain forbidden', 'scopeProtectionFlags', { apiImplemented: true });
assertFails('B2-18 real platform network remains forbidden', 'realBoundaryFlags', { realPlatformNetworkEnabled: true });
assertFails('B2-19 staged files fail gate', 'stagedFileCount', {}, {}, { stagedFileCount: 1 });
assertFails('Batch 2 task IDs cannot be renamed', 'P9-601 status', { tasks: { ...taskEvidence(), 'P9-601': { status: 'renamed' } } });
assertFails('P10 boundary must stay preserved', 'p10BoundaryPreserved', { p10BoundaryPreserved: false });
assertFails('Production Ready must stay false', 'productionReady', { productionReady: true });

const report = {
  phase: 'P9',
  batchId: 'P9-TASK-BATCH-2',
  gate: 'P9-TASK-BATCH-2-SKU-CALIBRATION-FIXTURE',
  status: 'passed',
  fixtureAssertions: 22,
};
writeJSON('docs/p9-task-batch-2-sku-calibration-fixture-report.json', report);
console.log(JSON.stringify(report, null, 2));
