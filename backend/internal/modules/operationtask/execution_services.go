package operationtask

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ExecutionPortModeMock              = "mock"
	ExecutionPortModeSandboxFixture    = "sandbox_fixture"
	ExecutionPortModeLocalDraftFixture = "local_draft_fixture"
)

const (
	ExecutionIdempotencyStatusNew             = "new"
	ExecutionIdempotencyStatusInProgress      = "in_progress"
	ExecutionIdempotencyStatusSucceeded       = "succeeded"
	ExecutionIdempotencyStatusFailedRetryable = "failed_retryable"
	ExecutionIdempotencyStatusFailedFinal     = "failed_final"
)

const DefaultMaxManualRetryAttempts = 3

type DraftExecutionPort interface {
	ExecuteDraft(ctx context.Context, input DraftExecutionInput) (DraftExecutionResult, error)
}

type DraftExecutionInput struct {
	TenantID         int64
	OperationTaskID  uuid.UUID
	PlatformDraftID  uuid.UUID
	Platform         string
	AdapterMode      string
	DraftVersion     int
	DraftPayloadHash string
	Payload          datatypes.JSON
	RequestID        string
	IdempotencyKey   string
	ActorID          uuid.UUID
	AttemptNumber    int
}

type DraftExecutionResult struct {
	ResultType        string
	ExternalReference string
	SafeMetadata      datatypes.JSON
	CompletedAt       time.Time
}

type ExecutionAuthorizer interface {
	CanExecute(ctx context.Context, tenantID int64, actorID uuid.UUID, taskID uuid.UUID) error
}

type ManualRetryAuthorizer interface {
	CanRetry(ctx context.Context, tenantID int64, actorID uuid.UUID, taskID uuid.UUID) error
}

type ExecutionFailure struct {
	Category        string
	Code            string
	SafeMessage     string
	Retryable       bool
	RetryPolicy     string
	ResultCertainty string
	Details         datatypes.JSON
}

type ExecutionDomainError struct {
	Category        string
	Code            string
	SafeMessage     string
	Retryable       bool
	RetryPolicy     string
	ResultCertainty string
	Details         datatypes.JSON
}

func (e *ExecutionDomainError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Code) != "" {
		return strings.TrimSpace(e.Code)
	}
	return strings.TrimSpace(e.SafeMessage)
}

type ExecutionFailureClassifier struct{}

func NewExecutionFailureClassifier() *ExecutionFailureClassifier {
	return &ExecutionFailureClassifier{}
}

func (c *ExecutionFailureClassifier) Classify(err error) ExecutionFailure {
	if err == nil {
		return ExecutionFailure{}
	}
	var domain *ExecutionDomainError
	if errors.As(err, &domain) {
		return sanitizeFailure(ExecutionFailure{
			Category:        domain.Category,
			Code:            domain.Code,
			SafeMessage:     domain.SafeMessage,
			Retryable:       domain.Retryable,
			RetryPolicy:     domain.RetryPolicy,
			ResultCertainty: domain.ResultCertainty,
			Details:         domain.Details,
		})
	}
	category, code, retryable := ExecutionErrorCategoryInternal, "internal_error", false
	switch {
	case errors.Is(err, ErrValidation), errors.Is(err, ErrDraftNotLatest), errors.Is(err, ErrDraftHashMismatch), errors.Is(err, ErrDraftVersionMismatch):
		category, code = ExecutionErrorCategoryValidation, ErrCodeValidation
	case errors.Is(err, ErrPermissionDenied):
		category, code = ExecutionErrorCategoryPermissionDenied, ErrCodePermissionDenied
	case errors.Is(err, ErrStateConflict), errors.Is(err, ErrInvalidTransition), errors.Is(err, ErrRevisionConflict), errors.Is(err, ErrExecutionInProgress):
		category, code = ExecutionErrorCategoryStateConflict, ErrCodeStateConflict
	case errors.Is(err, ErrIdemPayloadConflict), errors.Is(err, ErrDuplicateRequest), errors.Is(err, ErrApprovalIdemConflict):
		category, code = ExecutionErrorCategoryIdempotencyConflict, ErrCodeIdemPayloadConflict
	case errors.Is(err, context.DeadlineExceeded):
		category, code, retryable = ExecutionErrorCategoryProviderTimeout, "context_deadline_exceeded", true
	case errors.Is(err, context.Canceled):
		category, code = ExecutionErrorCategoryProviderTimeout, "context_cancelled"
	}
	return sanitizeFailure(ExecutionFailure{
		Category:        category,
		Code:            code,
		SafeMessage:     safeExecutionMessage(code),
		Retryable:       retryable,
		RetryPolicy:     retryPolicyFor(category, retryable),
		ResultCertainty: resultCertaintyFor(category),
		Details:         datatypes.JSON([]byte(`{}`)),
	})
}

