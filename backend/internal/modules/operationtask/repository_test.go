package operationtask_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	hash1 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	hash2 = "1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	hash3 = "2123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

func openOperationTaskTestDB(t testing.TB) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:operationtask_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)
	require.NoError(t, operationtask.Migrate(db))
	return db
}

func sampleTask(tenant int64, idem string) operationtask.OperationTask {
	var key *string
	if idem != "" {
		key = &idem
	}
	return operationtask.OperationTask{
		TenantID:        tenant,
		SourceType:      operationtask.OperationTaskSourceAISuggestion,
		SourceReference: "product-1",
		TaskType:        operationtask.OperationTaskTypeProductContent,
		Platform:        operationtask.PlatformDouyin,
		Title:           "Review product content",
		Summary:         "AI suggested title update",
		Payload:         datatypes.JSON([]byte(`{"title":"summer dress"}`)),
		Status:          operationtask.OperationTaskStatusSuggested,
		Priority:        operationtask.OperationTaskPriorityNormal,
		IdempotencyKey:  key,
	}
}

func sampleDraft(task operationtask.OperationTask, version int, hash string) operationtask.PlatformDraft {
	return operationtask.PlatformDraft{
		TenantID:        task.TenantID,
		OperationTaskID: task.ID,
		Platform:        task.Platform,
		AdapterMode:     operationtask.AdapterModeSandbox,
		DraftVersion:    version,
		Payload:         datatypes.JSON([]byte(`{"draft":{"title":"summer dress v1"}}`)),
		PayloadHash:     hash,
		Status:          operationtask.PlatformDraftStatusEditable,
		ChangeReason:    "initial draft",
	}
}

func sampleApproval(task operationtask.OperationTask, draft operationtask.PlatformDraft, key string) operationtask.ApprovalRecord {
	var idem *string
	if key != "" {
		idem = &key
	}
	return operationtask.ApprovalRecord{
		TenantID:         task.TenantID,
		OperationTaskID:  task.ID,
		PlatformDraftID:  draft.ID,
		Decision:         operationtask.ApprovalDecisionApproved,
		DraftVersion:     draft.DraftVersion,
		DraftPayloadHash: draft.PayloadHash,
		ReviewerID:       uuid.New(),
		ReviewerRole:     operationtask.ReviewerRoleReviewer,
		Reason:           "content checked",
		RequestID:        "req-approval",
		IdempotencyKey:   idem,
	}
}

func sampleAttempt(task operationtask.OperationTask, draft operationtask.PlatformDraft, approval operationtask.ApprovalRecord, number int, key string) operationtask.ExecutionAttempt {
	var idem *string
	if key != "" {
		idem = &key
	}
	return operationtask.ExecutionAttempt{
		TenantID:                 task.TenantID,
		OperationTaskID:          task.ID,
		PlatformDraftID:          draft.ID,
		ApprovalRecordID:         approval.ID,
		AttemptNumber:            number,
		Status:                   operationtask.ExecutionAttemptStatusQueued,
		AdapterMode:              operationtask.AdapterModeSandbox,
		Platform:                 task.Platform,
		ApprovedDraftVersion:     approval.DraftVersion,
		ApprovedDraftPayloadHash: approval.DraftPayloadHash,
		ExecutedDraftVersion:     draft.DraftVersion,
		ExecutedDraftPayloadHash: draft.PayloadHash,
		RequestID:                "req-execution",
		IdempotencyKey:           idem,
	}
}

func sampleExecutionError(attempt operationtask.ExecutionAttempt, sequence int) operationtask.ExecutionError {
	return operationtask.ExecutionError{
		TenantID:           attempt.TenantID,
		ExecutionAttemptID: attempt.ID,
		Sequence:           sequence,
		Category:           operationtask.ExecutionErrorCategoryProviderTimeout,
		Code:               "PROVIDER_TIMEOUT",
		SafeMessage:        "Provider timed out after sandbox request",
		Retryable:          true,
		Details:            datatypes.JSON([]byte(`{"timeoutMs":3000}`)),
	}
}

func sampleEvent(task operationtask.OperationTask, sequence int) operationtask.OperationTaskEvent {
	actorID := uuid.New()
	return operationtask.OperationTaskEvent{
		TenantID:        task.TenantID,
		OperationTaskID: task.ID,
		Sequence:        sequence,
		EventType:       operationtask.OperationTaskEventTypeTaskCreated,
		ActorType:       operationtask.OperationTaskEventActorUser,
		ActorID:         &actorID,
		AfterState:      operationtask.OperationTaskStatusSuggested,
		RequestID:       "req-event",
		Reason:          "created from suggestion",
		Metadata:        datatypes.JSON([]byte(`{"source":"test"}`)),
	}
}

