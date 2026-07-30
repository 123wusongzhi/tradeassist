package inventorysyncp9

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/pagination"
	"gorm.io/gorm"
)

type APIRepository struct {
	DB *gorm.DB
}

type snapshotRelations struct {
	Bindings       map[uuid.UUID]SKUBinding
	Calibrations   map[uuid.UUID]SKUBindingCalibration
	ManualRequests map[uuid.UUID]ManualBindingRequest
}

func NewAPIRepository(db *gorm.DB) *APIRepository {
	return &APIRepository{DB: db}
}

func (r *APIRepository) GetRun(ctx context.Context, tenantID int64, runID uuid.UUID) (*InventorySyncRun, error) {
	return NewInventorySyncRunRepository(r.DB).GetByID(ctx, tenantID, runID)
}

func (r *APIRepository) ListRuns(ctx context.Context, tenantID int64, params InventorySyncRunListParams) ([]InventorySyncRun, string, bool, error) {
	scope := pagination.Fingerprint(map[string]any{"endpoint": "p9-runs", "status": params.Status, "providerMode": params.ProviderMode})
	shopScope := ""
	tx := r.DB.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if params.ShopConnectionID != nil {
		shopScope = params.ShopConnectionID.String()
		tx = tx.Where("shop_connection_id = ?", *params.ShopConnectionID)
	}
	if params.Status != "" {
		tx = tx.Where("status = ?", params.Status)
	}
	if params.ProviderMode != "" {
		tx = tx.Where("provider_mode = ?", params.ProviderMode)
	}
	cursor, err := pagination.DecodeCursor(params.Cursor, tenantID, shopScope, scope)
	if err != nil {
		return nil, "", false, err
	}
	tx, err = pagination.ApplyDescKeyset(tx, "created_at", "id", cursor)
	if err != nil {
		return nil, "", false, err
	}
	var rows []InventorySyncRun
	if err := tx.Order("created_at DESC, id DESC").Limit(params.Limit + 1).Find(&rows).Error; err != nil {
		return nil, "", false, stableError(err, ErrStateConflict)
	}
	return trimPage(rows, params.Limit, tenantID, shopScope, scope, func(row InventorySyncRun) (time.Time, string) { return row.CreatedAt, row.ID.String() })
}

func (r *APIRepository) GetSnapshot(ctx context.Context, tenantID int64, snapshotID uuid.UUID) (*InventorySnapshotItem, error) {
	var row InventorySnapshotItem
	err := r.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, snapshotID).First(&row).Error
	return &row, apiRepositoryError(err)
}

func (r *APIRepository) ListSnapshots(ctx context.Context, tenantID int64, runID uuid.UUID, params SnapshotListParams) ([]InventorySnapshotItem, snapshotRelations, string, bool, error) {
	run, err := r.GetRun(ctx, tenantID, runID)
	if err != nil {
		return nil, snapshotRelations{}, "", false, err
	}
	scope := pagination.Fingerprint(map[string]any{"endpoint": "p9-run-snapshots", "runId": runID.String(), "bindingResult": params.BindingResult})
	tx := r.DB.WithContext(ctx).Where("tenant_id = ? AND inventory_sync_run_id = ?", tenantID, runID)
	switch params.BindingResult {
	case "":
	case "matched":
		tx = tx.Where("EXISTS (SELECT 1 FROM p9_sku_bindings b WHERE b.tenant_id = p9_inventory_snapshot_items.tenant_id AND b.shop_connection_id = p9_inventory_snapshot_items.shop_connection_id AND b.external_sku_id = p9_inventory_snapshot_items.external_sku_id AND b.binding_status = ?)", SKUBindingStatusConfirmed)
	case "manual_review":
		tx = tx.Where("EXISTS (SELECT 1 FROM p9_manual_binding_requests m WHERE m.tenant_id = p9_inventory_snapshot_items.tenant_id AND m.inventory_snapshot_item_id = p9_inventory_snapshot_items.id AND m.status = ?)", ManualBindingStatusPending)
	case "unmatched":
		tx = tx.Where("NOT EXISTS (SELECT 1 FROM p9_sku_bindings b WHERE b.tenant_id = p9_inventory_snapshot_items.tenant_id AND b.shop_connection_id = p9_inventory_snapshot_items.shop_connection_id AND b.external_sku_id = p9_inventory_snapshot_items.external_sku_id AND b.binding_status = ?) AND NOT EXISTS (SELECT 1 FROM p9_manual_binding_requests m WHERE m.tenant_id = p9_inventory_snapshot_items.tenant_id AND m.inventory_snapshot_item_id = p9_inventory_snapshot_items.id AND m.status = ?)", SKUBindingStatusConfirmed, ManualBindingStatusPending)
	default:
		return nil, snapshotRelations{}, "", false, ErrValidation
	}
	cursor, err := pagination.DecodeCursor(params.Cursor, tenantID, run.ShopConnectionID.String(), scope)
	if err != nil {
		return nil, snapshotRelations{}, "", false, err
	}
	tx, err = pagination.ApplyDescKeyset(tx, "observed_at", "id", cursor)
	if err != nil {
		return nil, snapshotRelations{}, "", false, err
	}
	var rows []InventorySnapshotItem
	if err := tx.Order("observed_at DESC, id DESC").Limit(params.Limit + 1).Find(&rows).Error; err != nil {
		return nil, snapshotRelations{}, "", false, stableError(err, ErrStateConflict)
	}
	hasMore := len(rows) > params.Limit
	if hasMore {
		rows = rows[:params.Limit]
	}
	next := ""
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		next, err = pagination.BuildNextCursor(true, tenantID, run.ShopConnectionID.String(), scope, "observed_at", last.ObservedAt, last.ID.String())
		if err != nil {
			return nil, snapshotRelations{}, "", false, err
		}
	}
	relations, err := r.LoadSnapshotRelations(ctx, tenantID, rows)
	if err != nil {
		return nil, snapshotRelations{}, "", false, err
	}
	return rows, relations, next, hasMore, nil
}

