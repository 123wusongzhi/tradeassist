package httpclient

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/taskretry"
)

// Config holds safe defaults for outbound HTTP.
type Config struct {
	ConnectTimeout        time.Duration
	RequestTimeout        time.Duration
	ResponseHeaderTimeout time.Duration
	MaxResponseBytes      int64
	MaxRedirects          int
	RetryPolicy           taskretry.Policy
	UserAgent             string
}

// DefaultConfig returns production-safe HTTP client defaults.
func DefaultConfig() Config {
	return Config{
		ConnectTimeout:        10 * time.Second,
		RequestTimeout:        60 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		MaxResponseBytes:      32 << 20,
		MaxRedirects:          3,
		RetryPolicy:           taskretry.DefaultPolicy(),
		UserAgent:             "TradeMind/1.0",
	}
}

// Client wraps http.Client with timeout, size limits, and structured errors.
type Client struct {
	cfg     Config
	http    *http.Client
	log     *slog.Logger
	breaker *CircuitBreaker
	sem     chan struct{}
}

// New builds a Client from config. maxConcurrent 0 = unlimited.
func New(cfg Config, log *slog.Logger, maxConcurrent int) *Client {
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 10 * time.Second
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 60 * time.Second
	}
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: cfg.ConnectTimeout,
		}).DialContext,
		ResponseHeaderTimeout: cfg.ResponseHeaderTimeout,
		MaxIdleConns:          64,
		IdleConnTimeout:       90 * time.Second,
	}
	hc := &http.Client{
		Timeout:   cfg.RequestTimeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			max := cfg.MaxRedirects
			if max <= 0 {
				max = 3
			}
			if len(via) >= max {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	c := &Client{cfg: cfg, http: hc, log: log}
	if maxConcurrent > 0 {
		c.sem = make(chan struct{}, maxConcurrent)
	}
	return c
}

// SetCircuitBreaker attaches a per-provider circuit breaker.
func (c *Client) SetCircuitBreaker(b *CircuitBreaker) {
	if c != nil {
		c.breaker = b
	}
}

// Do executes an HTTP request with optional concurrency gate.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if c == nil || c.http == nil {
		return nil, fmt.Errorf("httpclient: unavailable")
	}
	if c.breaker != nil && !c.breaker.Allow() {
		return nil, fmt.Errorf("circuit_open: provider temporarily unavailable")
	}
	if c.sem != nil {
		select {
		case c.sem <- struct{}{}:
			defer func() { <-c.sem }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if req.Header.Get("User-Agent") == "" && c.cfg.UserAgent != "" {
		req.Header.Set("User-Agent", c.cfg.UserAgent)
	}
	resp, err := c.http.Do(req.WithContext(ctx))
	if err != nil {
		if c.breaker != nil {
			c.breaker.RecordFailure()
		}
		return nil, err
	}
	if c.breaker != nil {
		if resp.StatusCode >= 500 {
			c.breaker.RecordFailure()
		} else {
			c.breaker.RecordSuccess()
		}
	}
	return resp, nil
}

// DoWithRetry executes with automatic retry for retryable failures.
func (c *Client) DoWithRetry(ctx context.Context, build func(attempt int) (*http.Request, error)) (*http.Response, error) {
	policy := c.cfg.RetryPolicy
	if policy.MaxAttempts <= 0 {
		policy = taskretry.DefaultPolicy()
	}
	var lastErr error
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		req, err := build(attempt)
		if err != nil {
			return nil, err
		}
		resp, err := c.Do(ctx, req)
		if err == nil && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}
		if err != nil {
			lastErr = err
			cls := taskretry.Classify(err, 0)
			if !policy.ShouldRetry(attempt, cls.Retryable) {
				return nil, err
			}
			wait := policy.NextRunAt(attempt, 0).Sub(time.Now().UTC())
			if wait > 0 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(wait):
				}
			}
			continue
		}
		retryAfter := time.Duration(0)
		if resp.StatusCode == http.StatusTooManyRequests {
			if sec, ok := taskretry.ParseRetryAfter(resp.Header.Get("Retry-After")); ok {
				retryAfter = time.Duration(sec) * time.Second
			}
		}
		_ = resp.Body.Close()
		lastErr = fmt.Errorf("http %d", resp.StatusCode)
		cls := taskretry.Classify(lastErr, resp.StatusCode)
		if !policy.ShouldRetry(attempt, cls.Retryable) {
			return resp, lastErr
		}
		wait := policy.NextRunAt(attempt, retryAfter).Sub(time.Now().UTC())
		if wait > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("httpclient: max attempts reached")
}

// ReadLimitedBody reads response body up to MaxResponseBytes.
func (c *Client) ReadLimitedBody(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, fmt.Errorf("no body")
	}
	defer resp.Body.Close()
	limit := c.cfg.MaxResponseBytes
	if limit <= 0 {
		limit = 32 << 20
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

// RedactURL removes query credentials from URLs for logging.
func RedactURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "@") {
		return "[redacted-url]"
	}
	if idx := strings.Index(raw, "?"); idx >= 0 {
		return raw[:idx] + "?[redacted]"
	}
	return raw
}

// CircuitBreaker implements closed/open/half-open states per provider.
type CircuitBreaker struct {
	mu              sync.Mutex
	state           string
	failures        int
	threshold       int
	openUntil       time.Time
	halfOpenAllowed int
	halfOpenCount   int
}

const (
	StateClosed   = "closed"
	StateOpen     = "open"
	StateHalfOpen = "half_open"
)

// NewCircuitBreaker creates a breaker with failure threshold and 30s open window.
func NewCircuitBreaker(threshold int, openDuration time.Duration) *CircuitBreaker {
	if threshold <= 0 {
		threshold = 5
	}
	if openDuration <= 0 {
		openDuration = 30 * time.Second
	}
	return &CircuitBreaker{
		state:           StateClosed,
		threshold:       threshold,
		halfOpenAllowed: 2,
		openUntil:       time.Time{},
	}
}

// Allow reports whether a request may proceed.
func (b *CircuitBreaker) Allow() bool {
	if b == nil {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now().UTC()
	switch b.state {
	case StateOpen:
		if now.After(b.openUntil) {
			b.state = StateHalfOpen
			b.halfOpenCount = 0
			return true
		}
		return false
	case StateHalfOpen:
		if b.halfOpenCount >= b.halfOpenAllowed {
			return false
		}
		b.halfOpenCount++
		return true
	default:
		return true
	}
}

// RecordFailure increments failures and may open the circuit.
func (b *CircuitBreaker) RecordFailure() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	if b.state == StateHalfOpen {
		b.state = StateOpen
		b.openUntil = time.Now().UTC().Add(30 * time.Second)
		b.failures = 0
		return
	}
	if b.failures >= b.threshold {
		b.state = StateOpen
		b.openUntil = time.Now().UTC().Add(30 * time.Second)
		b.failures = 0
	}
}

// RecordSuccess resets the breaker on success.
func (b *CircuitBreaker) RecordSuccess() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.state = StateClosed
	b.halfOpenCount = 0
}

// State returns current breaker state.
func (b *CircuitBreaker) State() string {
	if b == nil {
		return StateClosed
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}