func createTaskDraftApproval(t *testing.T, db *gorm.DB) (operationtask.OperationTask, operationtask.PlatformDraft, operationtask.ApprovalRecord) {
	t.Helper()
	ctx := context.Background()
	taskRepo := operationtask.NewOperationTaskRepository(db)
	draftRepo := operationtask.NewPlatformDraftRepository(db)
	approvalRepo := operationtask.NewApprovalRecordRepository(db)
	task := sampleTask(101, uuid.NewString())
	require.NoError(t, taskRepo.Create(ctx, &task))
	draft := sampleDraft(task, 1, hash1)
	require.NoError(t, draftRepo.CreateVersion(ctx, &draft))
	approval := sampleApproval(task, draft, uuid.NewString())
	require.NoError(t, approvalRepo.CreateDecision(ctx, &approval))
	return task, draft, approval
}

func TestOperationTaskRepositoryCreateReadTenantIdempotencyRevisionAndList(t *testing.T) {
	db := openOperationTaskTestDB(t)
	ctx := context.Background()
	repo := operationtask.NewOperationTaskRepository(db)

	task := sampleTask(101, "idem-1")
	require.NoError(t, repo.Create(ctx, &task))
	require.Equal(t, 1, task.Revision)

	got, err := repo.GetByID(ctx, 101, task.ID)
	require.NoError(t, err)
	require.Equal(t, task.ID, got.ID)

	_, err = repo.GetByID(ctx, 202, task.ID)
	require.ErrorIs(t, err, operationtask.ErrNotFound)

	byKey, err := repo.GetByIdempotencyKey(ctx, 101, "idem-1")
	require.NoError(t, err)
	require.Equal(t, task.ID, byKey.ID)

	dup := sampleTask(101, "idem-1")
	require.ErrorIs(t, repo.Create(ctx, &dup), operationtask.ErrDuplicateIdempotencyKey)

	otherTenant := sampleTask(202, "idem-1")
	require.NoError(t, repo.Create(ctx, &otherTenant))

	emptyA := sampleTask(101, "")
	emptyA.SourceReference = "empty-a"
	emptyB := sampleTask(101, "")
	emptyB.SourceReference = "empty-b"
	require.NoError(t, repo.Create(ctx, &emptyA))
	require.NoError(t, repo.Create(ctx, &emptyB))
	baseTime := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	require.NoError(t, db.Model(&operationtask.OperationTask{}).Where("id = ?", emptyA.ID).UpdateColumn("updated_at", baseTime.Add(2*time.Minute)).Error)
	require.NoError(t, db.Model(&operationtask.OperationTask{}).Where("id = ?", emptyB.ID).UpdateColumn("updated_at", baseTime.Add(time.Minute)).Error)

	nextStatus := operationtask.OperationTaskStatusDraftPreparing
	updated, err := repo.UpdateRevision(ctx, 101, task.ID, 1, operationtask.OperationTaskPatch{Status: &nextStatus})
	require.NoError(t, err)
	require.Equal(t, 2, updated.Revision)
	require.Equal(t, nextStatus, updated.Status)

	_, err = repo.UpdateRevision(ctx, 101, task.ID, 1, operationtask.OperationTaskPatch{Status: &nextStatus})
	require.ErrorIs(t, err, operationtask.ErrRevisionConflict)

	filtered, err := repo.List(ctx, operationtask.OperationTaskListParams{
		TenantID: 101,
		Status:   operationtask.OperationTaskStatusSuggested,
		Platform: operationtask.PlatformDouyin,
		TaskType: operationtask.OperationTaskTypeProductContent,
		Limit:    1,
	})
	require.NoError(t, err)
	require.Len(t, filtered.Items, 1)
	require.True(t, filtered.HasMore)
	require.NotEmpty(t, filtered.NextCursor)

	page2, err := repo.List(ctx, operationtask.OperationTaskListParams{
		TenantID: 101,
		Status:   operationtask.OperationTaskStatusSuggested,
		Platform: operationtask.PlatformDouyin,
		TaskType: operationtask.OperationTaskTypeProductContent,
		Limit:    10,
		Cursor:   filtered.NextCursor,
	})
	require.NoError(t, err)
	require.NotEmpty(t, page2.Items)
	require.NotEqual(t, filtered.Items[0].ID, page2.Items[0].ID)
}

