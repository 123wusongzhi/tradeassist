//go:build p9postgres

package inventorysyncp9

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var p9PostgresTenantCounter int64 = 790000

type p9PostgresAllowManualAuthorizer struct{}

func (p9PostgresAllowManualAuthorizer) CanResolveManualBinding(context.Context, int64, uuid.UUID, uuid.UUID) error {
	return nil
}

func newP9PostgresTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	harness := postgrestest.Require(t)
	harness.EmitMetadata(t)
	require.NoError(t, harness.DB.AutoMigrate(
		&admin.AdminUser{},
		&admin.UserStorePermission{},
		&idempotency.Record{},
		&operationlog.OperationLog{},
		&shop.Shop{},
		&product.Product{},
		&product.ProductImage{},
		&product.ProductSKU{},
	))
	require.NoError(t, Migrate(harness.DB))
	return harness.DB
}

func p9PostgresTenantID() int64 {
	return atomic.AddInt64(&p9PostgresTenantCounter, 1)
}

func p9PostgresHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func p9PostgresSeedShopAndSKU(t *testing.T, db *gorm.DB, tenantID int64, suffix string) (shop.Shop, product.Product, product.ProductSKU) {
	t.Helper()
	store := shop.Shop{
		TenantID:       tenantID,
		Platform:       PlatformDouyin,
		ShopName:       fmt.Sprintf("p9-postgres-shop-%d-%s", tenantID, suffix),
		ExternalShopID: fmt.Sprintf("p9-postgres-safe-shop-%d-%s", tenantID, suffix),
		Status:         "active",
		AuthStatus:     "mock",
	}
	require.NoError(t, db.Create(&store).Error)
	prod := product.Product{TenantID: tenantID, Source: "manual", Title: fmt.Sprintf("p9-postgres-local-product-%d-%s", tenantID, suffix), Status: product.StatusDraft}
	require.NoError(t, db.Create(&prod).Error)
	sku := product.ProductSKU{ProductID: prod.ID, SKUCode: fmt.Sprintf("P9PG-SKU-%d-%s", tenantID, suffix), SKUName: "P9 PostgreSQL Local SKU"}
	require.NoError(t, db.Create(&sku).Error)
	return store, prod, sku
}

func p9PostgresValidRun(tenantID int64, shopID uuid.UUID, suffix string) InventorySyncRun {
	now := time.Now().UTC()
	return InventorySyncRun{
		TenantID:              tenantID,
		ShopConnectionID:      shopID,
		Platform:              PlatformDouyin,
		ProviderMode:          ProviderModeMock,
		ExternalShopReference: fmt.Sprintf("safe-shop-ref-%d-%s", tenantID, suffix),
		Status:                InventorySyncRunStatusRunning,
		Cursor:                datatypes.JSON([]byte(`{"page":1}`)),
		Checkpoint:            datatypes.JSON([]byte(`{"cursor":"safe"}`)),
		SafeErrorMetadata:     datatypes.JSON([]byte(`{}`)),
		RequestID:             fmt.Sprintf("p9pg-run-%d-%s", tenantID, suffix),
		IdempotencyKeyHash:    p9PostgresHash("run-key-" + suffix),
		InputFingerprint:      p9PostgresHash("run-fingerprint-" + suffix),
		Revision:              1,
		StartedAt:             &now,
	}
}

func p9PostgresCreateRun(t *testing.T, ctx context.Context, db *gorm.DB, tenantID int64, shopID uuid.UUID, suffix string) *InventorySyncRun {
	t.Helper()
	run, err := NewInventorySyncRunRepository(db).Create(ctx, ptr(p9PostgresValidRun(tenantID, shopID, suffix)))
	require.NoError(t, err)
	return run
}

