package ratelimit

import (
	"context"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestLocalLimiterRejectsAfterBurst(t *testing.T) {
	l := NewLocalLimiter(Policy{ID: "test", Rate: rate.Limit(1), Burst: 1, RetryHint: time.Second})
	if d := l.Allow(context.Background(), "user@example.com"); !d.Allowed {
		t.Fatalf("first request should pass: %+v", d)
	}
	if d := l.Allow(context.Background(), "user@example.com"); d.Allowed || d.RetryAfter <= 0 {
		t.Fatalf("second request should be limited: %+v", d)
	}
}

func TestSafeKeyHashesPII(t *testing.T) {
	a := safeKey("user@example.com")
	b := safeKey("USER@example.com")
	if a != b {
		t.Fatal("safe keys should normalize case")
	}
	if a == "user@example.com" || len(a) == 0 {
		t.Fatalf("unsafe key %q", a)
	}
}