func TestOperationTaskValidationRejectsBadPayloadsAndEnums(t *testing.T) {
	db := openOperationTaskTestDB(t)
	ctx := context.Background()
	repo := operationtask.NewOperationTaskRepository(db)

	missingTenant := sampleTask(0, "missing-tenant")
	require.ErrorIs(t, repo.Create(ctx, &missingTenant), operationtask.ErrValidation)

	badJSON := sampleTask(101, "bad-json")
	badJSON.Payload = datatypes.JSON([]byte(`{"unterminated"`))
	require.ErrorIs(t, repo.Create(ctx, &badJSON), operationtask.ErrValidation)

	secretPayload := sampleTask(101, "secret-json")
	secretPayload.Payload = datatypes.JSON([]byte(`{"accessToken":"should-not-persist"}`))
	require.ErrorIs(t, repo.Create(ctx, &secretPayload), operationtask.ErrValidation)

	badStatus := sampleTask(101, "bad-status")
	badStatus.Status = "surprise"
	require.ErrorIs(t, repo.Create(ctx, &badStatus), operationtask.ErrValidation)
}

func TestPlatformDraftRepositoryVersionsValidationTenantAndForeignKey(t *testing.T) {
	db := openOperationTaskTestDB(t)
	ctx := context.Background()
	taskRepo := operationtask.NewOperationTaskRepository(db)
	draftRepo := operationtask.NewPlatformDraftRepository(db)

	task := sampleTask(101, "draft-task")
	require.NoError(t, taskRepo.Create(ctx, &task))

	d1 := sampleDraft(task, 1, hash1)
	require.NoError(t, draftRepo.CreateVersion(ctx, &d1))

	got, err := draftRepo.GetByID(ctx, 101, d1.ID)
	require.NoError(t, err)
	require.Equal(t, d1.ID, got.ID)

	v1, err := draftRepo.GetVersion(ctx, 101, task.ID, 1)
	require.NoError(t, err)
	require.Equal(t, d1.ID, v1.ID)

	missingTask := sampleDraft(task, 1, hash2)
	missingTask.OperationTaskID = uuid.New()
	require.ErrorIs(t, draftRepo.CreateVersion(ctx, &missingTask), operationtask.ErrNotFound)

	tenantMismatch := sampleDraft(task, 2, hash2)
	tenantMismatch.TenantID = 202
	require.ErrorIs(t, draftRepo.CreateVersion(ctx, &tenantMismatch), operationtask.ErrTenantMismatch)

	duplicate := sampleDraft(task, 1, hash2)
	require.ErrorIs(t, draftRepo.CreateVersion(ctx, &duplicate), operationtask.ErrDuplicateDraftVersion)

	badHash := sampleDraft(task, 2, "not-a-sha")
	require.ErrorIs(t, draftRepo.CreateVersion(ctx, &badHash), operationtask.ErrValidation)

	for _, mode := range []string{"production", "real_write", "auto_publish"} {
		badMode := sampleDraft(task, 2, hash2)
		badMode.AdapterMode = mode
		require.ErrorIs(t, draftRepo.CreateVersion(ctx, &badMode), operationtask.ErrValidation)
	}

	d2 := sampleDraft(task, 2, hash2)
	require.NoError(t, draftRepo.CreateVersion(ctx, &d2))
	d3 := sampleDraft(task, 3, hash3)
	require.NoError(t, draftRepo.CreateVersion(ctx, &d3))

	latest, err := draftRepo.GetLatest(ctx, 101, task.ID)
	require.NoError(t, err)
	require.Equal(t, 3, latest.DraftVersion)

	versions, err := draftRepo.ListVersions(ctx, 101, task.ID)
	require.NoError(t, err)
	require.Len(t, versions, 3)
	require.Equal(t, []int{3, 2, 1}, []int{versions[0].DraftVersion, versions[1].DraftVersion, versions[2].DraftVersion})

	del := db.Delete(&operationtask.OperationTask{}, "tenant_id = ? AND id = ?", 101, task.ID)
	require.Error(t, del.Error)
}