type ExecutionOrchestrator struct {
	DB           *gorm.DB
	StateMachine *TaskStateMachine
	Authorizer   ExecutionAuthorizer
	Port         DraftExecutionPort
	Classifier   *ExecutionFailureClassifier
	Now          func() time.Time
}

type ExecutionInput struct {
	TenantID        int64
	OperationTaskID uuid.UUID
	ActorID         uuid.UUID
	RequestID       string
	IdempotencyKey  string
	AdapterMode     string
}

type ExecutionOutput struct {
	Status  string
	Attempt ExecutionAttempt
	Result  DraftExecutionResult
	Failure *ExecutionFailure
}

type preparedExecution struct {
	attempt ExecutionAttempt
	draft   PlatformDraft
	mode    string
	replay  *ExecutionOutput
}

func NewExecutionOrchestrator(db *gorm.DB, authorizer ExecutionAuthorizer, port DraftExecutionPort) *ExecutionOrchestrator {
	return &ExecutionOrchestrator{
		DB:           db,
		StateMachine: NewTaskStateMachine(),
		Authorizer:   authorizer,
		Port:         port,
		Classifier:   NewExecutionFailureClassifier(),
	}
}

func (s *ExecutionOrchestrator) Execute(ctx context.Context, in ExecutionInput) (*ExecutionOutput, error) {
	return s.execute(ctx, in, false)
}

func (s *ExecutionOrchestrator) execute(ctx context.Context, in ExecutionInput, retry bool) (*ExecutionOutput, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("execution orchestrator: db is nil")
	}
	if in.TenantID <= 0 || in.OperationTaskID == uuid.Nil || in.ActorID == uuid.Nil ||
		strings.TrimSpace(in.RequestID) == "" || strings.TrimSpace(executionIdempotencyKey(in)) == "" {
		return nil, ErrValidation
	}
	if s.Authorizer == nil {
		return nil, ErrPermissionDenied
	}
	if err := s.Authorizer.CanExecute(ctx, in.TenantID, in.ActorID, in.OperationTaskID); err != nil {
		return nil, ErrPermissionDenied
	}
	if s.Port == nil {
		return nil, &ExecutionDomainError{
			Category:    ExecutionErrorCategoryAdapterUnavailable,
			Code:        "execution_port_unavailable",
			SafeMessage: "Execution port is unavailable",
			Retryable:   true,
		}
	}
	if forbiddenExecutionMode(in.AdapterMode) {
		return nil, ErrExecutionModeForbidden
	}
	prepared, err := s.prepare(ctx, in, retry)
	if err != nil {
		return nil, err
	}
	if prepared.replay != nil {
		return prepared.replay, nil
	}
	result, execErr := s.Port.ExecuteDraft(ctx, DraftExecutionInput{
		TenantID:         in.TenantID,
		OperationTaskID:  in.OperationTaskID,
		PlatformDraftID:  prepared.draft.ID,
		Platform:         prepared.draft.Platform,
		AdapterMode:      prepared.mode,
		DraftVersion:     prepared.draft.DraftVersion,
		DraftPayloadHash: prepared.draft.PayloadHash,
		Payload:          prepared.draft.Payload,
		RequestID:        strings.TrimSpace(in.RequestID),
		IdempotencyKey:   executionIdempotencyKey(in),
		ActorID:          in.ActorID,
		AttemptNumber:    prepared.attempt.AttemptNumber,
	})
	if execErr == nil {
		out, err := s.finalizeSuccess(ctx, in, prepared.attempt, result)
		if err != nil {
			return nil, err
		}
		return out, nil
	}
	failure := s.classifier().Classify(execErr)
	out, err := s.finalizeFailure(ctx, in, prepared.attempt, failure)
	if err != nil {
		return nil, err
	}
	return out, execErr
}

