package providerlimit

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type ProviderName string
type ProviderOperation string

const (
	ProviderDouyinShop ProviderName = "douyin_shop"
	ProviderAI         ProviderName = "ai"
	ProviderImage      ProviderName = "image"
	ProviderStorage    ProviderName = "storage"
	ProviderSecurity   ProviderName = "security_scan"

	OperationRequest      ProviderOperation = "request"
	OperationRead         ProviderOperation = "read"
	OperationDraftWrite   ProviderOperation = "draft_write"
	OperationTokenRefresh ProviderOperation = "token_refresh"
	OperationText         ProviderOperation = "text"
	OperationImage        ProviderOperation = "image"
	OperationObject       ProviderOperation = "object"
)

type Lease interface {
	Release()
}

type ProviderConcurrencyLimiter interface {
	Acquire(ctx context.Context, provider ProviderName, operation ProviderOperation) (Lease, error)
}

type Config struct {
	DefaultConcurrency int
	ProviderOverrides  map[ProviderName]int
	OperationOverrides map[ProviderOperation]int
	MaxEntries         int
	EntryTTL           time.Duration
	MaxWait            time.Duration
}

type Registry struct {
	mu      sync.Mutex
	cfg     Config
	entries map[string]*entry
	now     func() time.Time
}

type entry struct {
	sem      chan struct{}
	limit    int
	lastSeen time.Time
	adaptive *AdaptiveController
}

type lease struct {
	once sync.Once
	ch   chan struct{}
}

func (l *lease) Release() {
	if l == nil || l.ch == nil {
		return
	}
	l.once.Do(func() { <-l.ch })
}

func NewRegistry(cfg Config) *Registry {
	if cfg.DefaultConcurrency < 1 {
		cfg.DefaultConcurrency = 8
	}
	if cfg.MaxEntries < 1 {
		cfg.MaxEntries = 128
	}
	if cfg.EntryTTL <= 0 {
		cfg.EntryTTL = 10 * time.Minute
	}
	if cfg.MaxWait <= 0 {
		cfg.MaxWait = 30 * time.Second
	}
	return &Registry{cfg: cfg, entries: map[string]*entry{}, now: time.Now}
}

func (r *Registry) Acquire(ctx context.Context, provider ProviderName, operation ProviderOperation) (Lease, error) {
	if r == nil {
		return &lease{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	e, err := r.getEntry(provider, operation)
	if err != nil {
		return nil, err
	}
	waitCtx := ctx
	cancel := func() {}
	if r.cfg.MaxWait > 0 {
		waitCtx, cancel = context.WithTimeout(ctx, r.cfg.MaxWait)
	}
	defer cancel()
	select {
	case e.sem <- struct{}{}:
		return &lease{ch: e.sem}, nil
	case <-waitCtx.Done():
		return nil, waitCtx.Err()
	}
}

func (r *Registry) Observe(provider ProviderName, operation ProviderOperation, status int, retryAfter time.Duration, err error) {
	if r == nil {
		return
	}
	e, getErr := r.getEntry(provider, operation)
	if getErr != nil {
		return
	}
	e.adaptive.Observe(status, retryAfter, err)
}

func (r *Registry) Snapshot(provider ProviderName, operation ProviderOperation) Snapshot {
	if r == nil {
		return Snapshot{}
	}
	e, err := r.getEntry(provider, operation)
	if err != nil {
		return Snapshot{}
	}
	return e.adaptive.Snapshot(len(e.sem), cap(e.sem))
}

func (r *Registry) getEntry(provider ProviderName, operation ProviderOperation) (*entry, error) {
	p := normalizeProvider(provider)
	op := normalizeOperation(operation)
	k := string(p) + ":" + string(op)
	now := r.now()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evictLocked(now)
	if e := r.entries[k]; e != nil {
		e.lastSeen = now
		return e, nil
	}
	if len(r.entries) >= r.cfg.MaxEntries {
		return nil, fmt.Errorf("providerlimit: max registry entries reached")
	}
	lim := r.limitFor(p, op)
	e := &entry{
		sem:      make(chan struct{}, lim),
		limit:    lim,
		lastSeen: now,
		adaptive: NewAdaptiveController(AdaptiveConfig{NormalConcurrency: lim}),
	}
	r.entries[k] = e
	return e, nil
}

func (r *Registry) limitFor(provider ProviderName, operation ProviderOperation) int {
	if v := r.cfg.OperationOverrides[operation]; v > 0 {
		return v
	}
	if v := r.cfg.ProviderOverrides[provider]; v > 0 {
		return v
	}
	return r.cfg.DefaultConcurrency
}

func (r *Registry) evictLocked(now time.Time) {
	for k, e := range r.entries {
		if len(e.sem) == 0 && now.Sub(e.lastSeen) > r.cfg.EntryTTL {
			delete(r.entries, k)
		}
	}
}

func normalizeProvider(provider ProviderName) ProviderName {
	v := ProviderName(strings.TrimSpace(strings.ToLower(string(provider))))
	if v == "" {
		return "unknown"
	}
	return v
}

func normalizeOperation(operation ProviderOperation) ProviderOperation {
	v := ProviderOperation(strings.TrimSpace(strings.ToLower(string(operation))))
	if v == "" {
		return OperationRequest
	}
	return v
}