func TestConcurrentIdempotencyAndDraftVersionUseDatabaseConstraints(t *testing.T) {
	db := openOperationTaskTestDB(t)
	ctx := context.Background()
	taskRepo := operationtask.NewOperationTaskRepository(db)
	draftRepo := operationtask.NewPlatformDraftRepository(db)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			task := sampleTask(101, "concurrent-idem")
			task.SourceReference = fmt.Sprintf("source-%d", i)
			errs <- taskRepo.Create(ctx, &task)
		}(i)
	}
	wg.Wait()
	close(errs)
	var success, duplicate int
	for err := range errs {
		switch {
		case err == nil:
			success++
		case errors.Is(err, operationtask.ErrDuplicateIdempotencyKey):
			duplicate++
		default:
			t.Fatalf("unexpected idempotency error: %v", err)
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, duplicate)
	var taskCount int64
	require.NoError(t, db.Model(&operationtask.OperationTask{}).Where("tenant_id = ? AND idempotency_key = ?", 101, "concurrent-idem").Count(&taskCount).Error)
	require.Equal(t, int64(1), taskCount)

	task := sampleTask(101, "draft-concurrent-task")
	require.NoError(t, taskRepo.Create(ctx, &task))

	errs = make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			draft := sampleDraft(task, 1, hash1)
			draft.ChangeReason = fmt.Sprintf("concurrent-%d", i)
			errs <- draftRepo.CreateVersion(ctx, &draft)
		}(i)
	}
	wg.Wait()
	close(errs)
	success, duplicate = 0, 0
	for err := range errs {
		switch {
		case err == nil:
			success++
		case errors.Is(err, operationtask.ErrDuplicateDraftVersion):
			duplicate++
		default:
			t.Fatalf("unexpected draft error: %v", err)
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, duplicate)
	var draftCount int64
	require.NoError(t, db.Model(&operationtask.PlatformDraft{}).Where("tenant_id = ? AND operation_task_id = ? AND draft_version = ?", 101, task.ID, 1).Count(&draftCount).Error)
	require.Equal(t, int64(1), draftCount)
}

func TestApprovalRecordRepositoryValidationIdempotencyTenantAndImmutable(t *testing.T) {
	db := openOperationTaskTestDB(t)
	ctx := context.Background()
	taskRepo := operationtask.NewOperationTaskRepository(db)
	draftRepo := operationtask.NewPlatformDraftRepository(db)
	approvalRepo := operationtask.NewApprovalRecordRepository(db)

	task := sampleTask(101, "approval-task")
	require.NoError(t, taskRepo.Create(ctx, &task))
	draft := sampleDraft(task, 1, hash1)
	require.NoError(t, draftRepo.CreateVersion(ctx, &draft))

	approved := sampleApproval(task, draft, "approval-idem")
	require.NoError(t, approvalRepo.CreateDecision(ctx, &approved))
	require.NotEqual(t, uuid.Nil, approved.ID)

	got, err := approvalRepo.GetByID(ctx, 101, approved.ID)
	require.NoError(t, err)
	require.Equal(t, approved.ID, got.ID)

	latest, err := approvalRepo.GetLatestByTask(ctx, 101, task.ID)
	require.NoError(t, err)
	require.Equal(t, approved.ID, latest.ID)

	duplicate := sampleApproval(task, draft, "approval-idem")
	require.NoError(t, approvalRepo.CreateDecision(ctx, &duplicate))
	require.Equal(t, approved.ID, duplicate.ID)
	var count int64
	require.NoError(t, db.Model(&operationtask.ApprovalRecord{}).Where("tenant_id = ? AND operation_task_id = ? AND idempotency_key = ?", 101, task.ID, "approval-idem").Count(&count).Error)
	require.Equal(t, int64(1), count)

	otherTenant := sampleTask(202, "approval-other-tenant")
	require.NoError(t, taskRepo.Create(ctx, &otherTenant))
	otherDraft := sampleDraft(otherTenant, 1, hash1)
	require.NoError(t, draftRepo.CreateVersion(ctx, &otherDraft))
	reusedKey := sampleApproval(otherTenant, otherDraft, "approval-idem")
	require.NoError(t, approvalRepo.CreateDecision(ctx, &reusedKey))

	rejected := sampleApproval(task, draft, "approval-rejected")
	rejected.Decision = operationtask.ApprovalDecisionRejected
	rejected.Reason = "missing required description"
	require.NoError(t, approvalRepo.CreateDecision(ctx, &rejected))

	missingReason := sampleApproval(task, draft, "approval-missing-reason")
	missingReason.Decision = operationtask.ApprovalDecisionRejected
	missingReason.Reason = ""
	require.ErrorIs(t, approvalRepo.CreateDecision(ctx, &missingReason), operationtask.ErrValidation)

	mismatchedVersion := sampleApproval(task, draft, "approval-bad-version")
	mismatchedVersion.DraftVersion = 2
	require.ErrorIs(t, approvalRepo.CreateDecision(ctx, &mismatchedVersion), operationtask.ErrReferenceMismatch)

	mismatchedHash := sampleApproval(task, draft, "approval-bad-hash")
	mismatchedHash.DraftPayloadHash = hash2
	require.ErrorIs(t, approvalRepo.CreateDecision(ctx, &mismatchedHash), operationtask.ErrReferenceMismatch)

	foreignTask := sampleTask(101, "approval-foreign-task")
	require.NoError(t, taskRepo.Create(ctx, &foreignTask))
	foreignDraftApproval := sampleApproval(foreignTask, draft, "approval-foreign-draft")
	require.ErrorIs(t, approvalRepo.CreateDecision(ctx, &foreignDraftApproval), operationtask.ErrReferenceMismatch)

	crossTenant := sampleApproval(task, draft, "approval-cross-tenant")
	crossTenant.TenantID = 202
	require.ErrorIs(t, approvalRepo.CreateDecision(ctx, &crossTenant), operationtask.ErrNotFound)

	require.Error(t, db.Model(&operationtask.ApprovalRecord{}).Where("id = ?", approved.ID).UpdateColumn("comment", "mutated").Error)
	require.ErrorIs(t, db.Delete(&operationtask.ApprovalRecord{}, "id = ?", approved.ID).Error, operationtask.ErrImmutableRecord)
}