func (s *ExecutionOrchestrator) prepare(ctx context.Context, in ExecutionInput, retry bool) (preparedExecution, error) {
	var out preparedExecution
	key := executionIdempotencyKey(in)
	mode := strings.TrimSpace(strings.ToLower(in.AdapterMode))
	sm := s.stateMachine()
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		task, err := lockTask(tx, in.TenantID, in.OperationTaskID)
		if err != nil {
			return err
		}
		latestDraft, err := findLatestDraftTx(tx, in.TenantID, in.OperationTaskID)
		if err != nil {
			return err
		}
		if mode == "" {
			mode = portModeForDraft(latestDraft.AdapterMode)
		}
		if !allowedExecutionPortMode(mode) {
			return ErrExecutionModeForbidden
		}
		existing, err := findAttemptByIdempotencyTx(tx, in.TenantID, in.OperationTaskID, key)
		if err == nil {
			replay, err := replayAttemptTx(tx, existing, latestDraft, OperationTaskEventActorUser)
			if err != nil {
				return err
			}
			out.replay = replay
			return nil
		}
		if !errors.Is(err, ErrNotFound) {
			return err
		}
		if retry {
			if task.Status != OperationTaskStatusExecutionFailed {
				return ErrStateConflict
			}
		} else if !sm.CanExecute(task.Status) {
			return ErrStateConflict
		}
		approval, err := findLatestApprovedApprovalTx(tx, in.TenantID, in.OperationTaskID)
		if err != nil {
			return err
		}
		if approval.PlatformDraftID != latestDraft.ID || approval.DraftVersion != latestDraft.DraftVersion || approval.DraftPayloadHash != latestDraft.PayloadHash {
			return ErrDraftBindingConflict
		}
		if hasActiveAttemptTx(tx, in.TenantID, in.OperationTaskID) {
			return ErrExecutionInProgress
		}
		if retry {
			if err := appendAuditEventTx(tx, OperationTaskEvent{
				TenantID:        in.TenantID,
				OperationTaskID: in.OperationTaskID,
				EventType:       OperationTaskEventTypeRetryRequested,
				ActorType:       OperationTaskEventActorUser,
				ActorID:         &in.ActorID,
				BeforeState:     task.Status,
				AfterState:      task.Status,
				PlatformDraftID: &latestDraft.ID,
				DraftVersion:    latestDraft.DraftVersion,
				RequestID:       strings.TrimSpace(in.RequestID),
				Reason:          "manual retry requested",
				Metadata:        datatypes.JSON([]byte(`{}`)),
			}); err != nil {
				return err
			}
		}
		attemptNumber, err := nextAttemptNumberTx(tx, in.TenantID, in.OperationTaskID)
		if err != nil {
			return err
		}
		now := s.now()
		out.attempt = ExecutionAttempt{
			TenantID:                 in.TenantID,
			OperationTaskID:          in.OperationTaskID,
			PlatformDraftID:          latestDraft.ID,
			ApprovalRecordID:         approval.ID,
			AttemptNumber:            attemptNumber,
			Status:                   ExecutionAttemptStatusRunning,
			AdapterMode:              latestDraft.AdapterMode,
			Platform:                 latestDraft.Platform,
			ApprovedDraftVersion:     approval.DraftVersion,
			ApprovedDraftPayloadHash: approval.DraftPayloadHash,
			ExecutedDraftVersion:     latestDraft.DraftVersion,
			ExecutedDraftPayloadHash: latestDraft.PayloadHash,
			RequestID:                strings.TrimSpace(in.RequestID),
			IdempotencyKey:           &key,
			StartedAt:                &now,
			SafeMetadata:             datatypes.JSON([]byte(`{}`)),
		}
		if out.attempt.ApprovedDraftVersion != out.attempt.ExecutedDraftVersion ||
			out.attempt.ApprovedDraftPayloadHash != out.attempt.ExecutedDraftPayloadHash {
			return ErrDraftBindingConflict
		}
		if err := validateExecutionAttempt(&out.attempt); err != nil {
			return err
		}
		if err := validateAttemptReferences(tx, &out.attempt); err != nil {
			return err
		}
		if err := tx.Create(&out.attempt).Error; err != nil {
			if isUniqueViolation(err) {
				return ErrDuplicateAttemptNumber
			}
			return stableError(err, ErrConflict)
		}
		if err := updateTaskStatusRevisionTx(tx, task, OperationTaskStatusExecutionQueued, &in.ActorID); err != nil {
			return err
		}
		if err := appendAuditEventTx(tx, OperationTaskEvent{
			TenantID:        in.TenantID,
			OperationTaskID: in.OperationTaskID,
			EventType:       OperationTaskEventTypeExecutionQueued,
			ActorType:       OperationTaskEventActorUser,
			ActorID:         &in.ActorID,
			BeforeState:     task.Status,
			AfterState:      OperationTaskStatusExecutionQueued,
			PlatformDraftID: &latestDraft.ID,
			DraftVersion:    latestDraft.DraftVersion,
			RequestID:       strings.TrimSpace(in.RequestID),
			Reason:          "execution queued",
			Metadata:        datatypes.JSON([]byte(`{}`)),
		}); err != nil {
			return err
		}
		task.Status = OperationTaskStatusExecutionQueued
		task.Revision++
		if err := updateTaskStatusRevisionTx(tx, task, OperationTaskStatusExecuting, &in.ActorID); err != nil {
			return err
		}
		if err := appendAuditEventTx(tx, OperationTaskEvent{
			TenantID:        in.TenantID,
			OperationTaskID: in.OperationTaskID,
			EventType:       OperationTaskEventTypeExecutionStarted,
			ActorType:       OperationTaskEventActorUser,
			ActorID:         &in.ActorID,
			BeforeState:     OperationTaskStatusExecutionQueued,
			AfterState:      OperationTaskStatusExecuting,
			PlatformDraftID: &latestDraft.ID,
			DraftVersion:    latestDraft.DraftVersion,
			RequestID:       strings.TrimSpace(in.RequestID),
			Reason:          "execution started",
			Metadata:        datatypes.JSON([]byte(`{}`)),
		}); err != nil {
			return err
		}
		out.draft = *latestDraft
		out.mode = mode
		return nil
	})
	return out, err
}

