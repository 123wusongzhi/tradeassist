package inventorysyncp9

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	InventorySyncTriggerManual       = "manual"
	InventorySyncTriggerManualRerun  = "manual_rerun"
	InventorySyncStatisticsInvariant = "total_equals_matched_plus_unmatched_plus_conflict_plus_failed"
	InventorySyncInvalidItemPolicy   = "page_fails_and_cursor_does_not_advance"
)

type InventorySyncAuthorizer interface {
	CanRunInventorySync(ctx context.Context, tenantID int64, actorID uuid.UUID, shopConnectionID uuid.UUID) error
	CanRerunInventorySync(ctx context.Context, tenantID int64, actorID uuid.UUID, sourceRunID uuid.UUID) error
}

type InventorySyncOrchestrator struct {
	DB                 *gorm.DB
	Registry           *InventoryProviderRegistry
	CalibrationService *SKUBindingCalibrationService
	Authorizer         InventorySyncAuthorizer
	Audit              *InventorySyncAuditService
	locks              *inventorySyncLockRegistry
	Now                func() time.Time
}

type InventorySyncOrchestratorInput struct {
	TenantID           int64
	ShopConnectionID   uuid.UUID
	Platform           string
	ProviderMode       string
	FixtureScenario    string
	TriggerType        string
	PageSize           int
	MaxPagesPerRun     int
	MaxItemsPerPage    int
	MaxItemsPerRun     int
	ActorID            uuid.UUID
	RequestID          string
	IdempotencyKeyHash string
	SourceRunID        uuid.UUID
	SourceRunRevision  int
}

type InventorySyncOrchestratorResult struct {
	InventorySyncRunID        uuid.UUID      `json:"inventorySyncRunId"`
	Status                    string         `json:"status"`
	TotalRecordCount          int            `json:"totalRecordCount"`
	MatchedRecordCount        int            `json:"matchedRecordCount"`
	UnmatchedRecordCount      int            `json:"unmatchedRecordCount"`
	ConflictRecordCount       int            `json:"conflictRecordCount"`
	FailedRecordCount         int            `json:"failedRecordCount"`
	CursorAfter               datatypes.JSON `json:"cursorAfter"`
	StartedAt                 *time.Time     `json:"startedAt,omitempty"`
	FinishedAt                *time.Time     `json:"finishedAt,omitempty"`
	ManualBindingRequestCount int            `json:"manualBindingRequestCount"`
	ConfirmedBindingCount     int            `json:"confirmedBindingCount"`
	SafeErrorSummary          datatypes.JSON `json:"safeErrorSummary"`
}

type inventorySyncCheckpoint struct {
	FixtureScenario           string                        `json:"fixtureScenario"`
	TriggerType               string                        `json:"triggerType"`
	StatisticsInvariant       string                        `json:"statisticsInvariant"`
	InvalidItemPolicy         string                        `json:"invalidItemPolicy"`
	TotalRecordCount          int                           `json:"totalRecordCount"`
	MatchedRecordCount        int                           `json:"matchedRecordCount"`
	UnmatchedRecordCount      int                           `json:"unmatchedRecordCount"`
	ConflictRecordCount       int                           `json:"conflictRecordCount"`
	FailedRecordCount         int                           `json:"failedRecordCount"`
	ManualBindingRequestCount int                           `json:"manualBindingRequestCount"`
	ConfirmedBindingCount     int                           `json:"confirmedBindingCount"`
	PagesProcessed            int                           `json:"pagesProcessed"`
	ProviderNetworkCalls      int                           `json:"providerNetworkCalls"`
	RerunOfRunID              string                        `json:"rerunOfRunId,omitempty"`
	BindingResults            []BindingResolutionItemResult `json:"bindingResults,omitempty"`
}

type inventorySyncLockRegistry struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func NewInventorySyncOrchestrator(db *gorm.DB, registry *InventoryProviderRegistry, calibrationService *SKUBindingCalibrationService, authorizer InventorySyncAuthorizer) *InventorySyncOrchestrator {
	return &InventorySyncOrchestrator{DB: db, Registry: registry, CalibrationService: calibrationService, Authorizer: authorizer, Audit: NewInventorySyncAuditService(db), locks: &inventorySyncLockRegistry{locks: map[string]*sync.Mutex{}}, Now: utcNow}
}

