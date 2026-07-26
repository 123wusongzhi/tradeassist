package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCacheTTLAndInvalidation(t *testing.T) {
	now := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	c := New[string, string](Options{
		MaxEntries: 2,
		DefaultTTL: time.Minute,
		Now:        func() time.Time { return now },
	})
	c.Set("a", "one", time.Second)
	if got, ok := c.Get("a"); !ok || got != "one" {
		t.Fatalf("expected cache hit, got %q %v", got, ok)
	}
	now = now.Add(2 * time.Second)
	if _, ok := c.Get("a"); ok {
		t.Fatal("expected expired entry to miss")
	}
	c.Set("b", "two", time.Minute)
	c.Delete("b")
	if _, ok := c.Get("b"); ok {
		t.Fatal("expected deleted entry to miss")
	}
}

func TestCacheEvictionBound(t *testing.T) {
	c := New[string, int](Options{MaxEntries: 2, DefaultTTL: time.Minute})
	c.Set("a", 1, 0)
	c.Set("b", 2, 0)
	c.Set("c", 3, 0)
	if c.Len() != 2 {
		t.Fatalf("expected bounded len 2, got %d", c.Len())
	}
	if _, ok := c.Get("a"); ok {
		t.Fatal("expected oldest entry to be evicted")
	}
}

func TestCacheNegativeEntry(t *testing.T) {
	c := New[string, string](Options{MaxEntries: 4, DefaultTTL: time.Minute})
	c.SetNegative("missing", time.Minute)
	if _, ok, negative := c.GetEntry("missing"); ok || !negative {
		t.Fatalf("expected negative cache entry, ok=%v negative=%v", ok, negative)
	}
	if _, err := c.Load(context.Background(), "missing", time.Minute, func(context.Context, string) (string, error) {
		t.Fatal("loader should not run for negative entry")
		return "", nil
	}); !errors.Is(err, ErrNegative) {
		t.Fatalf("expected ErrNegative, got %v", err)
	}
}

func TestCacheSingleflight(t *testing.T) {
	c := New[string, string](Options{MaxEntries: 4, DefaultTTL: time.Minute})
	var calls int32
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := c.Load(context.Background(), "k", time.Minute, func(context.Context, string) (string, error) {
				atomic.AddInt32(&calls, 1)
				time.Sleep(10 * time.Millisecond)
				return "loaded", nil
			})
			if err != nil {
				errs <- err
				return
			}
			if got != "loaded" {
				errs <- errors.New("unexpected loaded value")
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("expected one loader call, got %d", calls)
	}
}

func TestCacheLoadFailureNotCached(t *testing.T) {
	c := New[string, string](Options{MaxEntries: 4, DefaultTTL: time.Minute})
	_, err := c.Load(context.Background(), "k", time.Minute, func(context.Context, string) (string, error) {
		return "", errors.New("boom")
	})
	if err == nil {
		t.Fatal("expected load failure")
	}
	if _, ok := c.Get("k"); ok {
		t.Fatal("failed load must not be cached")
	}
}

func TestScopeKeyRequiresTenant(t *testing.T) {
	if _, err := ScopeKey(0, "", "permission", "read"); err == nil {
		t.Fatal("expected missing tenant to fail")
	}
	key, err := ScopeKey(7, "shop-a", "permission", "read|write")
	if err != nil {
		t.Fatalf("ScopeKey: %v", err)
	}
	if key != "tenant:7|shop:shop-a|permission|read_write" {
		t.Fatalf("unexpected key %q", key)
	}
}
