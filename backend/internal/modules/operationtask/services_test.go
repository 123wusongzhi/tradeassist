package operationtask_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type allowReviewAuthorizer struct {
	err error
}

func (a allowReviewAuthorizer) CanReview(context.Context, int64, uuid.UUID) error {
	return a.err
}

func createOperationTask(t *testing.T, db *gorm.DB, status string) operationtask.OperationTask {
	t.Helper()
	task := sampleTask(101, uuid.NewString())
	task.Status = status
	require.NoError(t, operationtask.NewOperationTaskRepository(db).Create(context.Background(), &task))
	return task
}

func moveTaskToDraftPreparing(t *testing.T, db *gorm.DB, task operationtask.OperationTask, actor uuid.UUID) operationtask.OperationTask {
	t.Helper()
	updated, err := operationtask.NewTaskTransitionService(db).Transition(context.Background(), operationtask.TaskTransitionInput{
		TenantID:         task.TenantID,
		OperationTaskID:  task.ID,
		ExpectedRevision: task.Revision,
		ToStatus:         operationtask.OperationTaskStatusDraftPreparing,
		ActorType:        operationtask.OperationTaskEventActorUser,
		ActorID:          &actor,
		RequestID:        uuid.NewString(),
		Reason:           "prepare draft",
	})
	require.NoError(t, err)
	return *updated
}

func createPendingReviewDraft(t *testing.T, db *gorm.DB) (operationtask.OperationTask, operationtask.PlatformDraft, uuid.UUID) {
	t.Helper()
	actor := uuid.New()
	task := createOperationTask(t, db, operationtask.OperationTaskStatusSuggested)
	task = moveTaskToDraftPreparing(t, db, task, actor)
	draft, err := operationtask.NewDraftVersionService(db).CreateInitialDraft(context.Background(), operationtask.DraftVersionInput{
		TenantID:         task.TenantID,
		OperationTaskID:  task.ID,
		ExpectedRevision: task.Revision,
		Payload:          datatypes.JSON([]byte(`{"draft":{"title":"summer dress v1"}}`)),
		ActorID:          &actor,
		RequestID:        "req-draft-initial",
		IdempotencyKey:   "idem-draft-initial",
		ChangeReason:     "initial draft",
	})
	require.NoError(t, err)
	updated, err := operationtask.NewOperationTaskRepository(db).GetByID(context.Background(), task.TenantID, task.ID)
	require.NoError(t, err)
	return *updated, *draft, actor
}

func TestTaskTransitionServiceAtomicUpdateAndEvent(t *testing.T) {
	db := openOperationTaskTestDB(t)
	actor := uuid.New()
	task := createOperationTask(t, db, operationtask.OperationTaskStatusSuggested)

	updated, err := operationtask.NewTaskTransitionService(db).Transition(context.Background(), operationtask.TaskTransitionInput{
		TenantID:         task.TenantID,
		OperationTaskID:  task.ID,
		ExpectedRevision: task.Revision,
		ToStatus:         operationtask.OperationTaskStatusDraftPreparing,
		ActorType:        operationtask.OperationTaskEventActorUser,
		ActorID:          &actor,
		RequestID:        "req-transition",
		Reason:           "prepare",
	})
	require.NoError(t, err)
	require.Equal(t, operationtask.OperationTaskStatusDraftPreparing, updated.Status)
	require.Equal(t, 2, updated.Revision)

	event, err := operationtask.NewOperationTaskEventRepository(db).GetLatestByTask(context.Background(), task.TenantID, task.ID)
	require.NoError(t, err)
	require.Equal(t, operationtask.OperationTaskEventTypeDraftGenerated, event.EventType)
	require.Equal(t, operationtask.OperationTaskStatusSuggested, event.BeforeState)
	require.Equal(t, operationtask.OperationTaskStatusDraftPreparing, event.AfterState)
}

