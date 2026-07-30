package inventorysyncp9

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type unsafeInventoryProvider struct {
	key          InventoryProviderKey
	capabilities InventoryProviderCapabilities
}

func (p unsafeInventoryProvider) Key() InventoryProviderKey                   { return p.key }
func (p unsafeInventoryProvider) Capabilities() InventoryProviderCapabilities { return p.capabilities }
func (p unsafeInventoryProvider) FetchInventoryPage(ctx context.Context, request InventoryFetchRequest) (InventoryFetchPageResult, error) {
	return InventoryFetchPageResult{}, nil
}

type testSyncAuthorizer struct{ allowed bool }

func (a testSyncAuthorizer) CanRunInventorySync(ctx context.Context, tenantID int64, actorID uuid.UUID, shopConnectionID uuid.UUID) error {
	if !a.allowed {
		return ErrPermissionDenied
	}
	return nil
}

func (a testSyncAuthorizer) CanRerunInventorySync(ctx context.Context, tenantID int64, actorID uuid.UUID, sourceRunID uuid.UUID) error {
	if !a.allowed {
		return ErrPermissionDenied
	}
	return nil
}

func TestInventoryProviderRegistryAndFixtureProviderSafety(t *testing.T) {
	registry, err := NewInventoryProviderRegistry(NewDouyinInventoryMockProvider(), NewDouyinInventoryFixtureProvider(FixtureScenarioSuccessMultiPage), NewLocalInventoryFixtureProvider(FixtureScenarioSuccessSinglePage))
	require.NoError(t, err)
	_, err = registry.Resolve(PlatformDouyin, ProviderModeMock)
	require.NoError(t, err)
	_, err = registry.Resolve(PlatformDouyin, "production")
	require.ErrorIs(t, err, ErrProductionCapabilityForbidden)
	_, err = registry.Resolve("unknown", ProviderModeMock)
	require.ErrorIs(t, err, ErrProviderNotRegistered)
	err = registry.Register(unsafeInventoryProvider{key: InventoryProviderKey{Platform: PlatformDouyin, ProviderMode: ProviderModeMock}, capabilities: InventoryProviderCapabilities{NetworkAccess: true}})
	require.ErrorIs(t, err, ErrProviderCapabilityForbidden)

	provider, err := registry.Resolve(PlatformDouyin, ProviderModeSandbox)
	require.NoError(t, err)
	page, err := provider.FetchInventoryPage(context.Background(), InventoryFetchRequest{Platform: PlatformDouyin, ProviderMode: ProviderModeSandbox, FixtureScenario: FixtureScenarioSuccessMultiPage, PageSize: 1, MaxItemsPerPage: 10})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.True(t, page.HasMore)
	require.Equal(t, 0, page.NetworkCalls)
	again, err := provider.FetchInventoryPage(context.Background(), InventoryFetchRequest{Platform: PlatformDouyin, ProviderMode: ProviderModeSandbox, FixtureScenario: FixtureScenarioSuccessMultiPage, Cursor: page.Cursor, PageSize: 1, MaxItemsPerPage: 10})
	require.NoError(t, err)
	require.Equal(t, page, again)
	_, err = provider.FetchInventoryPage(context.Background(), InventoryFetchRequest{Platform: PlatformDouyin, ProviderMode: ProviderModeSandbox, FixtureScenario: FixtureScenarioSuccessMultiPage, Cursor: []byte(`{"scenario":"other"}`), PageSize: 1, MaxItemsPerPage: 10})
	require.ErrorIs(t, err, ErrProviderCursorInvalid)
}