func (o *InventorySyncOrchestrator) Run(ctx context.Context, input InventorySyncOrchestratorInput) (*InventorySyncOrchestratorResult, error) {
	if o == nil || o.DB == nil || o.Registry == nil || o.CalibrationService == nil {
		return nil, fmt.Errorf("inventory sync orchestrator: dependencies are nil")
	}
	if err := normalizeOrchestratorInput(&input); err != nil {
		return nil, err
	}
	if err := o.authorizeRun(ctx, input); err != nil {
		if auditErr := o.auditPermissionDenied(ctx, input); auditErr != nil {
			return nil, auditErr
		}
		return nil, err
	}
	provider, err := o.Registry.Resolve(input.Platform, input.ProviderMode)
	if err != nil {
		if errors.Is(err, ErrProductionCapabilityForbidden) || errors.Is(err, ErrProviderCapabilityForbidden) {
			if auditErr := o.auditProductionCapabilityBlocked(ctx, input, err); auditErr != nil {
				return nil, auditErr
			}
		}
		return nil, err
	}
	unlock := o.locks.acquire(input.TenantID, input.ShopConnectionID, input.Platform, input.ProviderMode)
	defer unlock()
	fingerprint := inventorySyncInputFingerprint(input)
	run := &InventorySyncRun{
		TenantID:            input.TenantID,
		ShopConnectionID:    input.ShopConnectionID,
		Platform:            input.Platform,
		ProviderMode:        input.ProviderMode,
		Status:              InventorySyncRunStatusPending,
		Cursor:              datatypes.JSON([]byte(`{}`)),
		Checkpoint:          datatypes.JSON([]byte(`{}`)),
		SafeErrorMetadata:   datatypes.JSON([]byte(`{}`)),
		RequestID:           input.RequestID,
		IdempotencyKeyHash:  input.IdempotencyKeyHash,
		InputFingerprint:    fingerprint,
		RerunSourceRevision: input.SourceRunRevision,
		Revision:            1,
	}
	if input.SourceRunID != zeroUUID {
		run.RerunOfRunID = &input.SourceRunID
	}
	if err := o.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateInventorySyncRun(run); err != nil {
			return err
		}
		if err := verifyShopConnection(ctx, tx, run.TenantID, run.ShopConnectionID, run.Platform); err != nil {
			return err
		}
		if run.RerunOfRunID != nil {
			var source InventorySyncRun
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", run.TenantID, *run.RerunOfRunID).First(&source).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrNotFound
				}
				return stableError(err, ErrStateConflict)
			}
			if source.Revision != run.RerunSourceRevision {
				return ErrRevisionConflict
			}
			if source.Status != InventorySyncRunStatusFailed && source.Status != InventorySyncRunStatusCancelled {
				return ErrRetryNotAllowed
			}
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
			if err == nil {
				if existing.InputFingerprint != run.InputFingerprint {
					return ErrIdempotencyPayloadConflict
				}
				*run = *existing
				return nil
			}
			if !errors.Is(err, ErrNotFound) {
				return stableError(err, ErrStateConflict)
			}
			if run.RerunOfRunID != nil {
				var claimed InventorySyncRun
				claimErr := tx.Where("tenant_id = ? AND rerun_of_run_id = ? AND rerun_source_revision = ?", run.TenantID, *run.RerunOfRunID, run.RerunSourceRevision).First(&claimed).Error
				if claimErr == nil {
					return ErrRevisionConflict
				}
				if !errors.Is(claimErr, gorm.ErrRecordNotFound) {
					return stableError(claimErr, ErrStateConflict)
				}
			}
			return ErrStateConflict
		}
		return o.writeAuditWithDB(ctx, tx, InventorySyncAuditEvent{TenantID: input.TenantID, ActorID: input.ActorID, Action: "inventory_sync.run_created", Resource: inventorySyncAuditResourceRun, ResourceID: run.ID.String(), ShopID: input.ShopConnectionID, Platform: input.Platform, Permission: inventorySyncPermission(input), Status: inventorySyncAuditStatusSuccess, RequestID: input.RequestID, Metadata: inventorySyncAuditMetadata(input, map[string]any{"runStatusAfter": InventorySyncRunStatusPending})})
	}); err != nil {
		return nil, err
	}
	if isTerminalRunStatus(run.Status) {
		return o.resultFromRun(run)
	}
	if run.Status == InventorySyncRunStatusPending {
		now := o.now()
		if err := o.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			started, err := updateRunStatusWithDB(ctx, tx, input.TenantID, run.ID, run.Revision, InventorySyncRunStatusRunning, InventorySyncRunStatusPatch{StartedAt: &now})
			if err != nil {
				return err
			}
			if err := o.writeAuditWithDB(ctx, tx, InventorySyncAuditEvent{TenantID: input.TenantID, ActorID: input.ActorID, Action: "inventory_sync.started", Resource: inventorySyncAuditResourceRun, ResourceID: started.ID.String(), ShopID: input.ShopConnectionID, Platform: input.Platform, Permission: inventorySyncPermission(input), Status: inventorySyncAuditStatusSuccess, RequestID: input.RequestID, Metadata: inventorySyncAuditMetadata(input, map[string]any{"runStatusBefore": InventorySyncRunStatusPending, "runStatusAfter": InventorySyncRunStatusRunning})}); err != nil {
				return err
			}
			run = started
			return nil
		}); err != nil {
			return nil, err
		}
	}
	checkpoint := inventorySyncCheckpoint{FixtureScenario: input.FixtureScenario, TriggerType: input.TriggerType, StatisticsInvariant: InventorySyncStatisticsInvariant, InvalidItemPolicy: InventorySyncInvalidItemPolicy}
	if input.SourceRunID != zeroUUID {
		checkpoint.RerunOfRunID = input.SourceRunID.String()
	}
	seenCursors := map[string]bool{}
	cursor := run.Cursor
	for page := 0; page < input.MaxPagesPerRun; page++ {
		if err := ctx.Err(); err != nil {
			return o.finishWithError(ctx, input, run.ID, run.Revision, InventorySyncRunStatusCancelled, ErrSyncCancelled, checkpoint)
		}
		request := InventoryFetchRequest{TenantID: input.TenantID, ShopConnectionID: input.ShopConnectionID.String(), Platform: input.Platform, ProviderMode: input.ProviderMode, FixtureScenario: input.FixtureScenario, Cursor: cursor, PageSize: input.PageSize, MaxItemsPerPage: input.MaxItemsPerPage}
		pageResult, err := provider.FetchInventoryPage(ctx, request)
		if err != nil {
			return o.finishWithError(ctx, input, run.ID, run.Revision, InventorySyncRunStatusFailed, err, checkpoint)
		}
		checkpoint.ProviderNetworkCalls += pageResult.NetworkCalls
		if pageResult.NetworkCalls != 0 {
			return o.finishWithError(ctx, input, run.ID, run.Revision, InventorySyncRunStatusFailed, ErrProviderCapabilityForbidden, checkpoint)
		}
		if err := validateFetchedPage(input, cursor, pageResult, checkpoint); err != nil {
			return o.finishWithError(ctx, input, run.ID, run.Revision, InventorySyncRunStatusFailed, err, checkpoint)
		}
		cursorKey := string(pageResult.Cursor)
		if seenCursors[cursorKey] {
			return o.finishWithError(ctx, input, run.ID, run.Revision, InventorySyncRunStatusFailed, ErrProviderCursorLoop, checkpoint)
		}
		seenCursors[cursorKey] = true
		if err := validateProviderPageNoDuplicateExternalSKU(pageResult.Items); err != nil {
			return o.finishWithError(ctx, input, run.ID, run.Revision, InventorySyncRunStatusFailed, err, checkpoint)
		}
		if checkpoint.TotalRecordCount+len(pageResult.Items) > input.MaxItemsPerRun {
			return o.finishWithError(ctx, input, run.ID, run.Revision, InventorySyncRunStatusFailed, ErrProviderPageLimitExceeded, checkpoint)
		}
		currentRun := run
		run, err = o.commitPage(ctx, input, run, pageResult, &checkpoint)
		if err != nil {
			runID := currentRun.ID
			revision := currentRun.Revision
			if run != nil {
				runID = run.ID
				revision = run.Revision
			}
			return o.finishWithError(ctx, input, runID, revision, InventorySyncRunStatusFailed, err, checkpoint)
		}
		cursor = pageResult.NextCursor
		if !pageResult.HasMore {
			finished := o.now()
			checkpointJSON, jsonErr := safeCheckpointJSON(checkpoint)
			if jsonErr != nil {
				return nil, jsonErr
			}
			var finalRun *InventorySyncRun
			if err := o.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				updated, err := updateRunStatusWithDB(ctx, tx, input.TenantID, run.ID, run.Revision, InventorySyncRunStatusSucceeded, InventorySyncRunStatusPatch{FinishedAt: &finished, Checkpoint: checkpointJSON, Cursor: cursor})
				if err != nil {
					return err
				}
				if err := o.writeAuditWithDB(ctx, tx, InventorySyncAuditEvent{TenantID: input.TenantID, ActorID: input.ActorID, Action: "inventory_sync.completed", Resource: inventorySyncAuditResourceRun, ResourceID: updated.ID.String(), ShopID: input.ShopConnectionID, Platform: input.Platform, Permission: inventorySyncPermission(input), Status: inventorySyncAuditStatusSuccess, RequestID: input.RequestID, Metadata: inventorySyncAuditMetadata(input, map[string]any{"runStatusBefore": InventorySyncRunStatusRunning, "runStatusAfter": InventorySyncRunStatusSucceeded, "totalRecordCount": checkpoint.TotalRecordCount, "matchedRecordCount": checkpoint.MatchedRecordCount, "unmatchedRecordCount": checkpoint.UnmatchedRecordCount, "conflictRecordCount": checkpoint.ConflictRecordCount, "failedRecordCount": checkpoint.FailedRecordCount, "pagesProcessed": checkpoint.PagesProcessed, "cursorHash": safeCursorHash(cursor)})}); err != nil {
					return err
				}
				finalRun = updated
				return nil
			}); err != nil {
				return nil, err
			}
			return o.resultFromRun(finalRun)
		}
	}
	return o.finishWithError(ctx, input, run.ID, run.Revision, InventorySyncRunStatusFailed, ErrProviderPageLimitExceeded, checkpoint)
}

