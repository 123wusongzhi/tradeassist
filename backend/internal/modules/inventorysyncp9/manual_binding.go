package inventorysyncp9

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ManualBindingAuthorizer interface {
	CanResolveManualBinding(ctx context.Context, tenantID int64, actorID uuid.UUID, requestID uuid.UUID) error
}

type ManualBindingService struct {
	DB         *gorm.DB
	Authorizer ManualBindingAuthorizer
	Audit      *InventorySyncAuditService
}

func NewManualBindingService(db *gorm.DB, authorizer ManualBindingAuthorizer) *ManualBindingService {
	return &ManualBindingService{DB: db, Authorizer: authorizer, Audit: NewInventorySyncAuditService(db)}
}

type ManualBindingActor struct {
	TenantID int64
	ActorID  uuid.UUID
}

type ConfirmManualBindingInput struct {
	Actor              ManualBindingActor
	RequestID          uuid.UUID
	CorrelationID      string
	ExpectedRevision   int
	SelectedLocalSKUID uuid.UUID
	IdempotencyKeyHash string
	Comment            string
}

type RejectManualBindingInput struct {
	Actor              ManualBindingActor
	RequestID          uuid.UUID
	CorrelationID      string
	ExpectedRevision   int
	ReasonCode         string
	IdempotencyKeyHash string
	Comment            string
}

type ManualBindingResult struct {
	Request *ManualBindingRequest `json:"request"`
	Binding *SKUBinding           `json:"binding,omitempty"`
}

func (s *ManualBindingService) CreateOrGetPendingRequest(ctx context.Context, request *ManualBindingRequest) (*ManualBindingRequest, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("manual binding service: db is nil")
	}
	return NewManualBindingRequestRepository(s.DB).Create(ctx, request)
}

