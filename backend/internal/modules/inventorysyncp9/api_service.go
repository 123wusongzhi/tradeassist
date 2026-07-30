package inventorysyncp9

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/safefields"
	"gorm.io/gorm"
)

type APIService struct {
	DB            *gorm.DB
	Repo          *APIRepository
	Authorizer    *RBACAuthorizer
	Orchestrator  *InventorySyncOrchestrator
	Calibration   *SKUBindingCalibrationService
	ManualBinding *ManualBindingService
	Idempotency   *idempotency.Service
	Audit         *InventorySyncAuditService
}

func NewAPIService(db *gorm.DB) *APIService {
	authorizer := NewRBACAuthorizer(db)
	registry, _ := NewDefaultInventoryProviderRegistry()
	policy, _ := NewCalibrationThresholdPolicy(CalibrationThresholdConfig{HighConfidenceThreshold: 9500, AutoConfirmationEnabled: false, PolicyVersion: ThresholdPolicyVersionV1})
	calibration := NewSKUBindingCalibrationService(db, NewGORMLocalSKUCandidateProvider(db), policy)
	return &APIService{
		DB:            db,
		Repo:          NewAPIRepository(db),
		Authorizer:    authorizer,
		Orchestrator:  NewInventorySyncOrchestrator(db, registry, calibration, authorizer),
		Calibration:   calibration,
		ManualBinding: NewManualBindingService(db, authorizer),
		Idempotency:   &idempotency.Service{DB: db},
		Audit:         NewInventorySyncAuditService(db),
	}
}

func (s *APIService) ready() error {
	if s == nil || s.DB == nil || s.Repo == nil || s.Authorizer == nil || s.Orchestrator == nil || s.Calibration == nil || s.ManualBinding == nil {
		return ErrStateConflict
	}
	return nil
}

func (s *APIService) require(ctx context.Context, actor APIActor, permission string) error {
	if err := s.ready(); err != nil {
		return err
	}
	if actor.TenantID <= 0 || actor.ActorID == uuid.Nil {
		return ErrAuthenticationRequired
	}
	return s.Authorizer.require(ctx, actor.TenantID, actor.ActorID, permission)
}

func (s *APIService) CreateRun(ctx context.Context, actor APIActor, req CreateInventorySyncRunRequest, requestID, idemHash string) (*InventorySyncRunResponse, error) {
	if err := s.require(ctx, actor, adminperm.PermInventorySyncRun); err != nil {
		return nil, err
	}
	req.Platform = strings.ToLower(strings.TrimSpace(req.Platform))
	req.ProviderMode = strings.ToLower(strings.TrimSpace(req.ProviderMode))
	req.FixtureScenario = strings.TrimSpace(req.FixtureScenario)
	if req.ShopConnectionID == uuid.Nil || req.Platform != PlatformDouyin || (!allowedProviderModes[req.ProviderMode] && !forbiddenProductionProviderModes[req.ProviderMode]) || strings.TrimSpace(requestID) == "" || len(idemHash) != 64 {
		return nil, ErrValidation
	}
	result, err := s.Orchestrator.Run(ctx, InventorySyncOrchestratorInput{
		TenantID:           actor.TenantID,
		ShopConnectionID:   req.ShopConnectionID,
		Platform:           req.Platform,
		ProviderMode:       req.ProviderMode,
		FixtureScenario:    req.FixtureScenario,
		TriggerType:        InventorySyncTriggerManual,
		ActorID:            actor.ActorID,
		RequestID:          requestID,
		IdempotencyKeyHash: idemHash,
	})
	if err != nil && result == nil {
		return nil, err
	}
	if result == nil {
		return nil, ErrStateConflict
	}
	run, err := s.Repo.GetRun(ctx, actor.TenantID, result.InventorySyncRunID)
	if err != nil {
		return nil, err
	}
	return s.mapRun(ctx, actor, run), nil
}

