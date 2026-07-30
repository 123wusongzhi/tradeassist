package inventorysyncp9

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	PlatformDouyin = "douyin"
)

const (
	ProviderModeMock           = "mock"
	ProviderModeSandbox        = "sandbox"
	ProviderModeLocalDraftOnly = "local_draft_only"
)

const (
	InventorySyncRunStatusPending   = "pending"
	InventorySyncRunStatusRunning   = "running"
	InventorySyncRunStatusSucceeded = "succeeded"
	InventorySyncRunStatusFailed    = "failed"
	InventorySyncRunStatusCancelled = "cancelled"
)

const (
	SKUBindingSourceAutomatic = "automatic"
	SKUBindingSourceManual    = "manual"
)

const (
	SKUBindingStatusProposed  = "proposed"
	SKUBindingStatusConfirmed = "confirmed"
	SKUBindingStatusRejected  = "rejected"
	SKUBindingStatusStale     = "stale"
	SKUBindingStatusConflict  = "conflict"
)

const (
	MatchStrategyExactSKUCode           = "exact_sku_code"
	MatchStrategyExactBarcode           = "exact_barcode"
	MatchStrategyNormalizedSKUCode      = "normalized_sku_code"
	MatchStrategyNormalizedBarcode      = "normalized_barcode"
	MatchStrategyNormalizedTitleVariant = "normalized_title_variant"
	MatchStrategyCompositeMatch         = "composite_match"
	MatchStrategyManual                 = "manual"
)

const (
	CalibrationStatusCandidate = "candidate"
	CalibrationStatusSelected  = "selected"
	CalibrationStatusRejected  = "rejected"
	CalibrationStatusConflict  = "conflict"
)

const (
	ManualBindingStatusPending   = "pending"
	ManualBindingStatusConfirmed = "confirmed"
	ManualBindingStatusRejected  = "rejected"
	ManualBindingStatusCancelled = "cancelled"
)