func (o *InventorySyncOrchestrator) ManualRerun(ctx context.Context, input InventorySyncOrchestratorInput) (*InventorySyncOrchestratorResult, error) {
	input.TriggerType = InventorySyncTriggerManualRerun
	return o.Run(ctx, input)
}

func (o *InventorySyncOrchestrator) commitPage(ctx context.Context, input InventorySyncOrchestratorInput, run *InventorySyncRun, page InventoryFetchPageResult, checkpoint *inventorySyncCheckpoint) (*InventorySyncRun, error) {
	workingCheckpoint := *checkpoint
	workingCheckpoint.BindingResults = append([]BindingResolutionItemResult(nil), checkpoint.BindingResults...)
	var updated *InventorySyncRun
	err := o.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current InventorySyncRun
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", input.TenantID, run.ID).First(&current).Error; err != nil {
			return stableError(err, ErrStateConflict)
		}
		if current.Revision != run.Revision || current.Status != InventorySyncRunStatusRunning {
			return ErrRevisionConflict
		}
		if !providerCursorEqual(current.Cursor, run.Cursor) {
			return ErrProviderCursorInvalid
		}
		now := o.now()
		snapshots, err := providerItemsToSnapshots(input, current, page.Items, now)
		if err != nil {
			return err
		}
		if len(snapshots) > 0 {
			if err := NewInventorySnapshotRepository(tx).CreateBatch(ctx, input.TenantID, snapshots); err != nil {
				return err
			}
		}
		stored := make([]InventorySnapshotItem, 0, len(snapshots))
		for _, snapshot := range snapshots {
			row, err := NewInventorySnapshotRepository(tx).GetByRunAndExternalSKU(ctx, input.TenantID, current.ID, snapshot.ExternalSKUID)
			if err != nil {
				return err
			}
			stored = append(stored, *row)
		}
		pageResolution, err := NewBindingResolutionPipeline(tx, o.CalibrationService).ResolvePageWithDB(ctx, tx, input.TenantID, current.ID, stored)
		if err != nil {
			return err
		}
		workingCheckpoint.TotalRecordCount += pageResolution.TotalRecordCount
		workingCheckpoint.MatchedRecordCount += pageResolution.MatchedRecordCount
		workingCheckpoint.UnmatchedRecordCount += pageResolution.UnmatchedRecordCount
		workingCheckpoint.ConflictRecordCount += pageResolution.ConflictRecordCount
		workingCheckpoint.FailedRecordCount += pageResolution.FailedRecordCount
		workingCheckpoint.ManualBindingRequestCount += pageResolution.ManualBindingRequestCount
		workingCheckpoint.ConfirmedBindingCount += pageResolution.ConfirmedBindingCount
		workingCheckpoint.PagesProcessed++
		workingCheckpoint.BindingResults = append(workingCheckpoint.BindingResults, pageResolution.Results...)
		checkpointJSON, err := safeCheckpointJSON(workingCheckpoint)
		if err != nil {
			return err
		}
		snapshotCount := current.SnapshotCount + len(stored)
		calibrationCount := current.CalibrationCount + pageResolution.CalibrationCount
		manualCount := current.ManualRequestCount + pageResolution.ManualBindingRequestCount
		patched, err := updateRunStatusWithDB(ctx, tx, input.TenantID, current.ID, current.Revision, InventorySyncRunStatusRunning, InventorySyncRunStatusPatch{SnapshotCount: &snapshotCount, CalibrationCount: &calibrationCount, ManualRequestCount: &manualCount, Cursor: page.NextCursor, Checkpoint: checkpointJSON})
		if err != nil {
			return err
		}
		if err := o.writeAuditWithDB(ctx, tx, InventorySyncAuditEvent{TenantID: input.TenantID, ActorID: input.ActorID, Action: "inventory_sync.page_processed", Resource: inventorySyncAuditResourceRun, ResourceID: current.ID.String(), ShopID: input.ShopConnectionID, Platform: input.Platform, Permission: inventorySyncPermission(input), Status: inventorySyncAuditStatusSuccess, RequestID: input.RequestID, Metadata: inventorySyncAuditMetadata(input, map[string]any{"pageNumber": workingCheckpoint.PagesProcessed, "pageItemCount": len(stored), "totalRecordCount": workingCheckpoint.TotalRecordCount, "matchedRecordCount": workingCheckpoint.MatchedRecordCount, "unmatchedRecordCount": workingCheckpoint.UnmatchedRecordCount, "conflictRecordCount": workingCheckpoint.ConflictRecordCount, "failedRecordCount": workingCheckpoint.FailedRecordCount, "cursorHash": safeCursorHash(page.NextCursor), "pagesProcessed": workingCheckpoint.PagesProcessed})}); err != nil {
			return err
		}
		updated = patched
		return nil
	})
	if err != nil {
		return nil, err
	}
	*checkpoint = workingCheckpoint
	return updated, nil
}

