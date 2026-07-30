package inventorysyncp9

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type staticCandidateProvider struct {
	candidates []LocalSKUCandidate
}

func (p staticCandidateProvider) ListLocalSKUCandidates(ctx context.Context, tenantID int64) ([]LocalSKUCandidate, error) {
	return append([]LocalSKUCandidate(nil), p.candidates...), nil
}

type testAuthorizer struct {
	allowed bool
}

func (a testAuthorizer) CanResolveManualBinding(ctx context.Context, tenantID int64, actorID uuid.UUID, requestID uuid.UUID) error {
	if !a.allowed {
		return ErrPermissionDenied
	}
	return nil
}

func TestSKUIdentifierNormalizationRules(t *testing.T) {
	normalizer := NewDefaultSKUIdentifierNormalizer()
	first := normalizer.NormalizeSKUCode("  sku—001   red ")
	second := normalizer.NormalizeSKUCode("  sku—001   red ")
	require.Equal(t, first, second)
	require.True(t, first.Valid)
	require.Equal(t, "SKU-001 RED", first.NormalizedValue)
	require.Equal(t, NormalizationVersionV1, first.NormalizationVersion)
	require.Contains(t, first.AppliedRules, "unicode_nfkc")
	require.Contains(t, first.AppliedRules, "trim_space")
	require.Contains(t, first.AppliedRules, "collapse_whitespace")
	require.Contains(t, first.AppliedRules, "normalize_dash_forms")
	require.Contains(t, first.AppliedRules, "uppercase")
	require.Equal(t, "A/B-001", normalizer.NormalizeSKUCode(" a/b-001 ").NormalizedValue)

	barcode := normalizer.NormalizeBarcode("  0012345  ")
	require.True(t, barcode.Valid)
	require.Equal(t, "0012345", barcode.NormalizedValue)
	require.False(t, normalizer.NormalizeBarcode("12\n34").Valid)
	require.False(t, normalizer.NormalizeBarcode(string(make([]byte, MaxBarcodeBytes+1))).Valid)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			require.Equal(t, first, normalizer.NormalizeSKUCode("  sku—001   red "))
		}()
	}
	wg.Wait()
}

func TestExactMatchingScoringAndPolicy(t *testing.T) {
	normalizer := NewDefaultSKUIdentifierNormalizer()
	matcher := NewExactIdentifierMatcher(normalizer)
	snapshot := InventorySnapshotItem{TenantID: 601, ExternalSKUCode: " sku-1 ", Barcode: "001", ExternalSKUID: "remote-sku-1"}
	productID := uuid.New()
	candidateID := uuid.New()
	result := matcher.Match(snapshot, []LocalSKUCandidate{{TenantID: 601, LocalProductID: productID, LocalSKUID: candidateID, SKUCode: "SKU-1", Barcode: "001"}})
	require.Equal(t, MatchResultSingleCandidate, result.Status)
	require.Len(t, result.Candidates, 1)
	require.Equal(t, MatchStrategyExactBarcode, result.Candidates[0].MatchStrategy)

	scored, err := NewCandidateScoringService().Score(result)
	require.NoError(t, err)
	require.Equal(t, 10000, scored[0].Confidence)
	require.NotEmpty(t, scored[0].ScoreBreakdown)
	require.Contains(t, scored[0].ReasonCodes, ReasonExactBarcodeMatch)
	require.Equal(t, scored[0].InputFingerprint, result.Candidates[0].InputFingerprint)

	policy, err := NewCalibrationThresholdPolicy(CalibrationThresholdConfig{HighConfidenceThreshold: 9500, AutoConfirmationEnabled: false})
	require.NoError(t, err)
	policyResult := policy.Evaluate(result.Status, scored)
	require.False(t, policyResult.AutoConfirmEligible)
	require.True(t, policyResult.ManualReviewRequired)
	require.Contains(t, policyResult.ReasonCodes, ReasonAutoConfirmationNotAuthorized)

	_, err = NewCalibrationThresholdPolicy(CalibrationThresholdConfig{HighConfidenceThreshold: 10001})
	require.ErrorIs(t, err, ErrCalibrationPolicyInvalid)
}

