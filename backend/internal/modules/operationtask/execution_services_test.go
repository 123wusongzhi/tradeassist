package operationtask_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type allowExecutionAuthorizer struct {
	err error
}

func (a allowExecutionAuthorizer) CanExecute(context.Context, int64, uuid.UUID, uuid.UUID) error {
	return a.err
}

type recordingExecutionPort struct {
	mu          sync.Mutex
	db          *gorm.DB
	result      operationtask.DraftExecutionResult
	err         error
	calls       int
	started     chan struct{}
	release     chan struct{}
	statusInRun string
}

func newRecordingExecutionPort() *recordingExecutionPort {
	return &recordingExecutionPort{
		result: operationtask.DraftExecutionResult{
			ResultType:        "sandbox_fixture",
			ExternalReference: "fixture-draft-1",
			SafeMetadata:      datatypes.JSON([]byte(`{"fixture":true}`)),
			CompletedAt:       time.Now().UTC(),
		},
	}
}

func (p *recordingExecutionPort) ExecuteDraft(ctx context.Context, in operationtask.DraftExecutionInput) (operationtask.DraftExecutionResult, error) {
	p.mu.Lock()
	p.calls++
	if p.db != nil {
		task, err := operationtask.NewOperationTaskRepository(p.db).GetByID(ctx, in.TenantID, in.OperationTaskID)
		if err == nil {
			p.statusInRun = task.Status
		}
	}
	if p.started != nil && p.calls == 1 {
		close(p.started)
	}
	release := p.release
	p.mu.Unlock()
	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
			return operationtask.DraftExecutionResult{}, ctx.Err()
		}
	}
	return p.result, p.err
}

func (p *recordingExecutionPort) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

func (p *recordingExecutionPort) statusDuringRun() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.statusInRun
}

func createApprovedExecutionTask(t *testing.T, db *gorm.DB) (operationtask.OperationTask, operationtask.PlatformDraft, operationtask.ApprovalRecord, uuid.UUID) {
	t.Helper()
	task, draft, actor := createPendingReviewDraft(t, db)
	reviewer := uuid.New()
	approval, err := operationtask.NewApprovalService(db, allowReviewAuthorizer{}).Approve(context.Background(), operationtask.ApprovalInput{
		TenantID:         task.TenantID,
		OperationTaskID:  task.ID,
		ExpectedRevision: task.Revision,
		DraftVersion:     draft.DraftVersion,
		DraftPayloadHash: draft.PayloadHash,
		ReviewerID:       reviewer,
		ReviewerRole:     operationtask.ReviewerRoleReviewer,
		Reason:           "approved for fixture execution",
		RequestID:        uuid.NewString(),
		IdempotencyKey:   uuid.NewString(),
	})
	require.NoError(t, err)
	approved, err := operationtask.NewOperationTaskRepository(db).GetByID(context.Background(), task.TenantID, task.ID)
	require.NoError(t, err)
	return *approved, draft, *approval, actor
}