func (s *ManualBindingService) ConfirmBinding(ctx context.Context, input ConfirmManualBindingInput) (*ManualBindingResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("manual binding service: db is nil")
	}
	if err := s.authorize(ctx, input.Actor, input.RequestID); err != nil {
		if auditErr := s.auditPermissionDenied(ctx, input.Actor, input.RequestID, input.CorrelationID); auditErr != nil {
			return nil, auditErr
		}
		return nil, err
	}
	if validateTenantID(input.Actor.TenantID) != nil || input.Actor.ActorID == zeroUUID || input.RequestID == zeroUUID || input.ExpectedRevision < 1 || input.SelectedLocalSKUID == zeroUUID {
		return nil, ErrValidation
	}
	input.Comment = safeManualBindingComment(input.Comment)
	input.IdempotencyKeyHash = normalizeString(input.IdempotencyKeyHash)
	if err := validateHashField(input.IdempotencyKeyHash, true); err != nil {
		return nil, err
	}
	payload := manualDecisionFingerprint("confirmed", input.Actor.ActorID, input.SelectedLocalSKUID, ReasonExistingConfirmedBinding, input.Comment)
	var result ManualBindingResult
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		request, err := lockManualRequest(ctx, tx, input.Actor.TenantID, input.RequestID)
		if err != nil {
			return err
		}
		if existingDecision, err := getManualDecisionByIdempotency(ctx, tx, input.Actor.TenantID, input.RequestID, ManualBindingStatusConfirmed, input.IdempotencyKeyHash); err == nil {
			if existingDecision.PayloadFingerprint != payload {
				return ErrIdempotencyPayloadConflict
			}
			resolved, binding, err := loadManualBindingResult(ctx, tx, input.Actor.TenantID, input.RequestID)
			if err != nil {
				return err
			}
			result = ManualBindingResult{Request: resolved, Binding: binding}
			return nil
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
		if request.Revision != input.ExpectedRevision {
			return ErrRevisionConflict
		}
		if request.Status != ManualBindingStatusPending {
			return ErrManualBindingAlreadyResolved
		}
		localProductID, err := localSKUProductID(ctx, tx, input.Actor.TenantID, input.SelectedLocalSKUID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrCandidateLocalSKUNotFound
			}
			if errors.Is(err, ErrTenantMismatch) {
				return ErrCandidateLocalSKUTenantMismatch
			}
			return err
		}
		if current, err := getCurrentConfirmedWithDB(ctx, tx, input.Actor.TenantID, request.ShopConnectionID, request.ExternalSKUID); err == nil && current.LocalSKUID != input.SelectedLocalSKUID {
			return ErrBindingConflict
		} else if err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
		binding := &SKUBinding{
			TenantID:           request.TenantID,
			ShopConnectionID:   request.ShopConnectionID,
			Platform:           PlatformDouyin,
			ExternalProductID:  snapshotExternalProductID(ctx, tx, request),
			ExternalSKUID:      request.ExternalSKUID,
			LocalProductID:     localProductID,
			LocalSKUID:         input.SelectedLocalSKUID,
			BindingSource:      SKUBindingSourceManual,
			BindingStatus:      SKUBindingStatusConfirmed,
			Confidence:         10000,
			CalibrationVersion: CalibrationVersionV1,
			CalibrationReason:  ReasonExistingConfirmedBinding,
			ConfirmedBy:        &input.Actor.ActorID,
			Revision:           1,
		}
		if err := validateSKUBinding(binding); err != nil {
			return err
		}
		if err := tx.Create(binding).Error; err != nil {
			if isUniqueViolation(err) {
				return stableError(err, ErrBindingConflict)
			}
			return stableError(err, ErrStateConflict)
		}
		updated, err := resolveManualRequestWithDB(ctx, tx, request, ManualBindingResolutionPatch{
			Status:                 ManualBindingStatusConfirmed,
			ExpectedRevision:       input.ExpectedRevision,
			ResolvedBy:             input.Actor.ActorID,
			Resolution:             ReasonExistingConfirmedBinding,
			Comment:                input.Comment,
			SelectedLocalProductID: &localProductID,
			SelectedLocalSKUID:     &input.SelectedLocalSKUID,
		})
		if err != nil {
			return err
		}
		decision := &ManualBindingDecision{
			TenantID:               input.Actor.TenantID,
			ManualBindingRequestID: input.RequestID,
			Operation:              ManualBindingStatusConfirmed,
			IdempotencyKeyHash:     input.IdempotencyKeyHash,
			PayloadFingerprint:     payload,
			ActorID:                input.Actor.ActorID,
			SelectedLocalProductID: &localProductID,
			SelectedLocalSKUID:     &input.SelectedLocalSKUID,
			ReasonCode:             ReasonExistingConfirmedBinding,
			Comment:                input.Comment,
			RequestRevision:        input.ExpectedRevision,
		}
		if err := createManualDecision(ctx, tx, decision); err != nil {
			return err
		}
		if err := s.writeAuditWithDB(ctx, tx, InventorySyncAuditEvent{TenantID: input.Actor.TenantID, ActorID: input.Actor.ActorID, Action: "sku_binding.manual_confirmed", Resource: inventorySyncAuditResourceManualBinding, ResourceID: input.RequestID.String(), ShopID: request.ShopConnectionID, Platform: PlatformDouyin, Permission: adminperm.PermSKUBindingResolveManual, Status: inventorySyncAuditStatusSuccess, RequestID: input.CorrelationID, Metadata: map[string]any{"bindingStatusBefore": ManualBindingStatusPending, "bindingStatusAfter": ManualBindingStatusConfirmed, "bindingSource": SKUBindingSourceManual, "localSkuId": input.SelectedLocalSKUID.String(), "externalSkuId": request.ExternalSKUID, "reasonCodes": []string{ReasonExistingConfirmedBinding}}}); err != nil {
			return err
		}
		result = ManualBindingResult{Request: updated, Binding: binding}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *ManualBindingService) RejectBinding(ctx context.Context, input RejectManualBindingInput) (*ManualBindingResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("manual binding service: db is nil")
	}
	if err := s.authorize(ctx, input.Actor, input.RequestID); err != nil {
		if auditErr := s.auditPermissionDenied(ctx, input.Actor, input.RequestID, input.CorrelationID); auditErr != nil {
			return nil, auditErr
		}
		return nil, err
	}
	input.ReasonCode = normalizeString(input.ReasonCode)
	input.Comment = safeManualBindingComment(input.Comment)
	input.IdempotencyKeyHash = normalizeString(input.IdempotencyKeyHash)
	if validateTenantID(input.Actor.TenantID) != nil || input.Actor.ActorID == zeroUUID || input.RequestID == zeroUUID || input.ExpectedRevision < 1 || input.ReasonCode == "" {
		return nil, ErrValidation
	}
	if err := validateHashField(input.IdempotencyKeyHash, true); err != nil {
		return nil, err
	}
	payload := manualDecisionFingerprint("rejected", input.Actor.ActorID, zeroUUID, input.ReasonCode, input.Comment)
	var result ManualBindingResult
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		request, err := lockManualRequest(ctx, tx, input.Actor.TenantID, input.RequestID)
		if err != nil {
			return err
		}
		if existingDecision, err := getManualDecisionByIdempotency(ctx, tx, input.Actor.TenantID, input.RequestID, ManualBindingStatusRejected, input.IdempotencyKeyHash); err == nil {
			if existingDecision.PayloadFingerprint != payload {
				return ErrIdempotencyPayloadConflict
			}
			resolved, _, err := loadManualBindingResult(ctx, tx, input.Actor.TenantID, input.RequestID)
			if err != nil {
				return err
			}
			result = ManualBindingResult{Request: resolved}
			return nil
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
		if request.Revision != input.ExpectedRevision {
			return ErrRevisionConflict
		}
		if request.Status != ManualBindingStatusPending {
			return ErrManualBindingAlreadyResolved
		}
		updated, err := resolveManualRequestWithDB(ctx, tx, request, ManualBindingResolutionPatch{
			Status:           ManualBindingStatusRejected,
			ExpectedRevision: input.ExpectedRevision,
			ResolvedBy:       input.Actor.ActorID,
			Resolution:       input.ReasonCode,
			Comment:          input.Comment,
		})
		if err != nil {
			return err
		}
		decision := &ManualBindingDecision{
			TenantID:               input.Actor.TenantID,
			ManualBindingRequestID: input.RequestID,
			Operation:              ManualBindingStatusRejected,
			IdempotencyKeyHash:     input.IdempotencyKeyHash,
			PayloadFingerprint:     payload,
			ActorID:                input.Actor.ActorID,
			ReasonCode:             input.ReasonCode,
			Comment:                input.Comment,
			RequestRevision:        input.ExpectedRevision,
		}
		if err := createManualDecision(ctx, tx, decision); err != nil {
			return err
		}
		if err := s.writeAuditWithDB(ctx, tx, InventorySyncAuditEvent{TenantID: input.Actor.TenantID, ActorID: input.Actor.ActorID, Action: "sku_binding.manual_rejected", Resource: inventorySyncAuditResourceManualBinding, ResourceID: input.RequestID.String(), ShopID: request.ShopConnectionID, Platform: PlatformDouyin, Permission: adminperm.PermSKUBindingResolveManual, Status: inventorySyncAuditStatusSuccess, RequestID: input.CorrelationID, Metadata: map[string]any{"bindingStatusBefore": ManualBindingStatusPending, "bindingStatusAfter": ManualBindingStatusRejected, "bindingSource": SKUBindingSourceManual, "externalSkuId": request.ExternalSKUID, "reasonCodes": []string{input.ReasonCode}}}); err != nil {
			return err
		}
		result = ManualBindingResult{Request: updated}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *ManualBindingService) auditWithDB(db *gorm.DB) *InventorySyncAuditService {
	if s.Audit == nil {
		return nil
	}
	return s.Audit.WithDB(db)
}

