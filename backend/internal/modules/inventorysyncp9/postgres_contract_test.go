//go:build p9postgres

package inventorysyncp9

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/middleware"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

type p9PostgresBarrierProvider struct {
	base    InventoryProvider
	barrier <-chan struct{}
}

func (p p9PostgresBarrierProvider) Key() InventoryProviderKey { return p.base.Key() }
func (p p9PostgresBarrierProvider) Capabilities() InventoryProviderCapabilities {
	return p.base.Capabilities()
}
func (p p9PostgresBarrierProvider) FetchInventoryPage(ctx context.Context, req InventoryFetchRequest) (InventoryFetchPageResult, error) {
	select {
	case <-ctx.Done():
		return InventoryFetchPageResult{}, ErrSyncCancelled
	case <-p.barrier:
		return p.base.FetchInventoryPage(ctx, req)
	}
}

func TestP9PGSchemaForeignKeysPartialIndexesAndTimeContract(t *testing.T) {
	db := newP9PostgresTestDB(t)

	for _, spec := range []struct {
		name      string
		predicate string
	}{
		{"ux_p9_sku_bindings_current_confirmed", "confirmed"},
		{"ux_p9_manual_binding_requests_pending", "pending"},
		{"ux_p9_inventory_sync_runs_tenant_idempotency", "idempotency_key_hash"},
	} {
		var index struct {
			IsUnique  bool
			Predicate string
		}
		require.NoError(t, db.Raw(`SELECT i.indisunique AS is_unique, COALESCE(pg_get_expr(i.indpred, i.indrelid), '') AS predicate FROM pg_index i JOIN pg_class idx ON idx.oid = i.indexrelid JOIN pg_namespace n ON n.oid = idx.relnamespace WHERE n.nspname = current_schema() AND idx.relname = ?`, spec.name).Scan(&index).Error)
		require.True(t, index.IsUnique, spec.name)
		require.Contains(t, index.Predicate, spec.predicate, spec.name)
	}

	for _, table := range []string{"p9_inventory_sync_runs", "p9_inventory_snapshot_items", "p9_sku_bindings", "p9_sku_binding_calibrations", "p9_manual_binding_requests", "p9_manual_binding_decisions"} {
		var count int64
		require.NoError(t, db.Raw(`SELECT COUNT(*) FROM pg_constraint c JOIN pg_class r ON r.oid = c.conrelid JOIN pg_namespace n ON n.oid = r.relnamespace WHERE c.contype = 'f' AND n.nspname = current_schema() AND r.relname = ?`, table).Scan(&count).Error)
		require.Greater(t, count, int64(0), table)
	}

	for _, column := range []struct{ table, name string }{
		{"p9_inventory_sync_runs", "created_at"},
		{"p9_inventory_sync_runs", "started_at"},
		{"p9_inventory_snapshot_items", "observed_at"},
		{"p9_inventory_snapshot_items", "source_updated_at"},
	} {
		var dataType string
		require.NoError(t, db.Raw("SELECT data_type FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?", column.table, column.name).Scan(&dataType).Error)
		require.Contains(t, []string{"timestamp with time zone", "timestamp without time zone"}, dataType)
	}
}