func (s *APIService) Rerun(ctx context.Context, actor APIActor, runID uuid.UUID, req RerunInventorySyncRequest, requestID, idemHash string) (*InventorySyncRunResponse, error) {
	if err := s.require(ctx, actor, adminperm.PermInventorySyncRerun); err != nil {
		return nil, err
	}
	source, err := s.Repo.GetRun(ctx, actor.TenantID, runID)
	if err != nil {
		return nil, err
	}
	if req.ExpectedRevision < 1 || source.Revision != req.ExpectedRevision {
		return nil, ErrRevisionConflict
	}
	if source.Status != InventorySyncRunStatusFailed && source.Status != InventorySyncRunStatusCancelled {
		return nil, ErrRetryNotAllowed
	}
	retryable, _ := safeRunError(source.SafeErrorMetadata)
	if !retryable {
		return nil, ErrRetryNotAllowed
	}
	checkpoint := decodeCheckpoint(source.Checkpoint)
	result, err := s.Orchestrator.ManualRerun(ctx, InventorySyncOrchestratorInput{
		TenantID:           actor.TenantID,
		ShopConnectionID:   source.ShopConnectionID,
		Platform:           source.Platform,
		ProviderMode:       source.ProviderMode,
		FixtureScenario:    checkpoint.FixtureScenario,
		TriggerType:        InventorySyncTriggerManualRerun,
		ActorID:            actor.ActorID,
		RequestID:          requestID,
		IdempotencyKeyHash: idemHash,
		SourceRunID:        source.ID,
		SourceRunRevision:  req.ExpectedRevision,
	})
	if err != nil && result == nil {
		return nil, err
	}
	if result == nil {
		return nil, ErrStateConflict
	}
	run, getErr := s.Repo.GetRun(ctx, actor.TenantID, result.InventorySyncRunID)
	if getErr != nil {
		return nil, getErr
	}
	return s.mapRun(ctx, actor, run), nil
}

func (s *APIService) ListRuns(ctx context.Context, actor APIActor, params InventorySyncRunListParams) (*PageResponse[InventorySyncRunResponse], error) {
	if err := s.require(ctx, actor, adminperm.PermInventorySyncRead); err != nil {
		return nil, err
	}
	rows, next, more, err := s.Repo.ListRuns(ctx, actor.TenantID, params)
	if err != nil {
		return nil, err
	}
	out := make([]InventorySyncRunResponse, 0, len(rows))
	for idx := range rows {
		out = append(out, *s.mapRun(ctx, actor, &rows[idx]))
	}
	return &PageResponse[InventorySyncRunResponse]{Items: out, NextCursor: next, HasMore: more, Limit: params.Limit}, nil
}

func (s *APIService) GetRun(ctx context.Context, actor APIActor, runID uuid.UUID) (*InventorySyncRunResponse, error) {
	if err := s.require(ctx, actor, adminperm.PermInventorySyncRead); err != nil {
		return nil, err
	}
	run, err := s.Repo.GetRun(ctx, actor.TenantID, runID)
	if err != nil {
		return nil, err
	}
	return s.mapRun(ctx, actor, run), nil
}

func (s *APIService) ListSnapshots(ctx context.Context, actor APIActor, runID uuid.UUID, params SnapshotListParams) (*PageResponse[InventorySnapshotResponse], error) {
	if err := s.require(ctx, actor, adminperm.PermInventorySnapshotRead); err != nil {
		return nil, err
	}
	rows, relations, next, more, err := s.Repo.ListSnapshots(ctx, actor.TenantID, runID, params)
	if err != nil {
		return nil, err
	}
	items := make([]InventorySnapshotResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapSnapshot(row, relations))
	}
	return &PageResponse[InventorySnapshotResponse]{Items: items, NextCursor: next, HasMore: more, Limit: params.Limit}, nil
}

func (s *APIService) GetSnapshot(ctx context.Context, actor APIActor, snapshotID uuid.UUID) (*InventorySnapshotResponse, error) {
	if err := s.require(ctx, actor, adminperm.PermInventorySnapshotRead); err != nil {
		return nil, err
	}
	row, err := s.Repo.GetSnapshot(ctx, actor.TenantID, snapshotID)
	if err != nil {
		return nil, err
	}
	relations, err := s.Repo.LoadSnapshotRelations(ctx, actor.TenantID, []InventorySnapshotItem{*row})
	if err != nil {
		return nil, err
	}
	out := mapSnapshot(*row, relations)
	return &out, nil
}