func normalizeOrchestratorInput(input *InventorySyncOrchestratorInput) error {
	if input == nil || validateTenantID(input.TenantID) != nil || input.ShopConnectionID == zeroUUID {
		return ErrValidation
	}
	input.Platform = normalizeLower(input.Platform)
	input.ProviderMode = normalizeLower(input.ProviderMode)
	input.FixtureScenario = normalizeFixtureScenario(input.FixtureScenario)
	input.TriggerType = normalizeString(input.TriggerType)
	input.RequestID = normalizeString(input.RequestID)
	input.IdempotencyKeyHash = normalizeString(input.IdempotencyKeyHash)
	if input.TriggerType == "" {
		input.TriggerType = InventorySyncTriggerManual
	}
	if input.Platform != PlatformDouyin || (!allowedProviderModes[input.ProviderMode] && !inventoryProviderKeyProductionForbidden(InventoryProviderKey{Platform: input.Platform, ProviderMode: input.ProviderMode})) {
		return ErrValidation
	}
	if input.PageSize <= 0 {
		input.PageSize = DefaultInventoryPageSize
	}
	if input.MaxPagesPerRun <= 0 {
		input.MaxPagesPerRun = DefaultMaxPagesPerRun
	}
	if input.MaxItemsPerPage <= 0 {
		input.MaxItemsPerPage = DefaultMaxItemsPerPage
	}
	if input.MaxItemsPerRun <= 0 {
		input.MaxItemsPerRun = DefaultMaxItemsPerRun
	}
	if input.PageSize <= 0 || input.PageSize > input.MaxItemsPerPage || input.MaxPagesPerRun <= 0 || input.MaxItemsPerRun <= 0 {
		return ErrValidation
	}
	if input.IdempotencyKeyHash == "" {
		input.IdempotencyKeyHash = hashString(input.TenantID, input.ShopConnectionID.String(), input.Platform, input.ProviderMode, input.FixtureScenario, input.TriggerType, input.RequestID)
	}
	if err := validateHashField(input.IdempotencyKeyHash, true); err != nil {
		return err
	}
	if input.TriggerType == InventorySyncTriggerManualRerun && (input.SourceRunID == zeroUUID || input.SourceRunRevision < 1) {
		return ErrValidation
	}
	return nil
}

