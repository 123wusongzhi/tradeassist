package productpublish

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
	"gorm.io/gorm"
)

func TestRecoverLeaseExpiredDoesNotClearRenewedLease(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:publish_recovery_%s?mode=memory&cache=shared", uuid.New())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&ProductPublishTask{}))
	id, worker := uuid.New(), "worker-a"
	past, future := time.Now().UTC().Add(-time.Minute), time.Now().UTC().Add(time.Minute)
	require.NoError(t, db.Create(&ProductPublishTask{HardDeleteBase: model.HardDeleteBase{ID: id}, TenantID: 0, ProductID: uuid.New(), ShopID: uuid.New(), TargetStoreID: uuid.New(), Platform: "shopee", TaskType: "publish", Mode: "manual", Status: TaskRunning, LockedBy: &worker, LockedUntil: &past, LockVersion: 1}).Error)
	var renew sync.Once
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("publish_renew_before_recovery", func(tx *gorm.DB) {
		renew.Do(func() {
			tx.Exec("UPDATE product_publish_tasks SET locked_until = ?, heartbeat_at = ? WHERE id = ?", future, future, id)
		})
	}))
	require.NoError(t, (&Service{DB: db}).RecoverLeaseExpired(context.Background(), id))
	var got ProductPublishTask
	require.NoError(t, db.First(&got, "id = ?", id).Error)
	require.Equal(t, TaskRunning, got.Status)
	require.NotNil(t, got.LockedUntil)
	require.True(t, got.LockedUntil.After(time.Now().UTC()))
	require.Equal(t, worker, *got.LockedBy)
}

func TestRecoverLegacyRunningDoesNotOverwriteNewClaim(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:publish_legacy_recovery_%s?mode=memory&cache=shared", uuid.New())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&ProductPublishTask{}))

	now := time.Now().UTC()
	old := now.Add(-time.Hour)
	cutoff := now.Add(-30 * time.Minute)
	id := uuid.New()
	require.NoError(t, db.Create(&ProductPublishTask{
		HardDeleteBase: model.HardDeleteBase{ID: id, CreatedAt: old, UpdatedAt: old},
		TenantID:       0,
		ProductID:      uuid.New(),
		ShopID:         uuid.New(),
		TargetStoreID:  uuid.New(),
		Platform:       "shopee",
		TaskType:       "publish",
		Mode:           "manual",
		Status:         TaskRunning,
	}).Error)

	worker := "worker-new"
	executionID := uuid.New().String()
	leaseUntil := now.Add(time.Minute)
	claimedAt := now.Add(time.Second)
	var claim sync.Once
	claimApplied := false
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("publish_claim_before_legacy_recovery", func(tx *gorm.DB) {
		claim.Do(func() {
			result := tx.Exec(
				"UPDATE product_publish_tasks SET locked_by = ?, locked_until = ?, heartbeat_at = ?, execution_id = ?, lock_version = lock_version + 1, updated_at = ? WHERE id = ?",
				worker, leaseUntil, claimedAt, executionID, claimedAt, id,
			)
			require.NoError(t, result.Error)
			require.EqualValues(t, 1, result.RowsAffected)
			claimApplied = true
		})
	}))

	require.NoError(t, (&Service{DB: db}).RecoverLegacyRunning(context.Background(), id, cutoff))
	require.True(t, claimApplied)
	var got ProductPublishTask
	require.NoError(t, db.First(&got, "id = ?", id).Error)
	require.Equal(t, TaskRunning, got.Status)
	require.Empty(t, got.ErrorMessage)
	require.Equal(t, 1, got.LockVersion)
	require.NotNil(t, got.LockedBy)
	require.Equal(t, worker, *got.LockedBy)
	require.NotNil(t, got.LockedUntil)
	require.Equal(t, leaseUntil.Unix(), got.LockedUntil.Unix())
	require.NotNil(t, got.HeartbeatAt)
	require.Equal(t, claimedAt.Unix(), got.HeartbeatAt.Unix())
	require.NotNil(t, got.ExecutionID)
	require.Equal(t, executionID, *got.ExecutionID)
}
