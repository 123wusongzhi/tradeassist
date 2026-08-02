package ordersync

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func TestRecoverLeaseExpiredDoesNotClearRenewedLease(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:ordersync_recovery_%s?mode=memory&cache=shared", uuid.New())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&OrderSyncTask{}))
	id, worker := uuid.New(), "worker-a"
	past, future := time.Now().UTC().Add(-time.Minute), time.Now().UTC().Add(time.Minute)
	require.NoError(t, db.Create(&OrderSyncTask{HardDeleteBase: model.HardDeleteBase{ID: id}, TenantID: 0, ShopID: uuid.New(), Platform: "douyin_shop", TaskType: "sync", Mode: "incremental", Status: StatusRunning, LockedBy: &worker, LockedUntil: &past, LockVersion: 1}).Error)
	var renew sync.Once
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("ordersync_renew_before_recovery", func(tx *gorm.DB) {
		renew.Do(func() {
			tx.Exec("UPDATE order_sync_tasks SET locked_until = ?, heartbeat_at = ? WHERE id = ?", future, future, id)
		})
	}))
	require.NoError(t, (&Service{DB: db}).RecoverLeaseExpired(context.Background(), id))
	var got OrderSyncTask
	require.NoError(t, db.First(&got, "id = ?", id).Error)
	require.Equal(t, StatusRunning, got.Status)
	require.NotNil(t, got.LockedUntil)
	require.True(t, got.LockedUntil.After(time.Now().UTC()))
	require.Equal(t, worker, *got.LockedBy)
}

func TestRecoverLegacyRunningDoesNotOverwriteRetriedClaim(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:ordersync_legacy_recovery_%s?mode=memory&cache=shared", uuid.New())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&OrderSyncTask{}))

	id := uuid.New()
	legacyUpdatedAt := time.Now().UTC().Add(-2 * time.Hour)
	cutoff := legacyUpdatedAt.Add(time.Hour)
	require.NoError(t, db.Create(&OrderSyncTask{
		HardDeleteBase: model.HardDeleteBase{ID: id, UpdatedAt: legacyUpdatedAt},
		TenantID:       0,
		ShopID:         uuid.New(),
		Platform:       "douyin_shop",
		TaskType:       "sync",
		Mode:           "incremental",
		Status:         StatusRunning,
	}).Error)

	service := &Service{DB: db}
	var race atomic.Bool
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("ordersync_retry_claim_before_legacy_recovery", func(tx *gorm.DB) {
		if !race.CompareAndSwap(false, true) {
			return
		}
		require.NoError(t, tx.Exec("UPDATE order_sync_tasks SET status = ? WHERE id = ?", StatusFailed, id).Error)
		require.NoError(t, tx.Exec("UPDATE order_sync_tasks SET status = ? WHERE id = ?", StatusPending, id).Error)
		claimedAt := time.Now().UTC()
		leaseUntil := claimedAt.Add(time.Minute)
		require.NoError(t, tx.Exec("UPDATE order_sync_tasks SET status = ?, locked_by = ?, locked_until = ?, execution_id = ?, heartbeat_at = ?, lock_version = lock_version + 1, updated_at = ? WHERE id = ?", StatusRunning, "worker-new", leaseUntil, uuid.NewString(), claimedAt, claimedAt, id).Error)
	}))

	require.NoError(t, service.RecoverLegacyRunning(context.Background(), id, cutoff))
	var got OrderSyncTask
	require.NoError(t, db.First(&got, "id = ?", id).Error)
	require.Equal(t, StatusRunning, got.Status)
	require.NotNil(t, got.LockedBy)
	require.Equal(t, "worker-new", *got.LockedBy)
	require.NotNil(t, got.LockedUntil)
	require.NotNil(t, got.ExecutionID)
	require.Greater(t, got.LockVersion, 0)
}