func (s *APIService) ListBindings(ctx context.Context, actor APIActor, params BindingListParams) (*PageResponse[SKUBindingResponse], error) {
	if err := s.require(ctx, actor, adminperm.PermSKUBindingRead); err != nil {
		return nil, err
	}
	rows, next, more, err := s.Repo.ListBindings(ctx, actor.TenantID, params)
	if err != nil {
		return nil, err
	}
	items := make([]SKUBindingResponse, 0, len(rows))
	for idx := range rows {
		items = append(items, mapBinding(ctx, actor, &rows[idx]))
	}
	return &PageResponse[SKUBindingResponse]{Items: items, NextCursor: next, HasMore: more, Limit: params.Limit}, nil
}

func (s *APIService) GetBinding(ctx context.Context, actor APIActor, bindingID uuid.UUID) (*SKUBindingResponse, error) {
	if err := s.require(ctx, actor, adminperm.PermSKUBindingRead); err != nil {
		return nil, err
	}
	binding, err := s.Repo.GetBinding(ctx, actor.TenantID, bindingID)
	if err != nil {
		return nil, err
	}
	out := mapBinding(ctx, actor, binding)
	return &out, nil
}

func (s *APIService) GetBindingHistory(ctx context.Context, actor APIActor, bindingID uuid.UUID) (*BindingHistoryResponse, error) {
	if err := s.require(ctx, actor, adminperm.PermSKUBindingRead); err != nil {
		return nil, err
	}
	binding, err := s.Repo.GetBinding(ctx, actor.TenantID, bindingID)
	if err != nil {
		return nil, err
	}
	calibrations, decisions, err := s.Repo.BindingHistory(ctx, binding)
	if err != nil {
		return nil, err
	}
	out := &BindingHistoryResponse{Binding: mapBinding(ctx, actor, binding), Calibrations: mapCalibrations(calibrations), Decisions: mapDecisions(decisions)}
	return out, nil
}

func (s *APIService) ListCalibrations(ctx context.Context, actor APIActor, snapshotID uuid.UUID, params CalibrationListParams) (*PageResponse[CalibrationResponse], error) {
	if err := s.require(ctx, actor, adminperm.PermSKUBindingRead); err != nil {
		return nil, err
	}
	if _, err := s.Repo.GetSnapshot(ctx, actor.TenantID, snapshotID); err != nil {
		return nil, err
	}
	rows, next, more, err := s.Repo.ListCalibrationPage(ctx, actor.TenantID, snapshotID, params)
	if err != nil {
		return nil, err
	}
	return &PageResponse[CalibrationResponse]{Items: mapCalibrations(rows), NextCursor: next, HasMore: more, Limit: params.Limit}, nil
}