func TestInventorySyncOrchestratorProcessesFixturePages(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	store, prod, sku := seedShopAndSKU(t, db, 701)
	registry, err := NewInventoryProviderRegistry(NewDouyinInventoryFixtureProvider(FixtureScenarioSuccessMultiPage))
	require.NoError(t, err)
	policy, err := NewCalibrationThresholdPolicy(CalibrationThresholdConfig{HighConfidenceThreshold: 9500, AutoConfirmationEnabled: false})
	require.NoError(t, err)
	service := NewSKUBindingCalibrationService(db, staticCandidateProvider{candidates: []LocalSKUCandidate{{TenantID: 701, LocalProductID: prod.ID, LocalSKUID: sku.ID, SKUCode: "SKU-701", Barcode: "B701"}}}, policy)
	orchestrator := NewInventorySyncOrchestrator(db, registry, service, testSyncAuthorizer{allowed: true})

	result, err := orchestrator.Run(ctx, InventorySyncOrchestratorInput{TenantID: 701, ShopConnectionID: store.ID, Platform: PlatformDouyin, ProviderMode: ProviderModeSandbox, FixtureScenario: FixtureScenarioSuccessMultiPage, PageSize: 1, MaxPagesPerRun: 5, MaxItemsPerPage: 2, MaxItemsPerRun: 10, ActorID: uuid.New(), RequestID: "req-b3-701"})
	require.NoError(t, err)
	require.Equal(t, InventorySyncRunStatusSucceeded, result.Status)
	require.Equal(t, 2, result.TotalRecordCount)
	require.Equal(t, 0, result.MatchedRecordCount)
	require.Equal(t, 2, result.UnmatchedRecordCount)
	require.Equal(t, 0, result.ConflictRecordCount)
	require.Equal(t, 2, result.ManualBindingRequestCount)

	snapshots, err := NewInventorySnapshotRepository(db).ListByRun(ctx, 701, result.InventorySyncRunID)
	require.NoError(t, err)
	require.Len(t, snapshots, 2)
	calibrations, err := NewSKUBindingCalibrationRepository(db).ListByRun(ctx, 701, result.InventorySyncRunID)
	require.NoError(t, err)
	require.Len(t, calibrations, 1)
	requests, err := NewManualBindingRequestRepository(db).ListPending(ctx, 701, 10)
	require.NoError(t, err)
	require.Len(t, requests, 2)
}

func TestBindingResolutionPipelinePrioritizesConfirmedBinding(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	store, prod, sku := seedShopAndSKU(t, db, 704)
	_, err := NewSKUBindingRepository(db).CreateProposed(ctx, &SKUBinding{TenantID: 704, ShopConnectionID: store.ID, Platform: PlatformDouyin, ExternalProductID: "douyin-product-001", ExternalSKUID: "douyin-sku-001", LocalProductID: prod.ID, LocalSKUID: sku.ID, BindingSource: SKUBindingSourceManual, BindingStatus: SKUBindingStatusConfirmed, Confidence: 10000, CalibrationVersion: 1, Revision: 1})
	require.NoError(t, err)
	registry, err := NewInventoryProviderRegistry(NewDouyinInventoryMockProvider())
	require.NoError(t, err)
	policy, err := NewCalibrationThresholdPolicy(CalibrationThresholdConfig{HighConfidenceThreshold: 9500, AutoConfirmationEnabled: false})
	require.NoError(t, err)
	service := NewSKUBindingCalibrationService(db, staticCandidateProvider{candidates: []LocalSKUCandidate{}}, policy)
	orchestrator := NewInventorySyncOrchestrator(db, registry, service, testSyncAuthorizer{allowed: true})

	result, err := orchestrator.Run(ctx, InventorySyncOrchestratorInput{TenantID: 704, ShopConnectionID: store.ID, Platform: PlatformDouyin, ProviderMode: ProviderModeMock, FixtureScenario: FixtureScenarioSuccessSinglePage, PageSize: 1, ActorID: uuid.New(), RequestID: "req-b3-confirmed"})
	require.NoError(t, err)
	require.Equal(t, 1, result.MatchedRecordCount)
	require.Equal(t, 0, result.UnmatchedRecordCount)
	calibrations, err := NewSKUBindingCalibrationRepository(db).ListByRun(ctx, 704, result.InventorySyncRunID)
	require.NoError(t, err)
	require.Empty(t, calibrations)
}