type InventorySyncRun struct {
	model.HardDeleteBase
	TenantID              int64          `gorm:"not null;index:idx_p9_inventory_sync_runs_tenant_shop_status,priority:1" json:"tenantId"`
	ShopConnectionID      uuid.UUID      `gorm:"type:char(36);not null;index:idx_p9_inventory_sync_runs_tenant_shop_status,priority:2" json:"shopConnectionId"`
	ShopConnection        shop.Shop      `gorm:"foreignKey:ShopConnectionID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	Platform              string         `gorm:"size:64;not null;index:idx_p9_inventory_sync_runs_tenant_shop_status,priority:3" json:"platform"`
	ProviderMode          string         `gorm:"size:32;not null" json:"providerMode"`
	ExternalShopReference string         `gorm:"size:255" json:"externalShopReference,omitempty"`
	Status                string         `gorm:"size:32;not null;index:idx_p9_inventory_sync_runs_tenant_shop_status,priority:4" json:"status"`
	Cursor                datatypes.JSON `gorm:"type:jsonb" json:"cursor,omitempty"`
	Checkpoint            datatypes.JSON `gorm:"type:jsonb" json:"checkpoint,omitempty"`
	SafeErrorMetadata     datatypes.JSON `gorm:"type:jsonb" json:"safeErrorMetadata,omitempty"`
	SnapshotCount         int            `gorm:"not null;default:0" json:"snapshotCount"`
	CalibrationCount      int            `gorm:"not null;default:0" json:"calibrationCount"`
	ManualRequestCount    int            `gorm:"not null;default:0" json:"manualRequestCount"`
	RequestID             string         `gorm:"size:128" json:"requestId,omitempty"`
	IdempotencyKeyHash    string         `gorm:"size:64" json:"idempotencyKeyHash,omitempty"`
	InputFingerprint      string         `gorm:"size:64" json:"inputFingerprint,omitempty"`
	RerunOfRunID          *uuid.UUID     `gorm:"type:char(36);index" json:"rerunOfRunId,omitempty"`
	RerunSourceRevision   int            `gorm:"not null;default:0" json:"rerunSourceRevision,omitempty"`
	Revision              int            `gorm:"not null;default:1" json:"revision"`
	StartedAt             *time.Time     `json:"startedAt,omitempty"`
	FinishedAt            *time.Time     `json:"finishedAt,omitempty"`
}

func (InventorySyncRun) TableName() string { return "p9_inventory_sync_runs" }

type InventorySnapshotItem struct {
	model.HardDeleteBase
	TenantID            int64            `gorm:"not null;uniqueIndex:ux_p9_inventory_snapshots_tenant_run_external_sku,priority:1;index:idx_p9_inventory_snapshots_tenant_run,priority:1" json:"tenantId"`
	InventorySyncRunID  uuid.UUID        `gorm:"type:char(36);not null;uniqueIndex:ux_p9_inventory_snapshots_tenant_run_external_sku,priority:2;index:idx_p9_inventory_snapshots_tenant_run,priority:2" json:"inventorySyncRunId"`
	InventorySyncRun    InventorySyncRun `gorm:"foreignKey:InventorySyncRunID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	ShopConnectionID    uuid.UUID        `gorm:"type:char(36);not null;index:idx_p9_inventory_snapshots_tenant_shop,priority:2" json:"shopConnectionId"`
	ShopConnection      shop.Shop        `gorm:"foreignKey:ShopConnectionID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	Platform            string           `gorm:"size:64;not null" json:"platform"`
	ExternalProductID   string           `gorm:"column:external_product_id;size:255;not null;index" json:"externalProductId"`
	ExternalSKUID       string           `gorm:"column:external_sku_id;size:255;not null;uniqueIndex:ux_p9_inventory_snapshots_tenant_run_external_sku,priority:3;index" json:"externalSkuId"`
	ExternalProductCode string           `gorm:"column:external_product_code;size:255" json:"externalProductCode,omitempty"`
	ExternalSKUCode     string           `gorm:"column:external_sku_code;size:255;index" json:"externalSkuCode,omitempty"`
	Barcode             string           `gorm:"size:255;index" json:"barcode,omitempty"`
	ProductTitle        string           `gorm:"size:512" json:"productTitle,omitempty"`
	VariantTitle        string           `gorm:"size:512" json:"variantTitle,omitempty"`
	AvailableQuantity   int              `gorm:"not null" json:"availableQuantity"`
	ReservedQuantity    int              `gorm:"not null" json:"reservedQuantity"`
	TotalQuantity       int              `gorm:"not null" json:"totalQuantity"`
	SourceUpdatedAt     *time.Time       `json:"sourceUpdatedAt,omitempty"`
	ObservedAt          time.Time        `gorm:"not null;index" json:"observedAt"`
	PayloadHash         string           `gorm:"size:64;not null" json:"payloadHash"`
	SafeMetadata        datatypes.JSON   `gorm:"type:jsonb" json:"safeMetadata,omitempty"`
}

func (InventorySnapshotItem) TableName() string              { return "p9_inventory_snapshot_items" }
func (InventorySnapshotItem) BeforeUpdate(tx *gorm.DB) error { return ErrImmutableRecord }
func (InventorySnapshotItem) BeforeDelete(tx *gorm.DB) error { return ErrImmutableRecord }

type SKUBinding struct {
	model.HardDeleteBase
	TenantID           int64              `gorm:"not null;index:idx_p9_sku_bindings_tenant_external,priority:1;index:idx_p9_sku_bindings_tenant_local,priority:1" json:"tenantId"`
	ShopConnectionID   uuid.UUID          `gorm:"type:char(36);not null;index:idx_p9_sku_bindings_tenant_external,priority:2" json:"shopConnectionId"`
	ShopConnection     shop.Shop          `gorm:"foreignKey:ShopConnectionID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	Platform           string             `gorm:"size:64;not null" json:"platform"`
	ExternalProductID  string             `gorm:"column:external_product_id;size:255;not null" json:"externalProductId"`
	ExternalSKUID      string             `gorm:"column:external_sku_id;size:255;not null;index:idx_p9_sku_bindings_tenant_external,priority:3" json:"externalSkuId"`
	ExternalSKUCode    string             `gorm:"column:external_sku_code;size:255;index" json:"externalSkuCode,omitempty"`
	LocalProductID     uuid.UUID          `gorm:"column:local_product_id;type:char(36);not null;index:idx_p9_sku_bindings_tenant_local,priority:2" json:"localProductId"`
	LocalProduct       product.Product    `gorm:"foreignKey:LocalProductID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	LocalSKUID         uuid.UUID          `gorm:"column:local_sku_id;type:char(36);not null;index:idx_p9_sku_bindings_tenant_local,priority:3" json:"localSkuId"`
	LocalSKU           product.ProductSKU `gorm:"foreignKey:LocalSKUID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	BindingSource      string             `gorm:"size:32;not null" json:"bindingSource"`
	BindingStatus      string             `gorm:"size:32;not null;index" json:"bindingStatus"`
	Confidence         int                `gorm:"not null;default:0" json:"confidence"`
	CalibrationVersion int                `gorm:"not null;default:1" json:"calibrationVersion"`
	CalibrationReason  string             `gorm:"type:text" json:"calibrationReason,omitempty"`
	ConfirmedBy        *uuid.UUID         `gorm:"type:char(36);index" json:"confirmedBy,omitempty"`
	ConfirmedAt        *time.Time         `json:"confirmedAt,omitempty"`
	Revision           int                `gorm:"not null;default:1" json:"revision"`
}

