package inventorysyncp9

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
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
	CanRerunInventorySync(ctx context.Context, tenantID int64, actorID uuid.UUID, sourceRunID uuid.UUID) error
}

type InventorySyncOrchestrator struct {
	DB                 *gorm.DB
	Registry           *InventoryProviderRegistry
	CalibrationService *SKUBindingCalibrationService
	Authorizer         InventorySyncAuthorizer
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
	return &InventorySyncOrchestrator{DB: db, Registry: registry, CalibrationService: calibrationService, Authorizer: authorizer, locks: &inventorySyncLockRegistry{locks: map[string]*sync.Mutex{}}, Now: utcNow}
}

func (o *InventorySyncOrchestrator) Run(ctx context.Context, input InventorySyncOrchestratorInput) (*InventorySyncOrchestratorResult, error) {
	if o == nil || o.DB == nil || o.Registry == nil || o.CalibrationService == nil {
		return nil, fmt.Errorf("inventory sync orchestrator: dependencies are nil")
	}
	if err := normalizeOrchestratorInput(&input); err != nil {
		return nil, err
	}
	if input.TriggerType == InventorySyncTriggerManualRerun {
		if err := o.authorizeRerun(ctx, input); err != nil {
			return nil, err
		}
	}
	provider, err := o.Registry.Resolve(input.Platform, input.ProviderMode)
	if err != nil {
		return nil, err
	}
	unlock := o.locks.acquire(input.TenantID, input.ShopConnectionID, input.Platform, input.ProviderMode)
	defer unlock()
	fingerprint := inventorySyncInputFingerprint(input)
	run, err := NewInventorySyncRunRepository(o.DB).Create(ctx, &InventorySyncRun{
		TenantID:           input.TenantID,
		ShopConnectionID:   input.ShopConnectionID,
		Platform:           input.Platform,
		ProviderMode:       input.ProviderMode,
		Status:             InventorySyncRunStatusPending,
		Cursor:             datatypes.JSON([]byte(`{}`)),
		Checkpoint:         datatypes.JSON([]byte(`{}`)),
		SafeErrorMetadata:  datatypes.JSON([]byte(`{}`)),
		RequestID:          input.RequestID,
		IdempotencyKeyHash: input.IdempotencyKeyHash,
		InputFingerprint:   fingerprint,
		Revision:           1,
	})
	if err != nil {
		return nil, err
	}
	if isTerminalRunStatus(run.Status) {
		return o.resultFromRun(run)
	}
	if run.Status == InventorySyncRunStatusPending {
		now := o.now()
		run, err = updateRunStatusWithDB(ctx, o.DB, input.TenantID, run.ID, run.Revision, InventorySyncRunStatusRunning, InventorySyncRunStatusPatch{StartedAt: &now})
		if err != nil {
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
			return o.finishWithError(ctx, input.TenantID, run.ID, run.Revision, InventorySyncRunStatusCancelled, ErrSyncCancelled, checkpoint)
		}
		request := InventoryFetchRequest{TenantID: input.TenantID, ShopConnectionID: input.ShopConnectionID.String(), Platform: input.Platform, ProviderMode: input.ProviderMode, FixtureScenario: input.FixtureScenario, Cursor: cursor, PageSize: input.PageSize, MaxItemsPerPage: input.MaxItemsPerPage}
		pageResult, err := provider.FetchInventoryPage(ctx, request)
		if err != nil {
			return o.finishWithError(ctx, input.TenantID, run.ID, run.Revision, InventorySyncRunStatusFailed, err, checkpoint)
		}
		checkpoint.ProviderNetworkCalls += pageResult.NetworkCalls
		if pageResult.NetworkCalls != 0 {
			return o.finishWithError(ctx, input.TenantID, run.ID, run.Revision, InventorySyncRunStatusFailed, ErrProviderCapabilityForbidden, checkpoint)
		}
		if err := validateFetchedPage(input, cursor, pageResult, checkpoint); err != nil {
			return o.finishWithError(ctx, input.TenantID, run.ID, run.Revision, InventorySyncRunStatusFailed, err, checkpoint)
		}
		cursorKey := string(pageResult.Cursor)
		if seenCursors[cursorKey] {
			return o.finishWithError(ctx, input.TenantID, run.ID, run.Revision, InventorySyncRunStatusFailed, ErrProviderCursorLoop, checkpoint)
		}
		seenCursors[cursorKey] = true
		if err := validateProviderPageNoDuplicateExternalSKU(pageResult.Items); err != nil {
			return o.finishWithError(ctx, input.TenantID, run.ID, run.Revision, InventorySyncRunStatusFailed, err, checkpoint)
		}
		if checkpoint.TotalRecordCount+len(pageResult.Items) > input.MaxItemsPerRun {
			return o.finishWithError(ctx, input.TenantID, run.ID, run.Revision, InventorySyncRunStatusFailed, ErrProviderPageLimitExceeded, checkpoint)
		}
		run, err = o.commitPage(ctx, input, run, pageResult, &checkpoint)
		if err != nil {
			return o.finishWithError(ctx, input.TenantID, run.ID, run.Revision, InventorySyncRunStatusFailed, err, checkpoint)
		}
		cursor = pageResult.NextCursor
		if !pageResult.HasMore {
			finished := o.now()
			checkpointJSON, jsonErr := safeCheckpointJSON(checkpoint)
			if jsonErr != nil {
				return nil, jsonErr
			}
			finalRun, err := updateRunStatusWithDB(ctx, o.DB, input.TenantID, run.ID, run.Revision, InventorySyncRunStatusSucceeded, InventorySyncRunStatusPatch{FinishedAt: &finished, Checkpoint: checkpointJSON, Cursor: cursor})
			if err != nil {
				return nil, err
			}
			return o.resultFromRun(finalRun)
		}
	}
	return o.finishWithError(ctx, input.TenantID, run.ID, run.Revision, InventorySyncRunStatusFailed, ErrProviderPageLimitExceeded, checkpoint)
}

