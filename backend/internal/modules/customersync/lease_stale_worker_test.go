package customersync

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasklease"
	"gorm.io/gorm"
)

func openCustomerSyncLeaseTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:csync_lease_%s?mode=memory&cache=shared", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CustomerMessageSyncTask{}))
	return db
}

func TestCustomerSyncStaleWorkerFinishFails(t *testing.T) {
	db := openCustomerSyncLeaseTestDB(t)
	svc := &Service{DB: db}
	ctx := context.Background()
	id := uuid.New()
	require.NoError(t, db.Create(&CustomerMessageSyncTask{
		HardDeleteBase: model.HardDeleteBase{ID: id},
		ShopID:         uuid.New(),
		Platform:       "douyin_shop",
		TaskType:       "pull_messages",
		Status:         StatusPending,
		Mode:           "incremental",
	}).Error)

	_, claimA, ok, err := svc.tryClaimTask(ctx, id, "worker-a", 40*time.Millisecond)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, claimA)

	time.Sleep(60 * time.Millisecond)
	require.NoError(t, db.Model(&CustomerMessageSyncTask{}).Where("id = ?", id).Updates(map[string]any{
		"status": StatusPending, "locked_by": nil, "locked_until": nil,
	}).Error)

	_, claimB, ok, err := svc.tryClaimTask(ctx, id, "worker-b", 90*time.Second)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, claimB)
	require.NotEqual(t, claimA.ExecutionID, claimB.ExecutionID)

	err = svc.finishCustomerSyncTask(ctx, id, "worker-a", claimA, map[string]any{
		"status": StatusSuccess,
	})
	require.ErrorIs(t, err, tasklease.ErrLeaseLost)

	var row CustomerMessageSyncTask
	require.NoError(t, db.First(&row, "id = ?", id).Error)
	require.Equal(t, StatusRunning, row.Status)
	require.NotNil(t, row.LockedBy)
	require.Equal(t, "worker-b", *row.LockedBy)
}