func (s *ExecutionOrchestrator) finalizeSuccess(ctx context.Context, in ExecutionInput, attempt ExecutionAttempt, result DraftExecutionResult) (*ExecutionOutput, error) {
	result = sanitizeExecutionResult(result)
	if result.CompletedAt.IsZero() {
		result.CompletedAt = s.now()
	}
	var out ExecutionOutput
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		task, err := lockTask(tx, in.TenantID, in.OperationTaskID)
		if err != nil {
			return err
		}
		if task.Status != OperationTaskStatusExecuting {
			return ErrFinalizeConflict
		}
		current, err := lockAttempt(tx, in.TenantID, attempt.ID)
		if err != nil {
			return err
		}
		if current.Status != ExecutionAttemptStatusRunning || current.Revision != attempt.Revision {
			return ErrFinalizeConflict
		}
		updates := map[string]any{
			"status":             ExecutionAttemptStatusSucceeded,
			"result_type":        result.ResultType,
			"external_reference": result.ExternalReference,
			"safe_metadata":      result.SafeMetadata,
			"finished_at":        result.CompletedAt.UTC(),
			"revision":           gorm.Expr("revision + 1"),
			"updated_at":         s.now(),
		}
		res := tx.Model(&ExecutionAttempt{}).
			Where("tenant_id = ? AND id = ? AND revision = ?", in.TenantID, attempt.ID, attempt.Revision).
			Updates(updates)
		if res.Error != nil {
			return stableError(res.Error, ErrConflict)
		}
		if res.RowsAffected == 0 {
			return ErrFinalizeConflict
		}
		if err := updateTaskStatusRevisionTx(tx, task, OperationTaskStatusDraftWritten, &in.ActorID); err != nil {
			return err
		}
		if err := appendAuditEventTx(tx, OperationTaskEvent{
			TenantID:        in.TenantID,
			OperationTaskID: in.OperationTaskID,
			EventType:       OperationTaskEventTypeDraftWritten,
			ActorType:       OperationTaskEventActorUser,
			ActorID:         &in.ActorID,
			BeforeState:     task.Status,
			AfterState:      OperationTaskStatusDraftWritten,
			PlatformDraftID: &attempt.PlatformDraftID,
			DraftVersion:    attempt.ExecutedDraftVersion,
			RequestID:       strings.TrimSpace(in.RequestID),
			Reason:          "draft written to local fixture",
			Metadata:        result.SafeMetadata,
		}); err != nil {
			return err
		}
		if err := tx.Model(&PlatformDraft{}).
			Where("tenant_id = ? AND id = ?", in.TenantID, attempt.PlatformDraftID).
			Update("status", PlatformDraftStatusWritten).Error; err != nil {
			return stableError(err, ErrConflict)
		}
		refreshed, err := lockAttempt(tx, in.TenantID, attempt.ID)
		if err != nil {
			return err
		}
		out = ExecutionOutput{Status: ExecutionIdempotencyStatusSucceeded, Attempt: *refreshed, Result: result}
		return nil
	})
	return &out, err
}