func TestP9PGConcurrentDatabaseConstraintsAndIdempotency(t *testing.T) {
	ctx := context.Background()
	db := newP9PostgresTestDB(t)
	tenantID := p9PostgresTenantID()
	store, prod, sku := p9PostgresSeedShopAndSKU(t, db, tenantID, "constraint-concurrency")
	run := p9PostgresCreateRun(t, ctx, db, tenantID, store.ID, "constraint-concurrency")
	snapshot := p9PostgresValidSnapshot(tenantID, run, "p9pg-concurrent-sku", "constraint-concurrency")
	require.NoError(t, NewInventorySnapshotRepository(db).CreateBatch(ctx, tenantID, []InventorySnapshotItem{snapshot}))
	stored, err := NewInventorySnapshotRepository(db).GetByRunAndExternalSKU(ctx, tenantID, run.ID, snapshot.ExternalSKUID)
	require.NoError(t, err)

	start := make(chan struct{})
	bindingErrors := runConcurrent(2, start, func(index int) error {
		binding := &SKUBinding{TenantID: tenantID, ShopConnectionID: store.ID, Platform: PlatformDouyin, ExternalProductID: stored.ExternalProductID, ExternalSKUID: stored.ExternalSKUID, LocalProductID: prod.ID, LocalSKUID: sku.ID, BindingSource: SKUBindingSourceAutomatic, BindingStatus: SKUBindingStatusConfirmed, Confidence: 9500, CalibrationVersion: 1, Revision: 1}
		_, createErr := NewSKUBindingRepository(db.Session(&gorm.Session{NewDB: true})).CreateProposed(ctx, binding)
		return createErr
	})
	require.Equal(t, 1, countSuccessful(bindingErrors))

	otherSnapshot := p9PostgresValidSnapshot(tenantID, run, "p9pg-concurrent-manual", "constraint-manual")
	require.NoError(t, NewInventorySnapshotRepository(db).CreateBatch(ctx, tenantID, []InventorySnapshotItem{otherSnapshot}))
	storedManual, err := NewInventorySnapshotRepository(db).GetByRunAndExternalSKU(ctx, tenantID, run.ID, otherSnapshot.ExternalSKUID)
	require.NoError(t, err)
	start = make(chan struct{})
	manualErrors := runConcurrent(2, start, func(index int) error {
		req := p9PostgresManualRequest(tenantID, run, storedManual, store, string(rune('a'+index)))
		req.RequestID = "p9pg-concurrent-manual-" + string(rune('a'+index))
		req.IdempotencyKeyHash = p9PostgresHash(req.RequestID)
		_, createErr := NewManualBindingRequestRepository(db.Session(&gorm.Session{NewDB: true})).Create(ctx, req)
		return createErr
	})
	require.Equal(t, 1, countSuccessful(manualErrors))

	base := p9PostgresValidRun(tenantID, store.ID, "same-run-concurrency")
	start = make(chan struct{})
	runIDs := make(chan uuid.UUID, 2)
	runErrors := runConcurrent(2, start, func(index int) error {
		candidate := base
		created, createErr := NewInventorySyncRunRepository(db.Session(&gorm.Session{NewDB: true})).Create(ctx, &candidate)
		if created != nil {
			runIDs <- created.ID
		}
		return createErr
	})
	close(runIDs)
	ids := map[uuid.UUID]bool{}
	for id := range runIDs {
		ids[id] = true
	}
	require.Len(t, ids, 1)
	for _, runErr := range runErrors {
		require.NoError(t, runErr)
	}

	replaySnapshot := p9PostgresValidSnapshot(tenantID, run, "p9pg-concurrent-manual-replay", "constraint-manual-replay")
	require.NoError(t, NewInventorySnapshotRepository(db).CreateBatch(ctx, tenantID, []InventorySnapshotItem{replaySnapshot}))
	storedReplay, err := NewInventorySnapshotRepository(db).GetByRunAndExternalSKU(ctx, tenantID, run.ID, replaySnapshot.ExternalSKUID)
	require.NoError(t, err)
	replayRequest := p9PostgresManualRequest(tenantID, run, storedReplay, store, "idempotent-replay")
	start = make(chan struct{})
	manualIDs := make(chan uuid.UUID, 2)
	replayErrors := runConcurrent(2, start, func(index int) error {
		candidate := *replayRequest
		created, createErr := NewManualBindingRequestRepository(db.Session(&gorm.Session{NewDB: true})).Create(ctx, &candidate)
		if created != nil {
			manualIDs <- created.ID
		}
		return createErr
	})
	close(manualIDs)
	replayedIDs := map[uuid.UUID]bool{}
	for id := range manualIDs {
		replayedIDs[id] = true
	}
	require.Len(t, replayedIDs, 1)
	for _, replayErr := range replayErrors {
		require.NoError(t, replayErr)
	}
}

