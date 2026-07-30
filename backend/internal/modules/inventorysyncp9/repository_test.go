package inventorysyncp9

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const testHashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const testHashB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
const testHashC = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/inventorysyncp9.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)
	require.NoError(t, db.AutoMigrate(&shop.Shop{}, &product.Product{}, &product.ProductSKU{}, &operationlog.OperationLog{}))
	require.NoError(t, Migrate(db))
	return db
}

func seedShopAndSKU(t *testing.T, db *gorm.DB, tenantID int64) (shop.Shop, product.Product, product.ProductSKU) {
	t.Helper()
	store := shop.Shop{
		TenantID:       tenantID,
		Platform:       PlatformDouyin,
		ShopName:       fmt.Sprintf("tenant-%d-shop", tenantID),
		ExternalShopID: fmt.Sprintf("safe-shop-%d", tenantID),
		Status:         "active",
		AuthStatus:     "mock",
	}
	require.NoError(t, db.Create(&store).Error)
	prod := product.Product{TenantID: tenantID, Source: "manual", Title: "local product", Status: product.StatusDraft}
	require.NoError(t, db.Create(&prod).Error)
	sku := product.ProductSKU{ProductID: prod.ID, SKUCode: fmt.Sprintf("SKU-%d", tenantID), SKUName: "Local SKU"}
	require.NoError(t, db.Create(&sku).Error)
	return store, prod, sku
}

func validRun(tenantID int64, shopID uuid.UUID, hash string, fingerprint string) InventorySyncRun {
	now := time.Now().UTC()
	return InventorySyncRun{
		TenantID:              tenantID,
		ShopConnectionID:      shopID,
		Platform:              PlatformDouyin,
		ProviderMode:          ProviderModeMock,
		ExternalShopReference: "safe-shop-ref",
		Status:                InventorySyncRunStatusRunning,
		Cursor:                datatypes.JSON([]byte(`{"page":1}`)),
		Checkpoint:            datatypes.JSON([]byte(`{"cursor":"safe"}`)),
		SafeErrorMetadata:     datatypes.JSON([]byte(`{}`)),
		RequestID:             "req-1",
		IdempotencyKeyHash:    hash,
		InputFingerprint:      fingerprint,
		Revision:              1,
		StartedAt:             &now,
	}
}

func createRun(t *testing.T, ctx context.Context, db *gorm.DB, tenantID int64, shopID uuid.UUID) *InventorySyncRun {
	t.Helper()
	run, err := NewInventorySyncRunRepository(db).Create(ctx, ptr(validRun(tenantID, shopID, testHashA, testHashB)))
	require.NoError(t, err)
	return run
}

func validSnapshot(tenantID int64, run *InventorySyncRun, externalSKUID string) InventorySnapshotItem {
	return InventorySnapshotItem{
		TenantID:           tenantID,
		InventorySyncRunID: run.ID,
		ShopConnectionID:   run.ShopConnectionID,
		Platform:           run.Platform,
		ExternalProductID:  "remote-product-1",
		ExternalSKUID:      externalSKUID,
		ExternalSKUCode:    "remote-code-1",
		Barcode:            "barcode-1",
		ProductTitle:       "remote product",
		VariantTitle:       "red",
		AvailableQuantity:  5,
		ReservedQuantity:   1,
		TotalQuantity:      8,
		ObservedAt:         time.Now().UTC(),
		PayloadHash:        testHashC,
		SafeMetadata:       datatypes.JSON([]byte(`{"quantityRelationshipContract":"provider_defined"}`)),
	}
}

func TestInventorySyncRunIdempotencyAndRevision(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	store, _, _ := seedShopAndSKU(t, db, 101)
	repo := NewInventorySyncRunRepository(db)

	first, err := repo.Create(ctx, ptr(validRun(101, store.ID, testHashA, testHashB)))
	require.NoError(t, err)
	second, err := repo.Create(ctx, ptr(validRun(101, store.ID, testHashA, testHashB)))
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	_, err = repo.Create(ctx, ptr(validRun(101, store.ID, testHashA, testHashC)))
	require.ErrorIs(t, err, ErrIdempotencyPayloadConflict)

	finished := time.Now().UTC().Add(time.Minute)
	updated, err := repo.UpdateStatusWithRevision(ctx, 101, first.ID, first.Revision, InventorySyncRunStatusSucceeded, InventorySyncRunStatusPatch{FinishedAt: &finished})
	require.NoError(t, err)
	require.Equal(t, 2, updated.Revision)
	_, err = repo.UpdateStatusWithRevision(ctx, 101, first.ID, first.Revision, InventorySyncRunStatusFailed, InventorySyncRunStatusPatch{})
	require.ErrorIs(t, err, ErrRevisionConflict)
	_, err = repo.UpdateStatusWithRevision(ctx, 101, updated.ID, updated.Revision, InventorySyncRunStatusRunning, InventorySyncRunStatusPatch{})
	require.ErrorIs(t, err, ErrStateConflict)
}