func (r *APIRepository) LoadSnapshotRelations(ctx context.Context, tenantID int64, snapshots []InventorySnapshotItem) (snapshotRelations, error) {
	out := snapshotRelations{Bindings: map[uuid.UUID]SKUBinding{}, Calibrations: map[uuid.UUID]SKUBindingCalibration{}, ManualRequests: map[uuid.UUID]ManualBindingRequest{}}
	if len(snapshots) == 0 {
		return out, nil
	}
	snapshotIDs := make([]uuid.UUID, 0, len(snapshots))
	externalIDs := make([]string, 0, len(snapshots))
	shopID := snapshots[0].ShopConnectionID
	for _, row := range snapshots {
		snapshotIDs = append(snapshotIDs, row.ID)
		externalIDs = append(externalIDs, row.ExternalSKUID)
	}
	var bindings []SKUBinding
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND shop_connection_id = ? AND external_sku_id IN ? AND binding_status = ?", tenantID, shopID, externalIDs, SKUBindingStatusConfirmed).Order("created_at DESC").Find(&bindings).Error; err != nil {
		return out, stableError(err, ErrStateConflict)
	}
	bindingByExternal := map[string]SKUBinding{}
	for _, binding := range bindings {
		if _, exists := bindingByExternal[binding.ExternalSKUID]; !exists {
			bindingByExternal[binding.ExternalSKUID] = binding
		}
	}
	var calibrations []SKUBindingCalibration
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND inventory_snapshot_item_id IN ?", tenantID, snapshotIDs).Order("calibration_version DESC, confidence DESC, created_at DESC").Find(&calibrations).Error; err != nil {
		return out, stableError(err, ErrStateConflict)
	}
	for _, calibration := range calibrations {
		if _, exists := out.Calibrations[calibration.InventorySnapshotItemID]; !exists {
			out.Calibrations[calibration.InventorySnapshotItemID] = calibration
		}
	}
	var manual []ManualBindingRequest
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND inventory_snapshot_item_id IN ?", tenantID, snapshotIDs).Order("created_at DESC").Find(&manual).Error; err != nil {
		return out, stableError(err, ErrStateConflict)
	}
	for _, request := range manual {
		if _, exists := out.ManualRequests[request.InventorySnapshotItemID]; !exists {
			out.ManualRequests[request.InventorySnapshotItemID] = request
		}
	}
	for _, snapshot := range snapshots {
		if binding, exists := bindingByExternal[snapshot.ExternalSKUID]; exists {
			out.Bindings[snapshot.ID] = binding
		}
	}
	return out, nil
}

func (r *APIRepository) GetBinding(ctx context.Context, tenantID int64, bindingID uuid.UUID) (*SKUBinding, error) {
	var row SKUBinding
	err := r.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, bindingID).First(&row).Error
	return &row, apiRepositoryError(err)
}

