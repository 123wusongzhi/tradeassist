package taskretry

import (
	"math"
	"math/rand"
	"time"
)

// Default schedule: attempt 1 immediate, 2=30s, 3=2m, 4=10m, 5=30m.
var DefaultDelays = []time.Duration{
	0,
	30 * time.Second,
	2 * time.Minute,
	10 * time.Minute,
	30 * time.Minute,
}

// Policy controls retry backoff behavior.
type Policy struct {
	MaxAttempts int
	Delays      []time.Duration
	MaxDelay    time.Duration
	JitterRatio float64
}

// DefaultPolicy returns the standard retry policy.
func DefaultPolicy() Policy {
	return Policy{
		MaxAttempts: 5,
		Delays:      DefaultDelays,
		MaxDelay:    30 * time.Minute,
		JitterRatio: 0.15,
	}
}

// NextRunAt computes the next scheduled run time for the given attempt (1-based).
func (p Policy) NextRunAt(attempt int, retryAfter time.Duration) time.Time {
	now := time.Now().UTC()
	if retryAfter > 0 {
		return now.Add(capDelay(retryAfter, p.MaxDelay))
	}
	delay := p.delayForAttempt(attempt)
	return now.Add(delay)
}

func (p Policy) delayForAttempt(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	var base time.Duration
	if len(p.Delays) > 0 {
		idx := attempt - 1
		if idx >= len(p.Delays) {
			idx = len(p.Delays) - 1
		}
		base = p.Delays[idx]
	}
	if base <= 0 && attempt > 1 {
		base = 30 * time.Second
	}
	base = capDelay(base, p.MaxDelay)
	if p.JitterRatio > 0 && base > 0 {
		j := base.Seconds() * p.JitterRatio
		base = base + time.Duration((rand.Float64()*2-1)*j)*time.Second
		if base < 0 {
			base = 0
		}
	}
	return capDelay(base, p.MaxDelay)
}

// ShouldRetry reports whether another attempt is allowed.
func (p Policy) ShouldRetry(attempt int, retryable bool) bool {
	if !retryable {
		return false
	}
	max := p.MaxAttempts
	if max <= 0 {
		max = 5
	}
	return attempt < max
}

// IsDeadLetter reports whether max attempts exhausted.
func (p Policy) IsDeadLetter(attempt int) bool {
	max := p.MaxAttempts
	if max <= 0 {
		max = 5
	}
	return attempt >= max
}

func capDelay(d, max time.Duration) time.Duration {
	if max > 0 && d > max {
		return max
	}
	return d
}

// ExponentialBackoff returns delay = base * 2^(attempt-1) capped at max.
func ExponentialBackoff(attempt int, base, max time.Duration) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	sec := base.Seconds() * math.Pow(2, float64(attempt-1))
	d := time.Duration(sec * float64(time.Second))
	return capDelay(d, max)
}