func TestTaskTransitionServiceRejectsInvalidTransitionWithoutEvent(t *testing.T) {
	db := openOperationTaskTestDB(t)
	task, _, _ := createPendingReviewDraft(t, db)
	before, err := operationtask.NewOperationTaskEventRepository(db).ListByTask(context.Background(), operationtask.OperationTaskEventListParams{
		TenantID:        task.TenantID,
		OperationTaskID: task.ID,
		Limit:           10,
	})
	require.NoError(t, err)

	_, err = operationtask.NewTaskTransitionService(db).Transition(context.Background(), operationtask.TaskTransitionInput{
		TenantID:         task.TenantID,
		OperationTaskID:  task.ID,
		ExpectedRevision: task.Revision,
		ToStatus:         operationtask.OperationTaskStatusExecuting,
		ActorType:        operationtask.OperationTaskEventActorSystem,
		RequestID:        "req-invalid-transition",
	})
	require.ErrorIs(t, err, operationtask.ErrInvalidTransition)

	afterTask, err := operationtask.NewOperationTaskRepository(db).GetByID(context.Background(), task.TenantID, task.ID)
	require.NoError(t, err)
	require.Equal(t, task.Status, afterTask.Status)
	require.Equal(t, task.Revision, afterTask.Revision)
	after, err := operationtask.NewOperationTaskEventRepository(db).ListByTask(context.Background(), operationtask.OperationTaskEventListParams{
		TenantID:        task.TenantID,
		OperationTaskID: task.ID,
		Limit:           10,
	})
	require.NoError(t, err)
	require.Len(t, after.Items, len(before.Items))
}

func TestTaskTransitionServiceRollbackWhenEventAppendFails(t *testing.T) {
	db := openOperationTaskTestDB(t)
	task := createOperationTask(t, db, operationtask.OperationTaskStatusSuggested)
	db.Callback().Create().Before("gorm:create").Register("operationtask_fail_events", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Name == "OperationTaskEvent" {
			tx.AddError(errors.New("injected event failure"))
		}
	})

	_, err := operationtask.NewTaskTransitionService(db).Transition(context.Background(), operationtask.TaskTransitionInput{
		TenantID:         task.TenantID,
		OperationTaskID:  task.ID,
		ExpectedRevision: task.Revision,
		ToStatus:         operationtask.OperationTaskStatusDraftPreparing,
		ActorType:        operationtask.OperationTaskEventActorSystem,
		RequestID:        "req-rollback",
	})
	require.Error(t, err)

	after, err := operationtask.NewOperationTaskRepository(db).GetByID(context.Background(), task.TenantID, task.ID)
	require.NoError(t, err)
	require.Equal(t, operationtask.OperationTaskStatusSuggested, after.Status)
	require.Equal(t, 1, after.Revision)
	_, err = operationtask.NewOperationTaskEventRepository(db).GetLatestByTask(context.Background(), task.TenantID, task.ID)
	require.ErrorIs(t, err, operationtask.ErrNotFound)
}