func TestSnapshotUniquenessTenantIsolationAndImmutability(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	storeA, _, _ := seedShopAndSKU(t, db, 201)
	seedShopAndSKU(t, db, 202)
	run := createRun(t, ctx, db, 201, storeA.ID)
	repo := NewInventorySnapshotRepository(db)
	item := validSnapshot(201, run, "remote-sku-1")
	require.NoError(t, repo.CreateBatch(ctx, 201, []InventorySnapshotItem{item}))
	_, err := repo.GetByRunAndExternalSKU(ctx, 202, run.ID, "remote-sku-1")
	require.ErrorIs(t, err, ErrNotFound)
	require.ErrorIs(t, repo.CreateBatch(ctx, 201, []InventorySnapshotItem{validSnapshot(201, run, "remote-sku-1")}), ErrDuplicateExternalSKU)

	secondRun, err := NewInventorySyncRunRepository(db).Create(ctx, ptr(validRun(201, storeA.ID, testHashC, testHashA)))
	require.NoError(t, err)
	require.NotEqual(t, run.ID, secondRun.ID)
	firstInMixedBatch := validSnapshot(201, run, "shared-across-runs")
	secondInMixedBatch := validSnapshot(201, secondRun, "shared-across-runs")
	require.NoError(t, repo.CreateBatch(ctx, 201, []InventorySnapshotItem{firstInMixedBatch, secondInMixedBatch}))

	stored, err := repo.GetByRunAndExternalSKU(ctx, 201, run.ID, "remote-sku-1")
	require.NoError(t, err)
	require.Error(t, db.Model(stored).Update("total_quantity", 9).Error)
	require.Error(t, db.Delete(stored).Error)
}

func TestBindingConfirmedUniquenessAndRevision(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	store, prod, sku := seedShopAndSKU(t, db, 301)
	repo := NewSKUBindingRepository(db)
	binding := &SKUBinding{
		TenantID:           301,
		ShopConnectionID:   store.ID,
		Platform:           PlatformDouyin,
		ExternalProductID:  "remote-product-1",
		ExternalSKUID:      "remote-sku-1",
		LocalProductID:     prod.ID,
		LocalSKUID:         sku.ID,
		BindingSource:      SKUBindingSourceAutomatic,
		BindingStatus:      SKUBindingStatusProposed,
		Confidence:         8000,
		CalibrationVersion: 1,
		Revision:           1,
	}
	created, err := repo.CreateProposed(ctx, binding)
	require.NoError(t, err)
	confirmed, err := repo.TransitionWithRevision(ctx, 301, created.ID, SKUBindingTransitionPatch{Status: SKUBindingStatusConfirmed, ExpectedRevision: created.Revision})
	require.NoError(t, err)
	require.Equal(t, SKUBindingStatusConfirmed, confirmed.BindingStatus)
	_, err = repo.TransitionWithRevision(ctx, 301, created.ID, SKUBindingTransitionPatch{Status: SKUBindingStatusStale, ExpectedRevision: created.Revision})
	require.ErrorIs(t, err, ErrRevisionConflict)

	second := *binding
	second.ID = uuid.Nil
	second.BindingStatus = SKUBindingStatusConfirmed
	_, err = repo.CreateProposed(ctx, &second)
	require.ErrorIs(t, err, ErrBindingConflict)
}