func (o *InventorySyncOrchestrator) authorizeRun(ctx context.Context, input InventorySyncOrchestratorInput) error {
	if o.Authorizer == nil {
		return ErrPermissionDenied
	}
	if input.ActorID == zeroUUID {
		return ErrValidation
	}
	if input.TriggerType == InventorySyncTriggerManualRerun {
		return o.Authorizer.CanRerunInventorySync(ctx, input.TenantID, input.ActorID, input.SourceRunID)
	}
	return o.Authorizer.CanRunInventorySync(ctx, input.TenantID, input.ActorID, input.ShopConnectionID)
}

func validateFetchedPage(input InventorySyncOrchestratorInput, expectedCursor datatypes.JSON, page InventoryFetchPageResult, checkpoint inventorySyncCheckpoint) error {
	if !providerCursorEqual(page.Cursor, expectedCursor) && len(expectedCursor) > 0 && string(expectedCursor) != "{}" {
		return ErrProviderCursorInvalid
	}
	if len(page.Items) > input.MaxItemsPerPage {
		return ErrProviderPageLimitExceeded
	}
	if page.HasMore {
		if len(page.NextCursor) == 0 || string(page.NextCursor) == "{}" {
			return ErrProviderCursorInvalid
		}
		if providerCursorEqual(page.Cursor, page.NextCursor) {
			return ErrProviderCursorLoop
		}
	}
	if checkpoint.PagesProcessed >= input.MaxPagesPerRun {
		return ErrProviderPageLimitExceeded
	}
	return nil
}

