package inventorysyncp9

import (
	"time"

	"github.com/google/uuid"
)

type APIActor struct {
	TenantID int64
	ActorID  uuid.UUID
	Role     string
}

type CreateInventorySyncRunRequest struct {
	ShopConnectionID uuid.UUID `json:"shopConnectionId"`
	Platform         string    `json:"platform"`
	ProviderMode     string    `json:"providerMode"`
	FixtureScenario  string    `json:"fixtureScenario,omitempty"`
}

type RerunInventorySyncRequest struct {
	ExpectedRevision int `json:"expectedRevision"`
}

type RecalibrateSnapshotRequest struct {
	ExpectedCalibrationVersion int    `json:"expectedCalibrationVersion"`
	Reason                     string `json:"reason"`
}

type ConfirmManualBindingRequest struct {
	ExpectedRevision   int       `json:"expectedRevision"`
	SelectedLocalSKUID uuid.UUID `json:"selectedLocalSkuId"`
	Comment            string    `json:"comment,omitempty"`
}

type RejectManualBindingRequest struct {
	ExpectedRevision int    `json:"expectedRevision"`
	ReasonCode       string `json:"reasonCode"`
	Comment          string `json:"comment,omitempty"`
}

type InventorySyncRunListParams struct {
	ShopConnectionID *uuid.UUID
	Status           string
	ProviderMode     string
	Limit            int
	Cursor           string
}

type SnapshotListParams struct {
	BindingResult string
	Limit         int
	Cursor        string
}

type BindingListParams struct {
	ShopConnectionID *uuid.UUID
	BindingStatus    string
	BindingSource    string
	Limit            int
	Cursor           string
}

type ManualBindingListParams struct {
	ShopConnectionID *uuid.UUID
	Status           string
	Limit            int
	Cursor           string
}

type AuditEventListParams struct {
	Limit  int
	Cursor string
}

type CalibrationListParams struct {
	Limit  int
	Cursor string
}

type PageResponse[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
	HasMore    bool   `json:"hasMore"`
	Limit      int    `json:"limit"`
}

type InventorySyncStatisticsResponse struct {
	TotalRecordCount          int `json:"totalRecordCount"`
	MatchedRecordCount        int `json:"matchedRecordCount"`
	UnmatchedRecordCount      int `json:"unmatchedRecordCount"`
	ConflictRecordCount       int `json:"conflictRecordCount"`
	FailedRecordCount         int `json:"failedRecordCount"`
	ManualBindingRequestCount int `json:"manualBindingRequestCount"`
	ConfirmedBindingCount     int `json:"confirmedBindingCount"`
	PagesProcessed            int `json:"pagesProcessed"`
}

type SafeRunErrorResponse struct {
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	Retryable bool   `json:"retryable"`
}

type InventorySyncRunAllowedActions struct {
	CanViewSnapshots bool `json:"canViewSnapshots"`
	CanRerun         bool `json:"canRerun"`
	CanViewAudit     bool `json:"canViewAudit"`
}

type InventorySyncRunResponse struct {
	ID               uuid.UUID                       `json:"id"`
	ShopConnectionID uuid.UUID                       `json:"shopConnectionId"`
	Platform         string                          `json:"platform"`
	ProviderMode     string                          `json:"providerMode"`
	Status           string                          `json:"status"`
	TriggerType      string                          `json:"triggerType"`
	FixtureScenario  string                          `json:"fixtureScenario,omitempty"`
	RerunOfRunID     *uuid.UUID                      `json:"rerunOfRunId,omitempty"`
	Statistics       InventorySyncStatisticsResponse `json:"statistics"`
	SafeError        *SafeRunErrorResponse           `json:"safeError,omitempty"`
	CursorHash       string                          `json:"cursorHash,omitempty"`
	Revision         int                             `json:"revision"`
	StartedAt        *time.Time                      `json:"startedAt,omitempty"`
	FinishedAt       *time.Time                      `json:"finishedAt,omitempty"`
	CreatedAt        time.Time                       `json:"createdAt"`
	UpdatedAt        time.Time                       `json:"updatedAt"`
	AllowedActions   InventorySyncRunAllowedActions  `json:"allowedActions"`
}