func p9PostgresValidSnapshot(tenantID int64, run *InventorySyncRun, externalSKUID string, suffix string) InventorySnapshotItem {
	return InventorySnapshotItem{
		TenantID:           tenantID,
		InventorySyncRunID: run.ID,
		ShopConnectionID:   run.ShopConnectionID,
		Platform:           run.Platform,
		ExternalProductID:  "p9pg-remote-product-" + suffix,
		ExternalSKUID:      externalSKUID,
		ExternalSKUCode:    "P9PG-REMOTE-CODE-" + suffix,
		Barcode:            "P9PG-BARCODE-" + suffix,
		ProductTitle:       "P9 PostgreSQL remote product",
		VariantTitle:       "safe variant",
		AvailableQuantity:  5,
		ReservedQuantity:   1,
		TotalQuantity:      8,
		ObservedAt:         time.Now().UTC(),
		PayloadHash:        p9PostgresHash("snapshot-payload-" + suffix),
		SafeMetadata:       datatypes.JSON([]byte(`{"quantityRelationshipContract":"provider_defined","fixture":"postgres"}`)),
	}
}

func p9PostgresCalibration(tenantID int64, run *InventorySyncRun, snapshot *InventorySnapshotItem, prod product.Product, sku product.ProductSKU, suffix string) SKUBindingCalibration {
	return SKUBindingCalibration{
		TenantID:                tenantID,
		InventorySyncRunID:      run.ID,
		InventorySnapshotItemID: snapshot.ID,
		ExternalSKUID:           snapshot.ExternalSKUID,
		CandidateLocalProductID: prod.ID,
		CandidateLocalSKUID:     sku.ID,
		MatchStrategy:           MatchStrategyExactSKUCode,
		Confidence:              9500,
		ScoreBreakdown:          datatypes.JSON([]byte(`[{"code":"exactSKUCodeScore","points":9500,"reason":"exact_sku_code_match"}]`)),
		ReasonCodes:             datatypes.JSON([]byte(`["exact_sku_code_match"]`)),
		CalibrationVersion:      CalibrationVersionV1,
		Status:                  CalibrationStatusCandidate,
		InputFingerprint:        p9PostgresHash("calibration-input-" + suffix),
	}
}

func p9PostgresManualRequest(tenantID int64, run *InventorySyncRun, snapshot *InventorySnapshotItem, store shop.Shop, suffix string) *ManualBindingRequest {
	return &ManualBindingRequest{
		TenantID:                tenantID,
		InventorySyncRunID:      run.ID,
		InventorySnapshotItemID: snapshot.ID,
		ShopConnectionID:        store.ID,
		ExternalSKUID:           snapshot.ExternalSKUID,
		Status:                  ManualBindingStatusPending,
		ReasonCode:              ReasonManualReviewRequired,
		CandidateCount:          2,
		RequestID:               fmt.Sprintf("p9pg-manual-%d-%s", tenantID, suffix),
		IdempotencyKeyHash:      p9PostgresHash("manual-idempotency-" + suffix),
		InputFingerprint:        p9PostgresHash("manual-input-" + suffix),
		Revision:                1,
	}
}