func TestExactMatchingConflictNoCandidateTenantIsolationAndStableOrdering(t *testing.T) {
	matcher := NewExactIdentifierMatcher(NewDefaultSKUIdentifierNormalizer())
	snapshot := InventorySnapshotItem{TenantID: 602, ExternalSKUCode: "ABC", Barcode: "", ExternalSKUID: "remote-sku-2"}
	c1 := LocalSKUCandidate{TenantID: 602, LocalProductID: uuid.New(), LocalSKUID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), SKUCode: "ABC"}
	c2 := LocalSKUCandidate{TenantID: 602, LocalProductID: uuid.New(), LocalSKUID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), SKUCode: "ABC"}
	crossTenant := LocalSKUCandidate{TenantID: 603, LocalProductID: uuid.New(), LocalSKUID: uuid.New(), SKUCode: "ABC"}
	result := matcher.Match(snapshot, []LocalSKUCandidate{c1, crossTenant, c2})
	require.Equal(t, MatchResultConflict, result.Status)
	require.Len(t, result.Candidates, 2)
	require.Equal(t, c2.LocalSKUID, result.Candidates[0].CandidateLocalSKUID)
	require.Contains(t, result.ReasonCodes, ReasonMultipleExactMatches)

	missing := matcher.Match(InventorySnapshotItem{TenantID: 602, ExternalSKUID: "remote-sku-3"}, []LocalSKUCandidate{c1})
	require.Equal(t, MatchResultNoCandidate, missing.Status)
	require.Contains(t, missing.ReasonCodes, ReasonMissingExternalIdentifier)
}