func (s *APIService) Recalibrate(ctx context.Context, actor APIActor, snapshotID uuid.UUID, req RecalibrateSnapshotRequest, requestID, rawIdemKey string) (*RecalibrationResponse, error) {
	if err := s.require(ctx, actor, adminperm.PermSKUBindingManage); err != nil {
		return nil, err
	}
	if req.ExpectedCalibrationVersion < 1 || strings.TrimSpace(requestID) == "" || apiValidateReason(req.Reason, true) != nil || len(strings.TrimSpace(rawIdemKey)) == 0 {
		return nil, ErrValidation
	}
	snapshot, err := s.Repo.GetSnapshot(ctx, actor.TenantID, snapshotID)
	if err != nil {
		return nil, err
	}
	keyHash := sha256Hex(rawIdemKey)
	requestHash := hashString(actor.TenantID, snapshotID.String(), req.ExpectedCalibrationVersion, strings.TrimSpace(req.Reason))
	owner := "p9-api:" + actor.ActorID.String()
	idemScope := "p9.inventory-sync.recalibrate:" + strconv.FormatInt(actor.TenantID, 10) + ":" + snapshotID.String()
	acquired, err := s.Idempotency.Acquire(ctx, idemScope, keyHash, requestHash, owner, 0)
	if err != nil {
		var opErr *idempotency.OpError
		if errors.As(err, &opErr) && opErr.Code == idempotency.ErrCodeAlreadySucceeded {
			version := 0
			var replay struct {
				Version int `json:"version"`
			}
			_ = json.Unmarshal([]byte(opErr.Record.ResponseSummary), &replay)
			version = replay.Version
			rows, listErr := s.Repo.ListCalibrations(ctx, actor.TenantID, snapshotID, version)
			if listErr != nil {
				return nil, listErr
			}
			return &RecalibrationResponse{SnapshotID: snapshotID, CalibrationVersion: version, Candidates: mapCalibrations(rows)}, nil
		}
		return nil, err
	}
	var response *RecalibrationResponse
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result, version, recalibrateErr := s.Calibration.recalibrateSnapshotItemWithDB(ctx, tx, actor.TenantID, snapshot.InventorySyncRunID, snapshot.ID, req.ExpectedCalibrationVersion)
		if recalibrateErr != nil {
			return recalibrateErr
		}
		var rows []SKUBindingCalibration
		if listErr := tx.Where("tenant_id = ? AND inventory_snapshot_item_id = ? AND calibration_version = ?", actor.TenantID, snapshotID, version).Order("confidence DESC, candidate_local_sku_id ASC, id ASC").Find(&rows).Error; listErr != nil {
			return stableError(listErr, ErrStateConflict)
		}
		response = &RecalibrationResponse{SnapshotID: snapshotID, CalibrationVersion: version, Candidates: mapCalibrations(rows)}
		summary, marshalErr := json.Marshal(map[string]any{"version": version})
		if marshalErr != nil {
			return stableError(marshalErr, ErrStateConflict)
		}
		if completeErr := s.Idempotency.WithDB(tx).Complete(ctx, acquired.Record.ID, owner, idempotency.CompleteResult{ResponseCode: "ok", ResponseSummary: string(summary), ResourceType: "p9_snapshot_calibration", ResourceID: snapshotID.String()}); completeErr != nil {
			return completeErr
		}
		if s.Audit != nil {
			if auditErr := s.Audit.WithDB(tx).Write(ctx, InventorySyncAuditEvent{TenantID: actor.TenantID, ActorID: actor.ActorID, Action: "sku_binding.recalibrated", Resource: inventorySyncAuditResourceRun, ResourceID: snapshot.InventorySyncRunID.String(), ShopID: snapshot.ShopConnectionID, Platform: snapshot.Platform, Permission: adminperm.PermSKUBindingManage, Status: inventorySyncAuditStatusSuccess, RequestID: requestID, Metadata: map[string]any{"calibrationVersion": version, "reasonCodes": []string{strings.TrimSpace(req.Reason)}}}); auditErr != nil {
				return auditErr
			}
		}
		_ = result
		return nil
	})
	if err != nil {
		_ = s.Idempotency.Fail(ctx, acquired.Record.ID, owner, errorCode(err), false)
		return nil, err
	}
	return response, nil
}

func (s *APIService) ListManualRequests(ctx context.Context, actor APIActor, params ManualBindingListParams) (*PageResponse[ManualBindingRequestResponse], error) {
	if err := s.require(ctx, actor, adminperm.PermSKUBindingRead); err != nil {
		return nil, err
	}
	rows, next, more, err := s.Repo.ListManualRequests(ctx, actor.TenantID, params)
	if err != nil {
		return nil, err
	}
	items := make([]ManualBindingRequestResponse, 0, len(rows))
	for idx := range rows {
		items = append(items, mapManualRequest(ctx, actor, &rows[idx]))
	}
	return &PageResponse[ManualBindingRequestResponse]{Items: items, NextCursor: next, HasMore: more, Limit: params.Limit}, nil
}

func (s *APIService) GetManualRequest(ctx context.Context, actor APIActor, requestID uuid.UUID) (*ManualBindingDetailResponse, error) {
	if err := s.require(ctx, actor, adminperm.PermSKUBindingRead); err != nil {
		return nil, err
	}
	request, err := s.Repo.GetManualRequest(ctx, actor.TenantID, requestID)
	if err != nil {
		return nil, err
	}
	decisions, err := s.Repo.ListManualDecisions(ctx, actor.TenantID, requestID)
	if err != nil {
		return nil, err
	}
	return &ManualBindingDetailResponse{Request: mapManualRequest(ctx, actor, request), Decisions: mapDecisions(decisions)}, nil
}