func TestTaskTransitionServiceConcurrentRevisionConflict(t *testing.T) {
	db := openOperationTaskTestDB(t)
	task := createOperationTask(t, db, operationtask.OperationTaskStatusSuggested)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, to := range []string{operationtask.OperationTaskStatusDraftPreparing, operationtask.OperationTaskStatusCancelled} {
		wg.Add(1)
		go func(to string) {
			defer wg.Done()
			_, err := operationtask.NewTaskTransitionService(db).Transition(context.Background(), operationtask.TaskTransitionInput{
				TenantID:         task.TenantID,
				OperationTaskID:  task.ID,
				ExpectedRevision: task.Revision,
				ToStatus:         to,
				ActorType:        operationtask.OperationTaskEventActorSystem,
				RequestID:        uuid.NewString(),
			})
			errs <- err
		}(to)
	}
	wg.Wait()
	close(errs)
	successes := 0
	conflicts := 0
	for err := range errs {
		if err == nil {
			successes++
			continue
		}
		if errors.Is(err, operationtask.ErrRevisionConflict) || errors.Is(err, operationtask.ErrInvalidTransition) {
			conflicts++
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
}

func TestDraftVersionServiceCreatesInitialAndNextAppendOnly(t *testing.T) {
	db := openOperationTaskTestDB(t)
	task, draft1, actor := createPendingReviewDraft(t, db)
	require.Equal(t, 1, draft1.DraftVersion)
	require.Equal(t, operationtask.PlatformDraftStatusPendingReview, draft1.Status)
	require.Equal(t, operationtask.OperationTaskStatusPendingReview, task.Status)

	draft2, err := operationtask.NewDraftVersionService(db).CreateNextVersion(context.Background(), operationtask.DraftVersionInput{
		TenantID:         task.TenantID,
		OperationTaskID:  task.ID,
		ExpectedRevision: task.Revision,
		Payload:          datatypes.JSON([]byte(`{"draft":{"title":"summer dress v2"}}`)),
		ActorID:          &actor,
		RequestID:        "req-draft-v2",
		IdempotencyKey:   "idem-draft-v2",
		ChangeReason:     "edit title",
	})
	require.NoError(t, err)
	require.Equal(t, 2, draft2.DraftVersion)

	versions, err := operationtask.NewPlatformDraftRepository(db).ListVersions(context.Background(), task.TenantID, task.ID)
	require.NoError(t, err)
	require.Len(t, versions, 2)
	old, err := operationtask.NewPlatformDraftRepository(db).GetVersion(context.Background(), task.TenantID, task.ID, 1)
	require.NoError(t, err)
	require.Equal(t, draft1.PayloadHash, old.PayloadHash)
	require.Equal(t, datatypes.JSON([]byte(`{"draft":{"title":"summer dress v1"}}`)), old.Payload)
}

func TestDraftVersionServiceIdempotencyAndCrossTenant(t *testing.T) {
	db := openOperationTaskTestDB(t)
	task, _, actor := createPendingReviewDraft(t, db)
	input := operationtask.DraftVersionInput{
		TenantID:         task.TenantID,
		OperationTaskID:  task.ID,
		ExpectedRevision: task.Revision,
		Payload:          datatypes.JSON([]byte(`{"draft":{"title":"summer dress v2"}}`)),
		ActorID:          &actor,
		RequestID:        "req-draft-idem",
		IdempotencyKey:   "idem-draft-repeat",
		ChangeReason:     "edit once",
	}
	first, err := operationtask.NewDraftVersionService(db).CreateNextVersion(context.Background(), input)
	require.NoError(t, err)
	second, err := operationtask.NewDraftVersionService(db).CreateNextVersion(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)

	versions, err := operationtask.NewPlatformDraftRepository(db).ListVersions(context.Background(), task.TenantID, task.ID)
	require.NoError(t, err)
	require.Len(t, versions, 2)

	input.TenantID = 202
	input.IdempotencyKey = "idem-cross-tenant"
	_, err = operationtask.NewDraftVersionService(db).CreateNextVersion(context.Background(), input)
	require.ErrorIs(t, err, operationtask.ErrNotFound)
}

func TestDraftVersionServiceApprovedEditRequiresReapproval(t *testing.T) {
	db := openOperationTaskTestDB(t)
	task, draft, actor := createPendingReviewDraft(t, db)
	reviewer := uuid.New()
	_, err := operationtask.NewApprovalService(db, allowReviewAuthorizer{}).Approve(context.Background(), operationtask.ApprovalInput{
		TenantID:         task.TenantID,
		OperationTaskID:  task.ID,
		ExpectedRevision: task.Revision,
		DraftVersion:     draft.DraftVersion,
		DraftPayloadHash: draft.PayloadHash,
		ReviewerID:       reviewer,
		ReviewerRole:     operationtask.ReviewerRoleReviewer,
		Reason:           "looks good",
		RequestID:        "req-approve-before-edit",
		IdempotencyKey:   "idem-approve-before-edit",
	})
	require.NoError(t, err)
	approvedTask, err := operationtask.NewOperationTaskRepository(db).GetByID(context.Background(), task.TenantID, task.ID)
	require.NoError(t, err)
	require.Equal(t, operationtask.OperationTaskStatusApproved, approvedTask.Status)

	_, err = operationtask.NewDraftVersionService(db).EditDraft(context.Background(), operationtask.DraftVersionInput{
		TenantID:         approvedTask.TenantID,
		OperationTaskID:  approvedTask.ID,
		ExpectedRevision: approvedTask.Revision,
		Payload:          datatypes.JSON([]byte(`{"draft":{"title":"summer dress reviewed again"}}`)),
		ActorID:          &actor,
		RequestID:        "req-edit-approved",
		IdempotencyKey:   "idem-edit-approved",
		ChangeReason:     "approved draft changed",
	})
	require.NoError(t, err)
	after, err := operationtask.NewOperationTaskRepository(db).GetByID(context.Background(), task.TenantID, task.ID)
	require.NoError(t, err)
	require.Equal(t, operationtask.OperationTaskStatusPendingReview, after.Status)
}

func TestApprovalServiceApprovesRejectsAndEnforcesAuthorizerLatestDraft(t *testing.T) {
	db := openOperationTaskTestDB(t)
	task, draft, _ := createPendingReviewDraft(t, db)
	reviewer := uuid.New()

	_, err := operationtask.NewApprovalService(db, nil).Approve(context.Background(), operationtask.ApprovalInput{
		TenantID:         task.TenantID,
		OperationTaskID:  task.ID,
		ExpectedRevision: task.Revision,
		DraftVersion:     draft.DraftVersion,
		DraftPayloadHash: draft.PayloadHash,
		ReviewerID:       reviewer,
		ReviewerRole:     operationtask.ReviewerRoleReviewer,
		Reason:           "ok",
		RequestID:        "req-no-authz",
		IdempotencyKey:   "idem-no-authz",
	})
	require.ErrorIs(t, err, operationtask.ErrPermissionDenied)

	record, err := operationtask.NewApprovalService(db, allowReviewAuthorizer{}).Approve(context.Background(), operationtask.ApprovalInput{
		TenantID:         task.TenantID,
		OperationTaskID:  task.ID,
		ExpectedRevision: task.Revision,
		DraftVersion:     draft.DraftVersion,
		DraftPayloadHash: draft.PayloadHash,
		ReviewerID:       reviewer,
		ReviewerRole:     operationtask.ReviewerRoleReviewer,
		Reason:           "ok",
		RequestID:        "req-approve",
		IdempotencyKey:   "idem-approve",
	})
	require.NoError(t, err)
	require.Equal(t, operationtask.ApprovalDecisionApproved, record.Decision)
	after, err := operationtask.NewOperationTaskRepository(db).GetByID(context.Background(), task.TenantID, task.ID)
	require.NoError(t, err)
	require.Equal(t, operationtask.OperationTaskStatusApproved, after.Status)

	duplicate, err := operationtask.NewApprovalService(db, allowReviewAuthorizer{}).Approve(context.Background(), operationtask.ApprovalInput{
		TenantID:         task.TenantID,
		OperationTaskID:  task.ID,
		ExpectedRevision: task.Revision,
		DraftVersion:     draft.DraftVersion,
		DraftPayloadHash: draft.PayloadHash,
		ReviewerID:       reviewer,
		ReviewerRole:     operationtask.ReviewerRoleReviewer,
		Reason:           "ok",
		RequestID:        "req-approve",
		IdempotencyKey:   "idem-approve",
	})
	require.NoError(t, err)
	require.Equal(t, record.ID, duplicate.ID)

	_, err = operationtask.NewApprovalService(db, allowReviewAuthorizer{}).Approve(context.Background(), operationtask.ApprovalInput{
		TenantID:         task.TenantID,
		OperationTaskID:  task.ID,
		ExpectedRevision: after.Revision,
		DraftVersion:     draft.DraftVersion,
		DraftPayloadHash: hash2,
		ReviewerID:       reviewer,
		ReviewerRole:     operationtask.ReviewerRoleReviewer,
		Reason:           "wrong hash",
		RequestID:        "req-wrong-hash",
		IdempotencyKey:   "idem-wrong-hash",
	})
	require.Error(t, err)
}

func TestApprovalServiceRejectReasonAndConcurrency(t *testing.T) {
	db := openOperationTaskTestDB(t)
	task, draft, _ := createPendingReviewDraft(t, db)
	reviewer := uuid.New()

	_, err := operationtask.NewApprovalService(db, allowReviewAuthorizer{}).Reject(context.Background(), operationtask.ApprovalInput{
		TenantID:         task.TenantID,
		OperationTaskID:  task.ID,
		ExpectedRevision: task.Revision,
		DraftVersion:     draft.DraftVersion,
		DraftPayloadHash: draft.PayloadHash,
		ReviewerID:       reviewer,
		ReviewerRole:     operationtask.ReviewerRoleReviewer,
		RequestID:        "req-reject-missing-reason",
		IdempotencyKey:   "idem-reject-missing-reason",
	})
	require.ErrorIs(t, err, operationtask.ErrValidation)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, reject := range []bool{false, true} {
		wg.Add(1)
		go func(reject bool) {
			defer wg.Done()
			input := operationtask.ApprovalInput{
				TenantID:         task.TenantID,
				OperationTaskID:  task.ID,
				ExpectedRevision: task.Revision,
				DraftVersion:     draft.DraftVersion,
				DraftPayloadHash: draft.PayloadHash,
				ReviewerID:       uuid.New(),
				ReviewerRole:     operationtask.ReviewerRoleReviewer,
				Reason:           "decision",
				RequestID:        uuid.NewString(),
				IdempotencyKey:   uuid.NewString(),
			}
			if reject {
				_, err := operationtask.NewApprovalService(db, allowReviewAuthorizer{}).Reject(context.Background(), input)
				errs <- err
				return
			}
			_, err := operationtask.NewApprovalService(db, allowReviewAuthorizer{}).Approve(context.Background(), input)
			errs <- err
		}(reject)
	}
	wg.Wait()
	close(errs)
	successes := 0
	failures := 0
	for err := range errs {
		if err == nil {
			successes++
		} else if errors.Is(err, operationtask.ErrRevisionConflict) || errors.Is(err, operationtask.ErrStateConflict) {
			failures++
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, failures)
}
