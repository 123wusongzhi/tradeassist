package providerlimit

import (
	"strings"
	"sync"
	"time"
)

type AdaptiveConfig struct {
	NormalConcurrency int
	MinConcurrency    int
	RecoveryStep      int
	RecoveryInterval  time.Duration
	MaxRetryAfter     time.Duration
}

type Snapshot struct {
	Inflight            int       `json:"inflight"`
	Concurrency         int       `json:"concurrency"`
	NormalConcurrency   int       `json:"normalConcurrency"`
	SlowdownUntil       time.Time `json:"slowdownUntil,omitempty"`
	RetryAfterRespected bool      `json:"retryAfterRespected"`
	RecoveryIsGradual   bool      `json:"recoveryIsGradual"`
	RateNeverZero       bool      `json:"rateNeverZero"`
}

type AdaptiveController struct {
	mu          sync.Mutex
	cfg         AdaptiveConfig
	current     int
	slowUntil   time.Time
	lastRecover time.Time
	retryAfter  bool
}

func NewAdaptiveController(cfg AdaptiveConfig) *AdaptiveController {
	if cfg.NormalConcurrency < 1 {
		cfg.NormalConcurrency = 8
	}
	if cfg.MinConcurrency < 1 {
		cfg.MinConcurrency = 1
	}
	if cfg.RecoveryStep < 1 {
		cfg.RecoveryStep = 1
	}
	if cfg.RecoveryInterval <= 0 {
		cfg.RecoveryInterval = 10 * time.Second
	}
	if cfg.MaxRetryAfter <= 0 {
		cfg.MaxRetryAfter = time.Minute
	}
	return &AdaptiveController{cfg: cfg, current: cfg.NormalConcurrency, lastRecover: time.Now().UTC()}
}

func (a *AdaptiveController) Observe(status int, retryAfter time.Duration, err error) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now().UTC()
	if status == 429 || isTimeout(err) {
		a.current = max(a.cfg.MinConcurrency, a.current/2)
		if retryAfter > a.cfg.MaxRetryAfter {
			retryAfter = a.cfg.MaxRetryAfter
		}
		if retryAfter > 0 {
			a.retryAfter = true
			a.slowUntil = now.Add(retryAfter)
		} else {
			a.slowUntil = now.Add(a.cfg.RecoveryInterval)
		}
		return
	}
	if status >= 200 && status < 400 && now.After(a.slowUntil) && now.Sub(a.lastRecover) >= a.cfg.RecoveryInterval {
		if a.current < a.cfg.NormalConcurrency {
			a.current += a.cfg.RecoveryStep
			if a.current > a.cfg.NormalConcurrency {
				a.current = a.cfg.NormalConcurrency
			}
			a.lastRecover = now
		}
	}
}

func (a *AdaptiveController) Snapshot(inflight int, physicalCap int) Snapshot {
	if a == nil {
		return Snapshot{}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return Snapshot{
		Inflight:            inflight,
		Concurrency:         a.current,
		NormalConcurrency:   a.cfg.NormalConcurrency,
		SlowdownUntil:       a.slowUntil,
		RetryAfterRespected: a.retryAfter,
		RecoveryIsGradual:   a.cfg.RecoveryStep < a.cfg.NormalConcurrency,
		RateNeverZero:       a.current >= 1 && physicalCap >= 1,
	}
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded")
}
