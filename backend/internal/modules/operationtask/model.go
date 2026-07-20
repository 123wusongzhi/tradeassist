package operationtask

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
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
	TenantID        int64          `gorm:"not null;index:idx_platform_drafts_task_version,priority:1;index:idx_platform_drafts_tenant_status_updated,priority:1;index:idx_platform_drafts_tenant_platform_status,priority:1;uniqueIndex:ux_platform_drafts_tenant_task_version,priority:1" json:"tenantId"`
	OperationTaskID uuid.UUID      `gorm:"type:char(36);not null;index:idx_platform_drafts_task_version,priority:2;uniqueIndex:ux_platform_drafts_tenant_task_version,priority:2" json:"operationTaskId"`
	OperationTask   OperationTask  `gorm:"foreignKey:OperationTaskID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"-"`
	Platform        string         `gorm:"size:64;not null;index:idx_platform_drafts_tenant_platform_status,priority:2" json:"platform"`
	AdapterMode     string         `gorm:"size:32;not null" json:"adapterMode"`
	DraftVersion    int            `gorm:"not null;default:1;index:idx_platform_drafts_task_version,priority:3,sort:desc;uniqueIndex:ux_platform_drafts_tenant_task_version,priority:3;check:chk_platform_drafts_version,draft_version >= 1" json:"draftVersion"`
	Payload         datatypes.JSON `gorm:"type:jsonb;not null" json:"payload"`
	PayloadHash     string         `gorm:"size:64;not null" json:"payloadHash"`
	Status          string         `gorm:"size:32;not null;index:idx_platform_drafts_tenant_status_updated,priority:2;index:idx_platform_drafts_tenant_platform_status,priority:3" json:"status"`
	ChangeReason    string         `gorm:"type:text" json:"changeReason,omitempty"`
	CreatedBy       *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	UpdatedBy       *uuid.UUID     `gorm:"type:char(36);index" json:"updatedBy,omitempty"`
}

func (PlatformDraft) TableName() string { return "platform_drafts" }

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
	if d.Status == "" {
		d.Status = PlatformDraftStatusEditable
	}
	if d.DraftVersion == 0 {
		d.DraftVersion = 1
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