func (SKUBinding) TableName() string { return "p9_sku_bindings" }

type SKUBindingCalibration struct {
	model.HardDeleteBase
	TenantID                int64                 `gorm:"not null;uniqueIndex:ux_p9_sku_calibrations_candidate_version,priority:1;index:idx_p9_sku_calibrations_tenant_run,priority:1" json:"tenantId"`
	InventorySyncRunID      uuid.UUID             `gorm:"type:char(36);not null;uniqueIndex:ux_p9_sku_calibrations_candidate_version,priority:2;index:idx_p9_sku_calibrations_tenant_run,priority:2" json:"inventorySyncRunId"`
	InventorySyncRun        InventorySyncRun      `gorm:"foreignKey:InventorySyncRunID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	InventorySnapshotItemID uuid.UUID             `gorm:"type:char(36);not null;uniqueIndex:ux_p9_sku_calibrations_candidate_version,priority:3;index:idx_p9_sku_calibrations_snapshot,priority:2" json:"inventorySnapshotItemId"`
	InventorySnapshotItem   InventorySnapshotItem `gorm:"foreignKey:InventorySnapshotItemID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	ExternalSKUID           string                `gorm:"column:external_sku_id;size:255;not null;index" json:"externalSkuId"`
	CandidateLocalProductID uuid.UUID             `gorm:"column:candidate_local_product_id;type:char(36);not null" json:"candidateLocalProductId"`
	CandidateLocalProduct   product.Product       `gorm:"foreignKey:CandidateLocalProductID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	CandidateLocalSKUID     uuid.UUID             `gorm:"column:candidate_local_sku_id;type:char(36);not null;uniqueIndex:ux_p9_sku_calibrations_candidate_version,priority:4" json:"candidateLocalSkuId"`
	CandidateLocalSKU       product.ProductSKU    `gorm:"foreignKey:CandidateLocalSKUID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	MatchStrategy           string                `gorm:"size:64;not null;index" json:"matchStrategy"`
	Confidence              int                   `gorm:"not null" json:"confidence"`
	ScoreBreakdown          datatypes.JSON        `gorm:"type:jsonb;not null" json:"scoreBreakdown"`
	ReasonCodes             datatypes.JSON        `gorm:"type:jsonb;not null" json:"reasonCodes"`
	CalibrationVersion      int                   `gorm:"not null;default:1;uniqueIndex:ux_p9_sku_calibrations_candidate_version,priority:5" json:"calibrationVersion"`
	Status                  string                `gorm:"size:32;not null;index" json:"status"`
	InputFingerprint        string                `gorm:"size:64;not null" json:"inputFingerprint"`
}

func (SKUBindingCalibration) TableName() string              { return "p9_sku_binding_calibrations" }
func (SKUBindingCalibration) BeforeUpdate(tx *gorm.DB) error { return ErrImmutableRecord }
func (SKUBindingCalibration) BeforeDelete(tx *gorm.DB) error { return ErrImmutableRecord }