func providerItemsToSnapshots(input InventorySyncOrchestratorInput, run InventorySyncRun, items []InventoryProviderItem, observedAt time.Time) ([]InventorySnapshotItem, error) {
	snapshots := make([]InventorySnapshotItem, 0, len(items))
	for _, item := range items {
		metadata, err := providerSafeMetadata(item.SafeMetadata)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, InventorySnapshotItem{
			TenantID:            input.TenantID,
			InventorySyncRunID:  run.ID,
			ShopConnectionID:    input.ShopConnectionID,
			Platform:            input.Platform,
			ExternalProductID:   normalizeString(item.ExternalProductID),
			ExternalSKUID:       normalizeString(item.ExternalSKUID),
			ExternalProductCode: normalizeString(item.ExternalProductCode),
			ExternalSKUCode:     normalizeString(item.ExternalSKUCode),
			Barcode:             normalizeString(item.Barcode),
			ProductTitle:        normalizeString(item.ProductTitle),
			VariantTitle:        normalizeString(item.VariantTitle),
			AvailableQuantity:   item.AvailableQuantity,
			ReservedQuantity:    item.ReservedQuantity,
			TotalQuantity:       item.TotalQuantity,
			SourceUpdatedAt:     item.SourceUpdatedAt,
			ObservedAt:          observedAt,
			PayloadHash:         hashInventoryProviderItem(item),
			SafeMetadata:        metadata,
		})
	}
	return snapshots, nil
}