func TestExecutionOrchestratorSuccessIsAtomicAndPortOutsideTransaction(t *testing.T) {
	db := openOperationTaskTestDB(t)
	task, _, _, actor := createApprovedExecutionTask(t, db)
	port := newRecordingExecutionPort()
	port.db = db
	out, err := operationtask.NewExecutionOrchestrator(db, allowExecutionAuthorizer{}, port).Execute(context.Background(), operationtask.ExecutionInput{
		TenantID:        task.TenantID,
		OperationTaskID: task.ID,
		ActorID:         actor,
		RequestID:       "req-execute-success",
		IdempotencyKey:  "idem-execute-success",
	})
	require.NoError(t, err)
	require.Equal(t, operationtask.ExecutionIdempotencyStatusSucceeded, out.Status)
	require.Equal(t, operationtask.ExecutionAttemptStatusSucceeded, out.Attempt.Status)
	require.Equal(t, "sandbox_fixture", out.Attempt.ResultType)
	require.Equal(t, 1, port.callCount())
	require.Equal(t, operationtask.OperationTaskStatusExecuting, port.statusDuringRun())

	after, err := operationtask.NewOperationTaskRepository(db).GetByID(context.Background(), task.TenantID, task.ID)
	require.NoError(t, err)
	require.Equal(t, operationtask.OperationTaskStatusDraftWritten, after.Status)
	events, err := operationtask.NewOperationTaskEventRepository(db).ListByTask(context.Background(), operationtask.OperationTaskEventListParams{
		TenantID:        task.TenantID,
		OperationTaskID: task.ID,
		Limit:           20,
	})
	require.NoError(t, err)
	var eventTypes []string
	for _, event := range events.Items {
		eventTypes = append(eventTypes, event.EventType)
	}
	require.Contains(t, eventTypes, operationtask.OperationTaskEventTypeExecutionQueued)
	require.Contains(t, eventTypes, operationtask.OperationTaskEventTypeExecutionStarted)
	require.Contains(t, eventTypes, operationtask.OperationTaskEventTypeDraftWritten)
}

func TestExecutionOrchestratorRejectsPreconditionsWithoutCallingPort(t *testing.T) {
	db := openOperationTaskTestDB(t)
	task, _, _ := createPendingReviewDraft(t, db)
	actor := uuid.New()
	port := newRecordingExecutionPort()

	_, err := operationtask.NewExecutionOrchestrator(db, allowExecutionAuthorizer{}, port).Execute(context.Background(), operationtask.ExecutionInput{
		TenantID:        task.TenantID,
		OperationTaskID: task.ID,
		ActorID:         actor,
		RequestID:       "req-pending",
		IdempotencyKey:  "idem-pending",
	})
	require.ErrorIs(t, err, operationtask.ErrStateConflict)
	require.Equal(t, 0, port.callCount())

	_, err = operationtask.NewExecutionOrchestrator(db, nil, port).Execute(context.Background(), operationtask.ExecutionInput{
		TenantID:        task.TenantID,
		OperationTaskID: task.ID,
		ActorID:         actor,
		RequestID:       "req-no-authz",
		IdempotencyKey:  "idem-no-authz",
	})
	require.ErrorIs(t, err, operationtask.ErrPermissionDenied)

	approved, _, _, actor := createApprovedExecutionTask(t, db)
	_, err = operationtask.NewExecutionOrchestrator(db, allowExecutionAuthorizer{}, port).Execute(context.Background(), operationtask.ExecutionInput{
		TenantID:        approved.TenantID,
		OperationTaskID: approved.ID,
		ActorID:         actor,
		RequestID:       "req-production",
		IdempotencyKey:  "idem-production",
		AdapterMode:     "production",
	})
	require.ErrorIs(t, err, operationtask.ErrExecutionModeForbidden)
	require.Equal(t, 0, port.callCount())
}

