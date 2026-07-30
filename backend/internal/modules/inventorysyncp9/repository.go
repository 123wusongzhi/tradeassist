package inventorysyncp9

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InventorySyncRunRepository struct {
	DB *gorm.DB
}

func NewInventorySyncRunRepository(db *gorm.DB) *InventorySyncRunRepository {
	return &InventorySyncRunRepository{DB: db}
}

func (r *InventorySyncRunRepository) Create(ctx context.Context, run *InventorySyncRun) (*InventorySyncRun, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("inventory sync run repository: db is nil")
	}
	if err := validateInventorySyncRun(run); err != nil {
		return nil, err
	}
	if err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := verifyShopConnection(ctx, tx, run.TenantID, run.ShopConnectionID, run.Platform); err != nil {
			return err
		}
		if existing, err := getRunByIdempotency(ctx, tx, run.TenantID, run.IdempotencyKeyHash); err == nil {
			if existing.InputFingerprint != run.InputFingerprint {
				return ErrIdempotencyPayloadConflict
			}
			*run = *existing
			return nil
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(run)
		if result.Error != nil {
			return stableError(result.Error, ErrStateConflict)
		}
		if result.RowsAffected == 0 {
			existing, err := getRunByIdempotency(ctx, tx, run.TenantID, run.IdempotencyKeyHash)
			if err != nil {
				return stableError(err, ErrStateConflict)
			}
			if existing.InputFingerprint != run.InputFingerprint {
				return ErrIdempotencyPayloadConflict
			}
			*run = *existing
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return run, nil
}

func (r *InventorySyncRunRepository) GetByID(ctx context.Context, tenantID int64, id uuid.UUID) (*InventorySyncRun, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("inventory sync run repository: db is nil")
	}
	if err := validateTenantID(tenantID); err != nil || id == zeroUUID {
		return nil, ErrValidation
	}
	var run InventorySyncRun
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&run).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrStateConflict)
	}
	return &run, nil
}

func (r *InventorySyncRunRepository) GetByIdempotency(ctx context.Context, tenantID int64, keyHash string) (*InventorySyncRun, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("inventory sync run repository: db is nil")
	}
	if err := validateTenantID(tenantID); err != nil {
		return nil, err
	}
	return getRunByIdempotency(ctx, r.DB, tenantID, keyHash)
}

func (r *InventorySyncRunRepository) UpdateStatusWithRevision(ctx context.Context, tenantID int64, id uuid.UUID, expectedRevision int, status string, patch InventorySyncRunStatusPatch) (*InventorySyncRun, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("inventory sync run repository: db is nil")
	}
	status = normalizeLower(status)
	if validateTenantID(tenantID) != nil || id == zeroUUID || expectedRevision < 1 || !allowedRunStatuses[status] {
		return nil, ErrValidation
	}
	return updateRunStatusWithDB(ctx, r.DB, tenantID, id, expectedRevision, status, patch)
}

type InventorySyncRunStatusPatch struct {
	SnapshotCount      *int
	CalibrationCount   *int
	ManualRequestCount *int
	SafeErrorMetadata  datatypes.JSON
	Cursor             datatypes.JSON
	Checkpoint         datatypes.JSON
	StartedAt          *time.Time
	FinishedAt         *time.Time
}

func getRunByIdempotency(ctx context.Context, db *gorm.DB, tenantID int64, keyHash string) (*InventorySyncRun, error) {
	keyHash = normalizeString(keyHash)
	if keyHash == "" {
		return nil, ErrNotFound
	}
	if err := validateHashField(keyHash, true); err != nil {
		return nil, err
	}
	var run InventorySyncRun
	if err := db.WithContext(ctx).Where("tenant_id = ? AND idempotency_key_hash = ?", tenantID, keyHash).First(&run).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrStateConflict)
	}
	return &run, nil
}

