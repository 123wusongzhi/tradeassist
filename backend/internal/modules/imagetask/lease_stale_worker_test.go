package imagetask

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasklease"
	"gorm.io/gorm"
)

func openImageLeaseTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:image_lease_%s?mode=memory&cache=shared", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&ImageTask{}))
	return db
}

func TestRecoverLeaseExpiredDoesNotClearRenewedLease(t *testing.T) {
	db := openImageLeaseTestDB(t)
	svc := &Service{DB: db}
	id := uuid.New()
	worker := "worker-a"
	past := time.Now().UTC().Add(-time.Minute)
	future := time.Now().UTC().Add(time.Minute)
	require.NoError(t, db.Create(&ImageTask{HardDeleteBase: model.HardDeleteBase{ID: id}, TenantID: 0, TaskType: TaskTypeRemoveBackground, Provider: "removebg", Status: StatusRunning, LockedBy: &worker, LockedUntil: &past, LockVersion: 1}).Error)
	var renew sync.Once
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("image_renew_before_recovery", func(tx *gorm.DB) {
		renew.Do(func() {
			tx.Exec("UPDATE image_tasks SET locked_until = ?, heartbeat_at = ? WHERE id = ?", future, future, id)
		})
	}))
	require.NoError(t, svc.RecoverLeaseExpired(context.Background(), id))
	var got ImageTask
	require.NoError(t, db.First(&got, "id = ?", id).Error)
	require.Equal(t, StatusRunning, got.Status)
	require.NotNil(t, got.LockedUntil)
	require.True(t, got.LockedUntil.After(time.Now().UTC()))
	require.NotNil(t, got.LockedBy)
	require.Equal(t, worker, *got.LockedBy)
}

func TestRecoverLegacyRunningDoesNotOverwriteRetriedAndClaimedTask(t *testing.T) {
	db := openImageLeaseTestDB(t)
	svc := &Service{DB: db}
	id := uuid.New()
	legacyUpdatedAt := time.Now().UTC().Add(-2 * time.Hour)
	cutoff := legacyUpdatedAt.Add(time.Hour)
	reclaimed := false
	reclaimedWorker := "worker-new"
	reclaimedExecutionID := uuid.New().String()
	reclaimedLockVersion := 1
	require.NoError(t, db.Create(&ImageTask{
		HardDeleteBase: model.HardDeleteBase{ID: id, UpdatedAt: legacyUpdatedAt},
		TenantID:       0,
		TaskType:       TaskTypeRemoveBackground,
		Provider:       "removebg",
		Status:         StatusRunning,
	}).Error)

	var retryAndClaim sync.Once
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("image_retry_claim_before_legacy_recovery", func(tx *gorm.DB) {
		retryAndClaim.Do(func() {
			now := time.Now().UTC()
			until := now.Add(time.Minute)
			// Another recovery has requeued the task and a new worker has claimed it.
			require.NoError(t, tx.Exec("UPDATE image_tasks SET status = ?, updated_at = ? WHERE id = ?", StatusPending, now, id).Error)
			claim := tx.Exec("UPDATE image_tasks SET status = ?, locked_by = ?, locked_until = ?, execution_id = ?, lock_version = ?, heartbeat_at = ?, updated_at = ? WHERE id = ? AND status = ?", StatusRunning, reclaimedWorker, until, reclaimedExecutionID, reclaimedLockVersion, now, now, id, StatusPending)
			require.NoError(t, claim.Error)
			require.Equal(t, int64(1), claim.RowsAffected)
			reclaimed = true
		})
	}))

	require.NoError(t, svc.RecoverLegacyRunning(context.Background(), id, cutoff))
	require.True(t, reclaimed)
	var got ImageTask
	require.NoError(t, db.First(&got, "id = ?", id).Error)
	require.Equal(t, StatusRunning, got.Status)
	require.NotNil(t, got.LockedBy)
	require.Equal(t, reclaimedWorker, *got.LockedBy)
	require.NotNil(t, got.LockedUntil)
	require.True(t, got.LockedUntil.After(time.Now().UTC()))
	require.NotNil(t, got.ExecutionID)
	require.Equal(t, reclaimedExecutionID, *got.ExecutionID)
	require.Equal(t, reclaimedLockVersion, got.LockVersion)
	require.Zero(t, got.RetryCount)
}

func TestImageStaleWorkerFinishFails(t *testing.T) {
	db := openImageLeaseTestDB(t)
	svc := &Service{DB: db}
	ctx := context.Background()
	id := uuid.New()
	require.NoError(t, db.Create(&ImageTask{
		HardDeleteBase: model.HardDeleteBase{ID: id},
		TaskType:       TaskTypeRemoveBackground,
		Provider:       "removebg",
		Status:         StatusPending,
	}).Error)

	_, claimA, ok, err := svc.tryClaimImageTask(ctx, id, "worker-a", 40*time.Millisecond)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, claimA)

	time.Sleep(60 * time.Millisecond)
	require.NoError(t, db.Model(&ImageTask{}).Where("id = ?", id).Updates(map[string]any{
		"status": StatusPending, "locked_by": nil, "locked_until": nil,
	}).Error)

	_, claimB, ok, err := svc.tryClaimImageTask(ctx, id, "worker-b", 90*time.Second)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, claimB)
	require.NotEqual(t, claimA.ExecutionID, claimB.ExecutionID)

	err = svc.finishImageTask(ctx, id, "worker-a", claimA, map[string]any{
		"status": StatusSuccess,
	})
	require.ErrorIs(t, err, tasklease.ErrLeaseLost)

	var row ImageTask
	require.NoError(t, db.First(&row, "id = ?", id).Error)
	require.Equal(t, StatusRunning, row.Status)
	require.NotNil(t, row.LockedBy)
	require.Equal(t, "worker-b", *row.LockedBy)
}