func TestCalibrationAtomicityAndImmutability(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	store, prod, sku := seedShopAndSKU(t, db, 401)
	run := createRun(t, ctx, db, 401, store.ID)
	snapRepo := NewInventorySnapshotRepository(db)
	require.NoError(t, snapRepo.CreateBatch(ctx, 401, []InventorySnapshotItem{validSnapshot(401, run, "remote-sku-1")}))
	snapshot, err := snapRepo.GetByRunAndExternalSKU(ctx, 401, run.ID, "remote-sku-1")
	require.NoError(t, err)
	calRepo := NewSKUBindingCalibrationRepository(db)
	calibration := SKUBindingCalibration{
		TenantID:                401,
		InventorySyncRunID:      run.ID,
		InventorySnapshotItemID: snapshot.ID,
		ExternalSKUID:           snapshot.ExternalSKUID,
		CandidateLocalProductID: prod.ID,
		CandidateLocalSKUID:     sku.ID,
		MatchStrategy:           MatchStrategyExactSKUCode,
		Confidence:              9000,
		ScoreBreakdown:          datatypes.JSON([]byte(`{"skuCode":9000}`)),
		ReasonCodes:             datatypes.JSON([]byte(`["exact_sku_code"]`)),
		CalibrationVersion:      1,
		Status:                  CalibrationStatusCandidate,
		InputFingerprint:        testHashB,
	}
	require.NoError(t, calRepo.CreateBatch(ctx, 401, []SKUBindingCalibration{calibration}))
	require.Error(t, calRepo.CreateBatch(ctx, 401, []SKUBindingCalibration{calibration}))
	rows, err := calRepo.ListBySnapshot(ctx, 401, snapshot.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Error(t, db.Model(&rows[0]).Update("confidence", 100).Error)
	require.Error(t, db.Delete(&rows[0]).Error)

	bad := calibration
	bad.ID = uuid.Nil
	bad.ExternalSKUID = "other-sku"
	require.Error(t, calRepo.CreateBatch(ctx, 401, []SKUBindingCalibration{bad, calibration}))
	rows, err = calRepo.ListBySnapshot(ctx, 401, snapshot.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

func TestManualBindingIdempotencyPendingUniqueAndResolveConcurrency(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	store, prod, sku := seedShopAndSKU(t, db, 501)
	run := createRun(t, ctx, db, 501, store.ID)
	snapRepo := NewInventorySnapshotRepository(db)
	require.NoError(t, snapRepo.CreateBatch(ctx, 501, []InventorySnapshotItem{validSnapshot(501, run, "remote-sku-1")}))
	snapshot, err := snapRepo.GetByRunAndExternalSKU(ctx, 501, run.ID, "remote-sku-1")
	require.NoError(t, err)
	repo := NewManualBindingRequestRepository(db)
	request := &ManualBindingRequest{
		TenantID:                501,
		InventorySyncRunID:      run.ID,
		InventorySnapshotItemID: snapshot.ID,
		ShopConnectionID:        store.ID,
		ExternalSKUID:           snapshot.ExternalSKUID,
		Status:                  ManualBindingStatusPending,
		ReasonCode:              "ambiguous_candidates",
		CandidateCount:          2,
		RequestID:               "manual-req-1",
		IdempotencyKeyHash:      testHashA,
		InputFingerprint:        testHashB,
		Revision:                1,
	}
	created, err := repo.Create(ctx, request)
	require.NoError(t, err)
	same, err := repo.Create(ctx, request)
	require.NoError(t, err)
	require.Equal(t, created.ID, same.ID)
	conflicting := *request
	conflicting.ID = uuid.Nil
	conflicting.InputFingerprint = testHashC
	_, err = repo.Create(ctx, &conflicting)
	require.ErrorIs(t, err, ErrIdempotencyPayloadConflict)
	second := *request
	second.ID = uuid.Nil
	second.RequestID = "manual-req-2"
	second.IdempotencyKeyHash = ""
	_, err = repo.Create(ctx, &second)
	require.ErrorIs(t, err, ErrManualBindingAlreadyPending)

	resolver := uuid.New()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.ResolveWithRevision(ctx, 501, created.ID, ManualBindingResolutionPatch{
				Status:                 ManualBindingStatusConfirmed,
				ExpectedRevision:       created.Revision,
				ResolvedBy:             resolver,
				Resolution:             "manual_selected",
				SelectedLocalProductID: &prod.ID,
				SelectedLocalSKUID:     &sku.ID,
			})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	successes := 0
	for err := range errs {
		if err == nil {
			successes++
			continue
		}
		require.True(t, errors.Is(err, ErrRevisionConflict) || errors.Is(err, ErrManualBindingAlreadyResolved) || errors.Is(err, ErrStateConflict), "unexpected error: %v", err)
	}
	require.Equal(t, 1, successes)
}

func ptr[T any](value T) *T {
	return &value
}