func updateRunStatusWithDB(ctx context.Context, db *gorm.DB, tenantID int64, id uuid.UUID, expectedRevision int, status string, patch InventorySyncRunStatusPatch) (*InventorySyncRun, error) {
	var current InventorySyncRun
	if err := db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&current).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrStateConflict)
	}
	if current.Revision != expectedRevision {
		return nil, ErrRevisionConflict
	}
	if isTerminalRunStatus(current.Status) && status == InventorySyncRunStatusRunning {
		return nil, ErrStateConflict
	}
	updates := map[string]any{
		"status":   status,
		"revision": gorm.Expr("revision + 1"),
	}
	if patch.SnapshotCount != nil {
		if *patch.SnapshotCount < 0 {
			return nil, ErrValidation
		}
		updates["snapshot_count"] = *patch.SnapshotCount
	}
	if patch.CalibrationCount != nil {
		if *patch.CalibrationCount < 0 {
			return nil, ErrValidation
		}
		updates["calibration_count"] = *patch.CalibrationCount
	}
	if patch.ManualRequestCount != nil {
		if *patch.ManualRequestCount < 0 {
			return nil, ErrValidation
		}
		updates["manual_request_count"] = *patch.ManualRequestCount
	}
	if len(patch.SafeErrorMetadata) > 0 {
		jsonValue, err := normalizeModelJSON(patch.SafeErrorMetadata, maxSafeJSONBytes)
		if err != nil {
			return nil, err
		}
		updates["safe_error_metadata"] = jsonValue
	}
	if len(patch.Cursor) > 0 {
		jsonValue, err := normalizeModelJSON(patch.Cursor, maxSafeJSONBytes)
		if err != nil {
			return nil, err
		}
		updates["cursor"] = jsonValue
	}
	if len(patch.Checkpoint) > 0 {
		jsonValue, err := normalizeModelJSON(patch.Checkpoint, maxSafeJSONBytes)
		if err != nil {
			return nil, err
		}
		updates["checkpoint"] = jsonValue
	}
	startedAt := current.StartedAt
	if patch.StartedAt != nil {
		startedAt = patch.StartedAt
		updates["started_at"] = patch.StartedAt
	}
	finishedAt := current.FinishedAt
	if patch.FinishedAt != nil {
		finishedAt = patch.FinishedAt
		updates["finished_at"] = patch.FinishedAt
	}
	if startedAt != nil && finishedAt != nil && finishedAt.Before(*startedAt) {
		return nil, ErrValidation
	}
	res := db.WithContext(ctx).Model(&InventorySyncRun{}).
		Where("tenant_id = ? AND id = ? AND revision = ?", tenantID, id, expectedRevision).
		Updates(updates)
	if res.Error != nil {
		return nil, stableError(res.Error, ErrStateConflict)
	}
	if res.RowsAffected == 0 {
		return nil, ErrRevisionConflict
	}
	var updated InventorySyncRun
	if err := db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&updated).Error; err != nil {
		return nil, stableError(err, ErrStateConflict)
	}
	return &updated, nil
}

func isTerminalRunStatus(status string) bool {
	return status == InventorySyncRunStatusSucceeded || status == InventorySyncRunStatusFailed || status == InventorySyncRunStatusCancelled
}

type InventorySnapshotRepository struct {
	DB *gorm.DB
}

func NewInventorySnapshotRepository(db *gorm.DB) *InventorySnapshotRepository {
	return &InventorySnapshotRepository{DB: db}
}

func (r *InventorySnapshotRepository) CreateBatch(ctx context.Context, tenantID int64, items []InventorySnapshotItem) error {
	if r == nil || r.DB == nil {
		return fmt.Errorf("inventory snapshot repository: db is nil")
	}
	if validateTenantID(tenantID) != nil || len(items) == 0 {
		return ErrValidation
	}
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		seen := map[string]bool{}
		for idx := range items {
			items[idx].TenantID = tenantID
			if err := validateInventorySnapshotItem(&items[idx]); err != nil {
				return err
			}
			uniqueKey := items[idx].InventorySyncRunID.String() + "\x00" + items[idx].ExternalSKUID
			if seen[uniqueKey] {
				return ErrDuplicateExternalSKU
			}
			seen[uniqueKey] = true
			if err := verifyRun(ctx, tx, tenantID, items[idx].InventorySyncRunID, items[idx].ShopConnectionID, items[idx].Platform); err != nil {
				return err
			}
		}
		if err := tx.Create(&items).Error; err != nil {
			if isUniqueViolation(err) {
				return stableError(err, ErrDuplicateExternalSKU)
			}
			return stableError(err, ErrStateConflict)
		}
		return nil
	})
}