func (s *APIService) ConfirmManual(ctx context.Context, actor APIActor, requestID uuid.UUID, req ConfirmManualBindingRequest, requestIDHeader, idemHash string) (*ManualBindingRequestResponse, error) {
	if req.ExpectedRevision < 1 || req.SelectedLocalSKUID == uuid.Nil || apiValidateComment(req.Comment) != nil {
		return nil, ErrValidation
	}
	result, err := s.ManualBinding.ConfirmBinding(ctx, ConfirmManualBindingInput{Actor: ManualBindingActor{TenantID: actor.TenantID, ActorID: actor.ActorID}, RequestID: requestID, CorrelationID: requestIDHeader, ExpectedRevision: req.ExpectedRevision, SelectedLocalSKUID: req.SelectedLocalSKUID, IdempotencyKeyHash: idemHash, Comment: req.Comment})
	if err != nil {
		return nil, err
	}
	if result == nil || result.Request == nil {
		return nil, ErrStateConflict
	}
	out := mapManualRequest(ctx, actor, result.Request)
	return &out, nil
}

func (s *APIService) RejectManual(ctx context.Context, actor APIActor, requestID uuid.UUID, req RejectManualBindingRequest, requestIDHeader, idemHash string) (*ManualBindingRequestResponse, error) {
	if req.ExpectedRevision < 1 || apiValidateReason(req.ReasonCode, true) != nil || apiValidateComment(req.Comment) != nil {
		return nil, ErrValidation
	}
	result, err := s.ManualBinding.RejectBinding(ctx, RejectManualBindingInput{Actor: ManualBindingActor{TenantID: actor.TenantID, ActorID: actor.ActorID}, RequestID: requestID, CorrelationID: requestIDHeader, ExpectedRevision: req.ExpectedRevision, ReasonCode: req.ReasonCode, IdempotencyKeyHash: idemHash, Comment: req.Comment})
	if err != nil {
		return nil, err
	}
	if result == nil || result.Request == nil {
		return nil, ErrStateConflict
	}
	out := mapManualRequest(ctx, actor, result.Request)
	return &out, nil
}

func (s *APIService) ListRunAudit(ctx context.Context, actor APIActor, runID uuid.UUID, params AuditEventListParams) (*PageResponse[AuditEventResponse], error) {
	if err := s.require(ctx, actor, adminperm.PermInventorySyncAuditRead); err != nil {
		return nil, err
	}
	if _, err := s.Repo.GetRun(ctx, actor.TenantID, runID); err != nil {
		return nil, err
	}
	rows, next, more, err := s.Repo.ListRunAuditEvents(ctx, actor.TenantID, runID, params)
	if err != nil {
		return nil, err
	}
	items := make([]AuditEventResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapAudit(row))
	}
	return &PageResponse[AuditEventResponse]{Items: items, NextCursor: next, HasMore: more, Limit: params.Limit}, nil
}

func (s *APIService) mapRun(ctx context.Context, actor APIActor, run *InventorySyncRun) *InventorySyncRunResponse {
	checkpoint := decodeCheckpoint(run.Checkpoint)
	retryable, safeErr := safeRunError(run.SafeErrorMetadata)
	trigger := checkpoint.TriggerType
	if trigger == "" {
		trigger = InventorySyncTriggerManual
	}
	out := &InventorySyncRunResponse{
		ID: run.ID, ShopConnectionID: run.ShopConnectionID, Platform: run.Platform, ProviderMode: run.ProviderMode, Status: run.Status,
		TriggerType: trigger, FixtureScenario: checkpoint.FixtureScenario, Statistics: InventorySyncStatisticsResponse{TotalRecordCount: checkpoint.TotalRecordCount, MatchedRecordCount: checkpoint.MatchedRecordCount, UnmatchedRecordCount: checkpoint.UnmatchedRecordCount, ConflictRecordCount: checkpoint.ConflictRecordCount, FailedRecordCount: checkpoint.FailedRecordCount, ManualBindingRequestCount: checkpoint.ManualBindingRequestCount, ConfirmedBindingCount: checkpoint.ConfirmedBindingCount, PagesProcessed: checkpoint.PagesProcessed},
		CursorHash: safeCursorHash(run.Cursor), Revision: run.Revision, StartedAt: run.StartedAt, FinishedAt: run.FinishedAt, CreatedAt: run.CreatedAt, UpdatedAt: run.UpdatedAt,
	}
	if rerun := strings.TrimSpace(checkpoint.RerunOfRunID); rerun != "" {
		if id, err := uuid.Parse(rerun); err == nil {
			out.RerunOfRunID = &id
		}
	}
	if safeErr != "" {
		out.SafeError = &SafeRunErrorResponse{Code: safeErr, Message: safeErr, Retryable: retryable}
	}
	out.AllowedActions = InventorySyncRunAllowedActions{
		CanViewSnapshots: s.can(ctx, actor, adminperm.PermInventorySnapshotRead),
		CanRerun:         (run.Status == InventorySyncRunStatusFailed || run.Status == InventorySyncRunStatusCancelled) && retryable && s.can(ctx, actor, adminperm.PermInventorySyncRerun),
		CanViewAudit:     s.can(ctx, actor, adminperm.PermInventorySyncAuditRead),
	}
	return out
}