func (s *ManualBindingService) writeAuditWithDB(ctx context.Context, db *gorm.DB, event InventorySyncAuditEvent) error {
	audit := s.auditWithDB(db)
	if audit == nil {
		return ErrStateConflict
	}
	return audit.Write(ctx, event)
}

func (s *ManualBindingService) auditPermissionDenied(ctx context.Context, actor ManualBindingActor, requestID uuid.UUID, correlationID string) error {
	audit := s.auditWithDB(s.DB)
	if audit == nil {
		return ErrStateConflict
	}
	return audit.Write(ctx, InventorySyncAuditEvent{TenantID: actor.TenantID, ActorID: actor.ActorID, Action: "inventory_sync.permission_denied", Resource: inventorySyncAuditResourceManualBinding, ResourceID: requestID.String(), Permission: adminperm.PermSKUBindingResolveManual, Status: inventorySyncAuditStatusDenied, RequestID: correlationID, Metadata: map[string]any{"errorCode": ErrCodePermissionDenied, "safeMessage": ErrCodePermissionDenied, "reasonCodes": []string{ErrCodePermissionDenied}, "stage": "manual_binding.resolve"}})
}

func (s *ManualBindingService) authorize(ctx context.Context, actor ManualBindingActor, requestID uuid.UUID) error {
	if s.Authorizer == nil {
		return ErrPermissionDenied
	}
	if validateTenantID(actor.TenantID) != nil || actor.ActorID == zeroUUID || requestID == zeroUUID {
		return ErrValidation
	}
	if err := s.Authorizer.CanResolveManualBinding(ctx, actor.TenantID, actor.ActorID, requestID); err != nil {
		return ErrPermissionDenied
	}
	return nil
}