func TestP9PGPageTransactionRollbackKeepsCursorAndStatistics(t *testing.T) {
	ctx := context.Background()
	db := newP9PostgresTestDB(t)
	tenantID := p9PostgresTenantID()
	store, _, _ := p9PostgresSeedShopAndSKU(t, db, tenantID, "page-rollback")

	callbackName := "p9pg:fail_page_audit"
	db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "operation_logs" {
			if row, ok := tx.Statement.Dest.(*operationlog.OperationLog); ok && row.Action == "inventory_sync.page_processed" {
				tx.AddError(errors.New("p9pg injected audit failure"))
			}
		}
	})
	t.Cleanup(func() { db.Callback().Create().Remove(callbackName) })

	registry, err := NewInventoryProviderRegistry(NewDouyinInventoryFixtureProvider(FixtureScenarioSuccessSinglePage))
	require.NoError(t, err)
	policy, err := NewCalibrationThresholdPolicy(CalibrationThresholdConfig{HighConfidenceThreshold: 9500, AutoConfirmationEnabled: false})
	require.NoError(t, err)
	calibration := NewSKUBindingCalibrationService(db, NewGORMLocalSKUCandidateProvider(db), policy)
	orchestrator := NewInventorySyncOrchestrator(db, registry, calibration, testSyncAuthorizer{allowed: true})
	result, runErr := orchestrator.Run(ctx, InventorySyncOrchestratorInput{TenantID: tenantID, ShopConnectionID: store.ID, Platform: PlatformDouyin, ProviderMode: ProviderModeSandbox, FixtureScenario: FixtureScenarioSuccessSinglePage, ActorID: uuid.New(), RequestID: "p9pg-page-rollback"})
	require.Error(t, runErr)
	require.NotNil(t, result)
	require.Equal(t, InventorySyncRunStatusFailed, result.Status)
	require.Equal(t, 0, result.TotalRecordCount)
	require.Equal(t, 0, result.ManualBindingRequestCount)

	var snapshotCount, calibrationCount, manualCount int64
	require.NoError(t, db.Model(&InventorySnapshotItem{}).Where("tenant_id = ? AND inventory_sync_run_id = ?", tenantID, result.InventorySyncRunID).Count(&snapshotCount).Error)
	require.NoError(t, db.Model(&SKUBindingCalibration{}).Where("tenant_id = ? AND inventory_sync_run_id = ?", tenantID, result.InventorySyncRunID).Count(&calibrationCount).Error)
	require.NoError(t, db.Model(&ManualBindingRequest{}).Where("tenant_id = ? AND inventory_sync_run_id = ?", tenantID, result.InventorySyncRunID).Count(&manualCount).Error)
	require.Zero(t, snapshotCount)
	require.Zero(t, calibrationCount)
	require.Zero(t, manualCount)

	run, err := NewInventorySyncRunRepository(db).GetByID(ctx, tenantID, result.InventorySyncRunID)
	require.NoError(t, err)
	require.JSONEq(t, `{}`, string(run.Cursor))
	checkpoint := decodeCheckpoint(run.Checkpoint)
	require.Zero(t, checkpoint.PagesProcessed)
	require.Zero(t, checkpoint.TotalRecordCount)
}

func TestP9PGKeysetPaginationNoDuplicateOmissionAndScopeProtection(t *testing.T) {
	ctx := context.Background()
	db := newP9PostgresTestDB(t)
	tenantID := p9PostgresTenantID()
	store, _, _ := p9PostgresSeedShopAndSKU(t, db, tenantID, "keyset")
	repo := NewAPIRepository(db)
	at := time.Now().UTC().Truncate(time.Microsecond)

	for index := 0; index < 7; index++ {
		run := p9PostgresValidRun(tenantID, store.ID, string(rune('a'+index)))
		run.CreatedAt = at
		run.UpdatedAt = at
		require.NoError(t, db.Create(&run).Error)
	}

	seen := map[uuid.UUID]bool{}
	cursor := ""
	for {
		rows, next, more, err := repo.ListRuns(ctx, tenantID, InventorySyncRunListParams{ShopConnectionID: &store.ID, Limit: 2, Cursor: cursor})
		require.NoError(t, err)
		for _, row := range rows {
			require.False(t, seen[row.ID], row.ID.String())
			seen[row.ID] = true
		}
		if !more {
			break
		}
		require.NotEmpty(t, next)
		cursor = next
	}
	require.Len(t, seen, 7)

	_, _, _, err := repo.ListRuns(ctx, tenantID+1, InventorySyncRunListParams{ShopConnectionID: &store.ID, Limit: 2, Cursor: cursor})
	require.Error(t, err)
	_, _, _, err = repo.ListRuns(ctx, tenantID, InventorySyncRunListParams{ShopConnectionID: &store.ID, Status: InventorySyncRunStatusSucceeded, Limit: 2, Cursor: cursor})
	require.Error(t, err)
	_, _, _, err = repo.ListBindings(ctx, tenantID, BindingListParams{ShopConnectionID: &store.ID, Limit: 2, Cursor: cursor})
	require.Error(t, err)
}

