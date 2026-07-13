package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Policy struct {
	ID        string
	Rate      rate.Limit
	Burst     int
	TTL       time.Duration
	RetryHint time.Duration
}

type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
	PolicyID   string
	KeyHash    string
}

type LocalLimiter struct {
	mu      sync.Mutex
	policy  Policy
	entries map[string]*entry
	now     func() time.Time
}

type entry struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

func NewLocalLimiter(p Policy) *LocalLimiter {
	if p.Rate <= 0 {
		p.Rate = rate.Limit(5)
	}
	if p.Burst < 1 {
		p.Burst = 10
	}
	if p.TTL <= 0 {
		p.TTL = 10 * time.Minute
	}
	if p.RetryHint <= 0 {
		p.RetryHint = time.Second
	}
	return &LocalLimiter{policy: p, entries: map[string]*entry{}, now: time.Now}
}

func (l *LocalLimiter) Allow(ctx context.Context, rawKey string) Decision {
	if l == nil {
		return Decision{Allowed: true}
	}
	select {
	case <-ctx.Done():
		return Decision{Allowed: false, RetryAfter: l.policy.RetryHint, PolicyID: l.policy.ID}
	default:
	}
	key := safeKey(rawKey)
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.evictLocked(now)
	e := l.entries[key]
	if e == nil {
		e = &entry{lim: rate.NewLimiter(l.policy.Rate, l.policy.Burst)}
		l.entries[key] = e
	}
	e.lastSeen = now
	if e.lim.Allow() {
		return Decision{Allowed: true, PolicyID: l.policy.ID, KeyHash: key}
	}
	return Decision{Allowed: false, RetryAfter: l.policy.RetryHint, PolicyID: l.policy.ID, KeyHash: key}
}

func (l *LocalLimiter) evictLocked(now time.Time) {
	for k, e := range l.entries {
		if now.Sub(e.lastSeen) > l.policy.TTL {
			delete(l.entries, k)
		}
	}
}

func safeKey(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		raw = "anonymous"
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:16])
}