func (o *InventorySyncOrchestrator) finishWithError(ctx context.Context, input InventorySyncOrchestratorInput, runID uuid.UUID, expectedRevision int, status string, err error, checkpoint inventorySyncCheckpoint) (*InventorySyncOrchestratorResult, error) {
	metadata, metaErr := safeErrorMetadata(err, checkpoint)
	if metaErr != nil {
		return nil, metaErr
	}
	finished := o.now()
	checkpointJSON, jsonErr := safeCheckpointJSON(checkpoint)
	if jsonErr != nil {
		return nil, jsonErr
	}
	var updated *InventorySyncRun
	updateErr := o.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		patched, updateErr := updateRunStatusWithDB(ctx, tx, input.TenantID, runID, expectedRevision, status, InventorySyncRunStatusPatch{FinishedAt: &finished, SafeErrorMetadata: metadata, Checkpoint: checkpointJSON})
		if updateErr != nil {
			return updateErr
		}
		code := providerErrorCode(err)
		if err := o.writeAuditWithDB(ctx, tx, InventorySyncAuditEvent{TenantID: input.TenantID, ActorID: input.ActorID, Action: "inventory_sync.failed", Resource: inventorySyncAuditResourceRun, ResourceID: runID.String(), ShopID: input.ShopConnectionID, Platform: input.Platform, Permission: inventorySyncPermission(input), Status: inventorySyncAuditStatusFailed, RequestID: input.RequestID, Metadata: inventorySyncAuditMetadata(input, map[string]any{"runStatusBefore": InventorySyncRunStatusRunning, "runStatusAfter": status, "errorCode": code, "safeMessage": code, "retryable": syncErrorRetryable(err), "pagesProcessed": checkpoint.PagesProcessed, "totalRecordCount": checkpoint.TotalRecordCount, "matchedRecordCount": checkpoint.MatchedRecordCount, "unmatchedRecordCount": checkpoint.UnmatchedRecordCount, "conflictRecordCount": checkpoint.ConflictRecordCount, "failedRecordCount": checkpoint.FailedRecordCount})}); err != nil {
			return err
		}
		updated = patched
		return nil
	})
	if updateErr != nil {
		return nil, updateErr
	}
	result, resultErr := o.resultFromRun(updated)
	if resultErr != nil {
		return nil, resultErr
	}
	return result, errWithCode(providerErrorCode(err))
}

func (o *InventorySyncOrchestrator) resultFromRun(run *InventorySyncRun) (*InventorySyncOrchestratorResult, error) {
	if run == nil {
		return nil, ErrNotFound
	}
	checkpoint := inventorySyncCheckpoint{}
	if len(run.Checkpoint) > 0 {
		_ = json.Unmarshal(run.Checkpoint, &checkpoint)
	}
	return &InventorySyncOrchestratorResult{InventorySyncRunID: run.ID, Status: run.Status, TotalRecordCount: checkpoint.TotalRecordCount, MatchedRecordCount: checkpoint.MatchedRecordCount, UnmatchedRecordCount: checkpoint.UnmatchedRecordCount, ConflictRecordCount: checkpoint.ConflictRecordCount, FailedRecordCount: checkpoint.FailedRecordCount, CursorAfter: run.Cursor, StartedAt: run.StartedAt, FinishedAt: run.FinishedAt, ManualBindingRequestCount: run.ManualRequestCount, ConfirmedBindingCount: checkpoint.ConfirmedBindingCount, SafeErrorSummary: run.SafeErrorMetadata}, nil
}