func (r *InventorySnapshotRepository) ListByRun(ctx context.Context, tenantID int64, runID uuid.UUID) ([]InventorySnapshotItem, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("inventory snapshot repository: db is nil")
	}
	if validateTenantID(tenantID) != nil || runID == zeroUUID {
		return nil, ErrValidation
	}
	var items []InventorySnapshotItem
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND inventory_sync_run_id = ?", tenantID, runID).Order("created_at ASC, id ASC").Find(&items).Error; err != nil {
		return nil, stableError(err, ErrStateConflict)
	}
	return items, nil
}

func (r *InventorySnapshotRepository) GetByRunAndExternalSKU(ctx context.Context, tenantID int64, runID uuid.UUID, externalSKUID string) (*InventorySnapshotItem, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("inventory snapshot repository: db is nil")
	}
	externalSKUID = normalizeString(externalSKUID)
	if validateTenantID(tenantID) != nil || runID == zeroUUID || externalSKUID == "" {
		return nil, ErrValidation
	}
	var item InventorySnapshotItem
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND inventory_sync_run_id = ? AND external_sku_id = ?", tenantID, runID, externalSKUID).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrStateConflict)
	}
	return &item, nil
}

func (r *InventorySnapshotRepository) CountByRun(ctx context.Context, tenantID int64, runID uuid.UUID) (int64, error) {
	if r == nil || r.DB == nil {
		return 0, fmt.Errorf("inventory snapshot repository: db is nil")
	}
	if validateTenantID(tenantID) != nil || runID == zeroUUID {
		return 0, ErrValidation
	}
	var count int64
	if err := r.DB.WithContext(ctx).Model(&InventorySnapshotItem{}).Where("tenant_id = ? AND inventory_sync_run_id = ?", tenantID, runID).Count(&count).Error; err != nil {
		return 0, stableError(err, ErrStateConflict)
	}
	return count, nil
}

type SKUBindingRepository struct {
	DB *gorm.DB
}

func NewSKUBindingRepository(db *gorm.DB) *SKUBindingRepository {
	return &SKUBindingRepository{DB: db}
}

func (r *SKUBindingRepository) CreateProposed(ctx context.Context, binding *SKUBinding) (*SKUBinding, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("sku binding repository: db is nil")
	}
	if binding != nil && binding.BindingStatus == "" {
		binding.BindingStatus = SKUBindingStatusProposed
	}
	if err := validateSKUBinding(binding); err != nil {
		return nil, err
	}
	if binding.BindingStatus != SKUBindingStatusProposed && binding.BindingStatus != SKUBindingStatusConfirmed {
		return nil, ErrValidation
	}
	if err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := verifyShopConnection(ctx, tx, binding.TenantID, binding.ShopConnectionID, binding.Platform); err != nil {
			return err
		}
		if err := verifyLocalSKU(ctx, tx, binding.TenantID, binding.LocalProductID, binding.LocalSKUID); err != nil {
			return err
		}
		if err := tx.Create(binding).Error; err != nil {
			if isUniqueViolation(err) {
				return stableError(err, ErrBindingConflict)
			}
			return stableError(err, ErrStateConflict)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return binding, nil
}

func (r *SKUBindingRepository) GetCurrentConfirmed(ctx context.Context, tenantID int64, shopConnectionID uuid.UUID, externalSKUID string) (*SKUBinding, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("sku binding repository: db is nil")
	}
	externalSKUID = normalizeString(externalSKUID)
	if validateTenantID(tenantID) != nil || shopConnectionID == zeroUUID || externalSKUID == "" {
		return nil, ErrValidation
	}
	var binding SKUBinding
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND shop_connection_id = ? AND external_sku_id = ? AND binding_status = ?", tenantID, shopConnectionID, externalSKUID, SKUBindingStatusConfirmed).First(&binding).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrStateConflict)
	}
	return &binding, nil
}