func (s *APIService) can(ctx context.Context, actor APIActor, permission string) bool {
	return actor.TenantID > 0 && actor.ActorID != uuid.Nil && adminperm.StrictHasPermission(actor.Role, permission)
}

func mapSnapshot(row InventorySnapshotItem, relations snapshotRelations) InventorySnapshotResponse {
	out := InventorySnapshotResponse{ID: row.ID, InventorySyncRunID: row.InventorySyncRunID, ShopConnectionID: row.ShopConnectionID, Platform: row.Platform, ExternalProductID: row.ExternalProductID, ExternalSKUID: row.ExternalSKUID, ExternalProductCode: row.ExternalProductCode, ExternalSKUCode: row.ExternalSKUCode, Barcode: row.Barcode, ProductTitle: row.ProductTitle, VariantTitle: row.VariantTitle, AvailableQuantity: row.AvailableQuantity, ReservedQuantity: row.ReservedQuantity, TotalQuantity: row.TotalQuantity, SourceUpdatedAt: row.SourceUpdatedAt, ObservedAt: row.ObservedAt, CreatedAt: row.CreatedAt}
	if binding, ok := relations.Bindings[row.ID]; ok {
		out.Binding = SnapshotBindingSummaryResponse{Result: "matched", BindingID: &binding.ID, BindingStatus: binding.BindingStatus, LocalProductID: &binding.LocalProductID, LocalSKUID: &binding.LocalSKUID, Confidence: binding.Confidence, CalibrationVersion: binding.CalibrationVersion}
		return out
	}
	if request, ok := relations.ManualRequests[row.ID]; ok {
		out.Binding = SnapshotBindingSummaryResponse{Result: "manual_review", ManualRequestID: &request.ID}
		return out
	}
	if calibration, ok := relations.Calibrations[row.ID]; ok {
		out.Binding = SnapshotBindingSummaryResponse{Result: "candidate", Confidence: calibration.Confidence, CalibrationVersion: calibration.CalibrationVersion}
		return out
	}
	out.Binding.Result = "unmatched"
	return out
}

func mapBinding(ctx context.Context, actor APIActor, row *SKUBinding) SKUBindingResponse {
	return SKUBindingResponse{ID: row.ID, ShopConnectionID: row.ShopConnectionID, Platform: row.Platform, ExternalProductID: row.ExternalProductID, ExternalSKUID: row.ExternalSKUID, ExternalSKUCode: row.ExternalSKUCode, LocalProductID: row.LocalProductID, LocalSKUID: row.LocalSKUID, BindingSource: row.BindingSource, BindingStatus: row.BindingStatus, Confidence: row.Confidence, CalibrationVersion: row.CalibrationVersion, CalibrationReason: safeText(row.CalibrationReason), ConfirmedBy: row.ConfirmedBy, ConfirmedAt: row.ConfirmedAt, Revision: row.Revision, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, AllowedActions: SKUBindingAllowedActions{CanViewHistory: true, CanViewCalibration: true}}
}

func mapCalibrations(rows []SKUBindingCalibration) []CalibrationResponse {
	out := make([]CalibrationResponse, 0, len(rows))
	for _, row := range rows {
		var breakdown []ScoreBreakdownItem
		var reasons []string
		_ = json.Unmarshal(row.ScoreBreakdown, &breakdown)
		_ = json.Unmarshal(row.ReasonCodes, &reasons)
		items := make([]ScoreBreakdownResponse, 0, len(breakdown))
		for _, item := range breakdown {
			items = append(items, ScoreBreakdownResponse{Signal: safeText(item.Code), Score: item.Points})
		}
		out = append(out, CalibrationResponse{ID: row.ID, InventorySyncRunID: row.InventorySyncRunID, InventorySnapshotItemID: row.InventorySnapshotItemID, ExternalSKUID: row.ExternalSKUID, CandidateLocalProductID: row.CandidateLocalProductID, CandidateLocalSKUID: row.CandidateLocalSKUID, MatchStrategy: row.MatchStrategy, Confidence: row.Confidence, ScoreBreakdown: items, ReasonCodes: safeStrings(reasons), CalibrationVersion: row.CalibrationVersion, Status: row.Status, CreatedAt: row.CreatedAt})
	}
	return out
}

