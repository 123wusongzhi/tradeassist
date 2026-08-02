package inventory

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func retryAtomicTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:inventory_retry_atomic_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// SQLite's shared in-memory writer lock makes simultaneous writers both
	// fail spuriously. One connection preserves the request race ordering while
	// still proving the second batch observes the first CAS claim.
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&InventorySyncTask{}))
	return db
}

func failedRetryTask(tenantID int64) InventorySyncTask {
	return InventorySyncTask{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, TenantID: tenantID, ProductID: uuid.New(), ShopID: uuid.New(), Platform: "test", TaskType: "sync", Mode: "manual", Status: StatusFailed}
}

func TestClaimInventoryRetryTasksConcurrentIsAllOrNothing(t *testing.T) {
	db := retryAtomicTestDB(t)
	first, second := failedRetryTask(0), failedRetryTask(0)
	require.NoError(t, db.Create(&first).Error)
	require.NoError(t, db.Create(&second).Error)

	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- db.Transaction(func(tx *gorm.DB) error {
				_, err := claimInventoryRetryTasks(context.Background(), tx, 0, []uuid.UUID{first.ID, second.ID}, nil, nil, "")
				return err
			})
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	require.Equal(t, 1, successes)
	var tasks []InventorySyncTask
	require.NoError(t, db.Order("id").Find(&tasks).Error)
	require.Len(t, tasks, 2)
	for _, task := range tasks {
		require.Equal(t, StatusPending, task.Status)
	}
}

func TestClaimInventoryRetryTasksRejectsMixedTenantWithoutSideEffects(t *testing.T) {
	db := retryAtomicTestDB(t)
	allowed, otherTenant := failedRetryTask(0), failedRetryTask(1)
	require.NoError(t, db.Create(&allowed).Error)
	require.NoError(t, db.Create(&otherTenant).Error)
	err := db.Transaction(func(tx *gorm.DB) error {
		_, err := claimInventoryRetryTasks(context.Background(), tx, 0, []uuid.UUID{allowed.ID, otherTenant.ID}, nil, nil, "")
		return err
	})
	require.Error(t, err)
	var unchanged InventorySyncTask
	require.NoError(t, db.First(&unchanged, "id = ? AND tenant_id = ?", allowed.ID, int64(0)).Error)
	require.Equal(t, StatusFailed, unchanged.Status)
}
