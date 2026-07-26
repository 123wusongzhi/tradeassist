package operationtask

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	OperationTaskSourceManual         = "manual"
	OperationTaskSourceAISuggestion   = "ai_suggestion"
	OperationTaskSourceRuleEngine     = "rule_engine"
	OperationTaskSourceOrderException = "order_exception"
	OperationTaskSourceProductContent = "product_content"
)

const (
	OperationTaskTypeProductContent = "product_content"
	OperationTaskTypeOrderException = "order_exception"
	OperationTaskTypeProductPublish = "product_publish"
	OperationTaskTypeInventorySync  = "inventory_sync"
	OperationTaskTypeCustomerReply  = "customer_reply"
	OperationTaskTypeAIText         = "ai_text"
	OperationTaskTypeAIImage        = "ai_image"
	OperationTaskTypeManualReview   = "manual_review"
)

const (
	PlatformLocal  = "local"
	PlatformDouyin = "douyin"
)

const (
	OperationTaskStatusSuggested       = "suggested"
	OperationTaskStatusDraftPreparing  = "draft_preparing"
	OperationTaskStatusPendingReview   = "pending_review"
	OperationTaskStatusApproved        = "approved"
	OperationTaskStatusRejected        = "rejected"
	OperationTaskStatusExecutionQueued = "execution_queued"
	OperationTaskStatusExecuting       = "executing"
	OperationTaskStatusDraftWritten    = "draft_written"
	OperationTaskStatusExecutionFailed = "execution_failed"
	OperationTaskStatusCancelled       = "cancelled"
)

const (
	OperationTaskPriorityLow    = "low"
	OperationTaskPriorityNormal = "normal"
	OperationTaskPriorityHigh   = "high"
	OperationTaskPriorityUrgent = "urgent"
)

const (
	AdapterModeMock           = "mock"
	AdapterModeSandbox        = "sandbox"
	AdapterModeLocalDraftOnly = "local_draft_only"
)

const (
	PlatformDraftStatusEditable      = "editable"
	PlatformDraftStatusPendingReview = "pending_review"
	PlatformDraftStatusApproved      = "approved"
	PlatformDraftStatusSuperseded    = "superseded"
	PlatformDraftStatusWritten       = "written"
	PlatformDraftStatusFailed        = "failed"
)

const (
	ApprovalDecisionApproved = "approved"
	ApprovalDecisionRejected = "rejected"
)

const (
	ReviewerRoleReviewer = "reviewer"
	ReviewerRoleAdmin    = "admin"
)

const (
	ExecutionAttemptStatusQueued    = "queued"
	ExecutionAttemptStatusRunning   = "running"
	ExecutionAttemptStatusSucceeded = "succeeded"
	ExecutionAttemptStatusFailed    = "failed"
	ExecutionAttemptStatusCancelled = "cancelled"
)

const (
	ExecutionErrorCategoryValidation          = "validation_error"
	ExecutionErrorCategoryPermissionDenied    = "permission_denied"
	ExecutionErrorCategoryStateConflict       = "state_conflict"
	ExecutionErrorCategoryAdapterUnavailable  = "adapter_unavailable"
	ExecutionErrorCategoryProviderTimeout     = "provider_timeout"
	ExecutionErrorCategoryProviderRejected    = "provider_rejected"
	ExecutionErrorCategoryIdempotencyConflict = "idempotency_conflict"
	ExecutionErrorCategoryInternal            = "internal_error"
)

const (
	OperationTaskEventTypeTaskCreated      = "task_created"
	OperationTaskEventTypeDraftGenerated   = "draft_generated"
	OperationTaskEventTypeDraftUpdated     = "draft_updated"
	OperationTaskEventTypeReviewRequested  = "review_requested"
	OperationTaskEventTypeApproved         = "approved"
	OperationTaskEventTypeRejected         = "rejected"
	OperationTaskEventTypeExecutionQueued  = "execution_queued"
	OperationTaskEventTypeExecutionStarted = "execution_started"
	OperationTaskEventTypeDraftWritten     = "draft_written"
	OperationTaskEventTypeExecutionFailed  = "execution_failed"
	OperationTaskEventTypeRetryRequested   = "retry_requested"
	OperationTaskEventTypeCancelled        = "cancelled"
)