type SnapshotBindingSummaryResponse struct {
	Result             string     `json:"result"`
	BindingID          *uuid.UUID `json:"bindingId,omitempty"`
	BindingStatus      string     `json:"bindingStatus,omitempty"`
	LocalProductID     *uuid.UUID `json:"localProductId,omitempty"`
	LocalSKUID         *uuid.UUID `json:"localSkuId,omitempty"`
	Confidence         int        `json:"confidence,omitempty"`
	CalibrationVersion int        `json:"calibrationVersion,omitempty"`
	ManualRequestID    *uuid.UUID `json:"manualRequestId,omitempty"`
}

type InventorySnapshotResponse struct {
	ID                  uuid.UUID                      `json:"id"`
	InventorySyncRunID  uuid.UUID                      `json:"inventorySyncRunId"`
	ShopConnectionID    uuid.UUID                      `json:"shopConnectionId"`
	Platform            string                         `json:"platform"`
	ExternalProductID   string                         `json:"externalProductId"`
	ExternalSKUID       string                         `json:"externalSkuId"`
	ExternalProductCode string                         `json:"externalProductCode,omitempty"`
	ExternalSKUCode     string                         `json:"externalSkuCode,omitempty"`
	Barcode             string                         `json:"barcode,omitempty"`
	ProductTitle        string                         `json:"productTitle,omitempty"`
	VariantTitle        string                         `json:"variantTitle,omitempty"`
	AvailableQuantity   int                            `json:"availableQuantity"`
	ReservedQuantity    int                            `json:"reservedQuantity"`
	TotalQuantity       int                            `json:"totalQuantity"`
	SourceUpdatedAt     *time.Time                     `json:"sourceUpdatedAt,omitempty"`
	ObservedAt          time.Time                      `json:"observedAt"`
	Binding             SnapshotBindingSummaryResponse `json:"binding"`
	CreatedAt           time.Time                      `json:"createdAt"`
}

type SKUBindingAllowedActions struct {
	CanViewHistory     bool `json:"canViewHistory"`
	CanViewCalibration bool `json:"canViewCalibration"`
}

type SKUBindingResponse struct {
	ID                 uuid.UUID                `json:"id"`
	ShopConnectionID   uuid.UUID                `json:"shopConnectionId"`
	Platform           string                   `json:"platform"`
	ExternalProductID  string                   `json:"externalProductId"`
	ExternalSKUID      string                   `json:"externalSkuId"`
	ExternalSKUCode    string                   `json:"externalSkuCode,omitempty"`
	LocalProductID     uuid.UUID                `json:"localProductId"`
	LocalSKUID         uuid.UUID                `json:"localSkuId"`
	BindingSource      string                   `json:"bindingSource"`
	BindingStatus      string                   `json:"bindingStatus"`
	Confidence         int                      `json:"confidence"`
	CalibrationVersion int                      `json:"calibrationVersion"`
	CalibrationReason  string                   `json:"calibrationReason,omitempty"`
	ConfirmedBy        *uuid.UUID               `json:"confirmedBy,omitempty"`
	ConfirmedAt        *time.Time               `json:"confirmedAt,omitempty"`
	Revision           int                      `json:"revision"`
	CreatedAt          time.Time                `json:"createdAt"`
	UpdatedAt          time.Time                `json:"updatedAt"`
	AllowedActions     SKUBindingAllowedActions `json:"allowedActions"`
}

type ScoreBreakdownResponse struct {
	Signal string `json:"signal"`
	Score  int    `json:"score"`
}

type CalibrationResponse struct {
	ID                      uuid.UUID                `json:"id"`
	InventorySyncRunID      uuid.UUID                `json:"inventorySyncRunId"`
	InventorySnapshotItemID uuid.UUID                `json:"inventorySnapshotItemId"`
	ExternalSKUID           string                   `json:"externalSkuId"`
	CandidateLocalProductID uuid.UUID                `json:"candidateLocalProductId"`
	CandidateLocalSKUID     uuid.UUID                `json:"candidateLocalSkuId"`
	MatchStrategy           string                   `json:"matchStrategy"`
	Confidence              int                      `json:"confidence"`
	ScoreBreakdown          []ScoreBreakdownResponse `json:"scoreBreakdown"`
	ReasonCodes             []string                 `json:"reasonCodes"`
	CalibrationVersion      int                      `json:"calibrationVersion"`
	Status                  string                   `json:"status"`
	CreatedAt               time.Time                `json:"createdAt"`
}