func (r *APIRepository) ListBindings(ctx context.Context, tenantID int64, params BindingListParams) ([]SKUBinding, string, bool, error) {
	scope := pagination.Fingerprint(map[string]any{"endpoint": "p9-bindings", "status": params.BindingStatus, "source": params.BindingSource})
	shopScope := ""
	tx := r.DB.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if params.ShopConnectionID != nil {
		shopScope = params.ShopConnectionID.String()
		tx = tx.Where("shop_connection_id = ?", *params.ShopConnectionID)
	}
	if params.BindingStatus != "" {
		tx = tx.Where("binding_status = ?", params.BindingStatus)
	}
	if params.BindingSource != "" {
		tx = tx.Where("binding_source = ?", params.BindingSource)
	}
	cursor, err := pagination.DecodeCursor(params.Cursor, tenantID, shopScope, scope)
	if err != nil {
		return nil, "", false, err
	}
	tx, err = pagination.ApplyDescKeyset(tx, "created_at", "id", cursor)
	if err != nil {
		return nil, "", false, err
	}
	var rows []SKUBinding
	if err := tx.Order("created_at DESC, id DESC").Limit(params.Limit + 1).Find(&rows).Error; err != nil {
		return nil, "", false, stableError(err, ErrStateConflict)
	}
	return trimPage(rows, params.Limit, tenantID, shopScope, scope, func(row SKUBinding) (time.Time, string) { return row.CreatedAt, row.ID.String() })
}

func (r *APIRepository) ListCalibrations(ctx context.Context, tenantID int64, snapshotID uuid.UUID, version int) ([]SKUBindingCalibration, error) {
	tx := r.DB.WithContext(ctx).Where("tenant_id = ? AND inventory_snapshot_item_id = ?", tenantID, snapshotID)
	if version > 0 {
		tx = tx.Where("calibration_version = ?", version)
	}
	var rows []SKUBindingCalibration
	if err := tx.Order("calibration_version DESC, confidence DESC, created_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, stableError(err, ErrStateConflict)
	}
	return rows, nil
}

func (r *APIRepository) ListCalibrationPage(ctx context.Context, tenantID int64, snapshotID uuid.UUID, params CalibrationListParams) ([]SKUBindingCalibration, string, bool, error) {
	scope := pagination.Fingerprint(map[string]any{"endpoint": "p9-calibrations", "snapshotId": snapshotID.String()})
	cursor, err := pagination.DecodeCursor(params.Cursor, tenantID, snapshotID.String(), scope)
	if err != nil {
		return nil, "", false, err
	}
	tx := r.DB.WithContext(ctx).Where("tenant_id = ? AND inventory_snapshot_item_id = ?", tenantID, snapshotID)
	tx, err = pagination.ApplyDescKeyset(tx, "created_at", "id", cursor)
	if err != nil {
		return nil, "", false, err
	}
	var rows []SKUBindingCalibration
	if err := tx.Order("created_at DESC, id DESC").Limit(params.Limit + 1).Find(&rows).Error; err != nil {
		return nil, "", false, stableError(err, ErrStateConflict)
	}
	return trimPage(rows, params.Limit, tenantID, snapshotID.String(), scope, func(row SKUBindingCalibration) (time.Time, string) { return row.CreatedAt, row.ID.String() })
}

func (r *APIRepository) LatestCalibrationVersion(ctx context.Context, tenantID int64, snapshotID uuid.UUID) (int, error) {
	var version int
	if err := r.DB.WithContext(ctx).Model(&SKUBindingCalibration{}).Where("tenant_id = ? AND inventory_snapshot_item_id = ?", tenantID, snapshotID).Select("COALESCE(MAX(calibration_version), 0)").Scan(&version).Error; err != nil {
		return 0, stableError(err, ErrStateConflict)
	}
	return version, nil
}

func (r *APIRepository) ListManualRequests(ctx context.Context, tenantID int64, params ManualBindingListParams) ([]ManualBindingRequest, string, bool, error) {
	scope := pagination.Fingerprint(map[string]any{"endpoint": "p9-manual-requests", "status": params.Status})
	shopScope := ""
	tx := r.DB.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if params.ShopConnectionID != nil {
		shopScope = params.ShopConnectionID.String()
		tx = tx.Where("shop_connection_id = ?", *params.ShopConnectionID)
	}
	if params.Status != "" {
		tx = tx.Where("status = ?", params.Status)
	}
	cursor, err := pagination.DecodeCursor(params.Cursor, tenantID, shopScope, scope)
	if err != nil {
		return nil, "", false, err
	}
	tx, err = pagination.ApplyDescKeyset(tx, "created_at", "id", cursor)
	if err != nil {
		return nil, "", false, err
	}
	var rows []ManualBindingRequest
	if err := tx.Order("created_at DESC, id DESC").Limit(params.Limit + 1).Find(&rows).Error; err != nil {
		return nil, "", false, stableError(err, ErrStateConflict)
	}
	return trimPage(rows, params.Limit, tenantID, shopScope, scope, func(row ManualBindingRequest) (time.Time, string) { return row.CreatedAt, row.ID.String() })
}