func (o *InventorySyncOrchestrator) auditWithDB(db *gorm.DB) *InventorySyncAuditService {
	if o.Audit == nil {
		return nil
	}
	return o.Audit.WithDB(db)
}

func (o *InventorySyncOrchestrator) writeAuditWithDB(ctx context.Context, db *gorm.DB, event InventorySyncAuditEvent) error {
	audit := o.auditWithDB(db)
	if audit == nil {
		return ErrStateConflict
	}
	return audit.Write(ctx, event)
}

func (o *InventorySyncOrchestrator) auditPermissionDenied(ctx context.Context, input InventorySyncOrchestratorInput) error {
	audit := o.auditWithDB(o.DB)
	if audit == nil {
		return ErrStateConflict
	}
	return audit.PermissionDenied(ctx, input.TenantID, input.ActorID, input.TriggerType, inventorySyncPermission(input), input.RequestID)
}

func (o *InventorySyncOrchestrator) auditProductionCapabilityBlocked(ctx context.Context, input InventorySyncOrchestratorInput, err error) error {
	audit := o.auditWithDB(o.DB)
	if audit == nil {
		return ErrStateConflict
	}
	return audit.ProductionCapabilityBlocked(ctx, input, err)
}

func inventorySyncPermission(input InventorySyncOrchestratorInput) string {
	if input.TriggerType == InventorySyncTriggerManualRerun {
		return adminperm.PermInventorySyncRerun
	}
	return adminperm.PermInventorySyncRun
}

func inventorySyncAuditMetadata(input InventorySyncOrchestratorInput, extra map[string]any) map[string]any {
	metadata := map[string]any{"platform": input.Platform, "providerMode": input.ProviderMode, "fixtureScenario": input.FixtureScenario, "requestId": normalizeString(input.RequestID)}
	maps.Copy(metadata, extra)
	return metadata
}

func (o *InventorySyncOrchestrator) now() time.Time {
	if o.Now != nil {
		return o.Now().UTC()
	}
	return utcNow()
}

func safeCheckpointJSON(checkpoint inventorySyncCheckpoint) (datatypes.JSON, error) {
	encoded, err := json.Marshal(checkpoint)
	if err != nil {
		return nil, ErrValidation
	}
	return normalizeModelJSON(datatypes.JSON(encoded), maxSafeJSONBytes)
}

func safeErrorMetadata(err error, checkpoint inventorySyncCheckpoint) (datatypes.JSON, error) {
	code := providerErrorCode(err)
	if code == "" {
		code = ErrCodeStateConflict
	}
	metadata := map[string]any{"errorCode": code, "safeMessage": code, "pagesProcessed": checkpoint.PagesProcessed, "retryable": syncErrorRetryable(err)}
	encoded, jsonErr := json.Marshal(metadata)
	if jsonErr != nil {
		return nil, ErrValidation
	}
	return normalizeModelJSON(datatypes.JSON(encoded), maxSafeJSONBytes)
}

func syncErrorRetryable(err error) bool {
	return errors.Is(err, ErrProviderTimeout) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrRevisionConflict) || errors.Is(err, ErrStateConflict)
}

func inventorySyncInputFingerprint(input InventorySyncOrchestratorInput) string {
	return hashString(input.TenantID, input.ShopConnectionID.String(), input.Platform, input.ProviderMode, input.FixtureScenario, input.TriggerType, input.PageSize, input.MaxPagesPerRun, input.MaxItemsPerPage, input.MaxItemsPerRun, input.SourceRunID.String(), input.SourceRunRevision)
}

func hashString(parts ...any) string {
	payload, _ := json.Marshal(parts)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (r *inventorySyncLockRegistry) acquire(tenantID int64, shopID uuid.UUID, platform string, providerMode string) func() {
	if r == nil {
		return func() {}
	}
	key := fmt.Sprintf("%d:%s:%s:%s", tenantID, shopID.String(), platform, providerMode)
	r.mu.Lock()
	lock := r.locks[key]
	if lock == nil {
		lock = &sync.Mutex{}
		r.locks[key] = lock
	}
	r.mu.Unlock()
	lock.Lock()
	return lock.Unlock
}