func (s *ExecutionOrchestrator) finalizeFailure(ctx context.Context, in ExecutionInput, attempt ExecutionAttempt, failure ExecutionFailure) (*ExecutionOutput, error) {
	failure = sanitizeFailure(failure)
	var out ExecutionOutput
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		task, err := lockTask(tx, in.TenantID, in.OperationTaskID)
		if err != nil {
			return err
		}
		if task.Status != OperationTaskStatusExecuting {
			return ErrResultUnknown
		}
		current, err := lockAttempt(tx, in.TenantID, attempt.ID)
		if err != nil {
			return err
		}
		if current.Status != ExecutionAttemptStatusRunning || current.Revision != attempt.Revision {
			return ErrResultUnknown
		}
		finished := s.now()
		res := tx.Model(&ExecutionAttempt{}).
			Where("tenant_id = ? AND id = ? AND revision = ?", in.TenantID, attempt.ID, attempt.Revision).
			Updates(map[string]any{
				"status":      ExecutionAttemptStatusFailed,
				"finished_at": finished,
				"revision":    gorm.Expr("revision + 1"),
				"updated_at":  finished,
			})
		if res.Error != nil {
			return stableError(res.Error, ErrConflict)
		}
		if res.RowsAffected == 0 {
			return ErrResultUnknown
		}
		seq, err := nextExecutionErrorSequenceTx(tx, in.TenantID, attempt.ID)
		if err != nil {
			return err
		}
		errRecord := ExecutionError{
			TenantID:           in.TenantID,
			ExecutionAttemptID: attempt.ID,
			Sequence:           seq,
			Category:           failure.Category,
			Code:               failure.Code,
			SafeMessage:        failure.SafeMessage,
			Retryable:          failure.Retryable,
			Details:            failure.Details,
			OccurredAt:         finished,
		}
		if err := validateExecutionError(&errRecord); err != nil {
			return err
		}
		if err := tx.Create(&errRecord).Error; err != nil {
			if isUniqueViolation(err) {
				return ErrDuplicateErrorSequence
			}
			return stableError(err, ErrConflict)
		}
		if err := updateTaskStatusRevisionTx(tx, task, OperationTaskStatusExecutionFailed, &in.ActorID); err != nil {
			return err
		}
		if err := appendAuditEventTx(tx, OperationTaskEvent{
			TenantID:        in.TenantID,
			OperationTaskID: in.OperationTaskID,
			EventType:       OperationTaskEventTypeExecutionFailed,
			ActorType:       OperationTaskEventActorUser,
			ActorID:         &in.ActorID,
			BeforeState:     task.Status,
			AfterState:      OperationTaskStatusExecutionFailed,
			PlatformDraftID: &attempt.PlatformDraftID,
			DraftVersion:    attempt.ExecutedDraftVersion,
			RequestID:       strings.TrimSpace(in.RequestID),
			Reason:          failure.SafeMessage,
			Metadata:        failure.Details,
		}); err != nil {
			return err
		}
		refreshed, err := lockAttempt(tx, in.TenantID, attempt.ID)
		if err != nil {
			return err
		}
		status := ExecutionIdempotencyStatusFailedFinal
		if failure.Retryable {
			status = ExecutionIdempotencyStatusFailedRetryable
		}
		out = ExecutionOutput{Status: status, Attempt: *refreshed, Failure: &failure}
		return nil
	})
	return &out, err
}

