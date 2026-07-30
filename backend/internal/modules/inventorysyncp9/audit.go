package inventorysyncp9

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

const (
	inventorySyncAuditResourceRun           = "inventory_sync"
	inventorySyncAuditResourceManualBinding = "sku_binding"
	inventorySyncAuditStatusSuccess         = "success"
	inventorySyncAuditStatusFailed          = "failed"
	inventorySyncAuditStatusDenied          = "denied"
	inventorySyncAuditStatusBlocked         = "blocked"
)

type InventorySyncAuditService struct {
	Log *operationlog.Service
}

type InventorySyncAuditEvent struct {
	TenantID   int64
	ActorID    uuid.UUID
	Action     string
	Resource   string
	ResourceID string
	ShopID     uuid.UUID
	Platform   string
	Permission string
	Status     string
	RequestID  string
	Metadata   map[string]any
}

func NewInventorySyncAuditService(db *gorm.DB) *InventorySyncAuditService {
	if db == nil {
		return nil
	}
	return &InventorySyncAuditService{Log: &operationlog.Service{DB: db}}
}

func (s *InventorySyncAuditService) WithDB(db *gorm.DB) *InventorySyncAuditService {
	if db == nil {
		return s
	}
	return NewInventorySyncAuditService(db)
}

func (s *InventorySyncAuditService) Write(ctx context.Context, event InventorySyncAuditEvent) error {
	if s == nil || s.Log == nil || s.Log.DB == nil {
		return ErrStateConflict
	}
	if validateTenantID(event.TenantID) != nil || strings.TrimSpace(event.Action) == "" || strings.TrimSpace(event.Resource) == "" || strings.TrimSpace(event.Status) == "" {
		return ErrValidation
	}
	message, err := safeAuditMessage(event.Metadata)
	if err != nil {
		return err
	}
	var actor *uuid.UUID
	if event.ActorID != zeroUUID {
		actor = &event.ActorID
	}
	var shopID *uuid.UUID
	if event.ShopID != zeroUUID {
		shopID = &event.ShopID
	}
	return s.Log.WriteBackground(ctx, operationlog.WriteOpts{
		TenantID:    event.TenantID,
		AdminUserID: actor,
		Action:      strings.TrimSpace(event.Action),
		Resource:    strings.TrimSpace(event.Resource),
		ResourceID:  strings.TrimSpace(event.ResourceID),
		ShopID:      shopID,
		Platform:    strings.TrimSpace(event.Platform),
		Permission:  strings.TrimSpace(event.Permission),
		RequestID:   normalizeString(event.RequestID),
		Status:      strings.TrimSpace(event.Status),
		Message:     message,
	})
}

func (s *InventorySyncAuditService) PermissionDenied(ctx context.Context, tenantID int64, actorID uuid.UUID, action string, permission string, requestID string) error {
	return s.Write(ctx, InventorySyncAuditEvent{
		TenantID:   tenantID,
		ActorID:    actorID,
		Action:     "inventory_sync.permission_denied",
		Resource:   inventorySyncAuditResourceRun,
		Permission: permission,
		Status:     inventorySyncAuditStatusDenied,
		RequestID:  requestID,
		Metadata: map[string]any{
			"errorCode":   ErrCodePermissionDenied,
			"safeMessage": ErrCodePermissionDenied,
			"reasonCodes": []string{ErrCodePermissionDenied},
			"stage":       strings.TrimSpace(action),
			"requestId":   normalizeString(requestID),
		},
	})
}

func (s *InventorySyncAuditService) ProductionCapabilityBlocked(ctx context.Context, input InventorySyncOrchestratorInput, err error) error {
	return s.Write(ctx, InventorySyncAuditEvent{
		TenantID:   input.TenantID,
		ActorID:    input.ActorID,
		Action:     "inventory_sync.production_capability_blocked",
		Resource:   inventorySyncAuditResourceRun,
		ShopID:     input.ShopConnectionID,
		Platform:   input.Platform,
		Permission: adminperm.PermInventorySyncRun,
		Status:     inventorySyncAuditStatusBlocked,
		RequestID:  input.RequestID,
		Metadata: map[string]any{
			"platform":          input.Platform,
			"providerMode":      input.ProviderMode,
			"fixtureScenario":   input.FixtureScenario,
			"capabilityBlocked": true,
			"errorCode":         providerErrorCode(err),
			"safeMessage":       providerErrorCode(err),
			"stage":             "provider_capability_guard",
			"requestId":         normalizeString(input.RequestID),
		},
	})
}

func safeAuditMessage(meta map[string]any) (string, error) {
	payload, err := safeInventorySyncMetadataJSON(meta)
	if err != nil {
		return "", err
	}
	if len(payload) == 0 {
		return `{}`, nil
	}
	var compact map[string]any
	if err := json.Unmarshal(payload, &compact); err != nil {
		return "", ErrValidation
	}
	encoded, err := json.Marshal(compact)
	if err != nil {
		return "", ErrValidation
	}
	return string(encoded), nil
}