func TestCalibrationServicePersistsCandidatesAndManualRequestIdempotently(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	store, prod, sku := seedShopAndSKU(t, db, 604)
	run := createRun(t, ctx, db, 604, store.ID)
	snapRepo := NewInventorySnapshotRepository(db)
	snapshot := validSnapshot(604, run, "remote-sku-cal-1")
	snapshot.ExternalSKUCode = " sku-604 "
	require.NoError(t, snapRepo.CreateBatch(ctx, 604, []InventorySnapshotItem{snapshot}))
	stored, err := snapRepo.GetByRunAndExternalSKU(ctx, 604, run.ID, "remote-sku-cal-1")
	require.NoError(t, err)
	policy, err := NewCalibrationThresholdPolicy(CalibrationThresholdConfig{HighConfidenceThreshold: 9500, AutoConfirmationEnabled: false})
	require.NoError(t, err)
	service := NewSKUBindingCalibrationService(db, staticCandidateProvider{candidates: []LocalSKUCandidate{{TenantID: 604, LocalProductID: prod.ID, LocalSKUID: sku.ID, SKUCode: "SKU-604"}}}, policy)

	first, err := service.CalibrateSnapshotItem(ctx, 604, run.ID, stored.ID)
	require.NoError(t, err)
	require.Equal(t, MatchResultSingleCandidate, first.MatchStatus)
	require.Len(t, first.Candidates, 1)
	require.NotNil(t, first.ManualBindingRequest)
	require.True(t, first.PolicyResult.ManualReviewRequired)
	rows, err := NewSKUBindingCalibrationRepository(db).ListBySnapshot(ctx, 604, stored.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	second, err := service.CalibrateSnapshotItem(ctx, 604, run.ID, stored.ID)
	require.NoError(t, err)
	require.Equal(t, first.InputFingerprint, second.InputFingerprint)
	rows, err = NewSKUBindingCalibrationRepository(db).ListBySnapshot(ctx, 604, stored.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	requests, err := NewManualBindingRequestRepository(db).ListPending(ctx, 604, 10)
	require.NoError(t, err)
	require.Len(t, requests, 1)
}

func TestCalibrationServiceRecalibrationCreatesImmutableVersion(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	store, prod, sku := seedShopAndSKU(t, db, 607)
	run := createRun(t, ctx, db, 607, store.ID)
	snapshot := validSnapshot(607, run, "remote-sku-recalibrate-1")
	snapshot.ExternalSKUCode = "SKU-607"
	require.NoError(t, NewInventorySnapshotRepository(db).CreateBatch(ctx, 607, []InventorySnapshotItem{snapshot}))
	stored, err := NewInventorySnapshotRepository(db).GetByRunAndExternalSKU(ctx, 607, run.ID, snapshot.ExternalSKUID)
	require.NoError(t, err)
	policy, err := NewCalibrationThresholdPolicy(CalibrationThresholdConfig{HighConfidenceThreshold: 9500, AutoConfirmationEnabled: false})
	require.NoError(t, err)
	service := NewSKUBindingCalibrationService(db, staticCandidateProvider{candidates: []LocalSKUCandidate{{TenantID: 607, LocalProductID: prod.ID, LocalSKUID: sku.ID, SKUCode: "SKU-607"}}}, policy)
	_, err = service.CalibrateSnapshotItem(ctx, 607, run.ID, stored.ID)
	require.NoError(t, err)

	recalibrated, version, err := service.RecalibrateSnapshotItem(ctx, 607, run.ID, stored.ID, 1)
	require.NoError(t, err)
	require.Equal(t, 2, version)
	require.Len(t, recalibrated.Candidates, 1)
	require.Equal(t, 2, recalibrated.Candidates[0].CalibrationVersion)
	rows, err := NewSKUBindingCalibrationRepository(db).ListBySnapshot(ctx, 607, stored.ID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.ElementsMatch(t, []int{1, 2}, []int{rows[0].CalibrationVersion, rows[1].CalibrationVersion})
	_, _, err = service.RecalibrateSnapshotItem(ctx, 607, run.ID, stored.ID, 1)
	require.ErrorIs(t, err, ErrRevisionConflict)
}

func TestManualBindingServiceAuthorizationIdempotencyAndConcurrency(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	store, prod, sku := seedShopAndSKU(t, db, 605)
	run := createRun(t, ctx, db, 605, store.ID)
	snapRepo := NewInventorySnapshotRepository(db)
	snapshot := validSnapshot(605, run, "remote-sku-manual-1")
	require.NoError(t, snapRepo.CreateBatch(ctx, 605, []InventorySnapshotItem{snapshot}))
	stored, err := snapRepo.GetByRunAndExternalSKU(ctx, 605, run.ID, "remote-sku-manual-1")
	require.NoError(t, err)
	request, err := NewManualBindingRequestRepository(db).Create(ctx, &ManualBindingRequest{
		TenantID:                605,
		InventorySyncRunID:      run.ID,
		InventorySnapshotItemID: stored.ID,
		ShopConnectionID:        store.ID,
		ExternalSKUID:           stored.ExternalSKUID,
		Status:                  ManualBindingStatusPending,
		ReasonCode:              ReasonManualReviewRequired,
		CandidateCount:          1,
		RequestID:               "manual-service-req-1",
		IdempotencyKeyHash:      testHashB,
		InputFingerprint:        testHashC,
		Revision:                1,
	})
	require.NoError(t, err)

	actorID := uuid.New()
	denied := NewManualBindingService(db, nil)
	_, err = denied.ConfirmBinding(ctx, ConfirmManualBindingInput{Actor: ManualBindingActor{TenantID: 605, ActorID: actorID}, RequestID: request.ID, ExpectedRevision: request.Revision, SelectedLocalSKUID: sku.ID, IdempotencyKeyHash: testHashA})
	require.ErrorIs(t, err, ErrPermissionDenied)
	fresh, err := NewManualBindingRequestRepository(db).GetByID(ctx, 605, request.ID)
	require.NoError(t, err)
	require.Equal(t, ManualBindingStatusPending, fresh.Status)

	service := NewManualBindingService(db, testAuthorizer{allowed: true})
	var wg sync.WaitGroup
	type confirmAttempt struct {
		key string
		err error
	}
	errs := make(chan confirmAttempt, 2)
	keys := []string{testHashA, testHashB}
	for _, key := range keys {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			_, err := service.ConfirmBinding(ctx, ConfirmManualBindingInput{Actor: ManualBindingActor{TenantID: 605, ActorID: actorID}, RequestID: request.ID, ExpectedRevision: request.Revision, SelectedLocalSKUID: sku.ID, IdempotencyKeyHash: key})
			errs <- confirmAttempt{key: key, err: err}
		}(key)
	}
	wg.Wait()
	close(errs)
	successes := 0
	successKey := ""
	for attempt := range errs {
		if attempt.err == nil {
			successes++
			successKey = attempt.key
			continue
		}
		require.True(t, errors.Is(attempt.err, ErrRevisionConflict) || errors.Is(attempt.err, ErrManualBindingAlreadyResolved) || errors.Is(attempt.err, ErrStateConflict), "unexpected error: %v", attempt.err)
	}
	require.Equal(t, 1, successes)
	require.NotEmpty(t, successKey)
	resolved, err := NewManualBindingRequestRepository(db).GetByID(ctx, 605, request.ID)
	require.NoError(t, err)
	require.Equal(t, ManualBindingStatusConfirmed, resolved.Status)
	require.Equal(t, actorID, *resolved.ResolvedBy)
	require.Equal(t, prod.ID, *resolved.SelectedLocalProductID)

	replayed, err := service.ConfirmBinding(ctx, ConfirmManualBindingInput{Actor: ManualBindingActor{TenantID: 605, ActorID: actorID}, RequestID: request.ID, ExpectedRevision: request.Revision, SelectedLocalSKUID: sku.ID, IdempotencyKeyHash: successKey})
	require.NoError(t, err)
	require.NotNil(t, replayed.Binding)
	_, err = service.ConfirmBinding(ctx, ConfirmManualBindingInput{Actor: ManualBindingActor{TenantID: 605, ActorID: actorID}, RequestID: request.ID, ExpectedRevision: request.Revision, SelectedLocalSKUID: uuid.New(), IdempotencyKeyHash: successKey})
	require.ErrorIs(t, err, ErrIdempotencyPayloadConflict)
}

func TestManualBindingRejectPreservesCandidatesAndRequest(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	store, prod, sku := seedShopAndSKU(t, db, 606)
	run := createRun(t, ctx, db, 606, store.ID)
	snapRepo := NewInventorySnapshotRepository(db)
	snapshot := validSnapshot(606, run, "remote-sku-reject-1")
	require.NoError(t, snapRepo.CreateBatch(ctx, 606, []InventorySnapshotItem{snapshot}))
	stored, err := snapRepo.GetByRunAndExternalSKU(ctx, 606, run.ID, "remote-sku-reject-1")
	require.NoError(t, err)
	calibration := SKUBindingCalibration{TenantID: 606, InventorySyncRunID: run.ID, InventorySnapshotItemID: stored.ID, ExternalSKUID: stored.ExternalSKUID, CandidateLocalProductID: prod.ID, CandidateLocalSKUID: sku.ID, MatchStrategy: MatchStrategyNormalizedSKUCode, Confidence: 8500, ScoreBreakdown: []byte(`[{"code":"normalizedSKUCodeScore","points":8500,"reason":"normalized_sku_code_match"}]`), ReasonCodes: []byte(`["normalized_sku_code_match"]`), CalibrationVersion: 1, Status: CalibrationStatusCandidate, InputFingerprint: testHashA}
	require.NoError(t, NewSKUBindingCalibrationRepository(db).CreateBatch(ctx, 606, []SKUBindingCalibration{calibration}))
	request, err := NewManualBindingRequestRepository(db).Create(ctx, &ManualBindingRequest{TenantID: 606, InventorySyncRunID: run.ID, InventorySnapshotItemID: stored.ID, ShopConnectionID: store.ID, ExternalSKUID: stored.ExternalSKUID, Status: ManualBindingStatusPending, ReasonCode: ReasonCalibrationThresholdNotMet, CandidateCount: 1, RequestID: "manual-reject-req-1", IdempotencyKeyHash: testHashB, InputFingerprint: testHashC, Revision: 1})
	require.NoError(t, err)
	service := NewManualBindingService(db, testAuthorizer{allowed: true})
	actorID := uuid.New()
	result, err := service.RejectBinding(ctx, RejectManualBindingInput{Actor: ManualBindingActor{TenantID: 606, ActorID: actorID}, RequestID: request.ID, ExpectedRevision: request.Revision, ReasonCode: ReasonCalibrationThresholdNotMet, IdempotencyKeyHash: testHashA, Comment: "safe comment"})
	require.NoError(t, err)
	require.Equal(t, ManualBindingStatusRejected, result.Request.Status)
	require.Equal(t, actorID, *result.Request.ResolvedBy)
	rows, err := NewSKUBindingCalibrationRepository(db).ListBySnapshot(ctx, 606, stored.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	loaded, err := NewManualBindingRequestRepository(db).GetByID(ctx, 606, request.ID)
	require.NoError(t, err)
	require.Equal(t, ManualBindingStatusRejected, loaded.Status)
}