type ManualRetryService struct {
	DB                     *gorm.DB
	Orchestrator           *ExecutionOrchestrator
	MaxManualRetryAttempts int
}

func NewManualRetryService(db *gorm.DB, orchestrator *ExecutionOrchestrator, maxManualRetryAttempts int) *ManualRetryService {
	if maxManualRetryAttempts <= 0 {
		maxManualRetryAttempts = DefaultMaxManualRetryAttempts
	}
	return &ManualRetryService{DB: db, Orchestrator: orchestrator, MaxManualRetryAttempts: maxManualRetryAttempts}
}

func (s *ManualRetryService) Retry(ctx context.Context, in ExecutionInput) (*ExecutionOutput, error) {
	if s == nil || s.DB == nil || s.Orchestrator == nil {
		return nil, fmt.Errorf("manual retry service: dependency is nil")
	}
	if s.MaxManualRetryAttempts <= 0 {
		return nil, ErrRetryLimitExceeded
	}
	if retryAuthorizer, ok := s.Orchestrator.Authorizer.(ManualRetryAuthorizer); ok {
		if err := retryAuthorizer.CanRetry(ctx, in.TenantID, in.ActorID, in.OperationTaskID); err != nil {
			return nil, ErrPermissionDenied
		}
	}
	if err := s.validateRetry(ctx, in); err != nil {
		return nil, err
	}
	return s.Orchestrator.execute(ctx, in, true)
}

func (s *ManualRetryService) validateRetry(ctx context.Context, in ExecutionInput) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		task, err := lockTask(tx, in.TenantID, in.OperationTaskID)
		if err != nil {
			return err
		}
		if task.Status != OperationTaskStatusExecutionFailed {
			return ErrStateConflict
		}
		attempt, err := latestAttemptTx(tx, in.TenantID, in.OperationTaskID)
		if err != nil {
			return err
		}
		if attempt.Status != ExecutionAttemptStatusFailed {
			return ErrStateConflict
		}
		latestErr, err := latestExecutionErrorTx(tx, in.TenantID, attempt.ID)
		if err != nil {
			return err
		}
		if !latestErr.Retryable {
			return ErrStateConflict
		}
		if attempt.AttemptNumber >= s.MaxManualRetryAttempts+1 {
			return ErrRetryLimitExceeded
		}
		latestDraft, err := findLatestDraftTx(tx, in.TenantID, in.OperationTaskID)
		if err != nil {
			return err
		}
		approval, err := findLatestApprovedApprovalTx(tx, in.TenantID, in.OperationTaskID)
		if err != nil {
			return err
		}
		if approval.PlatformDraftID != latestDraft.ID || approval.DraftVersion != latestDraft.DraftVersion || approval.DraftPayloadHash != latestDraft.PayloadHash {
			return ErrDraftBindingConflict
		}
		return nil
	})
}

func (s *ExecutionOrchestrator) stateMachine() *TaskStateMachine {
	if s.StateMachine == nil {
		return NewTaskStateMachine()
	}
	return s.StateMachine
}