func (o *InventorySyncOrchestrator) ManualRerun(ctx context.Context, input InventorySyncOrchestratorInput) (*InventorySyncOrchestratorResult, error) {
	input.TriggerType = InventorySyncTriggerManualRerun
	return o.Run(ctx, input)
}

func (o *InventorySyncOrchestrator) commitPage(ctx context.Context, input InventorySyncOrchestratorInput, run *InventorySyncRun, page InventoryFetchPageResult, checkpoint *inventorySyncCheckpoint) (*InventorySyncRun, error) {
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
		checkpoint.TotalRecordCount += pageResolution.TotalRecordCount
		checkpoint.MatchedRecordCount += pageResolution.MatchedRecordCount
		checkpoint.UnmatchedRecordCount += pageResolution.UnmatchedRecordCount
		checkpoint.ConflictRecordCount += pageResolution.ConflictRecordCount
		checkpoint.FailedRecordCount += pageResolution.FailedRecordCount
		checkpoint.ManualBindingRequestCount += pageResolution.ManualBindingRequestCount
		checkpoint.ConfirmedBindingCount += pageResolution.ConfirmedBindingCount
		checkpoint.PagesProcessed++
		checkpoint.BindingResults = append(checkpoint.BindingResults, pageResolution.Results...)
		checkpointJSON, err := safeCheckpointJSON(*checkpoint)
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
		updated = patched
		return nil
	})
	if err != nil {
		return nil, err
	}
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
	if input.Platform != PlatformDouyin || !allowedProviderModes[input.ProviderMode] {
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
	if input.TriggerType == InventorySyncTriggerManualRerun && input.SourceRunID == zeroUUID {
		return ErrValidation
	}
	return nil
}

func (o *InventorySyncOrchestrator) authorizeRerun(ctx context.Context, input InventorySyncOrchestratorInput) error {
	if o.Authorizer == nil {
		return ErrPermissionDenied
	}
	if input.ActorID == zeroUUID {
		return ErrValidation
	}
	return o.Authorizer.CanRerunInventorySync(ctx, input.TenantID, input.ActorID, input.SourceRunID)
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

func (o *InventorySyncOrchestrator) finishWithError(ctx context.Context, tenantID int64, runID uuid.UUID, expectedRevision int, status string, err error, checkpoint inventorySyncCheckpoint) (*InventorySyncOrchestratorResult, error) {
	metadata, metaErr := safeErrorMetadata(err, checkpoint)
	if metaErr != nil {
		return nil, metaErr
	}
	finished := o.now()
	checkpointJSON, jsonErr := safeCheckpointJSON(checkpoint)
	if jsonErr != nil {
		return nil, jsonErr
	}
	updated, updateErr := updateRunStatusWithDB(ctx, o.DB, tenantID, runID, expectedRevision, status, InventorySyncRunStatusPatch{FinishedAt: &finished, SafeErrorMetadata: metadata, Checkpoint: checkpointJSON})
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
	return hashString(input.TenantID, input.ShopConnectionID.String(), input.Platform, input.ProviderMode, input.FixtureScenario, input.TriggerType, input.PageSize, input.MaxPagesPerRun, input.MaxItemsPerPage, input.MaxItemsPerRun, input.SourceRunID.String())
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