func TestInventorySyncOrchestratorFailureCancellationIdempotencyAndRerun(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	store, prod, sku := seedShopAndSKU(t, db, 702)
	registry, err := NewInventoryProviderRegistry(NewDouyinInventoryFixtureProvider(FixtureScenarioProviderRejected), NewDouyinInventoryMockProvider())
	require.NoError(t, err)
	policy, err := NewCalibrationThresholdPolicy(CalibrationThresholdConfig{HighConfidenceThreshold: 9500, AutoConfirmationEnabled: false})
	require.NoError(t, err)
	service := NewSKUBindingCalibrationService(db, staticCandidateProvider{candidates: []LocalSKUCandidate{{TenantID: 702, LocalProductID: prod.ID, LocalSKUID: sku.ID, SKUCode: "SKU-701"}}}, policy)
	orchestrator := NewInventorySyncOrchestrator(db, registry, service, testSyncAuthorizer{allowed: true})

	failed, err := orchestrator.Run(ctx, InventorySyncOrchestratorInput{TenantID: 702, ShopConnectionID: store.ID, Platform: PlatformDouyin, ProviderMode: ProviderModeSandbox, FixtureScenario: FixtureScenarioProviderRejected, PageSize: 1, ActorID: uuid.New(), RequestID: "req-b3-fail"})
	require.ErrorIs(t, err, ErrProviderRejected)
	require.Equal(t, InventorySyncRunStatusFailed, failed.Status)
	require.Contains(t, string(failed.SafeErrorSummary), ErrCodeProviderRejected)

	same, err := orchestrator.Run(ctx, InventorySyncOrchestratorInput{TenantID: 702, ShopConnectionID: store.ID, Platform: PlatformDouyin, ProviderMode: ProviderModeSandbox, FixtureScenario: FixtureScenarioProviderRejected, PageSize: 1, ActorID: uuid.New(), RequestID: "req-b3-fail"})
	require.NoError(t, err)
	require.Equal(t, failed.InventorySyncRunID, same.InventorySyncRunID)

	failedRun, err := NewInventorySyncRunRepository(db).GetByID(ctx, 702, failed.InventorySyncRunID)
	require.NoError(t, err)
	denied := NewInventorySyncOrchestrator(db, registry, service, testSyncAuthorizer{allowed: false})
	_, err = denied.ManualRerun(ctx, InventorySyncOrchestratorInput{TenantID: 702, ShopConnectionID: store.ID, Platform: PlatformDouyin, ProviderMode: ProviderModeMock, FixtureScenario: FixtureScenarioSuccessSinglePage, PageSize: 1, SourceRunID: failed.InventorySyncRunID, SourceRunRevision: failedRun.Revision, ActorID: uuid.New(), RequestID: "req-b3-rerun-deny"})
	require.ErrorIs(t, err, ErrPermissionDenied)

	allowed := NewInventorySyncOrchestrator(db, registry, service, testSyncAuthorizer{allowed: true})
	rerun, err := allowed.ManualRerun(ctx, InventorySyncOrchestratorInput{TenantID: 702, ShopConnectionID: store.ID, Platform: PlatformDouyin, ProviderMode: ProviderModeMock, FixtureScenario: FixtureScenarioSuccessSinglePage, PageSize: 1, SourceRunID: failed.InventorySyncRunID, SourceRunRevision: failedRun.Revision, ActorID: uuid.New(), RequestID: "req-b3-rerun-allow"})
	require.NoError(t, err)
	require.NotEqual(t, failed.InventorySyncRunID, rerun.InventorySyncRunID)
	require.Equal(t, InventorySyncRunStatusSucceeded, rerun.Status)
}

func TestInventorySyncOrchestratorConcurrentSameRequestCreatesOneRun(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	store, prod, sku := seedShopAndSKU(t, db, 703)
	registry, err := NewInventoryProviderRegistry(NewDouyinInventoryMockProvider())
	require.NoError(t, err)
	policy, err := NewCalibrationThresholdPolicy(CalibrationThresholdConfig{HighConfidenceThreshold: 9500, AutoConfirmationEnabled: false})
	require.NoError(t, err)
	service := NewSKUBindingCalibrationService(db, staticCandidateProvider{candidates: []LocalSKUCandidate{{TenantID: 703, LocalProductID: prod.ID, LocalSKUID: sku.ID, SKUCode: "SKU-701", Barcode: "B701"}}}, policy)
	orchestrator := NewInventorySyncOrchestrator(db, registry, service, testSyncAuthorizer{allowed: true})
	input := InventorySyncOrchestratorInput{TenantID: 703, ShopConnectionID: store.ID, Platform: PlatformDouyin, ProviderMode: ProviderModeMock, FixtureScenario: FixtureScenarioSuccessSinglePage, PageSize: 1, ActorID: uuid.New(), RequestID: "req-b3-concurrent"}

	var wg sync.WaitGroup
	results := make(chan *InventorySyncOrchestratorResult, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := orchestrator.Run(ctx, input)
			if err != nil && !errors.Is(err, ErrRevisionConflict) && !errors.Is(err, ErrStateConflict) {
				errs <- err
				return
			}
			if result != nil {
				results <- result
			}
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	require.Empty(t, errs)
	ids := map[uuid.UUID]bool{}
	for result := range results {
		ids[result.InventorySyncRunID] = true
	}
	require.Len(t, ids, 1)
	snapshots, err := NewInventorySnapshotRepository(db).ListByRun(ctx, 703, firstUUID(ids))
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
}

func firstUUID(values map[uuid.UUID]bool) uuid.UUID {
	for id := range values {
		return id
	}
	return uuid.Nil
}