func (s *ExecutionOrchestrator) classifier() *ExecutionFailureClassifier {
	if s.Classifier == nil {
		return NewExecutionFailureClassifier()
	}
	return s.Classifier
}

func (s *ExecutionOrchestrator) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return utcNow()
}

func executionIdempotencyKey(in ExecutionInput) string {
	if key := strings.TrimSpace(in.IdempotencyKey); key != "" {
		return key
	}
	return strings.TrimSpace(in.RequestID)
}

func forbiddenExecutionMode(mode string) bool {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "production", "real_write", "live", "auto_publish", "auto_listing":
		return true
	default:
		return false
	}
}

func allowedExecutionPortMode(mode string) bool {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case ExecutionPortModeMock, ExecutionPortModeSandboxFixture, ExecutionPortModeLocalDraftFixture:
		return true
	default:
		return false
	}
}

func portModeForDraft(adapterMode string) string {
	switch strings.TrimSpace(strings.ToLower(adapterMode)) {
	case AdapterModeMock:
		return ExecutionPortModeMock
	case AdapterModeSandbox:
		return ExecutionPortModeSandboxFixture
	default:
		return ExecutionPortModeLocalDraftFixture
	}
}

func sanitizeExecutionResult(result DraftExecutionResult) DraftExecutionResult {
	result.ResultType = strings.TrimSpace(strings.ToLower(result.ResultType))
	if !allowedExecutionResultTypes[result.ResultType] {
		result.ResultType = "local_draft"
	}
	result.ExternalReference = strings.TrimSpace(result.ExternalReference)
	result.SafeMetadata = redactSafeJSON(result.SafeMetadata)
	return result
}

func sanitizeFailure(f ExecutionFailure) ExecutionFailure {
	f.Category = strings.TrimSpace(strings.ToLower(f.Category))
	if !allowedExecutionErrorCategories[f.Category] {
		f.Category = ExecutionErrorCategoryInternal
	}
	f.Code = strings.TrimSpace(f.Code)
	if f.Code == "" {
		f.Code = f.Category
	}
	f.SafeMessage = strings.TrimSpace(f.SafeMessage)
	if f.SafeMessage == "" || safeTextHasSecret(f.SafeMessage) {
		f.SafeMessage = safeExecutionMessage(f.Code)
	}
	f.Details = redactSafeJSON(f.Details)
	if f.RetryPolicy == "" {
		f.RetryPolicy = retryPolicyFor(f.Category, f.Retryable)
	}
	if f.ResultCertainty == "" {
		f.ResultCertainty = resultCertaintyFor(f.Category)
	}
	return f
}

func safeExecutionMessage(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		code = "execution_failed"
	}
	return "Execution failed with safe code: " + code
}

func retryPolicyFor(category string, retryable bool) string {
	if !retryable {
		return "manual_retry_not_allowed"
	}
	switch category {
	case ExecutionErrorCategoryAdapterUnavailable, ExecutionErrorCategoryProviderTimeout:
		return "manual_retry_only"
	default:
		return "explicit_contract_manual_retry_only"
	}
}

func resultCertaintyFor(category string) string {
	if category == ExecutionErrorCategoryProviderTimeout {
		return "unknown"
	}
	return "known_failed"
}

func findLatestApprovedApprovalTx(tx *gorm.DB, tenantID int64, taskID uuid.UUID) (*ApprovalRecord, error) {
	var record ApprovalRecord
	err := tx.Where("tenant_id = ? AND operation_task_id = ? AND decision = ?", tenantID, taskID, ApprovalDecisionApproved).
		Order("created_at DESC, id DESC").
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, stableError(err, ErrConflict)
	}
	return &record, nil
}

func findAttemptByIdempotencyTx(tx *gorm.DB, tenantID int64, taskID uuid.UUID, key string) (*ExecutionAttempt, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, ErrNotFound
	}
	var attempt ExecutionAttempt
	err := tx.Where("tenant_id = ? AND operation_task_id = ? AND idempotency_key = ?", tenantID, taskID, key).
		First(&attempt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, stableError(err, ErrConflict)
	}
	return &attempt, nil
}

