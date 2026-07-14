package providerlimit

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestAcquireReleaseAndDoubleRelease(t *testing.T) {
	r := NewRegistry(Config{DefaultConcurrency: 1, MaxWait: 20 * time.Millisecond})
	l, err := r.Acquire(context.Background(), ProviderDouyinShop, OperationRead)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if _, err := r.Acquire(ctx, ProviderDouyinShop, OperationRead); err == nil {
		t.Fatal("expected bounded wait to fail")
	}
	l.Release()
	l.Release()
	if _, err := r.Acquire(context.Background(), ProviderDouyinShop, OperationRead); err != nil {
		t.Fatalf("Acquire after release: %v", err)
	}
}

func TestConcurrentAcquireRespectsLimit(t *testing.T) {
	r := NewRegistry(Config{DefaultConcurrency: 2, MaxWait: time.Second})
	var wg sync.WaitGroup
	start := make(chan struct{})
	inflight := 0
	maxInflight := 0
	var mu sync.Mutex
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			l, err := r.Acquire(context.Background(), ProviderAI, OperationText)
			if err != nil {
				t.Errorf("Acquire: %v", err)
				return
			}
			mu.Lock()
			inflight++
			if inflight > maxInflight {
				maxInflight = inflight
			}
			mu.Unlock()
			time.Sleep(time.Millisecond)
			mu.Lock()
			inflight--
			mu.Unlock()
			l.Release()
		}()
	}
	close(start)
	wg.Wait()
	if maxInflight > 2 {
		t.Fatalf("max inflight = %d, want <= 2", maxInflight)
	}
}

func TestAdaptiveSlowdownAndGradualRecovery(t *testing.T) {
	a := NewAdaptiveController(AdaptiveConfig{NormalConcurrency: 8, RecoveryInterval: time.Millisecond})
	a.Observe(429, 10*time.Millisecond, nil)
	s := a.Snapshot(0, 8)
	if s.Concurrency >= s.NormalConcurrency || !s.RetryAfterRespected || !s.RateNeverZero {
		t.Fatalf("slowdown snapshot = %+v", s)
	}
	time.Sleep(12 * time.Millisecond)
	a.Observe(200, 0, nil)
	recovered := a.Snapshot(0, 8)
	if recovered.Concurrency <= s.Concurrency || recovered.Concurrency > recovered.NormalConcurrency || !recovered.RecoveryIsGradual {
		t.Fatalf("recovery snapshot = before %+v after %+v", s, recovered)
	}
}