func lockManualRequest(ctx context.Context, db *gorm.DB, tenantID int64, requestID uuid.UUID) (*ManualBindingRequest, error) {
	var request ManualBindingRequest
	if err := db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, requestID).First(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrStateConflict)
	}
	return &request, nil
}

func resolveManualRequestWithDB(ctx context.Context, db *gorm.DB, request *ManualBindingRequest, patch ManualBindingResolutionPatch) (*ManualBindingRequest, error) {
	repo := NewManualBindingRequestRepository(db)
	return repo.ResolveWithRevision(ctx, request.TenantID, request.ID, patch)
}

func createManualDecision(ctx context.Context, db *gorm.DB, decision *ManualBindingDecision) error {
	if err := validateManualBindingDecision(decision); err != nil {
		return err
	}
	if err := db.WithContext(ctx).Create(decision).Error; err != nil {
		if isUniqueViolation(err) {
			return stableError(err, ErrIdempotencyPayloadConflict)
		}
		return stableError(err, ErrStateConflict)
	}
	return nil
}

func getManualDecisionByIdempotency(ctx context.Context, db *gorm.DB, tenantID int64, requestID uuid.UUID, operation string, keyHash string) (*ManualBindingDecision, error) {
	var decision ManualBindingDecision
	if err := db.WithContext(ctx).Where("tenant_id = ? AND manual_binding_request_id = ? AND operation = ? AND idempotency_key_hash = ?", tenantID, requestID, operation, keyHash).First(&decision).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrStateConflict)
	}
	return &decision, nil
}

func getCurrentConfirmedWithDB(ctx context.Context, db *gorm.DB, tenantID int64, shopConnectionID uuid.UUID, externalSKUID string) (*SKUBinding, error) {
	var binding SKUBinding
	if err := db.WithContext(ctx).Where("tenant_id = ? AND shop_connection_id = ? AND external_sku_id = ? AND binding_status = ?", tenantID, shopConnectionID, externalSKUID, SKUBindingStatusConfirmed).First(&binding).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrStateConflict)
	}
	return &binding, nil
}

func loadManualBindingResult(ctx context.Context, db *gorm.DB, tenantID int64, requestID uuid.UUID) (*ManualBindingRequest, *SKUBinding, error) {
	request, err := lockManualRequest(ctx, db, tenantID, requestID)
	if err != nil {
		return nil, nil, err
	}
	if request.SelectedLocalSKUID == nil {
		return request, nil, nil
	}
	var binding SKUBinding
	if err := db.WithContext(ctx).Where("tenant_id = ? AND shop_connection_id = ? AND external_sku_id = ? AND local_sku_id = ? AND binding_status = ?", tenantID, request.ShopConnectionID, request.ExternalSKUID, *request.SelectedLocalSKUID, SKUBindingStatusConfirmed).First(&binding).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return request, nil, nil
		}
		return nil, nil, stableError(err, ErrStateConflict)
	}
	return request, &binding, nil
}

func snapshotExternalProductID(ctx context.Context, db *gorm.DB, request *ManualBindingRequest) string {
	var snapshot InventorySnapshotItem
	if err := db.WithContext(ctx).Select("external_product_id").Where("tenant_id = ? AND id = ?", request.TenantID, request.InventorySnapshotItemID).First(&snapshot).Error; err != nil {
		return request.ExternalSKUID
	}
	return snapshot.ExternalProductID
}

func manualDecisionFingerprint(operation string, actorID uuid.UUID, selectedLocalSKUID uuid.UUID, reasonCode string, comment string) string {
	payload := struct {
		Operation          string `json:"operation"`
		ActorID            string `json:"actorId"`
		SelectedLocalSKUID string `json:"selectedLocalSkuId"`
		ReasonCode         string `json:"reasonCode"`
		Comment            string `json:"comment"`
	}{
		Operation:          operation,
		ActorID:            actorID.String(),
		SelectedLocalSKUID: selectedLocalSKUID.String(),
		ReasonCode:         normalizeString(reasonCode),
		Comment:            strings.TrimSpace(comment),
	}
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
