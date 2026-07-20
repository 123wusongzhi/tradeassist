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
