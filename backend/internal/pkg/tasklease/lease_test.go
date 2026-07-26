package tasklease_test

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
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasklease"
	"gorm.io/gorm"
)

type leaseTestTask struct {
	ID          uuid.UUID `gorm:"type:char(36);primaryKey"`
	Status      string    `gorm:"size:32"`
	LockedBy    *string   `gorm:"size:220"`
	LockedUntil *time.Time
	LockVersion int `gorm:"column:lock_version"`
	HeartbeatAt *time.Time
	ExecutionID *string `gorm:"size:36"`
	StartedAt   *time.Time
	UpdatedAt   time.Time
}

func (leaseTestTask) TableName() string { return "lease_test_tasks" }

func openLeaseTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:tasklease_%s?mode=memory&cache=shared", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&leaseTestTask{}))
	return db
}

func TestTryClaimConcurrent(t *testing.T) {
	db := openLeaseTestDB(t)
	id := uuid.New()
	require.NoError(t, db.Create(&leaseTestTask{ID: id, Status: "pending"}).Error)
	ctx := context.Background()
	const n = 20
	var wins int32
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(w string) {
			defer wg.Done()
			_, ok, err := tasklease.TryClaim(ctx, db, "lease_test_tasks", "pending", "running", id, w, 90*time.Second)
			require.NoError(t, err)
			if ok {
				atomic.AddInt32(&wins, 1)
			}
		}(fmt.Sprintf("worker-%d", i))
	}
	wg.Wait()
	require.Equal(t, int32(1), wins)
}

func TestValidateLeaseStaleWorker(t *testing.T) {
	db := openLeaseTestDB(t)
	id := uuid.New()
	require.NoError(t, db.Create(&leaseTestTask{ID: id, Status: "pending"}).Error)
	ctx := context.Background()
	claimA, ok, err := tasklease.TryClaim(ctx, db, "lease_test_tasks", "pending", "running", id, "worker-a", 50*time.Millisecond)
	require.NoError(t, err)
	require.True(t, ok)
	time.Sleep(80 * time.Millisecond)
	err = tasklease.ValidateLease(ctx, db, "lease_test_tasks", "running", id, "worker-a", claimA.ExecutionID, claimA.LeaseVersion)
	require.ErrorIs(t, err, tasklease.ErrLeaseLost)
}

type leaseRetryTask struct {
	ID              uuid.UUID `gorm:"type:char(36);primaryKey"`
	Status          string    `gorm:"size:32"`
	LockedBy        *string   `gorm:"size:220"`
	LockedUntil     *time.Time
	LockVersion     int `gorm:"column:lock_version"`
	HeartbeatAt     *time.Time
	ExecutionID     *string `gorm:"size:36"`
	StartedAt       *time.Time
	FinishedAt      *time.Time
	NextRetryAt     *time.Time
	RetryEnqueuedAt *time.Time
	ErrorMessage    string
	UpdatedAt       time.Time
}

func (leaseRetryTask) TableName() string { return "lease_retry_tasks" }

func TestTryClaimPendingOrRetrying(t *testing.T) {
	dsn := fmt.Sprintf("file:tasklease_retry_%s?mode=memory&cache=shared", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&leaseRetryTask{}))
	ctx := context.Background()

	pendingID := uuid.New()
	require.NoError(t, db.Create(&leaseRetryTask{ID: pendingID, Status: "pending", ErrorMessage: "old"}).Error)
	claim, ok, err := tasklease.TryClaimPendingOrRetrying(ctx, db, "lease_retry_tasks", "pending", "retrying", "running", pendingID, "w1", 90*time.Second)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotEqual(t, uuid.Nil, claim.ExecutionID)

	var row leaseRetryTask
	require.NoError(t, db.First(&row, "id = ?", pendingID).Error)
	require.Equal(t, "running", row.Status)
	require.Equal(t, "", row.ErrorMessage)
	require.Nil(t, row.FinishedAt)
	require.Nil(t, row.RetryEnqueuedAt)

	retryID := uuid.New()
	require.NoError(t, db.Create(&leaseRetryTask{ID: retryID, Status: "retrying", NextRetryAt: nil, ErrorMessage: "prev"}).Error)
	_, ok, err = tasklease.TryClaimPendingOrRetrying(ctx, db, "lease_retry_tasks", "pending", "retrying", "running", retryID, "w2", 90*time.Second)
	require.NoError(t, err)
	require.True(t, ok)

	blockedID := uuid.New()
	future := time.Now().UTC().Add(time.Hour)
	require.NoError(t, db.Create(&leaseRetryTask{ID: blockedID, Status: "retrying", NextRetryAt: &future}).Error)
	_, ok, err = tasklease.TryClaimPendingOrRetrying(ctx, db, "lease_retry_tasks", "pending", "retrying", "running", blockedID, "w3", 90*time.Second)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestStaleWorkerFinishAfterReclaim(t *testing.T) {
	db := openLeaseTestDB(t)
	id := uuid.New()
	require.NoError(t, db.Create(&leaseTestTask{ID: id, Status: "pending"}).Error)
	ctx := context.Background()
	claimA, ok, err := tasklease.TryClaim(ctx, db, "lease_test_tasks", "pending", "running", id, "worker-a", 40*time.Millisecond)
	require.NoError(t, err)
	require.True(t, ok)
	time.Sleep(60 * time.Millisecond)
	// Simulate reaper resetting to pending then B claims
	require.NoError(t, db.Model(&leaseTestTask{}).Where("id = ?", id).Updates(map[string]any{
		"status": "pending", "locked_by": nil, "locked_until": nil,
	}).Error)
	claimB, ok, err := tasklease.TryClaim(ctx, db, "lease_test_tasks", "pending", "running", id, "worker-b", 90*time.Second)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotEqual(t, claimA.ExecutionID, claimB.ExecutionID)
	err = tasklease.ValidateLease(ctx, db, "lease_test_tasks", "running", id, "worker-a", claimA.ExecutionID, claimA.LeaseVersion)
	require.ErrorIs(t, err, tasklease.ErrLeaseLost)
}