func TestExecutionOrchestratorFailureClassificationSafeRecordingAndManualRetry(t *testing.T) {
	db := openOperationTaskTestDB(t)
	task, _, _, actor := createApprovedExecutionTask(t, db)
	port := newRecordingExecutionPort()
	port.err = &operationtask.ExecutionDomainError{
		Category:    operationtask.ExecutionErrorCategoryProviderTimeout,
		Code:        "provider_timeout",
		SafeMessage: "Sandbox fixture timed out",
		Retryable:   true,
		Details:     datatypes.JSON([]byte(`{"resultCertainty":"unknown"}`)),
	}
	orch := operationtask.NewExecutionOrchestrator(db, allowExecutionAuthorizer{}, port)
	out, err := orch.Execute(context.Background(), operationtask.ExecutionInput{
		TenantID:        task.TenantID,
		OperationTaskID: task.ID,
		ActorID:         actor,
		RequestID:       "req-execute-timeout",
		IdempotencyKey:  "idem-execute-timeout",
	})
	require.Error(t, err)
	require.Equal(t, operationtask.ExecutionIdempotencyStatusFailedRetryable, out.Status)
	require.Equal(t, operationtask.ExecutionAttemptStatusFailed, out.Attempt.Status)
	latestErr, err := operationtask.NewExecutionErrorRepository(db).GetLatestByAttempt(context.Background(), task.TenantID, out.Attempt.ID)
	require.NoError(t, err)
	require.Equal(t, operationtask.ExecutionErrorCategoryProviderTimeout, latestErr.Category)
	require.True(t, latestErr.Retryable)

	port.err = nil
	retryOut, err := operationtask.NewManualRetryService(db, orch, 3).Retry(context.Background(), operationtask.ExecutionInput{
		TenantID:        task.TenantID,
		OperationTaskID: task.ID,
		ActorID:         actor,
		RequestID:       "req-manual-retry",
		IdempotencyKey:  "idem-manual-retry",
	})
	require.NoError(t, err)
	require.Equal(t, operationtask.ExecutionIdempotencyStatusSucceeded, retryOut.Status)
	require.Equal(t, 2, retryOut.Attempt.AttemptNumber)
	require.Equal(t, 2, port.callCount())
	events, err := operationtask.NewOperationTaskEventRepository(db).ListByTask(context.Background(), operationtask.OperationTaskEventListParams{
		TenantID:        task.TenantID,
		OperationTaskID: task.ID,
		Limit:           30,
	})
	require.NoError(t, err)
	foundRetry := false
	for _, event := range events.Items {
		if event.EventType == operationtask.OperationTaskEventTypeRetryRequested {
			foundRetry = true
		}
	}
	require.True(t, foundRetry)
}

func TestManualRetryBlocksNonRetryableAndLimitExceeded(t *testing.T) {
	db := openOperationTaskTestDB(t)
	task, _, _, actor := createApprovedExecutionTask(t, db)
	port := newRecordingExecutionPort()
	port.err = &operationtask.ExecutionDomainError{
		Category:    operationtask.ExecutionErrorCategoryProviderRejected,
		Code:        "provider_rejected",
		SafeMessage: "Sandbox fixture rejected the draft",
		Retryable:   false,
	}
	orch := operationtask.NewExecutionOrchestrator(db, allowExecutionAuthorizer{}, port)
	_, err := orch.Execute(context.Background(), operationtask.ExecutionInput{
		TenantID:        task.TenantID,
		OperationTaskID: task.ID,
		ActorID:         actor,
		RequestID:       "req-rejected",
		IdempotencyKey:  "idem-rejected",
	})
	require.Error(t, err)
	_, err = operationtask.NewManualRetryService(db, orch, 3).Retry(context.Background(), operationtask.ExecutionInput{
		TenantID:        task.TenantID,
		OperationTaskID: task.ID,
		ActorID:         actor,
		RequestID:       "req-retry-rejected",
		IdempotencyKey:  "idem-retry-rejected",
	})
	require.ErrorIs(t, err, operationtask.ErrStateConflict)

	task2, _, _, actor2 := createApprovedExecutionTask(t, db)
	port2 := newRecordingExecutionPort()
	port2.err = &operationtask.ExecutionDomainError{
		Category:    operationtask.ExecutionErrorCategoryProviderTimeout,
		Code:        "provider_timeout",
		SafeMessage: "Sandbox fixture timed out",
		Retryable:   true,
	}
	orch2 := operationtask.NewExecutionOrchestrator(db, allowExecutionAuthorizer{}, port2)
	_, err = orch2.Execute(context.Background(), operationtask.ExecutionInput{
		TenantID:        task2.TenantID,
		OperationTaskID: task2.ID,
		ActorID:         actor2,
		RequestID:       "req-timeout-limit",
		IdempotencyKey:  "idem-timeout-limit",
	})
	require.Error(t, err)
	_, err = (&operationtask.ManualRetryService{DB: db, Orchestrator: orch2}).Retry(context.Background(), operationtask.ExecutionInput{
		TenantID:        task2.TenantID,
		OperationTaskID: task2.ID,
		ActorID:         actor2,
		RequestID:       "req-limit",
		IdempotencyKey:  "idem-limit",
	})
	require.ErrorIs(t, err, operationtask.ErrRetryLimitExceeded)
}