func TestP9PostgresMigrationSchemaIndexesConstraintsAndJSONB(t *testing.T) {
	db := newP9PostgresTestDB(t)
	require.NoError(t, Migrate(db))

	for _, table := range []string{
		"p9_inventory_sync_runs",
		"p9_inventory_snapshot_items",
		"p9_sku_bindings",
		"p9_sku_binding_calibrations",
		"p9_manual_binding_requests",
		"p9_manual_binding_decisions",
	} {
		require.Truef(t, db.Migrator().HasTable(table), "expected P9 table %s", table)
	}

	for _, index := range []string{
		"ux_p9_inventory_sync_runs_tenant_idempotency",
		"ux_p9_inventory_snapshots_tenant_run_external_sku",
		"ux_p9_sku_bindings_current_confirmed",
		"ux_p9_manual_binding_requests_pending",
		"ux_p9_manual_binding_decisions_idempotency",
	} {
		var count int64
		require.NoError(t, db.Raw("SELECT COUNT(*) FROM pg_indexes WHERE schemaname = current_schema() AND indexname = ?", index).Scan(&count).Error)
		require.Equalf(t, int64(1), count, "expected PostgreSQL index %s", index)
	}

	for _, constraint := range []string{
		"chk_p9_inventory_sync_runs_revision",
		"chk_p9_inventory_sync_runs_provider_mode",
		"chk_p9_inventory_snapshot_items_quantities",
		"chk_p9_sku_bindings_revision_confidence",
		"chk_p9_sku_binding_calibrations_confidence",
		"chk_p9_manual_binding_requests_hashes",
	} {
		var count int64
		require.NoError(t, db.Raw("SELECT COUNT(*) FROM pg_constraint WHERE conname = ? AND connamespace = current_schema()::regnamespace", constraint).Scan(&count).Error)
		require.Equalf(t, int64(1), count, "expected PostgreSQL constraint %s", constraint)
	}

	for _, column := range []struct {
		table string
		name  string
	}{
		{"p9_inventory_sync_runs", "cursor"},
		{"p9_inventory_sync_runs", "checkpoint"},
		{"p9_inventory_sync_runs", "safe_error_metadata"},
		{"p9_inventory_snapshot_items", "safe_metadata"},
		{"p9_sku_binding_calibrations", "score_breakdown"},
		{"p9_sku_binding_calibrations", "reason_codes"},
	} {
		var dataType string
		require.NoError(t, db.Raw("SELECT data_type FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?", column.table, column.name).Scan(&dataType).Error)
		require.Equalf(t, "jsonb", dataType, "expected %s.%s to be jsonb", column.table, column.name)
	}

	for _, trigger := range []string{
		"trg_p9_inventory_snapshot_items_no_update",
		"trg_p9_inventory_snapshot_items_no_delete",
		"trg_p9_sku_binding_calibrations_no_update",
		"trg_p9_sku_binding_calibrations_no_delete",
		"trg_p9_manual_binding_decisions_no_update",
		"trg_p9_manual_binding_decisions_no_delete",
		"trg_p9_operation_logs_no_update",
		"trg_p9_operation_logs_no_delete",
	} {
		var count int64
		require.NoError(t, db.Raw("SELECT COUNT(*) FROM pg_trigger t JOIN pg_class c ON c.oid = t.tgrelid JOIN pg_namespace n ON n.oid = c.relnamespace WHERE t.tgname = ? AND NOT t.tgisinternal AND n.nspname = current_schema()", trigger).Scan(&count).Error)
		require.Equalf(t, int64(1), count, "expected PostgreSQL trigger %s", trigger)
	}
}