func replayAttemptTx(tx *gorm.DB, attempt *ExecutionAttempt, latestDraft *PlatformDraft, actorType string) (*ExecutionOutput, error) {
	if attempt.ExecutedDraftVersion != latestDraft.DraftVersion || attempt.ExecutedDraftPayloadHash != latestDraft.PayloadHash {
		return nil, ErrIdemPayloadConflict
	}
	out := &ExecutionOutput{Attempt: *attempt}
	switch attempt.Status {
	case ExecutionAttemptStatusQueued, ExecutionAttemptStatusRunning:
		out.Status = ExecutionIdempotencyStatusInProgress
	case ExecutionAttemptStatusSucceeded:
		out.Status = ExecutionIdempotencyStatusSucceeded
		out.Result = DraftExecutionResult{
			ResultType:        attempt.ResultType,
			ExternalReference: attempt.ExternalReference,
			SafeMetadata:      attempt.SafeMetadata,
		}
	case ExecutionAttemptStatusFailed:
		latestErr, err := latestExecutionErrorTx(tx, attempt.TenantID, attempt.ID)
		if err != nil {
			return nil, err
		}
		failure := ExecutionFailure{
			Category:    latestErr.Category,
			Code:        latestErr.Code,
			SafeMessage: latestErr.SafeMessage,
			Retryable:   latestErr.Retryable,
			Details:     latestErr.Details,
		}
		out.Failure = &failure
		if latestErr.Retryable {
			out.Status = ExecutionIdempotencyStatusFailedRetryable
		} else {
			out.Status = ExecutionIdempotencyStatusFailedFinal
		}
	default:
		out.Status = ExecutionIdempotencyStatusFailedFinal
	}
	_ = actorType
	return out, nil
}

func hasActiveAttemptTx(tx *gorm.DB, tenantID int64, taskID uuid.UUID) bool {
	var count int64
	_ = tx.Model(&ExecutionAttempt{}).
		Where("tenant_id = ? AND operation_task_id = ? AND status IN ?", tenantID, taskID, []string{ExecutionAttemptStatusQueued, ExecutionAttemptStatusRunning}).
		Count(&count).Error
	return count > 0
}

func nextAttemptNumberTx(tx *gorm.DB, tenantID int64, taskID uuid.UUID) (int, error) {
	var latest ExecutionAttempt
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND operation_task_id = ?", tenantID, taskID).
		Order("attempt_number DESC, id DESC").
		First(&latest).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 1, nil
	}
	if err != nil {
		return 0, stableError(err, ErrConflict)
	}
	return latest.AttemptNumber + 1, nil
}

func latestAttemptTx(tx *gorm.DB, tenantID int64, taskID uuid.UUID) (*ExecutionAttempt, error) {
	var latest ExecutionAttempt
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND operation_task_id = ?", tenantID, taskID).
		Order("attempt_number DESC, id DESC").
		First(&latest).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, stableError(err, ErrConflict)
	}
	return &latest, nil
}

func lockAttempt(tx *gorm.DB, tenantID int64, id uuid.UUID) (*ExecutionAttempt, error) {
	var attempt ExecutionAttempt
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&attempt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, stableError(err, ErrConflict)
	}
	return &attempt, nil
}

func nextExecutionErrorSequenceTx(tx *gorm.DB, tenantID int64, attemptID uuid.UUID) (int, error) {
	var latest ExecutionError
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND execution_attempt_id = ?", tenantID, attemptID).
		Order("sequence DESC, id DESC").
		First(&latest).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 1, nil
	}
	if err != nil {
		return 0, stableError(err, ErrConflict)
	}
	return latest.Sequence + 1, nil
}

func latestExecutionErrorTx(tx *gorm.DB, tenantID int64, attemptID uuid.UUID) (*ExecutionError, error) {
	var latest ExecutionError
	err := tx.Where("tenant_id = ? AND execution_attempt_id = ?", tenantID, attemptID).
		Order("sequence DESC, id DESC").
		First(&latest).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, stableError(err, ErrConflict)
	}
	return &latest, nil
}