func TestExecutionIdempotencyDuplicateReplayInProgressAndPayloadConflict(t *testing.T) {
	db := openOperationTaskTestDB(t)
	task, _, _, actor := createApprovedExecutionTask(t, db)
	port := newRecordingExecutionPort()
	port.started = make(chan struct{})
	port.release = make(chan struct{})
	orch := operationtask.NewExecutionOrchestrator(db, allowExecutionAuthorizer{}, port)
	input := operationtask.ExecutionInput{
		TenantID:        task.TenantID,
		OperationTaskID: task.ID,
		ActorID:         actor,
		RequestID:       "req-idem",
		IdempotencyKey:  "idem-idem",
	}
	errs := make(chan error, 1)
	go func() {
		_, err := orch.Execute(context.Background(), input)
		errs <- err
	}()
	<-port.started
	replay, err := orch.Execute(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, operationtask.ExecutionIdempotencyStatusInProgress, replay.Status)
	require.Equal(t, 1, port.callCount())
	close(port.release)
	require.NoError(t, <-errs)

	successReplay, err := orch.Execute(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, operationtask.ExecutionIdempotencyStatusSucceeded, successReplay.Status)
	require.Equal(t, 1, port.callCount())

	newDraft := sampleDraft(task, 2, hash2)
	require.NoError(t, operationtask.NewPlatformDraftRepository(db).CreateVersion(context.Background(), &newDraft))
	_, err = orch.Execute(context.Background(), input)
	require.ErrorIs(t, err, operationtask.ErrIdemPayloadConflict)
}

func TestExecutionClassifierRulesAndSafeMessages(t *testing.T) {
	classifier := operationtask.NewExecutionFailureClassifier()
	cases := []struct {
		err       error
		category  string
		retryable bool
	}{
		{operationtask.ErrValidation, operationtask.ExecutionErrorCategoryValidation, false},
		{operationtask.ErrPermissionDenied, operationtask.ExecutionErrorCategoryPermissionDenied, false},
		{operationtask.ErrStateConflict, operationtask.ExecutionErrorCategoryStateConflict, false},
		{&operationtask.ExecutionDomainError{Category: operationtask.ExecutionErrorCategoryAdapterUnavailable, Code: "adapter_unavailable", SafeMessage: "Adapter unavailable", Retryable: true}, operationtask.ExecutionErrorCategoryAdapterUnavailable, true},
		{context.DeadlineExceeded, operationtask.ExecutionErrorCategoryProviderTimeout, true},
		{&operationtask.ExecutionDomainError{Category: operationtask.ExecutionErrorCategoryProviderRejected, Code: "provider_rejected", SafeMessage: "Provider rejected", Retryable: false}, operationtask.ExecutionErrorCategoryProviderRejected, false},
		{operationtask.ErrIdemPayloadConflict, operationtask.ExecutionErrorCategoryIdempotencyConflict, false},
		{errors.New("surprise"), operationtask.ExecutionErrorCategoryInternal, false},
	}
	for _, tc := range cases {
		got := classifier.Classify(tc.err)
		require.Equal(t, tc.category, got.Category)
		require.Equal(t, tc.retryable, got.Retryable)
		require.NotContains(t, got.SafeMessage, "Bearer ")
	}
	secret := classifier.Classify(&operationtask.ExecutionDomainError{
		Category:    operationtask.ExecutionErrorCategoryInternal,
		Code:        "internal_error",
		SafeMessage: "Bearer secret",
		Details:     datatypes.JSON([]byte(`{"accessToken":"secret","safe":"kept"}`)),
	})
	require.NotContains(t, secret.SafeMessage, "Bearer")
	require.NotContains(t, string(secret.Details), "secret")
	require.Contains(t, string(secret.Details), "safe")
}