func TestP9PGBearerAuthAndFixtureGoldenPath(t *testing.T) {
	db := newP9PostgresTestDB(t)
	require.NoError(t, db.AutoMigrate(&inventory.InventoryChangeLog{}))
	tenantID := p9PostgresTenantID()
	store, _, _ := p9PostgresSeedShopAndSKU(t, db, tenantID, "auth-golden")
	actorID := uuid.New()
	username := admin.NewInternalUsername()
	require.NoError(t, db.Create(&admin.AdminUser{Base: model.Base{ID: actorID}, TenantID: tenantID, Username: username, PasswordHash: "test", Role: admin.RoleAdmin, Status: admin.StatusActive}).Error)

	cfg := &config.Config{AppEnv: "test", JWTSecret: "p9-postgres-integration-jwt-secret-with-safe-length", JWTExpHrs: 1}
	keys, err := auth.BuildKeySet(cfg)
	require.NoError(t, err)
	token, _, err := auth.MintAccessToken(cfg, keys, auth.MintAccessInput{UserID: actorID, Username: username, TenantID: tenantID, TokenVersion: 1})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestID())
	api := router.Group("/api/v1")
	api.Use(middleware.BearerAuthWithDB(cfg, db, nil))
	Register(api, &Handler{Svc: NewAPIService(db)})

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/inventory-sync/runs", nil))
	require.Equal(t, http.StatusUnauthorized, unauthorized.Code)

	payload, _ := json.Marshal(map[string]any{"shopConnectionId": store.ID, "platform": PlatformDouyin, "providerMode": ProviderModeMock, "fixtureScenario": FixtureScenarioLowConfidenceBinding})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/inventory-sync/runs", bytes.NewReader(payload))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "p9pg-auth-golden")
	created := httptest.NewRecorder()
	router.ServeHTTP(created, request)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	require.NotContains(t, created.Body.String(), "checkpoint")
	require.NotContains(t, created.Body.String(), "idempotencyKeyHash")

	var runCount, snapshotCount, manualCount, auditCount int64
	require.NoError(t, db.Model(&InventorySyncRun{}).Where("tenant_id = ?", tenantID).Count(&runCount).Error)
	require.NoError(t, db.Model(&InventorySnapshotItem{}).Where("tenant_id = ?", tenantID).Count(&snapshotCount).Error)
	require.NoError(t, db.Model(&ManualBindingRequest{}).Where("tenant_id = ?", tenantID).Count(&manualCount).Error)
	require.NoError(t, db.Model(&operationlog.OperationLog{}).Where("tenant_id = ? AND resource = ?", tenantID, inventorySyncAuditResourceRun).Count(&auditCount).Error)
	require.Equal(t, int64(1), runCount)
	require.Equal(t, int64(1), snapshotCount)
	require.Equal(t, int64(1), manualCount)
	require.GreaterOrEqual(t, auditCount, int64(3))

	var inventoryMutationCount int64
	require.NoError(t, db.Table("inventory_change_logs").Where("tenant_id = ?", tenantID).Count(&inventoryMutationCount).Error)
	require.Zero(t, inventoryMutationCount)

	provider := NewDouyinInventoryMockProvider()
	require.False(t, provider.Capabilities().unsafe())
	page, err := provider.FetchInventoryPage(context.Background(), InventoryFetchRequest{TenantID: tenantID, ShopConnectionID: store.ID.String(), Platform: PlatformDouyin, ProviderMode: ProviderModeMock, FixtureScenario: FixtureScenarioEmptyInventory})
	require.NoError(t, err)
	require.Zero(t, page.NetworkCalls)

	mutation := httptest.NewRecorder()
	mutationReq := httptest.NewRequest(http.MethodPatch, "/api/v1/inventory-sync/snapshots/"+uuid.New().String(), nil)
	mutationReq.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(mutation, mutationReq)
	require.Equal(t, http.StatusNotFound, mutation.Code)
}

func runConcurrent(count int, start chan struct{}, fn func(index int) error) []error {
	errs := make([]error, count)
	var ready sync.WaitGroup
	var done sync.WaitGroup
	ready.Add(count)
	done.Add(count)
	for index := 0; index < count; index++ {
		go func(index int) {
			defer done.Done()
			ready.Done()
			<-start
			errs[index] = fn(index)
		}(index)
	}
	ready.Wait()
	close(start)
	done.Wait()
	return errs
}

func countSuccessful(errs []error) int {
	count := 0
	for _, err := range errs {
		if err == nil {
			count++
		}
	}
	return count
}