func (r *SKUBindingRepository) ListByExternalSKU(ctx context.Context, tenantID int64, shopConnectionID uuid.UUID, externalSKUID string) ([]SKUBinding, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("sku binding repository: db is nil")
	}
	externalSKUID = normalizeString(externalSKUID)
	if validateTenantID(tenantID) != nil || shopConnectionID == zeroUUID || externalSKUID == "" {
		return nil, ErrValidation
	}
	var bindings []SKUBinding
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND shop_connection_id = ? AND external_sku_id = ?", tenantID, shopConnectionID, externalSKUID).Order("created_at DESC, id DESC").Find(&bindings).Error; err != nil {
		return nil, stableError(err, ErrStateConflict)
	}
	return bindings, nil
}

func (r *SKUBindingRepository) ListByLocalSKU(ctx context.Context, tenantID int64, localSKUID uuid.UUID) ([]SKUBinding, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("sku binding repository: db is nil")
	}
	if validateTenantID(tenantID) != nil || localSKUID == zeroUUID {
		return nil, ErrValidation
	}
	var bindings []SKUBinding
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND local_sku_id = ?", tenantID, localSKUID).Order("created_at DESC, id DESC").Find(&bindings).Error; err != nil {
		return nil, stableError(err, ErrStateConflict)
	}
	return bindings, nil
}

type SKUBindingTransitionPatch struct {
	Status             string
	ExpectedRevision   int
	ConfirmedBy        *uuid.UUID
	CalibrationReason  string
	CalibrationVersion *int
	Confidence         *int
}

func (r *SKUBindingRepository) TransitionWithRevision(ctx context.Context, tenantID int64, id uuid.UUID, patch SKUBindingTransitionPatch) (*SKUBinding, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("sku binding repository: db is nil")
	}
	patch.Status = normalizeLower(patch.Status)
	if validateTenantID(tenantID) != nil || id == zeroUUID || patch.ExpectedRevision < 1 || !allowedBindingStatuses[patch.Status] {
		return nil, ErrValidation
	}
	var updated SKUBinding
	if err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current SKUBinding
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, id).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return stableError(err, ErrStateConflict)
		}
		if current.Revision != patch.ExpectedRevision {
			return ErrRevisionConflict
		}
		if !bindingTransitionAllowed(current.BindingStatus, patch.Status) {
			return ErrStateConflict
		}
		updates := map[string]any{
			"binding_status": patch.Status,
			"revision":       gorm.Expr("revision + 1"),
		}
		if patch.Status == SKUBindingStatusConfirmed {
			now := utcNow()
			updates["confirmed_at"] = &now
			updates["confirmed_by"] = patch.ConfirmedBy
		}
		if patch.CalibrationReason != "" {
			updates["calibration_reason"] = strings.TrimSpace(patch.CalibrationReason)
		}
		if patch.CalibrationVersion != nil {
			if *patch.CalibrationVersion < 1 {
				return ErrValidation
			}
			updates["calibration_version"] = *patch.CalibrationVersion
		}
		if patch.Confidence != nil {
			if *patch.Confidence < 0 || *patch.Confidence > 10000 {
				return ErrValidation
			}
			updates["confidence"] = *patch.Confidence
		}
		res := tx.Model(&SKUBinding{}).Where("tenant_id = ? AND id = ? AND revision = ?", tenantID, id, patch.ExpectedRevision).Updates(updates)
		if res.Error != nil {
			if isUniqueViolation(res.Error) {
				return stableError(res.Error, ErrBindingConflict)
			}
			return stableError(res.Error, ErrStateConflict)
		}
		if res.RowsAffected == 0 {
			return ErrRevisionConflict
		}
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, id).First(&updated).Error; err != nil {
			return stableError(err, ErrStateConflict)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &updated, nil
}

type SKUBindingCalibrationRepository struct {
	DB *gorm.DB
}

func NewSKUBindingCalibrationRepository(db *gorm.DB) *SKUBindingCalibrationRepository {
	return &SKUBindingCalibrationRepository{DB: db}
}