func TestExecutionAttemptRepositoryLifecycleValidationIdempotencyAndOrder(t *testing.T) {
	db := openOperationTaskTestDB(t)
	ctx := context.Background()
	task, draft, approval := createTaskDraftApproval(t, db)
	draftRepo := operationtask.NewPlatformDraftRepository(db)
	approvalRepo := operationtask.NewApprovalRecordRepository(db)
	attemptRepo := operationtask.NewExecutionAttemptRepository(db)

	attempt := sampleAttempt(task, draft, approval, 1, "execution-idem")
	require.NoError(t, attemptRepo.CreateAttempt(ctx, &attempt))
	require.Equal(t, 1, attempt.Revision)

	duplicateIDem := sampleAttempt(task, draft, approval, 2, "execution-idem")
	require.NoError(t, attemptRepo.CreateAttempt(ctx, &duplicateIDem))
	require.Equal(t, attempt.ID, duplicateIDem.ID)
	var idemCount int64
	require.NoError(t, db.Model(&operationtask.ExecutionAttempt{}).Where("tenant_id = ? AND operation_task_id = ? AND idempotency_key = ?", 101, task.ID, "execution-idem").Count(&idemCount).Error)
	require.Equal(t, int64(1), idemCount)

	duplicateNumber := sampleAttempt(task, draft, approval, 1, "execution-other-idem")
	require.ErrorIs(t, attemptRepo.CreateAttempt(ctx, &duplicateNumber), operationtask.ErrDuplicateAttemptNumber)

	missingApproval := sampleAttempt(task, draft, approval, 2, "execution-missing-approval")
	missingApproval.ApprovalRecordID = uuid.New()
	require.ErrorIs(t, attemptRepo.CreateAttempt(ctx, &missingApproval), operationtask.ErrNotFound)

	draft2 := sampleDraft(task, 2, hash2)
	require.NoError(t, draftRepo.CreateVersion(ctx, &draft2))
	badApprovalDraft := sampleAttempt(task, draft2, approval, 2, "execution-approval-draft-mismatch")
	require.ErrorIs(t, attemptRepo.CreateAttempt(ctx, &badApprovalDraft), operationtask.ErrReferenceMismatch)

	otherTask := sampleTask(202, "execution-cross-tenant-task")
	require.NoError(t, operationtask.NewOperationTaskRepository(db).Create(ctx, &otherTask))
	otherDraft := sampleDraft(otherTask, 1, hash1)
	require.NoError(t, draftRepo.CreateVersion(ctx, &otherDraft))
	crossApproval := sampleApproval(otherTask, otherDraft, "execution-cross-approval")
	require.NoError(t, approvalRepo.CreateDecision(ctx, &crossApproval))
	crossAttempt := sampleAttempt(task, draft, crossApproval, 2, "execution-cross-tenant")
	require.ErrorIs(t, attemptRepo.CreateAttempt(ctx, &crossAttempt), operationtask.ErrNotFound)

	for _, mode := range []string{"production", "real_write", "auto_publish"} {
		badMode := sampleAttempt(task, draft, approval, 2, "execution-bad-"+mode)
		badMode.AdapterMode = mode
		require.ErrorIs(t, attemptRepo.CreateAttempt(ctx, &badMode), operationtask.ErrValidation)
	}

	badHash := sampleAttempt(task, draft, approval, 2, "execution-bad-hash")
	badHash.ExecutedDraftPayloadHash = "not-a-sha"
	require.ErrorIs(t, attemptRepo.CreateAttempt(ctx, &badHash), operationtask.ErrValidation)

	running := operationtask.ExecutionAttemptStatusRunning
	now := time.Now().UTC()
	updated, err := attemptRepo.UpdateLifecycle(ctx, 101, attempt.ID, 1, operationtask.ExecutionAttemptLifecyclePatch{
		Status:    &running,
		StartedAt: &now,
	})
	require.NoError(t, err)
	require.Equal(t, 2, updated.Revision)
	require.Equal(t, running, updated.Status)

	_, err = attemptRepo.UpdateLifecycle(ctx, 101, attempt.ID, 1, operationtask.ExecutionAttemptLifecyclePatch{Status: &running})
	require.ErrorIs(t, err, operationtask.ErrRevisionConflict)

	approval2 := sampleApproval(task, draft2, "approval-draft2")
	require.NoError(t, approvalRepo.CreateDecision(ctx, &approval2))
	second := sampleAttempt(task, draft2, approval2, 2, "execution-second")
	require.NoError(t, attemptRepo.CreateAttempt(ctx, &second))
	attempts, err := attemptRepo.ListByTask(ctx, 101, task.ID)
	require.NoError(t, err)
	require.Len(t, attempts, 2)
	require.Equal(t, 1, attempts[0].AttemptNumber)
	require.Equal(t, 2, attempts[1].AttemptNumber)
}