func TestP9PostgresRepositoryConstraintsImmutabilityAndAtomicity(t *testing.T) {
	ctx := context.Background()
	db := newP9PostgresTestDB(t)
	tenantID := p9PostgresTenantID()
	suffix := fmt.Sprintf("repo-%d", tenantID)
	store, prod, sku := p9PostgresSeedShopAndSKU(t, db, tenantID, suffix)
	run := p9PostgresCreateRun(t, ctx, db, tenantID, store.ID, suffix)

	snapshotRepo := NewInventorySnapshotRepository(db)
	snapshot := p9PostgresValidSnapshot(tenantID, run, "p9pg-remote-sku-"+suffix, suffix)
	require.NoError(t, snapshotRepo.CreateBatch(ctx, tenantID, []InventorySnapshotItem{snapshot}))
	require.ErrorIs(t, snapshotRepo.CreateBatch(ctx, tenantID, []InventorySnapshotItem{p9PostgresValidSnapshot(tenantID, run, snapshot.ExternalSKUID, suffix+"-duplicate")}), ErrDuplicateExternalSKU)
	storedSnapshot, err := snapshotRepo.GetByRunAndExternalSKU(ctx, tenantID, run.ID, snapshot.ExternalSKUID)
	require.NoError(t, err)

	negativeQuantity := p9PostgresValidSnapshot(tenantID, run, "p9pg-negative-"+suffix, suffix+"-negative")
	negativeQuantity.ID = uuid.New()
	negativeQuantity.AvailableQuantity = -1
	require.Error(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&negativeQuantity).Error)

	require.Error(t, db.Session(&gorm.Session{SkipHooks: true}).Model(&InventorySnapshotItem{}).Where("tenant_id = ? AND id = ?", tenantID, storedSnapshot.ID).Update("total_quantity", 9).Error)
	require.Error(t, db.Session(&gorm.Session{SkipHooks: true}).Delete(&InventorySnapshotItem{}, "tenant_id = ? AND id = ?", tenantID, storedSnapshot.ID).Error)

	calRepo := NewSKUBindingCalibrationRepository(db)
	calibration := p9PostgresCalibration(tenantID, run, storedSnapshot, prod, sku, suffix)
	require.NoError(t, calRepo.CreateBatch(ctx, tenantID, []SKUBindingCalibration{calibration}))
	rows, err := calRepo.ListBySnapshot(ctx, tenantID, storedSnapshot.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	var scoreType, reasonType string
	require.NoError(t, db.Raw("SELECT jsonb_typeof(score_breakdown), jsonb_typeof(reason_codes) FROM p9_sku_binding_calibrations WHERE tenant_id = ? AND id = ?", tenantID, rows[0].ID).Row().Scan(&scoreType, &reasonType))
	require.Equal(t, "array", scoreType)
	require.Equal(t, "array", reasonType)
	require.Error(t, db.Session(&gorm.Session{SkipHooks: true}).Model(&SKUBindingCalibration{}).Where("tenant_id = ? AND id = ?", tenantID, rows[0].ID).Update("confidence", 100).Error)
	require.Error(t, db.Session(&gorm.Session{SkipHooks: true}).Delete(&SKUBindingCalibration{}, "tenant_id = ? AND id = ?", tenantID, rows[0].ID).Error)

	badCalibration := p9PostgresCalibration(tenantID, run, storedSnapshot, prod, sku, suffix+"-bad")
	badCalibration.ID = uuid.Nil
	badCalibration.ExternalSKUID = "other-sku-" + suffix
	require.Error(t, calRepo.CreateBatch(ctx, tenantID, []SKUBindingCalibration{badCalibration, calibration}))
	rows, err = calRepo.ListBySnapshot(ctx, tenantID, storedSnapshot.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	bindingRepo := NewSKUBindingRepository(db)
	binding := &SKUBinding{
		TenantID:           tenantID,
		ShopConnectionID:   store.ID,
		Platform:           PlatformDouyin,
		ExternalProductID:  storedSnapshot.ExternalProductID,
		ExternalSKUID:      storedSnapshot.ExternalSKUID,
		LocalProductID:     prod.ID,
		LocalSKUID:         sku.ID,
		BindingSource:      SKUBindingSourceAutomatic,
		BindingStatus:      SKUBindingStatusConfirmed,
		Confidence:         9500,
		CalibrationVersion: CalibrationVersionV1,
		Revision:           1,
	}
	_, err = bindingRepo.CreateProposed(ctx, binding)
	require.NoError(t, err)
	duplicateBinding := *binding
	duplicateBinding.ID = uuid.Nil
	_, err = bindingRepo.CreateProposed(ctx, &duplicateBinding)
	require.ErrorIs(t, err, ErrBindingConflict)
}

func TestP9PostgresIdempotencyOptimisticConcurrencyAndManualResolution(t *testing.T) {
	ctx := context.Background()
	db := newP9PostgresTestDB(t)
	tenantID := p9PostgresTenantID()
	suffix := fmt.Sprintf("concurrency-%d", tenantID)
	store, prod, sku := p9PostgresSeedShopAndSKU(t, db, tenantID, suffix)

	runRepo := NewInventorySyncRunRepository(db)
	first, err := runRepo.Create(ctx, ptr(p9PostgresValidRun(tenantID, store.ID, suffix)))
	require.NoError(t, err)
	second, err := runRepo.Create(ctx, ptr(p9PostgresValidRun(tenantID, store.ID, suffix)))
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	conflictingRun := p9PostgresValidRun(tenantID, store.ID, suffix)
	conflictingRun.InputFingerprint = p9PostgresHash("different-fingerprint-" + suffix)
	_, err = runRepo.Create(ctx, &conflictingRun)
	require.ErrorIs(t, err, ErrIdempotencyPayloadConflict)

	finished := time.Now().UTC().Add(time.Minute)
	updated, err := runRepo.UpdateStatusWithRevision(ctx, tenantID, first.ID, first.Revision, InventorySyncRunStatusSucceeded, InventorySyncRunStatusPatch{FinishedAt: &finished})
	require.NoError(t, err)
	require.Equal(t, first.Revision+1, updated.Revision)
	_, err = runRepo.UpdateStatusWithRevision(ctx, tenantID, first.ID, first.Revision, InventorySyncRunStatusFailed, InventorySyncRunStatusPatch{})
	require.ErrorIs(t, err, ErrRevisionConflict)

	failedRun := p9PostgresValidRun(tenantID, store.ID, suffix+"-rerun-source")
	failedRun.Status = InventorySyncRunStatusFailed
	failedRun.SafeErrorMetadata = datatypes.JSON([]byte(`{"errorCode":"provider_rejected","safeMessage":"fixture rejected","retryable":true}`))
	failedRun.FinishedAt = &finished
	source, err := runRepo.Create(ctx, &failedRun)
	require.NoError(t, err)
	runRegistry, err := NewDefaultInventoryProviderRegistry()
	require.NoError(t, err)
	policy, err := NewCalibrationThresholdPolicy(CalibrationThresholdConfig{HighConfidenceThreshold: 9500, AutoConfirmationEnabled: false, PolicyVersion: ThresholdPolicyVersionV1})
	require.NoError(t, err)
	calibration := NewSKUBindingCalibrationService(db, NewGORMLocalSKUCandidateProvider(db), policy)
	orchestrator := NewInventorySyncOrchestrator(db, runRegistry, calibration, testSyncAuthorizer{allowed: true})
	start := make(chan struct{})
	rerunIDs := make(chan uuid.UUID, 2)
	rerunErrors := runConcurrent(2, start, func(index int) error {
		result, rerunErr := orchestrator.ManualRerun(ctx, InventorySyncOrchestratorInput{TenantID: tenantID, ShopConnectionID: store.ID, Platform: PlatformDouyin, ProviderMode: ProviderModeMock, FixtureScenario: FixtureScenarioEmptyInventory, TriggerType: InventorySyncTriggerManualRerun, ActorID: uuid.New(), RequestID: fmt.Sprintf("p9pg-rerun-%d-%s", index, suffix), IdempotencyKeyHash: p9PostgresHash(fmt.Sprintf("p9pg-rerun-%d-%s", index, suffix)), SourceRunID: source.ID, SourceRunRevision: source.Revision})
		if result != nil {
			rerunIDs <- result.InventorySyncRunID
		}
		return rerunErr
	})
	close(rerunIDs)
	runClaims := map[uuid.UUID]bool{}
	for id := range rerunIDs {
		runClaims[id] = true
	}
	require.Len(t, runClaims, 1)
	require.Equal(t, 1, countSuccessful(rerunErrors))
	for _, rerunErr := range rerunErrors {
		if rerunErr != nil {
			require.True(t, errors.Is(rerunErr, ErrRevisionConflict) || errors.Is(rerunErr, ErrStateConflict), "unexpected rerun error: %v", rerunErr)
		}
	}

	activeRun := p9PostgresCreateRun(t, ctx, db, tenantID, store.ID, suffix+"-manual")
	snapshotRepo := NewInventorySnapshotRepository(db)
	snapshot := p9PostgresValidSnapshot(tenantID, activeRun, "p9pg-manual-sku-"+suffix, suffix+"-manual")
	require.NoError(t, snapshotRepo.CreateBatch(ctx, tenantID, []InventorySnapshotItem{snapshot}))
	storedSnapshot, err := snapshotRepo.GetByRunAndExternalSKU(ctx, tenantID, activeRun.ID, snapshot.ExternalSKUID)
	require.NoError(t, err)
	manualRepo := NewManualBindingRequestRepository(db)
	manualRequest, err := manualRepo.Create(ctx, p9PostgresManualRequest(tenantID, activeRun, storedSnapshot, store, suffix))
	require.NoError(t, err)

	secondPending := *manualRequest
	secondPending.ID = uuid.Nil
	secondPending.RequestID = "p9pg-manual-pending-duplicate-" + suffix
	secondPending.IdempotencyKeyHash = p9PostgresHash("manual-idempotency-duplicate-" + suffix)
	_, err = manualRepo.Create(ctx, &secondPending)
	require.ErrorIs(t, err, ErrManualBindingAlreadyPending)

	service := NewManualBindingService(db, p9PostgresAllowManualAuthorizer{})
	actorID := uuid.New()
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, confirmErr := service.ConfirmBinding(ctx, ConfirmManualBindingInput{
				Actor:              ManualBindingActor{TenantID: tenantID, ActorID: actorID},
				RequestID:          manualRequest.ID,
				CorrelationID:      fmt.Sprintf("p9pg-confirm-%d-%s", idx, suffix),
				ExpectedRevision:   manualRequest.Revision,
				SelectedLocalSKUID: sku.ID,
				IdempotencyKeyHash: p9PostgresHash(fmt.Sprintf("manual-confirm-%d-%s", idx, suffix)),
				Comment:            "P9 PostgreSQL manual confirmation",
			})
			errs <- confirmErr
		}(i)
	}
	wg.Wait()
	close(errs)
	successes := 0
	for confirmErr := range errs {
		if confirmErr == nil {
			successes++
			continue
		}
		require.True(t, errors.Is(confirmErr, ErrRevisionConflict) || errors.Is(confirmErr, ErrManualBindingAlreadyResolved) || errors.Is(confirmErr, ErrStateConflict), "unexpected concurrent manual binding error: %v", confirmErr)
	}
	require.Equal(t, 1, successes)

	var decision ManualBindingDecision
	require.NoError(t, db.Where("tenant_id = ? AND manual_binding_request_id = ?", tenantID, manualRequest.ID).First(&decision).Error)
	require.Error(t, db.Session(&gorm.Session{SkipHooks: true}).Model(&ManualBindingDecision{}).Where("tenant_id = ? AND id = ?", tenantID, decision.ID).Update("comment", "mutated").Error)
	require.Error(t, db.Session(&gorm.Session{SkipHooks: true}).Delete(&ManualBindingDecision{}, "tenant_id = ? AND id = ?", tenantID, decision.ID).Error)

	confirmed, err := NewSKUBindingRepository(db).GetCurrentConfirmed(ctx, tenantID, store.ID, storedSnapshot.ExternalSKUID)
	require.NoError(t, err)
	require.Equal(t, prod.ID, confirmed.LocalProductID)

	var audit operationlog.OperationLog
	require.NoError(t, db.Where("tenant_id = ? AND resource IN ?", tenantID, []string{"inventory_sync", "sku_binding"}).First(&audit).Error)
	require.Error(t, db.Model(&operationlog.OperationLog{}).Where("id = ?", audit.ID).Update("status", "mutated").Error)
	require.Error(t, db.Delete(&operationlog.OperationLog{}, "id = ?", audit.ID).Error)
}