func (r *SKUBindingCalibrationRepository) CreateBatch(ctx context.Context, tenantID int64, calibrations []SKUBindingCalibration) error {
	if r == nil || r.DB == nil {
		return fmt.Errorf("sku binding calibration repository: db is nil")
	}
	if validateTenantID(tenantID) != nil || len(calibrations) == 0 {
		return ErrValidation
	}
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		seen := map[string]bool{}
		for idx := range calibrations {
			calibrations[idx].TenantID = tenantID
			if err := validateSKUBindingCalibration(&calibrations[idx]); err != nil {
				return err
			}
			key := calibrations[idx].InventorySyncRunID.String() + ":" + calibrations[idx].InventorySnapshotItemID.String() + ":" + calibrations[idx].CandidateLocalSKUID.String() + ":" + fmt.Sprint(calibrations[idx].CalibrationVersion)
			if seen[key] {
				return ErrStateConflict
			}
			seen[key] = true
			if err := verifySnapshot(ctx, tx, tenantID, calibrations[idx].InventorySyncRunID, calibrations[idx].InventorySnapshotItemID, calibrations[idx].ExternalSKUID); err != nil {
				return err
			}
			if err := verifyLocalSKU(ctx, tx, tenantID, calibrations[idx].CandidateLocalProductID, calibrations[idx].CandidateLocalSKUID); err != nil {
				return err
			}
		}
		if err := tx.Create(&calibrations).Error; err != nil {
			if isUniqueViolation(err) {
				return stableError(err, ErrStateConflict)
			}
			return stableError(err, ErrStateConflict)
		}
		return nil
	})
}

func (r *SKUBindingCalibrationRepository) ListBySnapshot(ctx context.Context, tenantID int64, snapshotID uuid.UUID) ([]SKUBindingCalibration, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("sku binding calibration repository: db is nil")
	}
	if validateTenantID(tenantID) != nil || snapshotID == zeroUUID {
		return nil, ErrValidation
	}
	var rows []SKUBindingCalibration
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND inventory_snapshot_item_id = ?", tenantID, snapshotID).Order("confidence DESC, created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, stableError(err, ErrStateConflict)
	}
	return rows, nil
}

func (r *SKUBindingCalibrationRepository) ListByRun(ctx context.Context, tenantID int64, runID uuid.UUID) ([]SKUBindingCalibration, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("sku binding calibration repository: db is nil")
	}
	if validateTenantID(tenantID) != nil || runID == zeroUUID {
		return nil, ErrValidation
	}
	var rows []SKUBindingCalibration
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND inventory_sync_run_id = ?", tenantID, runID).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, stableError(err, ErrStateConflict)
	}
	return rows, nil
}

func (r *SKUBindingCalibrationRepository) GetBestCandidate(ctx context.Context, tenantID int64, snapshotID uuid.UUID) (*SKUBindingCalibration, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("sku binding calibration repository: db is nil")
	}
	if validateTenantID(tenantID) != nil || snapshotID == zeroUUID {
		return nil, ErrValidation
	}
	var row SKUBindingCalibration
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND inventory_snapshot_item_id = ?", tenantID, snapshotID).Order("confidence DESC, created_at ASC, id ASC").First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrStateConflict)
	}
	return &row, nil
}

type ManualBindingRequestRepository struct {
	DB *gorm.DB
}

func NewManualBindingRequestRepository(db *gorm.DB) *ManualBindingRequestRepository {
	return &ManualBindingRequestRepository{DB: db}
}