func TestExecutionErrorRepositoryAppendValidationAndImmutable(t *testing.T) {
	db := openOperationTaskTestDB(t)
	ctx := context.Background()
	task, draft, approval := createTaskDraftApproval(t, db)
	attemptRepo := operationtask.NewExecutionAttemptRepository(db)
	errorRepo := operationtask.NewExecutionErrorRepository(db)

	attempt := sampleAttempt(task, draft, approval, 1, "error-attempt")
	require.NoError(t, attemptRepo.CreateAttempt(ctx, &attempt))

	errRecord := sampleExecutionError(attempt, 1)
	require.NoError(t, errorRepo.AppendError(ctx, &errRecord))

	got, err := errorRepo.GetByID(ctx, 101, errRecord.ID)
	require.NoError(t, err)
	require.True(t, got.Retryable)
	require.JSONEq(t, `{"timeoutMs":3000}`, string(got.Details))

	duplicate := sampleExecutionError(attempt, 1)
	require.ErrorIs(t, errorRepo.AppendError(ctx, &duplicate), operationtask.ErrDuplicateErrorSequence)

	missingAttempt := sampleExecutionError(attempt, 2)
	missingAttempt.ExecutionAttemptID = uuid.New()
	require.ErrorIs(t, errorRepo.AppendError(ctx, &missingAttempt), operationtask.ErrNotFound)

	crossTenant := sampleExecutionError(attempt, 2)
	crossTenant.TenantID = 202
	require.ErrorIs(t, errorRepo.AppendError(ctx, &crossTenant), operationtask.ErrNotFound)

	badCategory := sampleExecutionError(attempt, 2)
	badCategory.Category = "surprise"
	require.ErrorIs(t, errorRepo.AppendError(ctx, &badCategory), operationtask.ErrValidation)

	secretMessage := sampleExecutionError(attempt, 2)
	secretMessage.SafeMessage = "Bearer should-not-persist"
	require.ErrorIs(t, errorRepo.AppendError(ctx, &secretMessage), operationtask.ErrValidation)

	secretDetails := sampleExecutionError(attempt, 2)
	secretDetails.Details = datatypes.JSON([]byte(`{"accessToken":"secret","safe":"kept"}`))
	require.NoError(t, errorRepo.AppendError(ctx, &secretDetails))
	persistedSecretDetails, err := errorRepo.GetLatestByAttempt(ctx, attempt.TenantID, attempt.ID)
	require.NoError(t, err)
	require.NotContains(t, string(persistedSecretDetails.Details), "secret")
	require.Contains(t, string(persistedSecretDetails.Details), "safe")

	second := sampleExecutionError(attempt, 3)
	second.Category = operationtask.ExecutionErrorCategoryValidation
	second.Code = "VALIDATION_ERROR"
	second.Retryable = false
	require.NoError(t, errorRepo.AppendError(ctx, &second))

	list, err := errorRepo.ListByAttempt(ctx, 101, attempt.ID)
	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3}, []int{list[0].Sequence, list[1].Sequence, list[2].Sequence})

	latest, err := errorRepo.GetLatestByAttempt(ctx, 101, attempt.ID)
	require.NoError(t, err)
	require.Equal(t, 3, latest.Sequence)

	require.Error(t, db.Model(&operationtask.ExecutionError{}).Where("id = ?", errRecord.ID).UpdateColumn("safe_message", "mutated").Error)
	require.ErrorIs(t, db.Delete(&operationtask.ExecutionError{}, "id = ?", errRecord.ID).Error, operationtask.ErrImmutableRecord)
}

