package collect

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

func openCollectLeaseTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:collect_lease_%s?mode=memory&cache=shared", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CollectTask{}))
	return db
}

func TestCollectStaleWorkerFinishFails(t *testing.T) {
	db := openCollectLeaseTestDB(t)
	svc := &Service{DB: db}
	ctx := context.Background()
	id := uuid.New()
	require.NoError(t, db.Create(&CollectTask{
		HardDeleteBase: model.HardDeleteBase{ID: id},
		Source:         "1688",
		SourceURL:      "https://detail.1688.com/offer/1.html",
		Status:         StatusPending,
	}).Error)

	taskA, claimA, ok := svc.tryClaimCollectTask(ctx, id, "worker-a", 40*time.Millisecond)
	require.True(t, ok)
	require.NotNil(t, taskA)
	require.NotNil(t, claimA)

	time.Sleep(60 * time.Millisecond)
	require.NoError(t, db.Model(&CollectTask{}).Where("id = ?", id).Updates(map[string]any{
		"status": StatusPending, "locked_by": nil, "locked_until": nil,
	}).Error)

	_, claimB, ok := svc.tryClaimCollectTask(ctx, id, "worker-b", 90*time.Second)
	require.True(t, ok)
	require.NotNil(t, claimB)
	require.NotEqual(t, claimA.ExecutionID, claimB.ExecutionID)

	err := svc.finishCollectTask(ctx, id, "worker-a", claimA, map[string]any{
		"status":        StatusSuccess,
		"error_message": "",
	})
	require.ErrorIs(t, err, tasklease.ErrLeaseLost)

	var row CollectTask
	require.NoError(t, db.First(&row, "id = ?", id).Error)
	require.Equal(t, StatusRunning, row.Status)
	require.NotNil(t, row.LockedBy)
	require.Equal(t, "worker-b", *row.LockedBy)
}