const (
	OperationTaskEventActorUser   = "user"
	OperationTaskEventActorSystem = "system"
	OperationTaskEventActorAI     = "ai"
	OperationTaskEventActorRule   = "rule"
)

// OperationTask is the P8 persisted operation task aggregate root.
type OperationTask struct {
	model.HardDeleteBase
	TenantID        int64          `gorm:"not null;index:idx_operation_tasks_tenant_status_updated,priority:1;index:idx_operation_tasks_tenant_platform_status_updated,priority:1;index:idx_operation_tasks_tenant_task_type_created,priority:1;index:idx_operation_tasks_tenant_source,priority:1" json:"tenantId"`
	SourceType      string         `gorm:"size:64;not null;index:idx_operation_tasks_tenant_source,priority:2" json:"sourceType"`
	SourceReference string         `gorm:"size:255;index:idx_operation_tasks_tenant_source,priority:3" json:"sourceReference,omitempty"`
	TaskType        string         `gorm:"size:64;not null;index:idx_operation_tasks_tenant_task_type_created,priority:2" json:"taskType"`
	Platform        string         `gorm:"size:64;not null;index:idx_operation_tasks_tenant_platform_status_updated,priority:2" json:"platform"`
	Title           string         `gorm:"size:512;not null" json:"title"`
	Summary         string         `gorm:"type:text" json:"summary,omitempty"`
	Payload         datatypes.JSON `gorm:"type:jsonb;not null" json:"payload"`
	Status          string         `gorm:"size:32;not null;index:idx_operation_tasks_tenant_status_updated,priority:2;index:idx_operation_tasks_tenant_platform_status_updated,priority:3" json:"status"`
	Priority        string         `gorm:"size:16;not null" json:"priority"`
	IdempotencyKey  *string        `gorm:"size:255" json:"idempotencyKey,omitempty"`
	Revision        int            `gorm:"not null;default:1;check:chk_operation_tasks_revision,revision >= 1" json:"revision"`
	CreatedBy       *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	UpdatedBy       *uuid.UUID     `gorm:"type:char(36);index" json:"updatedBy,omitempty"`
}

func (OperationTask) TableName() string { return "operation_tasks" }