func TestOperationTaskEventRepositoryAppendPaginationValidationAndImmutable(t *testing.T) {
	db := openOperationTaskTestDB(t)
	ctx := context.Background()
	task, draft, _ := createTaskDraftApproval(t, db)
	eventRepo := operationtask.NewOperationTaskEventRepository(db)

	first := sampleEvent(task, 1)
	first.PlatformDraftID = &draft.ID
	first.DraftVersion = draft.DraftVersion
	require.NoError(t, eventRepo.AppendEvent(ctx, &first))

	for i := 2; i <= 3; i++ {
		event := sampleEvent(task, i)
		event.EventType = operationtask.OperationTaskEventTypeDraftUpdated
		event.BeforeState = operationtask.OperationTaskStatusSuggested
		event.AfterState = operationtask.OperationTaskStatusPendingReview
		require.NoError(t, eventRepo.AppendEvent(ctx, &event))
	}

	duplicate := sampleEvent(task, 1)
	require.ErrorIs(t, eventRepo.AppendEvent(ctx, &duplicate), operationtask.ErrDuplicateEventSequence)

	crossTenant := sampleEvent(task, 4)
	crossTenant.TenantID = 202
	require.ErrorIs(t, eventRepo.AppendEvent(ctx, &crossTenant), operationtask.ErrNotFound)

	badType := sampleEvent(task, 4)
	badType.EventType = "surprise"
	require.ErrorIs(t, eventRepo.AppendEvent(ctx, &badType), operationtask.ErrValidation)

	missingActor := sampleEvent(task, 4)
	missingActor.ActorID = nil
	require.ErrorIs(t, eventRepo.AppendEvent(ctx, &missingActor), operationtask.ErrValidation)

	secretMetadata := sampleEvent(task, 4)
	secretMetadata.Metadata = datatypes.JSON([]byte(`{"cookie":"secret","safe":"kept"}`))
	require.NoError(t, eventRepo.AppendEvent(ctx, &secretMetadata))
	persistedSecretEvent, err := eventRepo.GetBySequence(ctx, task.TenantID, task.ID, 4)
	require.NoError(t, err)
	require.NotContains(t, string(persistedSecretEvent.Metadata), "secret")
	require.Contains(t, string(persistedSecretEvent.Metadata), "safe")

	badDraft := sampleEvent(task, 5)
	badDraft.PlatformDraftID = &draft.ID
	badDraft.DraftVersion = 2
	require.ErrorIs(t, eventRepo.AppendEvent(ctx, &badDraft), operationtask.ErrReferenceMismatch)

	page1, err := eventRepo.ListByTask(ctx, operationtask.OperationTaskEventListParams{
		TenantID:        101,
		OperationTaskID: task.ID,
		Limit:           2,
	})
	require.NoError(t, err)
	require.True(t, page1.HasMore)
	require.Equal(t, []int{1, 2}, []int{page1.Items[0].Sequence, page1.Items[1].Sequence})
	require.Equal(t, 2, page1.NextSequence)

	page2, err := eventRepo.ListByTask(ctx, operationtask.OperationTaskEventListParams{
		TenantID:        101,
		OperationTaskID: task.ID,
		AfterSequence:   page1.NextSequence,
		Limit:           2,
	})
	require.NoError(t, err)
	require.False(t, page2.HasMore)
	require.Equal(t, []int{3, 4}, []int{page2.Items[0].Sequence, page2.Items[1].Sequence})

	latest, err := eventRepo.GetLatestByTask(ctx, 101, task.ID)
	require.NoError(t, err)
	require.Equal(t, 4, latest.Sequence)

	require.Error(t, db.Model(&operationtask.OperationTaskEvent{}).Where("id = ?", first.ID).UpdateColumn("reason", "mutated").Error)
	require.ErrorIs(t, db.Delete(&operationtask.OperationTaskEvent{}, "id = ?", first.ID).Error, operationtask.ErrImmutableRecord)
}

