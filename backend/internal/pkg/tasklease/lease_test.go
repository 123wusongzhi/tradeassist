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