func (r *ManualBindingRequestRepository) Create(ctx context.Context, request *ManualBindingRequest) (*ManualBindingRequest, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("manual binding request repository: db is nil")
	}
	if request != nil && request.Status == "" {
		request.Status = ManualBindingStatusPending
	}
	if err := validateManualBindingRequest(request); err != nil {
		return nil, err
	}
	if err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if existing, err := getManualRequestByIdempotency(ctx, tx, request.TenantID, request.IdempotencyKeyHash); err == nil {
			if existing.InputFingerprint != request.InputFingerprint {
				return ErrIdempotencyPayloadConflict
			}
			*request = *existing
			return nil
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
		if err := verifyRun(ctx, tx, request.TenantID, request.InventorySyncRunID, request.ShopConnectionID, ""); err != nil {
			return err
		}
		if err := verifySnapshot(ctx, tx, request.TenantID, request.InventorySyncRunID, request.InventorySnapshotItemID, request.ExternalSKUID); err != nil {
			return err
		}
		if request.SuggestedLocalSKUID != nil {
			if _, err := localSKUProductID(ctx, tx, request.TenantID, *request.SuggestedLocalSKUID); err != nil {
				return err
			}
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(request)
		if result.Error != nil {
			return stableError(result.Error, ErrStateConflict)
		}
		if result.RowsAffected == 0 {
			existing, lookupErr := getManualRequestByIdempotency(ctx, tx, request.TenantID, request.IdempotencyKeyHash)
			if lookupErr == nil {
				if existing.InputFingerprint != request.InputFingerprint {
					return ErrIdempotencyPayloadConflict
				}
				*request = *existing
				return nil
			}
			if !errors.Is(lookupErr, ErrNotFound) {
				return lookupErr
			}
			return ErrManualBindingAlreadyPending
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return request, nil
}

func (r *ManualBindingRequestRepository) GetByID(ctx context.Context, tenantID int64, id uuid.UUID) (*ManualBindingRequest, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("manual binding request repository: db is nil")
	}
	if validateTenantID(tenantID) != nil || id == zeroUUID {
		return nil, ErrValidation
	}
	var request ManualBindingRequest
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrStateConflict)
	}
	return &request, nil
}

func (r *ManualBindingRequestRepository) GetPendingByExternalSKU(ctx context.Context, tenantID int64, shopConnectionID uuid.UUID, externalSKUID string) (*ManualBindingRequest, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("manual binding request repository: db is nil")
	}
	externalSKUID = normalizeString(externalSKUID)
	if validateTenantID(tenantID) != nil || shopConnectionID == zeroUUID || externalSKUID == "" {
		return nil, ErrValidation
	}
	var request ManualBindingRequest
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND shop_connection_id = ? AND external_sku_id = ? AND status = ?", tenantID, shopConnectionID, externalSKUID, ManualBindingStatusPending).First(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrStateConflict)
	}
	return &request, nil
}

func (r *ManualBindingRequestRepository) ListPending(ctx context.Context, tenantID int64, limit int) ([]ManualBindingRequest, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("manual binding request repository: db is nil")
	}
	if validateTenantID(tenantID) != nil {
		return nil, ErrValidation
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []ManualBindingRequest
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND status = ?", tenantID, ManualBindingStatusPending).Order("created_at ASC, id ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, stableError(err, ErrStateConflict)
	}
	return rows, nil
}

type ManualBindingResolutionPatch struct {
	Status                 string
	ExpectedRevision       int
	ResolvedBy             uuid.UUID
	Resolution             string
	Comment                string
	SelectedLocalProductID *uuid.UUID
	SelectedLocalSKUID     *uuid.UUID
}

func (r *ManualBindingRequestRepository) ResolveWithRevision(ctx context.Context, tenantID int64, id uuid.UUID, patch ManualBindingResolutionPatch) (*ManualBindingRequest, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("manual binding request repository: db is nil")
	}
	patch.Status = normalizeLower(patch.Status)
	if validateTenantID(tenantID) != nil || id == zeroUUID || patch.ExpectedRevision < 1 || patch.ResolvedBy == zeroUUID || !manualBindingResolutionStatus(patch.Status) {
		return nil, ErrValidation
	}
	var updated ManualBindingRequest
	if err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current ManualBindingRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, id).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return stableError(err, ErrStateConflict)
		}
		if current.Revision != patch.ExpectedRevision {
			return ErrRevisionConflict
		}
		if current.Status != ManualBindingStatusPending {
			return ErrManualBindingAlreadyResolved
		}
		updates := map[string]any{
			"status":      patch.Status,
			"resolved_by": patch.ResolvedBy,
			"resolved_at": utcNow(),
			"resolution":  strings.TrimSpace(patch.Resolution),
			"comment":     strings.TrimSpace(patch.Comment),
			"revision":    gorm.Expr("revision + 1"),
		}
		if patch.Status == ManualBindingStatusConfirmed {
			if patch.SelectedLocalProductID == nil || patch.SelectedLocalSKUID == nil {
				return ErrValidation
			}
			if err := verifyLocalSKU(ctx, tx, tenantID, *patch.SelectedLocalProductID, *patch.SelectedLocalSKUID); err != nil {
				return err
			}
			updates["selected_local_product_id"] = *patch.SelectedLocalProductID
			updates["selected_local_sku_id"] = *patch.SelectedLocalSKUID
		}
		res := tx.Model(&ManualBindingRequest{}).Where("tenant_id = ? AND id = ? AND revision = ? AND status = ?", tenantID, id, patch.ExpectedRevision, ManualBindingStatusPending).Updates(updates)
		if res.Error != nil {
			return stableError(res.Error, ErrStateConflict)
		}
		if res.RowsAffected == 0 {
			return ErrRevisionConflict
		}
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, id).First(&updated).Error; err != nil {
			return stableError(err, ErrStateConflict)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &updated, nil
}