type RecalibrationResponse struct {
	SnapshotID         uuid.UUID             `json:"snapshotId"`
	CalibrationVersion int                   `json:"calibrationVersion"`
	Candidates         []CalibrationResponse `json:"candidates"`
}

type ManualBindingAllowedActions struct {
	CanConfirm bool `json:"canConfirm"`
	CanReject  bool `json:"canReject"`
}

type ManualBindingRequestResponse struct {
	ID                      uuid.UUID                   `json:"id"`
	InventorySyncRunID      uuid.UUID                   `json:"inventorySyncRunId"`
	InventorySnapshotItemID uuid.UUID                   `json:"inventorySnapshotItemId"`
	ShopConnectionID        uuid.UUID                   `json:"shopConnectionId"`
	ExternalSKUID           string                      `json:"externalSkuId"`
	Status                  string                      `json:"status"`
	ReasonCode              string                      `json:"reasonCode"`
	CandidateCount          int                         `json:"candidateCount"`
	SuggestedLocalSKUID     *uuid.UUID                  `json:"suggestedLocalSkuId,omitempty"`
	AssignedTo              *uuid.UUID                  `json:"assignedTo,omitempty"`
	ResolvedBy              *uuid.UUID                  `json:"resolvedBy,omitempty"`
	ResolvedAt              *time.Time                  `json:"resolvedAt,omitempty"`
	Resolution              string                      `json:"resolution,omitempty"`
	SelectedLocalProductID  *uuid.UUID                  `json:"selectedLocalProductId,omitempty"`
	SelectedLocalSKUID      *uuid.UUID                  `json:"selectedLocalSkuId,omitempty"`
	Comment                 string                      `json:"comment,omitempty"`
	Revision                int                         `json:"revision"`
	CreatedAt               time.Time                   `json:"createdAt"`
	UpdatedAt               time.Time                   `json:"updatedAt"`
	AllowedActions          ManualBindingAllowedActions `json:"allowedActions"`
}

type ManualBindingDecisionResponse struct {
	ID                     uuid.UUID  `json:"id"`
	Operation              string     `json:"operation"`
	ActorID                uuid.UUID  `json:"actorId"`
	SelectedLocalProductID *uuid.UUID `json:"selectedLocalProductId,omitempty"`
	SelectedLocalSKUID     *uuid.UUID `json:"selectedLocalSkuId,omitempty"`
	ReasonCode             string     `json:"reasonCode"`
	Comment                string     `json:"comment,omitempty"`
	RequestRevision        int        `json:"requestRevision"`
	CreatedAt              time.Time  `json:"createdAt"`
}

type ManualBindingDetailResponse struct {
	Request   ManualBindingRequestResponse    `json:"request"`
	Decisions []ManualBindingDecisionResponse `json:"decisions"`
}

type BindingHistoryResponse struct {
	Binding      SKUBindingResponse              `json:"binding"`
	Calibrations []CalibrationResponse           `json:"calibrations"`
	Decisions    []ManualBindingDecisionResponse `json:"manualDecisions"`
}

type AuditEventResponse struct {
	ID         uuid.UUID  `json:"id"`
	Action     string     `json:"action"`
	Resource   string     `json:"resource"`
	ResourceID string     `json:"resourceId,omitempty"`
	ShopID     *uuid.UUID `json:"shopId,omitempty"`
	Platform   string     `json:"platform,omitempty"`
	Permission string     `json:"permission,omitempty"`
	RequestID  string     `json:"requestId,omitempty"`
	Status     string     `json:"status"`
	Metadata   any        `json:"metadata"`
	ActorID    *uuid.UUID `json:"actorId,omitempty"`
	ActorRole  string     `json:"actorRole,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}
