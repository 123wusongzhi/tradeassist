package providerhealth

import (
	"context"
	"sync"
	"time"
)

// Status values for provider health.
const (
	StatusAvailable            = "available"
	StatusDegraded             = "degraded"
	StatusRateLimited          = "rate_limited"
	StatusCircuitOpen          = "circuit_open"
	StatusUnauthorized         = "unauthorized"
	StatusNotConfigured        = "not_configured"
	StatusTemporaryUnavailable = "temporary_unavailable"
	StatusManualRequired       = "manual_required"
)

// Result is one health check outcome.
type Result struct {
	Provider    string    `json:"provider"`
	Capability  string    `json:"capability"`
	Status      string    `json:"status"`
	LatencyMs   int64     `json:"latencyMs"`
	ErrorCode   string    `json:"errorCode,omitempty"`
	CheckedAt   time.Time `json:"checkedAt"`
	NextCheckAt time.Time `json:"nextCheckAt"`
}

// Checker probes one provider capability.
type Checker interface {
	HealthCheck(ctx context.Context) Result
}

// Registry caches health results with TTL.
type Registry struct {
	mu       sync.RWMutex
	cache    map[string]Result
	ttl      time.Duration
	checkers map[string]Checker
}

// NewRegistry creates a health registry with default 5m cache TTL.
func NewRegistry(ttl time.Duration) *Registry {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Registry{
		cache:    make(map[string]Result),
		ttl:      ttl,
		checkers: make(map[string]Checker),
	}
}

// Register adds a checker under provider:capability key.
func (r *Registry) Register(provider, capability string, c Checker) {
	if r == nil || c == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checkers[key(provider, capability)] = c
}

// Get returns cached result or runs check if stale.
func (r *Registry) Get(ctx context.Context, provider, capability string, force bool) Result {
	if r == nil {
		return Result{Provider: provider, Capability: capability, Status: StatusNotConfigured}
	}
	k := key(provider, capability)
	r.mu.RLock()
	cached, ok := r.cache[k]
	r.mu.RUnlock()
	if ok && !force && time.Now().UTC().Before(cached.NextCheckAt) {
		return cached
	}
	r.mu.RLock()
	ch := r.checkers[k]
	r.mu.RUnlock()
	if ch == nil {
		return Result{
			Provider:    provider,
			Capability:  capability,
			Status:      StatusNotConfigured,
			CheckedAt:   time.Now().UTC(),
			NextCheckAt: time.Now().UTC().Add(r.ttl),
		}
	}
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	res := ch.HealthCheck(checkCtx)
	if res.NextCheckAt.IsZero() {
		res.NextCheckAt = time.Now().UTC().Add(r.ttl)
	}
	r.mu.Lock()
	r.cache[k] = res
	r.mu.Unlock()
	return res
}

// All returns all cached or freshly checked results.
func (r *Registry) All(ctx context.Context, force bool) []Result {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	keys := make([]string, 0, len(r.checkers))
	for k := range r.checkers {
		keys = append(keys, k)
	}
	r.mu.RUnlock()
	out := make([]Result, 0, len(keys))
	for _, k := range keys {
		parts := splitKey(k)
		out = append(out, r.Get(ctx, parts[0], parts[1], force))
	}
	return out
}

func key(provider, capability string) string {
	return provider + ":" + capability
}

func splitKey(k string) [2]string {
	for i := 0; i < len(k); i++ {
		if k[i] == ':' {
			return [2]string{k[:i], k[i+1:]}
		}
	}
	return [2]string{k, ""}
}