type ManualBindingRequest struct {
	model.HardDeleteBase
	TenantID                int64                 `gorm:"not null;index:idx_p9_manual_binding_requests_pending,priority:1;uniqueIndex:ux_p9_manual_binding_requests_tenant_request_id,priority:1" json:"tenantId"`
	InventorySyncRunID      uuid.UUID             `gorm:"type:char(36);not null;index:idx_p9_manual_binding_requests_run,priority:2" json:"inventorySyncRunId"`
	InventorySyncRun        InventorySyncRun      `gorm:"foreignKey:InventorySyncRunID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	InventorySnapshotItemID uuid.UUID             `gorm:"type:char(36);not null" json:"inventorySnapshotItemId"`
	InventorySnapshotItem   InventorySnapshotItem `gorm:"foreignKey:InventorySnapshotItemID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	ShopConnectionID        uuid.UUID             `gorm:"type:char(36);not null;index:idx_p9_manual_binding_requests_pending,priority:2" json:"shopConnectionId"`
	ShopConnection          shop.Shop             `gorm:"foreignKey:ShopConnectionID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	ExternalSKUID           string                `gorm:"column:external_sku_id;size:255;not null;index:idx_p9_manual_binding_requests_pending,priority:3" json:"externalSkuId"`
	Status                  string                `gorm:"size:32;not null;index:idx_p9_manual_binding_requests_pending,priority:4" json:"status"`
	ReasonCode              string                `gorm:"size:128;not null" json:"reasonCode"`
	CandidateCount          int                   `gorm:"not null;default:0" json:"candidateCount"`
	SuggestedLocalSKUID     *uuid.UUID            `gorm:"column:suggested_local_sku_id;type:char(36);index" json:"suggestedLocalSkuId,omitempty"`
	AssignedTo              *uuid.UUID            `gorm:"type:char(36);index" json:"assignedTo,omitempty"`
	ResolvedBy              *uuid.UUID            `gorm:"type:char(36);index" json:"resolvedBy,omitempty"`
	ResolvedAt              *time.Time            `json:"resolvedAt,omitempty"`
	Resolution              string                `gorm:"type:text" json:"resolution,omitempty"`
	SelectedLocalProductID  *uuid.UUID            `gorm:"column:selected_local_product_id;type:char(36);index" json:"selectedLocalProductId,omitempty"`
	SelectedLocalSKUID      *uuid.UUID            `gorm:"column:selected_local_sku_id;type:char(36);index" json:"selectedLocalSkuId,omitempty"`
	Comment                 string                `gorm:"type:text" json:"comment,omitempty"`
	RequestID               string                `gorm:"size:128;not null;uniqueIndex:ux_p9_manual_binding_requests_tenant_request_id,priority:2" json:"requestId"`
	IdempotencyKeyHash      string                `gorm:"size:64" json:"idempotencyKeyHash,omitempty"`
	InputFingerprint        string                `gorm:"size:64;not null" json:"inputFingerprint"`
	Revision                int                   `gorm:"not null;default:1" json:"revision"`
}

func (ManualBindingRequest) TableName() string { return "p9_manual_binding_requests" }

type ManualBindingDecision struct {
	model.HardDeleteBase
	TenantID               int64                `gorm:"not null;uniqueIndex:ux_p9_manual_binding_decisions_idempotency,priority:1;index:idx_p9_manual_binding_decisions_request,priority:1" json:"tenantId"`
	ManualBindingRequestID uuid.UUID            `gorm:"type:char(36);not null;uniqueIndex:ux_p9_manual_binding_decisions_idempotency,priority:2;index:idx_p9_manual_binding_decisions_request,priority:2" json:"manualBindingRequestId"`
	ManualBindingRequest   ManualBindingRequest `gorm:"foreignKey:ManualBindingRequestID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	Operation              string               `gorm:"size:32;not null;uniqueIndex:ux_p9_manual_binding_decisions_idempotency,priority:3" json:"operation"`
	IdempotencyKeyHash     string               `gorm:"size:64;not null;uniqueIndex:ux_p9_manual_binding_decisions_idempotency,priority:4" json:"idempotencyKeyHash"`
	PayloadFingerprint     string               `gorm:"size:64;not null" json:"payloadFingerprint"`
	ActorID                uuid.UUID            `gorm:"type:char(36);not null;index" json:"actorId"`
	SelectedLocalProductID *uuid.UUID           `gorm:"column:selected_local_product_id;type:char(36);index" json:"selectedLocalProductId,omitempty"`
	SelectedLocalSKUID     *uuid.UUID           `gorm:"column:selected_local_sku_id;type:char(36);index" json:"selectedLocalSkuId,omitempty"`
	ReasonCode             string               `gorm:"size:128;not null" json:"reasonCode"`
	Comment                string               `gorm:"type:text" json:"comment,omitempty"`
	RequestRevision        int                  `gorm:"not null" json:"requestRevision"`
}

func (ManualBindingDecision) TableName() string              { return "p9_manual_binding_decisions" }
func (ManualBindingDecision) BeforeUpdate(tx *gorm.DB) error { return ErrImmutableRecord }
func (ManualBindingDecision) BeforeDelete(tx *gorm.DB) error { return ErrImmutableRecord }