// PlatformDraft stores one immutable-ish draft version for an operation task.
type PlatformDraft struct {
	model.HardDeleteBase
	TenantID        int64          `gorm:"not null;index:idx_platform_drafts_task_version,priority:1;index:idx_platform_drafts_tenant_status_updated,priority:1;index:idx_platform_drafts_tenant_platform_status,priority:1;uniqueIndex:ux_platform_drafts_tenant_task_version,priority:1;uniqueIndex:ux_platform_drafts_task_idempotency,priority:1" json:"tenantId"`
	OperationTaskID uuid.UUID      `gorm:"type:char(36);not null;index:idx_platform_drafts_task_version,priority:2;uniqueIndex:ux_platform_drafts_tenant_task_version,priority:2;uniqueIndex:ux_platform_drafts_task_idempotency,priority:2" json:"operationTaskId"`
	OperationTask   OperationTask  `gorm:"foreignKey:OperationTaskID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	Platform        string         `gorm:"size:64;not null;index:idx_platform_drafts_tenant_platform_status,priority:2" json:"platform"`
	AdapterMode     string         `gorm:"size:32;not null" json:"adapterMode"`
	DraftVersion    int            `gorm:"not null;default:1;index:idx_platform_drafts_task_version,priority:3,sort:desc;uniqueIndex:ux_platform_drafts_tenant_task_version,priority:3;check:chk_platform_drafts_version,draft_version >= 1" json:"draftVersion"`
	Payload         datatypes.JSON `gorm:"type:jsonb;not null" json:"payload"`
	PayloadHash     string         `gorm:"size:64;not null" json:"payloadHash"`
	Status          string         `gorm:"size:32;not null;index:idx_platform_drafts_tenant_status_updated,priority:2;index:idx_platform_drafts_tenant_platform_status,priority:3" json:"status"`
	ChangeReason    string         `gorm:"type:text" json:"changeReason,omitempty"`
	IdempotencyKey  *string        `gorm:"size:255;uniqueIndex:ux_platform_drafts_task_idempotency,priority:3" json:"idempotencyKey,omitempty"`
	CreatedBy       *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	UpdatedBy       *uuid.UUID     `gorm:"type:char(36);index" json:"updatedBy,omitempty"`
}

func (PlatformDraft) TableName() string { return "platform_drafts" }

// ApprovalRecord stores one immutable human review decision for a precise draft version.
type ApprovalRecord struct {
	model.HardDeleteBase
	TenantID         int64         `gorm:"not null;index:idx_approval_records_task_created,priority:1;index:idx_approval_records_draft_created,priority:1;index:idx_approval_records_task_decision_created,priority:1;uniqueIndex:ux_approval_records_task_idempotency,priority:1" json:"tenantId"`
	OperationTaskID  uuid.UUID     `gorm:"type:char(36);not null;index:idx_approval_records_task_created,priority:2;index:idx_approval_records_task_decision_created,priority:2;uniqueIndex:ux_approval_records_task_idempotency,priority:2" json:"operationTaskId"`
	OperationTask    OperationTask `gorm:"foreignKey:OperationTaskID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	PlatformDraftID  uuid.UUID     `gorm:"type:char(36);not null;index:idx_approval_records_draft_created,priority:2" json:"platformDraftId"`
	PlatformDraft    PlatformDraft `gorm:"foreignKey:PlatformDraftID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	Decision         string        `gorm:"size:16;not null;index:idx_approval_records_task_decision_created,priority:3" json:"decision"`
	DraftVersion     int           `gorm:"not null;check:chk_approval_records_draft_version,draft_version >= 1" json:"draftVersion"`
	DraftPayloadHash string        `gorm:"size:64;not null" json:"draftPayloadHash"`
	ReviewerID       uuid.UUID     `gorm:"type:char(36);not null" json:"reviewerId"`
	ReviewerRole     string        `gorm:"size:32;not null" json:"reviewerRole"`
	Reason           string        `gorm:"type:text" json:"reason,omitempty"`
	Comment          string        `gorm:"type:text" json:"comment,omitempty"`
	RequestID        string        `gorm:"size:128" json:"requestId,omitempty"`
	IdempotencyKey   *string       `gorm:"size:255;uniqueIndex:ux_approval_records_task_idempotency,priority:3" json:"idempotencyKey,omitempty"`
}

func (ApprovalRecord) TableName() string { return "approval_records" }

func (ApprovalRecord) BeforeUpdate(tx *gorm.DB) error { return ErrImmutableRecord }
func (ApprovalRecord) BeforeDelete(tx *gorm.DB) error { return ErrImmutableRecord }

// ExecutionAttempt stores one persisted execution attempt without performing platform writes.
type ExecutionAttempt struct {
	model.HardDeleteBase
	TenantID                 int64          `gorm:"not null;index:idx_execution_attempts_task_attempt,priority:1;index:idx_execution_attempts_task_created,priority:1;uniqueIndex:ux_execution_attempts_task_attempt,priority:1;uniqueIndex:ux_execution_attempts_task_idempotency,priority:1" json:"tenantId"`
	OperationTaskID          uuid.UUID      `gorm:"type:char(36);not null;index:idx_execution_attempts_task_attempt,priority:2;index:idx_execution_attempts_task_created,priority:2;uniqueIndex:ux_execution_attempts_task_attempt,priority:2;uniqueIndex:ux_execution_attempts_task_idempotency,priority:2" json:"operationTaskId"`
	OperationTask            OperationTask  `gorm:"foreignKey:OperationTaskID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	PlatformDraftID          uuid.UUID      `gorm:"type:char(36);not null" json:"platformDraftId"`
	PlatformDraft            PlatformDraft  `gorm:"foreignKey:PlatformDraftID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	ApprovalRecordID         uuid.UUID      `gorm:"type:char(36);not null;index" json:"approvalRecordId"`
	ApprovalRecord           ApprovalRecord `gorm:"foreignKey:ApprovalRecordID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	AttemptNumber            int            `gorm:"not null;uniqueIndex:ux_execution_attempts_task_attempt,priority:3;index:idx_execution_attempts_task_attempt,priority:3;check:chk_execution_attempts_attempt_number,attempt_number >= 1" json:"attemptNumber"`
	Status                   string         `gorm:"size:32;not null;index" json:"status"`
	AdapterMode              string         `gorm:"size:32;not null" json:"adapterMode"`
	Platform                 string         `gorm:"size:64;not null" json:"platform"`
	ApprovedDraftVersion     int            `gorm:"not null;check:chk_execution_attempts_approved_version,approved_draft_version >= 1" json:"approvedDraftVersion"`
	ApprovedDraftPayloadHash string         `gorm:"size:64;not null" json:"approvedDraftPayloadHash"`
	ExecutedDraftVersion     int            `gorm:"not null;check:chk_execution_attempts_executed_version,executed_draft_version >= 1" json:"executedDraftVersion"`
	ExecutedDraftPayloadHash string         `gorm:"size:64;not null" json:"executedDraftPayloadHash"`
	RequestID                string         `gorm:"size:128" json:"requestId,omitempty"`
	IdempotencyKey           *string        `gorm:"size:255;uniqueIndex:ux_execution_attempts_task_idempotency,priority:3" json:"idempotencyKey,omitempty"`
	ResultType               string         `gorm:"size:64" json:"resultType,omitempty"`
	ExternalReference        string         `gorm:"size:255" json:"externalReference,omitempty"`
	SafeMetadata             datatypes.JSON `gorm:"type:jsonb" json:"safeMetadata,omitempty"`
	Revision                 int            `gorm:"not null;default:1;check:chk_execution_attempts_revision,revision >= 1" json:"revision"`
	StartedAt                *time.Time     `json:"startedAt,omitempty"`
	FinishedAt               *time.Time     `json:"finishedAt,omitempty"`
}

