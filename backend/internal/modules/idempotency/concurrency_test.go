package idempotency_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"gorm.io/gorm"
)

func openIdempotencyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:idempotency_%s?mode=memory&cache=shared", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	require.NoError(t, db.AutoMigrate(&idempotency.Record{}))
	return db
}

func TestAcquireConcurrentSameKey(t *testing.T) {
	db := openIdempotencyTestDB(t)
	svc := &idempotency.Service{DB: db}
	ctx := context.Background()
	const n = 20
	var acquired int32
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			owner := fmt.Sprintf("worker-%d", idx)
			res, err := svc.Acquire(ctx, "test", "concurrent-key", "hash-1", owner, idempotency.DefaultLease)
			if err == nil && res != nil && res.Acquired {
				atomic.AddInt32(&acquired, 1)
			}
		}(i)
	}
	wg.Wait()
	require.Equal(t, int32(1), acquired, "only one concurrent acquire should win")
}
