package operationtask

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type APIActor struct {
	TenantID int64
	ActorID  uuid.UUID
	Role     string
}

type CreateTaskRequest struct {
	SourceType      string          `json:"sourceType"`
	SourceReference string          `json:"sourceReference"`
	TaskType        string          `json:"taskType"`
	Platform        string          `json:"platform"`
	Title           string          `json:"title"`
	Summary         string          `json:"summary"`
	Payload         json.RawMessage `json:"payload"`
	Priority        string          `json:"priority"`
}

type CreateDraftRequest struct {
	Payload              json.RawMessage `json:"payload"`
	ChangeReason         string          `json:"changeReason"`
	ExpectedTaskRevision int             `json:"expectedTaskRevision"`
}

type EditDraftRequest struct {
	Payload              json.RawMessage `json:"payload"`
	ChangeReason         string          `json:"changeReason"`
	ExpectedTaskRevision int             `json:"expectedTaskRevision"`
	ExpectedDraftVersion int             `json:"expectedDraftVersion"`
}

type ApprovalRequest struct {
	DraftVersion         int    `json:"draftVersion"`
	DraftPayloadHash     string `json:"draftPayloadHash"`
	Reason               string `json:"reason"`
	Comment              string `json:"comment"`
	ExpectedTaskRevision int    `json:"expectedTaskRevision"`
}

type ExecuteRequest struct {
	ExpectedTaskRevision int    `json:"expectedTaskRevision"`
	AdapterMode          string `json:"adapterMode"`
}

type RetryRequest struct {
	FailedAttemptID      *uuid.UUID `json:"failedAttemptId"`
	Reason               string     `json:"reason"`
	ExpectedTaskRevision int        `json:"expectedTaskRevision"`
}

type CancelTaskRequest struct {
	Reason               string `json:"reason"`
	ExpectedTaskRevision int    `json:"expectedTaskRevision"`
}

type OperationTaskSummaryResponse struct {
	ID                    uuid.UUID  `json:"id"`
	SourceType            string     `json:"sourceType"`
	SourceReference       string     `json:"sourceReference,omitempty"`
	TaskType              string     `json:"taskType"`
	Platform              string     `json:"platform"`
	Title                 string     `json:"title"`
	Summary               string     `json:"summary,omitempty"`
	Status                string     `json:"status"`
	Priority              string     `json:"priority"`
	Revision              int        `json:"revision"`
	LatestDraftVersion    int        `json:"latestDraftVersion,omitempty"`
	LatestExecutionStatus string     `json:"latestExecutionStatus,omitempty"`
	CreatedBy             *uuid.UUID `json:"createdBy,omitempty"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

type OperationTaskDetailResponse struct {
	OperationTaskSummaryResponse
	Payload        any                     `json:"payload,omitempty"`
	LatestDraft    *DraftSummaryResponse   `json:"latestDraft,omitempty"`
	LatestApproval *ApprovalResponse       `json:"latestApproval,omitempty"`
	LatestAttempt  *AttemptSummaryResponse `json:"latestAttempt,omitempty"`
	AllowedActions AllowedTaskActions      `json:"allowedActions"`
}

type AllowedTaskActions struct {
	CanEditDraft bool `json:"canEditDraft"`
	CanApprove   bool `json:"canApprove"`
	CanReject    bool `json:"canReject"`
	CanExecute   bool `json:"canExecute"`
	CanRetry     bool `json:"canRetry"`
	CanCancel    bool `json:"canCancel"`
}

type TaskListResponse struct {
	Items      []OperationTaskSummaryResponse `json:"items"`
	NextCursor string                         `json:"nextCursor,omitempty"`
	HasMore    bool                           `json:"hasMore"`
	Limit      int                            `json:"limit"`
}

type DraftSummaryResponse struct {
	ID           uuid.UUID  `json:"draftId"`
	DraftVersion int        `json:"draftVersion"`
	PayloadHash  string     `json:"payloadHash"`
	Status       string     `json:"status"`
	ChangeReason string     `json:"changeReason,omitempty"`
	CreatedBy    *uuid.UUID `json:"createdBy,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type DraftListResponse struct {
	Items []DraftSummaryResponse `json:"items"`
	Limit int                    `json:"limit"`
}

type ApprovalResponse struct {
	ID               uuid.UUID `json:"approvalId"`
	Decision         string    `json:"decision"`
	DraftVersion     int       `json:"draftVersion"`
	DraftPayloadHash string    `json:"draftPayloadHash"`
	ReviewerID       uuid.UUID `json:"reviewerId"`
	Reason           string    `json:"reason,omitempty"`
	Comment          string    `json:"comment,omitempty"`
	RequestID        string    `json:"requestId,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
}

type AttemptSummaryResponse struct {
	ID                       uuid.UUID  `json:"attemptId"`
	AttemptNumber            int        `json:"attemptNumber"`
	Status                   string     `json:"status"`
	AdapterMode              string     `json:"adapterMode"`
	Platform                 string     `json:"platform"`
	ApprovedDraftVersion     int        `json:"approvedDraftVersion"`
	ApprovedDraftPayloadHash string     `json:"approvedDraftPayloadHash"`
	ExecutedDraftVersion     int        `json:"executedDraftVersion"`
	ExecutedDraftPayloadHash string     `json:"executedDraftPayloadHash"`
	ResultType               string     `json:"resultType,omitempty"`
	RequestID                string     `json:"requestId,omitempty"`
	StartedAt                *time.Time `json:"startedAt,omitempty"`
	FinishedAt               *time.Time `json:"finishedAt,omitempty"`
	CreatedAt                time.Time  `json:"createdAt"`
}

type ExecutionResponse struct {
	Status     string                 `json:"status"`
	Attempt    AttemptSummaryResponse `json:"attempt"`
	ResultType string                 `json:"resultType,omitempty"`
	TaskStatus string                 `json:"taskStatus,omitempty"`
	RequestID  string                 `json:"requestId,omitempty"`
	Failure    *ExecutionFailureDTO   `json:"failure,omitempty"`
}

type ExecutionFailureDTO struct {
	Category    string `json:"category"`
	Code        string `json:"code"`
	SafeMessage string `json:"safeMessage"`
	Retryable   bool   `json:"retryable"`
}

type AttemptListResponse struct {
	Items      []AttemptSummaryResponse `json:"items"`
	NextCursor string                   `json:"nextCursor,omitempty"`
	HasMore    bool                     `json:"hasMore"`
	Limit      int                      `json:"limit"`
}

type EventResponse struct {
	ID              uuid.UUID  `json:"eventId"`
	Sequence        int        `json:"sequence"`
	EventType       string     `json:"eventType"`
	ActorType       string     `json:"actorType"`
	ActorID         *uuid.UUID `json:"actorId,omitempty"`
	BeforeState     string     `json:"beforeState,omitempty"`
	AfterState      string     `json:"afterState,omitempty"`
	PlatformDraftID *uuid.UUID `json:"platformDraftId,omitempty"`
	DraftVersion    int        `json:"draftVersion,omitempty"`
	RequestID       string     `json:"requestId,omitempty"`
	Reason          string     `json:"reason,omitempty"`
	Metadata        any        `json:"metadata"`
	OccurredAt      time.Time  `json:"occurredAt"`
}

type EventListResponse struct {
	Items        []EventResponse `json:"items"`
	NextSequence int             `json:"nextSequence,omitempty"`
	HasMore      bool            `json:"hasMore"`
	Limit        int             `json:"limit"`
}
