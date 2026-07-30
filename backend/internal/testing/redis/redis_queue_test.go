package redis_test

import (
	"context"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
)

func TestRedisListQueueRoundTripInIsolatedDB(t *testing.T) {
	cfg, ok, err := safeenv.TestRedisURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_REDIS_URL is not set; skipping Redis queue integration test")
	}

	options, err := goredis.ParseURL(cfg.URL)
	require.NoError(t, err)
	client := goredis.NewClient(options)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.NoError(t, client.Ping(ctx).Err())

	key := "test:trademind:queue:roundtrip"
	require.NoError(t, client.Del(ctx, key).Err())
	defer client.Del(context.Background(), key)

	require.NoError(t, client.LPush(ctx, key, `{"taskId":"test-task-1"}`).Err())
	item, err := client.BRPop(ctx, time.Second, key).Result()
	require.NoError(t, err)
	require.Equal(t, []string{key, `{"taskId":"test-task-1"}`}, item)
}
