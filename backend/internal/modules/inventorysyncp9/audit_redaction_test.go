package inventorysyncp9

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"gorm.io/datatypes"
)

func TestInventorySyncRedactionAllowlistsAndHashesMetadata(t *testing.T) {
	meta, err := safeProviderMetadataJSON(map[string]string{
		"fixtureScenario":   FixtureScenarioSuccessSinglePage,
		"externalSkuId":     "douyin-sku-safe",
		"Authorization":     "Bearer secret-token",
		"unknownDebugField": "debug-value",
	})
	require.NoError(t, err)
	raw := string(meta)
	require.Contains(t, raw, "fixtureScenario")
	require.Contains(t, raw, "externalSkuId")
	require.NotContains(t, strings.ToLower(raw), "authorization")
	require.NotContains(t, raw, "secret-token")
	require.NotContains(t, raw, "unknownDebugField")

	message, err := safeAuditMessage(map[string]any{
		"platform":      PlatformDouyin,
		"providerMode":  ProviderModeMock,
		"safeMessage":   "provider rejected Authorization: Bearer secret-token",
		"accessToken":   "secret-token",
		"internalTrace": "C:/repo/file.go:10",
	})
	require.NoError(t, err)
	require.Contains(t, message, "providerMode")
	require.NotContains(t, message, "accessToken")
	require.NotContains(t, message, "secret-token")
	require.NotContains(t, message, "internalTrace")

	cursor := datatypes.JSON([]byte(`{"cursor":"secret-token","page":1}`))
	hash := safeCursorHash(cursor)
	require.Len(t, hash, 64)
	require.NotContains(t, hash, "secret-token")
}

func TestInventorySyncAuditPermissionDeniedAndLifecycle(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	store, prod, sku := seedShopAndSKU(t, db, 901)
	registry, err := NewInventoryProviderRegistry(NewDouyinInventoryMockProvider())
	require.NoError(t, err)
	policy, err := NewCalibrationThresholdPolicy(CalibrationThresholdConfig{HighConfidenceThreshold: 9500, AutoConfirmationEnabled: false})
	require.NoError(t, err)
	service := NewSKUBindingCalibrationService(db, staticCandidateProvider{candidates: []LocalSKUCandidate{{TenantID: 901, LocalProductID: prod.ID, LocalSKUID: sku.ID, SKUCode: "SKU-901"}}}, policy)

	denied := NewInventorySyncOrchestrator(db, registry, service, testSyncAuthorizer{allowed: false})
	_, err = denied.Run(ctx, InventorySyncOrchestratorInput{TenantID: 901, ShopConnectionID: store.ID, Platform: PlatformDouyin, ProviderMode: ProviderModeMock, FixtureScenario: FixtureScenarioSuccessSinglePage, PageSize: 1, ActorID: uuid.New(), RequestID: "req-audit-denied"})
	require.ErrorIs(t, err, ErrPermissionDenied)
	var deniedLog operationlog.OperationLog
	require.NoError(t, db.Where("tenant_id = ? AND action = ? AND status = ?", 901, "inventory_sync.permission_denied", inventorySyncAuditStatusDenied).First(&deniedLog).Error)
	require.Contains(t, deniedLog.Message, ErrCodePermissionDenied)
	require.Equal(t, "req-audit-denied", deniedLog.RequestID)

	allowed := NewInventorySyncOrchestrator(db, registry, service, testSyncAuthorizer{allowed: true})
	result, err := allowed.Run(ctx, InventorySyncOrchestratorInput{TenantID: 901, ShopConnectionID: store.ID, Platform: PlatformDouyin, ProviderMode: ProviderModeMock, FixtureScenario: FixtureScenarioSuccessSinglePage, PageSize: 1, ActorID: uuid.New(), RequestID: "req-audit-success"})
	require.NoError(t, err)
	require.Equal(t, InventorySyncRunStatusSucceeded, result.Status)
	var count int64
	require.NoError(t, db.Model(&operationlog.OperationLog{}).Where("tenant_id = ? AND resource_id = ? AND action IN ?", 901, result.InventorySyncRunID.String(), []string{"inventory_sync.run_created", "inventory_sync.started", "inventory_sync.page_processed", "inventory_sync.completed"}).Count(&count).Error)
	require.Equal(t, int64(4), count)
}

func TestManualBindingAuditAndCommentRedaction(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	store, _, sku := seedShopAndSKU(t, db, 902)
	run := createRun(t, ctx, db, 902, store.ID)
	snapshot := validSnapshot(902, run, "manual-audit-sku")
	require.NoError(t, NewInventorySnapshotRepository(db).CreateBatch(ctx, 902, []InventorySnapshotItem{snapshot}))
	stored, err := NewInventorySnapshotRepository(db).GetByRunAndExternalSKU(ctx, 902, run.ID, "manual-audit-sku")
	require.NoError(t, err)
	request, err := NewManualBindingRequestRepository(db).Create(ctx, &ManualBindingRequest{TenantID: 902, InventorySyncRunID: run.ID, InventorySnapshotItemID: stored.ID, ShopConnectionID: store.ID, ExternalSKUID: stored.ExternalSKUID, Status: ManualBindingStatusPending, ReasonCode: ReasonManualReviewRequired, CandidateCount: 1, RequestID: "manual-audit-request", IdempotencyKeyHash: testHashB, InputFingerprint: testHashC, Revision: 1})
	require.NoError(t, err)

	actorID := uuid.New()
	service := NewManualBindingService(db, testAuthorizer{allowed: true})
	_, err = service.ConfirmBinding(ctx, ConfirmManualBindingInput{Actor: ManualBindingActor{TenantID: 902, ActorID: actorID}, RequestID: request.ID, ExpectedRevision: request.Revision, SelectedLocalSKUID: sku.ID, IdempotencyKeyHash: testHashA, Comment: "ok token=secret-token"})
	require.NoError(t, err)
	var decision ManualBindingDecision
	require.NoError(t, db.Where("tenant_id = ? AND manual_binding_request_id = ?", 902, request.ID).First(&decision).Error)
	require.NotContains(t, decision.Comment, "secret-token")
	var audit operationlog.OperationLog
	require.NoError(t, db.Where("tenant_id = ? AND action = ? AND resource_id = ?", 902, "sku_binding.manual_confirmed", request.ID.String()).First(&audit).Error)
	require.Contains(t, audit.Message, "bindingStatusAfter")
	require.NotContains(t, audit.Message, "secret-token")
}
