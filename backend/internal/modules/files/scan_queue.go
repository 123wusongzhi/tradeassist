package files

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
)

type scanQueue interface {
	LPush(context.Context, string, string) error
	LLen(context.Context, string) (int64, error)
	RPopLPush(context.Context, string, string) (string, error)
	BRPopLPush(context.Context, string, string, time.Duration) (string, error)
	LRem(context.Context, string, int64, string) error
	Requeue(context.Context, string, string) error
	RestoreRecovery(context.Context, string) error
}

type redisScanQueue struct{ redis *rdb.Client }

func (q redisScanQueue) LPush(ctx context.Context, k, v string) error {
	return q.redis.LPush(ctx, k, v).Err()
}
func (q redisScanQueue) LLen(ctx context.Context, k string) (int64, error) {
	return q.redis.LLen(ctx, k).Result()
}
func (q redisScanQueue) RPopLPush(ctx context.Context, a, b string) (string, error) {
	v, err := q.redis.RPopLPush(ctx, a, b).Result()
	if err == redis.Nil {
		return "", nil
	}
	return v, err
}
func (q redisScanQueue) BRPopLPush(ctx context.Context, a, b string, d time.Duration) (string, error) {
	v, err := q.redis.BRPopLPush(ctx, a, b, d).Result()
	if err == redis.Nil {
		return "", nil
	}
	return v, err
}
func (q redisScanQueue) LRem(ctx context.Context, k string, n int64, v string) error {
	return q.redis.LRem(ctx, k, n, v).Err()
}
func (q redisScanQueue) Requeue(ctx context.Context, old, next string) error {
	_, err := q.redis.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.LRem(ctx, fileScanProcessingQueueName, 0, old)
		p.LRem(ctx, fileScanQueueName, 0, old)
		if next != old {
			p.LRem(ctx, fileScanProcessingQueueName, 0, next)
			p.LRem(ctx, fileScanQueueName, 0, next)
		}
		p.LPush(ctx, fileScanQueueName, next)
		return nil
	})
	return err
}
func (q redisScanQueue) RestoreRecovery(ctx context.Context, payload string) error {
	_, err := q.redis.TxPipelined(ctx, func(p redis.Pipeliner) error {
		// Collapse duplicate copies before restoring a single processing marker.
		// Multiple workers can briefly observe the same logical payload while a
		// recovery pass checks its database lease.
		p.LRem(ctx, fileScanQueueName, 0, payload)
		p.LRem(ctx, fileScanProcessingQueueName, 0, payload)
		p.LPush(ctx, fileScanProcessingQueueName, payload)
		return nil
	})
	return err
}

func (s *Service) fileScanQueue() scanQueue {
	if s != nil && s.scanQueue != nil {
		return s.scanQueue
	}
	if s != nil && s.Redis != nil && s.Redis.Client != nil {
		return redisScanQueue{redis: s.Redis}
	}
	return nil
}