func TestP9PostgresAPIKeysetSafetyAndP10Boundary(t *testing.T) {
	tenantID := p9PostgresTenantID()
	router, svc, actorID, shopID := newP9PostgresAPITestRouter(t, admin.RoleAdmin, tenantID)
	payload := map[string]any{"shopConnectionId": shopID, "platform": PlatformDouyin, "providerMode": ProviderModeMock, "fixtureScenario": FixtureScenarioSuccessSinglePage}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, p9PostgresRequest(t, http.MethodPost, "/api/v1/inventory-sync/runs", payload, "p9pg-create-"+fmt.Sprint(tenantID)))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "cursorAfter")
	require.NotContains(t, rec.Body.String(), "checkpoint")
	require.NotContains(t, rec.Body.String(), "idempotencyKeyHash")

	replay := httptest.NewRecorder()
	router.ServeHTTP(replay, p9PostgresRequest(t, http.MethodPost, "/api/v1/inventory-sync/runs", payload, "p9pg-create-"+fmt.Sprint(tenantID)))
	require.Equal(t, http.StatusCreated, replay.Code, replay.Body.String())

	_, err := svc.CreateRun(context.Background(), APIActor{TenantID: tenantID, ActorID: actorID, Role: admin.RoleAdmin}, CreateInventorySyncRunRequest{ShopConnectionID: shopID, Platform: PlatformDouyin, ProviderMode: ProviderModeMock, FixtureScenario: FixtureScenarioEmptyInventory}, "p9pg-api-list-"+fmt.Sprint(tenantID), p9PostgresHash("p9pg-api-list-"+fmt.Sprint(tenantID)))
	require.NoError(t, err)
	list := httptest.NewRecorder()
	router.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/inventory-sync/runs?limit=1", nil))
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	require.Contains(t, list.Body.String(), "nextCursor")
	require.Contains(t, list.Body.String(), "hasMore")
	require.NotContains(t, list.Body.String(), "offset")
	require.NotContains(t, list.Body.String(), "checkpoint")

	prodCap := httptest.NewRecorder()
	router.ServeHTTP(prodCap, p9PostgresRequest(t, http.MethodPost, "/api/v1/inventory-sync/runs", map[string]any{"shopConnectionId": shopID, "platform": PlatformDouyin, "providerMode": "prod"}, "p9pg-prod-cap-"+fmt.Sprint(tenantID)))
	require.Equal(t, http.StatusForbidden, prodCap.Code, prodCap.Body.String())

	readonly, _, _, readonlyShopID := newP9PostgresAPITestRouter(t, adminperm.RoleReadonly, p9PostgresTenantID())
	denied := httptest.NewRecorder()
	readonly.ServeHTTP(denied, p9PostgresRequest(t, http.MethodPost, "/api/v1/inventory-sync/runs", map[string]any{"shopConnectionId": readonlyShopID, "platform": PlatformDouyin, "providerMode": ProviderModeMock}, "p9pg-readonly-"+fmt.Sprint(tenantID)))
	require.Equal(t, http.StatusForbidden, denied.Code, denied.Body.String())

	mutation := httptest.NewRecorder()
	router.ServeHTTP(mutation, httptest.NewRequest(http.MethodPatch, "/api/v1/inventory-sync/snapshots/"+uuid.New().String(), nil))
	require.Equal(t, http.StatusNotFound, mutation.Code)
}

func newP9PostgresAPITestRouter(t *testing.T, role string, tenantID int64) (*gin.Engine, *APIService, uuid.UUID, uuid.UUID) {
	t.Helper()
	db := newP9PostgresTestDB(t)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &idempotency.Record{}, &operationlog.OperationLog{}))
	store, _, _ := p9PostgresSeedShopAndSKU(t, db, tenantID, "api")
	actorID := uuid.New()
	require.NoError(t, db.Create(&admin.AdminUser{Base: model.Base{ID: actorID}, TenantID: tenantID, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: role, Status: admin.StatusActive}).Error)
	svc := NewAPIService(db)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.TenantID, tenantID)
		c.Set(ctxkey.AdminID, actorID.String())
		c.Set(ctxkey.TraceID, "trace-p9-postgres-api-test")
		c.Next()
	})
	Register(r.Group("/api/v1"), &Handler{Svc: svc})
	return r, svc, actorID, store.ID
}

func p9PostgresRequest(t *testing.T, method, requestPath string, body any, idem string) *http.Request {
	t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, requestPath, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
	return req
}