func mapManualRequest(ctx context.Context, actor APIActor, row *ManualBindingRequest) ManualBindingRequestResponse {
	return ManualBindingRequestResponse{ID: row.ID, InventorySyncRunID: row.InventorySyncRunID, InventorySnapshotItemID: row.InventorySnapshotItemID, ShopConnectionID: row.ShopConnectionID, ExternalSKUID: row.ExternalSKUID, Status: row.Status, ReasonCode: row.ReasonCode, CandidateCount: row.CandidateCount, SuggestedLocalSKUID: row.SuggestedLocalSKUID, AssignedTo: row.AssignedTo, ResolvedBy: row.ResolvedBy, ResolvedAt: row.ResolvedAt, Resolution: safeText(row.Resolution), SelectedLocalProductID: row.SelectedLocalProductID, SelectedLocalSKUID: row.SelectedLocalSKUID, Comment: safeText(row.Comment), Revision: row.Revision, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, AllowedActions: ManualBindingAllowedActions{CanConfirm: row.Status == ManualBindingStatusPending && sCan(ctx, actor, adminperm.PermSKUBindingResolveManual), CanReject: row.Status == ManualBindingStatusPending && sCan(ctx, actor, adminperm.PermSKUBindingResolveManual)}}
}

func mapDecisions(rows []ManualBindingDecision) []ManualBindingDecisionResponse {
	out := make([]ManualBindingDecisionResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, ManualBindingDecisionResponse{ID: row.ID, Operation: row.Operation, ActorID: row.ActorID, SelectedLocalProductID: row.SelectedLocalProductID, SelectedLocalSKUID: row.SelectedLocalSKUID, ReasonCode: row.ReasonCode, Comment: safeText(row.Comment), RequestRevision: row.RequestRevision, CreatedAt: row.CreatedAt})
	}
	return out
}

func mapAudit(row operationlog.OperationLog) AuditEventResponse {
	var metadata any
	if json.Unmarshal([]byte(row.Message), &metadata) != nil {
		metadata = map[string]any{}
	}
	var actor *uuid.UUID
	if row.AdminUserID != nil {
		actor = row.AdminUserID
	}
	return AuditEventResponse{ID: row.ID, Action: row.Action, Resource: row.Resource, ResourceID: row.ResourceID, ShopID: row.ShopID, Platform: row.Platform, Permission: row.Permission, RequestID: row.RequestID, Status: row.Status, Metadata: metadata, ActorID: actor, ActorRole: row.AdminRole, CreatedAt: row.CreatedAt}
}

func decodeCheckpoint(raw []byte) inventorySyncCheckpoint {
	var out inventorySyncCheckpoint
	_ = json.Unmarshal(raw, &out)
	return out
}

func safeRunError(raw []byte) (bool, string) {
	var value struct {
		Code      string `json:"errorCode"`
		Message   string `json:"safeMessage"`
		Retryable bool   `json:"retryable"`
	}
	_ = json.Unmarshal(raw, &value)
	code := strings.TrimSpace(value.Code)
	if code == "" {
		code = strings.TrimSpace(value.Message)
	}
	return value.Retryable, code
}

func sha256Hex(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func errorCode(err error) string {
	if err == nil {
		return ErrCodeStateConflict
	}
	for _, candidate := range []struct {
		err  error
		code string
	}{{ErrProviderTimeout, ErrCodeProviderTimeout}, {ErrProviderRejected, ErrCodeProviderRejected}, {ErrRevisionConflict, ErrCodeRevisionConflict}} {
		if errors.Is(err, candidate.err) {
			return candidate.code
		}
	}
	return ErrCodeStateConflict
}

func safeText(value string) string {
	return safefields.RedactString(value)
}

func safeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, safeText(value))
	}
	return out
}

func sCan(ctx context.Context, actor APIActor, permission string) bool {
	return actor.TenantID > 0 && actor.ActorID != uuid.Nil && adminperm.StrictHasPermission(actor.Role, permission)
}
