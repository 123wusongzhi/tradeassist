package customersync

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func TestRecoverLegacyRunningDoesNotOverwriteRetriedClaim(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:customersync_legacy_recovery_%s?mode=memory&cache=shared", uuid.New())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CustomerMessageSyncTask{}))

	id := uuid.New()
	legacyUpdatedAt := time.Now().UTC().Add(-2 * time.Hour)
	cutoff := legacyUpdatedAt.Add(time.Hour)
	require.NoError(t, db.Create(&CustomerMessageSyncTask{
		HardDeleteBase: model.HardDeleteBase{ID: id, UpdatedAt: legacyUpdatedAt},
		TenantID:       0,
		ShopID:         uuid.New(),
		Platform:       "douyin_shop",
		TaskType:       "pull_messages",
		Mode:           "incremental",
		Status:         StatusRunning,
	}).Error)

	service := &Service{DB: db}
	var race atomic.Bool
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("customersync_retry_claim_before_legacy_recovery", func(tx *gorm.DB) {
		if !race.CompareAndSwap(false, true) {
			return
		}
		require.NoError(t, tx.Exec("UPDATE customer_message_sync_tasks SET status = ? WHERE id = ?", StatusFailed, id).Error)
		require.NoError(t, tx.Exec("UPDATE customer_message_sync_tasks SET status = ? WHERE id = ?", StatusPending, id).Error)
		claimedAt := time.Now().UTC()
		leaseUntil := claimedAt.Add(time.Minute)
		require.NoError(t, tx.Exec("UPDATE customer_message_sync_tasks SET status = ?, locked_by = ?, locked_until = ?, execution_id = ?, heartbeat_at = ?, lock_version = lock_version + 1, updated_at = ? WHERE id = ?", StatusRunning, "worker-new", leaseUntil, uuid.NewString(), claimedAt, claimedAt, id).Error)
	}))

	require.NoError(t, service.RecoverLegacyRunning(context.Background(), id, cutoff))
	var got CustomerMessageSyncTask
	require.NoError(t, db.First(&got, "id = ?", id).Error)
	require.Equal(t, StatusRunning, got.Status)
	require.NotNil(t, got.LockedBy)
	require.Equal(t, "worker-new", *got.LockedBy)
	require.NotNil(t, got.LockedUntil)
	require.NotNil(t, got.ExecutionID)
	require.Greater(t, got.LockVersion, 0)
}