func getManualRequestByIdempotency(ctx context.Context, db *gorm.DB, tenantID int64, keyHash string) (*ManualBindingRequest, error) {
	keyHash = normalizeString(keyHash)
	if keyHash == "" {
		return nil, ErrNotFound
	}
	if err := validateHashField(keyHash, true); err != nil {
		return nil, err
	}
	var request ManualBindingRequest
	if err := db.WithContext(ctx).Where("tenant_id = ? AND idempotency_key_hash = ?", tenantID, keyHash).First(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrStateConflict)
	}
	return &request, nil
}

func verifyRun(ctx context.Context, db *gorm.DB, tenantID int64, runID uuid.UUID, shopConnectionID uuid.UUID, platform string) error {
	var run InventorySyncRun
	if err := db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, runID).First(&run).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return stableError(err, ErrStateConflict)
	}
	if shopConnectionID != zeroUUID && run.ShopConnectionID != shopConnectionID {
		return ErrTenantMismatch
	}
	if platform != "" && !strings.EqualFold(run.Platform, platform) {
		return ErrTenantMismatch
	}
	return nil
}

func verifySnapshot(ctx context.Context, db *gorm.DB, tenantID int64, runID uuid.UUID, snapshotID uuid.UUID, externalSKUID string) error {
	var snapshot InventorySnapshotItem
	if err := db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, snapshotID).First(&snapshot).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return stableError(err, ErrStateConflict)
	}
	if snapshot.InventorySyncRunID != runID || snapshot.ExternalSKUID != normalizeString(externalSKUID) {
		return ErrTenantMismatch
	}
	return nil
}

func verifyShopConnection(ctx context.Context, db *gorm.DB, tenantID int64, shopConnectionID uuid.UUID, platform string) error {
	var row shop.Shop
	if err := db.WithContext(ctx).Select("id", "tenant_id", "platform").Where("tenant_id = ? AND id = ?", tenantID, shopConnectionID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return stableError(err, ErrStateConflict)
	}
	if platform != "" && !strings.EqualFold(row.Platform, platform) {
		return ErrTenantMismatch
	}
	return nil
}

func verifyLocalSKU(ctx context.Context, db *gorm.DB, tenantID int64, productID uuid.UUID, skuID uuid.UUID) error {
	skuProductID, err := localSKUProductID(ctx, db, tenantID, skuID)
	if err != nil {
		return err
	}
	if productID != skuProductID {
		return ErrTenantMismatch
	}
	return nil
}

func localSKUProductID(ctx context.Context, db *gorm.DB, tenantID int64, skuID uuid.UUID) (uuid.UUID, error) {
	var sku product.ProductSKU
	if err := db.WithContext(ctx).Select("id", "product_id").Where("id = ?", skuID).First(&sku).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return zeroUUID, ErrNotFound
		}
		return zeroUUID, stableError(err, ErrStateConflict)
	}
	var row product.Product
	if err := db.WithContext(ctx).Select("id", "tenant_id").Where("tenant_id = ? AND id = ?", tenantID, sku.ProductID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return zeroUUID, ErrTenantMismatch
		}
		return zeroUUID, stableError(err, ErrStateConflict)
	}
	return sku.ProductID, nil
}