func (ExecutionAttempt) TableName() string { return "execution_attempts" }

// ExecutionError is an immutable sanitized error fact appended to an attempt.
type ExecutionError struct {
	model.HardDeleteBase
	TenantID           int64            `gorm:"not null;index:idx_execution_errors_attempt_sequence,priority:1;uniqueIndex:ux_execution_errors_attempt_sequence,priority:1" json:"tenantId"`
	ExecutionAttemptID uuid.UUID        `gorm:"type:char(36);not null;index:idx_execution_errors_attempt_sequence,priority:2;uniqueIndex:ux_execution_errors_attempt_sequence,priority:2" json:"executionAttemptId"`
	ExecutionAttempt   ExecutionAttempt `gorm:"foreignKey:ExecutionAttemptID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	Sequence           int              `gorm:"not null;index:idx_execution_errors_attempt_sequence,priority:3;uniqueIndex:ux_execution_errors_attempt_sequence,priority:3;check:chk_execution_errors_sequence,sequence >= 1" json:"sequence"`
	Category           string           `gorm:"size:64;not null" json:"category"`
	Code               string           `gorm:"size:128;not null" json:"code"`
	SafeMessage        string           `gorm:"size:2048;not null" json:"safeMessage"`
	Retryable          bool             `gorm:"not null" json:"retryable"`
	Details            datatypes.JSON   `gorm:"type:jsonb;not null" json:"details"`
	OccurredAt         time.Time        `gorm:"not null;index" json:"occurredAt"`
}

func (ExecutionError) TableName() string { return "execution_errors" }

func (ExecutionError) BeforeUpdate(tx *gorm.DB) error { return ErrImmutableRecord }
func (ExecutionError) BeforeDelete(tx *gorm.DB) error { return ErrImmutableRecord }

// OperationTaskEvent is the P8 domain timeline event for one operation task.
type OperationTaskEvent struct {
	model.HardDeleteBase
	TenantID        int64          `gorm:"not null;index:idx_operation_task_events_task_sequence,priority:1;uniqueIndex:ux_operation_task_events_task_sequence,priority:1" json:"tenantId"`
	OperationTaskID uuid.UUID      `gorm:"type:char(36);not null;index:idx_operation_task_events_task_sequence,priority:2;uniqueIndex:ux_operation_task_events_task_sequence,priority:2" json:"operationTaskId"`
	OperationTask   OperationTask  `gorm:"foreignKey:OperationTaskID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	Sequence        int            `gorm:"not null;index:idx_operation_task_events_task_sequence,priority:3;uniqueIndex:ux_operation_task_events_task_sequence,priority:3;check:chk_operation_task_events_sequence,sequence >= 1" json:"sequence"`
	EventType       string         `gorm:"size:64;not null;index" json:"eventType"`
	ActorType       string         `gorm:"size:32;not null" json:"actorType"`
	ActorID         *uuid.UUID     `gorm:"type:char(36)" json:"actorId,omitempty"`
	BeforeState     string         `gorm:"size:32" json:"beforeState,omitempty"`
	AfterState      string         `gorm:"size:32" json:"afterState,omitempty"`
	PlatformDraftID *uuid.UUID     `gorm:"type:char(36);index" json:"platformDraftId,omitempty"`
	PlatformDraft   *PlatformDraft `gorm:"foreignKey:PlatformDraftID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	DraftVersion    int            `json:"draftVersion,omitempty"`
	RequestID       string         `gorm:"size:128" json:"requestId,omitempty"`
	Reason          string         `gorm:"type:text" json:"reason,omitempty"`
	Metadata        datatypes.JSON `gorm:"type:jsonb;not null" json:"metadata"`
	OccurredAt      time.Time      `gorm:"not null;index" json:"occurredAt"`
}

func (OperationTaskEvent) TableName() string { return "operation_task_events" }

func (OperationTaskEvent) BeforeUpdate(tx *gorm.DB) error { return ErrImmutableRecord }
func (OperationTaskEvent) BeforeDelete(tx *gorm.DB) error { return ErrImmutableRecord }

type OperationTaskListParams struct {
	TenantID int64
	Status   string
	Platform string
	TaskType string
	Limit    int
	Cursor   string
}

type OperationTaskListResult struct {
	Items      []OperationTask
	Limit      int
	HasMore    bool
	NextCursor string
}

type OperationTaskPatch struct {
	Title     *string
	Summary   *string
	Payload   *datatypes.JSON
	Status    *string
	Priority  *string
	UpdatedBy *uuid.UUID
}

type ExecutionAttemptLifecyclePatch struct {
	Status                   *string
	ExecutedDraftVersion     *int
	ExecutedDraftPayloadHash *string
	ResultType               *string
	ExternalReference        *string
	SafeMetadata             *datatypes.JSON
	StartedAt                *time.Time
	FinishedAt               *time.Time
}

type OperationTaskEventListParams struct {
	TenantID        int64
	OperationTaskID uuid.UUID
	AfterSequence   int
	Limit           int
}

type OperationTaskEventListResult struct {
	Items        []OperationTaskEvent
	Limit        int
	HasMore      bool
	NextSequence int
}

func normalizeOperationTask(t *OperationTask) {
	if t == nil {
		return
	}
	t.SourceType = strings.TrimSpace(strings.ToLower(t.SourceType))
	t.SourceReference = strings.TrimSpace(t.SourceReference)
	t.TaskType = strings.TrimSpace(strings.ToLower(t.TaskType))
	t.Platform = strings.TrimSpace(strings.ToLower(t.Platform))
	t.Title = strings.TrimSpace(t.Title)
	t.Status = strings.TrimSpace(strings.ToLower(t.Status))
	t.Priority = strings.TrimSpace(strings.ToLower(t.Priority))
	if t.Status == "" {
		t.Status = OperationTaskStatusSuggested
	}
	if t.Priority == "" {
		t.Priority = OperationTaskPriorityNormal
	}
	if t.Revision == 0 {
		t.Revision = 1
	}
	t.IdempotencyKey = normalizeOptionalString(t.IdempotencyKey)
}

func normalizePlatformDraft(d *PlatformDraft) {
	if d == nil {
		return
	}
	d.Platform = strings.TrimSpace(strings.ToLower(d.Platform))
	d.AdapterMode = strings.TrimSpace(strings.ToLower(d.AdapterMode))
	d.PayloadHash = strings.TrimSpace(strings.ToLower(d.PayloadHash))
	d.Status = strings.TrimSpace(strings.ToLower(d.Status))
	d.ChangeReason = strings.TrimSpace(d.ChangeReason)
	if d.Status == "" {
		d.Status = PlatformDraftStatusEditable
	}
	if d.DraftVersion == 0 {
		d.DraftVersion = 1
	}
	d.IdempotencyKey = normalizeOptionalString(d.IdempotencyKey)
}

func normalizeApprovalRecord(a *ApprovalRecord) {
	if a == nil {
		return
	}
	a.Decision = strings.TrimSpace(strings.ToLower(a.Decision))
	a.DraftPayloadHash = strings.TrimSpace(strings.ToLower(a.DraftPayloadHash))
	a.ReviewerRole = strings.TrimSpace(strings.ToLower(a.ReviewerRole))
	a.Reason = strings.TrimSpace(a.Reason)
	a.Comment = strings.TrimSpace(a.Comment)
	a.RequestID = strings.TrimSpace(a.RequestID)
	a.IdempotencyKey = normalizeOptionalString(a.IdempotencyKey)
}

func normalizeExecutionAttempt(a *ExecutionAttempt) {
	if a == nil {
		return
	}
	a.Status = strings.TrimSpace(strings.ToLower(a.Status))
	if a.Status == "" {
		a.Status = ExecutionAttemptStatusQueued
	}
	a.AdapterMode = strings.TrimSpace(strings.ToLower(a.AdapterMode))
	a.Platform = strings.TrimSpace(strings.ToLower(a.Platform))
	a.ApprovedDraftPayloadHash = strings.TrimSpace(strings.ToLower(a.ApprovedDraftPayloadHash))
	a.ExecutedDraftPayloadHash = strings.TrimSpace(strings.ToLower(a.ExecutedDraftPayloadHash))
	a.RequestID = strings.TrimSpace(a.RequestID)
	a.IdempotencyKey = normalizeOptionalString(a.IdempotencyKey)
	a.ResultType = strings.TrimSpace(strings.ToLower(a.ResultType))
	a.ExternalReference = strings.TrimSpace(a.ExternalReference)
	if len(a.SafeMetadata) == 0 {
		a.SafeMetadata = datatypes.JSON([]byte(`{}`))
	}
	if a.Revision == 0 {
		a.Revision = 1
	}
}

func normalizeExecutionError(e *ExecutionError) {
	if e == nil {
		return
	}
	e.Category = strings.TrimSpace(strings.ToLower(e.Category))
	e.Code = strings.TrimSpace(e.Code)
	e.SafeMessage = strings.TrimSpace(e.SafeMessage)
	if len(e.Details) == 0 {
		e.Details = datatypes.JSON([]byte(`{}`))
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = utcNow()
	}
}

func normalizeOperationTaskEvent(e *OperationTaskEvent) {
	if e == nil {
		return
	}
	e.EventType = strings.TrimSpace(strings.ToLower(e.EventType))
	e.ActorType = strings.TrimSpace(strings.ToLower(e.ActorType))
	e.BeforeState = strings.TrimSpace(strings.ToLower(e.BeforeState))
	e.AfterState = strings.TrimSpace(strings.ToLower(e.AfterState))
	e.RequestID = strings.TrimSpace(e.RequestID)
	e.Reason = strings.TrimSpace(e.Reason)
	if len(e.Metadata) == 0 {
		e.Metadata = datatypes.JSON([]byte(`{}`))
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = utcNow()
	}
}

func normalizeOptionalString(in *string) *string {
	if in == nil {
		return nil
	}
	v := strings.TrimSpace(*in)
	if v == "" {
		return nil
	}
	return &v
}

func isValidJSON(raw datatypes.JSON) bool {
	b := []byte(raw)
	return len(b) > 0 && json.Valid(b) && strings.TrimSpace(string(b)) != "null"
}

func utcNow() time.Time {
	return time.Now().UTC()
}
