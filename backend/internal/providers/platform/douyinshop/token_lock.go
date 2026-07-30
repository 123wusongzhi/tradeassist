package douyinshop

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
)

// TokenVersion is a monotonic counter on Client used to detect stale token refreshes.
// Persisted alongside the token row; incremented on each successful refresh.
//
// Usage: before writing new tokens, compare current version; reject if stale.

// TokenRefreshLocker provides distributed-safe token refresh deduplication.
// Implementations may use the idempotency service, Redis, or an in-memory fallback.
type TokenRefreshLocker interface {
	// AcquireRefresh tries to claim the refresh lock for the given idempotency key.
	// Returns (acquired, currentVersion, error).
	AcquireRefresh(ctx context.Context, key string, shopID string, tokenVersion int64) (bool, int64, error)

	// CommitRefresh marks the refresh as completed with the new token version.
	CommitRefresh(ctx context.Context, key string, newVersion int64) error

	// ReleaseRefresh releases the lock without committing (on failure).
	ReleaseRefresh(ctx context.Context, key string) error
}

// TokenVersionKey returns the idempotency key for a token refresh operation.
func TokenVersionKey(shopID string, tokenVersion int64) string {
	return fmt.Sprintf("douyin-token-refresh:%s:%d",
		strings.TrimSpace(shopID), tokenVersion)
}

// tokenVersionCounter is the in-memory token version on Client.
// It is read/written under tokenMu or via atomic ops for the fast path.
type tokenVersionCounter struct {
	v int64
}

func (c *tokenVersionCounter) load() int64 {
	return atomic.LoadInt64(&c.v)
}

func (c *tokenVersionCounter) increment() int64 {
	return atomic.AddInt64(&c.v, 1)
}

func (c *tokenVersionCounter) compareAndSwap(old, new int64) bool {
	return atomic.CompareAndSwapInt64(&c.v, old, new)
}

// inMemoryTokenLocker is a no-op locker for tests / single-instance deployments.
// For multi-instance deployments wire a Redis or DB-backed implementation.
type inMemoryTokenLocker struct{}

func (inMemoryTokenLocker) AcquireRefresh(_ context.Context, _ string, _ string, _ int64) (bool, int64, error) {
	return true, 0, nil
}

func (inMemoryTokenLocker) CommitRefresh(_ context.Context, _ string, _ int64) error {
	return nil
}

func (inMemoryTokenLocker) ReleaseRefresh(_ context.Context, _ string) error {
	return nil
}

// DefaultTokenLocker returns the in-memory locker suitable for single-instance deployments.
func DefaultTokenLocker() TokenRefreshLocker {
	return inMemoryTokenLocker{}
}

// TokenVersionConflictError wraps CodeDouyinTokenVersionConflict with context.
func TokenVersionConflictError(shopID string, expected, actual int64) *Error {
	e := NewError(CodeDouyinTokenVersionConflict,
		fmt.Sprintf("token version conflict for shop %s: expected %d, current %d", shopID, expected, actual),
		"", "token_version_conflict", "")
	e.Retryable = false
	return e
}