func TestBatch2ConcurrentConstraints(t *testing.T) {
	db := openOperationTaskTestDB(t)
	ctx := context.Background()
	task, draft, approval := createTaskDraftApproval(t, db)
	approvalRepo := operationtask.NewApprovalRecordRepository(db)
	attemptRepo := operationtask.NewExecutionAttemptRepository(db)
	errorRepo := operationtask.NewExecutionErrorRepository(db)
	eventRepo := operationtask.NewOperationTaskEventRepository(db)

	runPair := func(fn func(int) error) []error {
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				errs <- fn(i)
			}(i)
		}
		wg.Wait()
		close(errs)
		var out []error
		for err := range errs {
			out = append(out, err)
		}
		return out
	}

	approvalErrs := runPair(func(i int) error {
		record := sampleApproval(task, draft, "approval-concurrent")
		record.Comment = fmt.Sprintf("approval-%d", i)
		return approvalRepo.CreateDecision(ctx, &record)
	})
	require.Len(t, approvalErrs, 2)
	require.NoError(t, approvalErrs[0])
	require.NoError(t, approvalErrs[1])
	var approvalCount int64
	require.NoError(t, db.Model(&operationtask.ApprovalRecord{}).Where("tenant_id = ? AND operation_task_id = ? AND idempotency_key = ?", 101, task.ID, "approval-concurrent").Count(&approvalCount).Error)
	require.Equal(t, int64(1), approvalCount)

	attemptNumberErrs := runPair(func(i int) error {
		attempt := sampleAttempt(task, draft, approval, 1, fmt.Sprintf("attempt-number-%d", i))
		return attemptRepo.CreateAttempt(ctx, &attempt)
	})
	assertOneSuccessOne(t, attemptNumberErrs, operationtask.ErrDuplicateAttemptNumber)

	executionIdemErrs := runPair(func(i int) error {
		attempt := sampleAttempt(task, draft, approval, 2, "execution-concurrent-idem")
		attempt.RequestID = fmt.Sprintf("request-%d", i)
		return attemptRepo.CreateAttempt(ctx, &attempt)
	})
	require.NoError(t, executionIdemErrs[0])
	require.NoError(t, executionIdemErrs[1])
	var executionCount int64
	require.NoError(t, db.Model(&operationtask.ExecutionAttempt{}).Where("tenant_id = ? AND operation_task_id = ? AND idempotency_key = ?", 101, task.ID, "execution-concurrent-idem").Count(&executionCount).Error)
	require.Equal(t, int64(1), executionCount)

	attemptForErrors := sampleAttempt(task, draft, approval, 3, "attempt-errors")
	require.NoError(t, attemptRepo.CreateAttempt(ctx, &attemptForErrors))
	errorErrs := runPair(func(i int) error {
		errRecord := sampleExecutionError(attemptForErrors, 1)
		errRecord.Code = fmt.Sprintf("ERR_%d", i)
		return errorRepo.AppendError(ctx, &errRecord)
	})
	assertOneSuccessOne(t, errorErrs, operationtask.ErrDuplicateErrorSequence)

	eventErrs := runPair(func(i int) error {
		event := sampleEvent(task, 1)
		event.RequestID = fmt.Sprintf("event-%d", i)
		return eventRepo.AppendEvent(ctx, &event)
	})
	assertOneSuccessOne(t, eventErrs, operationtask.ErrDuplicateEventSequence)
}

func assertOneSuccessOne(t *testing.T, errs []error, expected error) {
	t.Helper()
	var success, duplicate int
	for _, err := range errs {
		switch {
		case err == nil:
			success++
		case errors.Is(err, expected):
			duplicate++
		default:
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, duplicate)
}