func (r *APIRepository) GetManualRequest(ctx context.Context, tenantID int64, requestID uuid.UUID) (*ManualBindingRequest, error) {
	return NewManualBindingRequestRepository(r.DB).GetByID(ctx, tenantID, requestID)
}

func (r *APIRepository) ListManualDecisions(ctx context.Context, tenantID int64, requestID uuid.UUID) ([]ManualBindingDecision, error) {
	var rows []ManualBindingDecision
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND manual_binding_request_id = ?", tenantID, requestID).Order("created_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, stableError(err, ErrStateConflict)
	}
	return rows, nil
}

func (r *APIRepository) BindingHistory(ctx context.Context, binding *SKUBinding) ([]SKUBindingCalibration, []ManualBindingDecision, error) {
	var calibrations []SKUBindingCalibration
	err := r.DB.WithContext(ctx).Table("p9_sku_binding_calibrations AS c").
		Select("c.*").
		Joins("JOIN p9_inventory_snapshot_items s ON s.id = c.inventory_snapshot_item_id").
		Where("c.tenant_id = ? AND s.shop_connection_id = ? AND s.external_sku_id = ?", binding.TenantID, binding.ShopConnectionID, binding.ExternalSKUID).
		Order("c.calibration_version DESC, c.confidence DESC, c.created_at DESC").
		Scan(&calibrations).Error
	if err != nil {
		return nil, nil, stableError(err, ErrStateConflict)
	}
	var decisions []ManualBindingDecision
	err = r.DB.WithContext(ctx).Table("p9_manual_binding_decisions AS d").
		Select("d.*").
		Joins("JOIN p9_manual_binding_requests m ON m.id = d.manual_binding_request_id").
		Where("d.tenant_id = ? AND m.shop_connection_id = ? AND m.external_sku_id = ?", binding.TenantID, binding.ShopConnectionID, binding.ExternalSKUID).
		Order("d.created_at DESC, d.id DESC").
		Scan(&decisions).Error
	if err != nil {
		return nil, nil, stableError(err, ErrStateConflict)
	}
	return calibrations, decisions, nil
}

func (r *APIRepository) ListRunAuditEvents(ctx context.Context, tenantID int64, runID uuid.UUID, params AuditEventListParams) ([]operationlog.OperationLog, string, bool, error) {
	scope := pagination.Fingerprint(map[string]any{"endpoint": "p9-run-audit", "runId": runID.String()})
	cursor, err := pagination.DecodeCursor(params.Cursor, tenantID, "", scope)
	if err != nil {
		return nil, "", false, err
	}
	tx := r.DB.WithContext(ctx).Where("tenant_id = ? AND resource = ? AND resource_id = ?", tenantID, inventorySyncAuditResourceRun, runID.String())
	tx, err = pagination.ApplyDescKeyset(tx, "created_at", "id", cursor)
	if err != nil {
		return nil, "", false, err
	}
	var rows []operationlog.OperationLog
	if err := tx.Order("created_at DESC, id DESC").Limit(params.Limit + 1).Find(&rows).Error; err != nil {
		return nil, "", false, stableError(err, ErrStateConflict)
	}
	return trimPage(rows, params.Limit, tenantID, "", scope, func(row operationlog.OperationLog) (time.Time, string) { return row.CreatedAt, row.ID.String() })
}

func trimPage[T any](rows []T, limit int, tenantID int64, shopID, scope string, key func(T) (time.Time, string)) ([]T, string, bool, error) {
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	if !hasMore || len(rows) == 0 {
		return rows, "", hasMore, nil
	}
	sortValue, tieID := key(rows[len(rows)-1])
	next, err := pagination.BuildNextCursor(true, tenantID, shopID, scope, "created_at", sortValue, tieID)
	return rows, next, hasMore, err
}

func apiRepositoryError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if strings.TrimSpace(err.Error()) == "" {
		return ErrStateConflict
	}
	return stableError(err, ErrStateConflict)
}
