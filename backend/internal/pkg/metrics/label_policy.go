package metrics

import (
	"fmt"
	"strings"
	"sync"
)

// Forbidden high-cardinality label keys.
var ForbiddenLabelKeys = []string{
	"request_id", "trace_id", "user_id", "tenant_id", "shop_id",
	"task_id", "order_id", "product_id", "sku_id", "event_id",
	"object_key", "raw_url", "error_message", "execution_id",
}

const defaultMaxLabelValues = 256

// LabelValidator enforces low-cardinality label policy.
type LabelValidator struct {
	MaxValuesPerLabel int
	seen              map[string]map[string]struct{}
	mu                sync.RWMutex
	warnings          int
}

// NewLabelValidator creates a validator.
func NewLabelValidator(max int) *LabelValidator {
	if max <= 0 {
		max = defaultMaxLabelValues
	}
	return &LabelValidator{
		MaxValuesPerLabel: max,
		seen:              make(map[string]map[string]struct{}),
	}
}

// ValidateKey checks only the label key (not value cardinality).
func (v *LabelValidator) ValidateKey(key string) error {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" {
		return fmt.Errorf("metrics: empty label key")
	}
	for _, forbidden := range ForbiddenLabelKeys {
		if k == forbidden {
			return fmt.Errorf("metrics: forbidden high-cardinality label %q", k)
		}
	}
	return nil
}

// Validate checks label key and value against policy.
func (v *LabelValidator) Validate(key, value string) error {
	if v == nil {
		return nil
	}
	if err := v.ValidateKey(key); err != nil {
		return err
	}
	k := strings.ToLower(strings.TrimSpace(key))
	val := strings.TrimSpace(value)
	if val == "" {
		val = "unknown"
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if _, ok := v.seen[k]; !ok {
		v.seen[k] = make(map[string]struct{})
	}
	if _, exists := v.seen[k][val]; !exists {
		if len(v.seen[k]) >= v.MaxValuesPerLabel {
			v.warnings++
			return fmt.Errorf("metrics: label %q exceeded cardinality limit %d", k, v.MaxValuesPerLabel)
		}
		v.seen[k][val] = struct{}{}
	}
	return nil
}

// Warnings returns cardinality warnings count.
func (v *LabelValidator) Warnings() int {
	if v == nil {
		return 0
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.warnings
}

// SeriesCount returns approximate unique series tracked.
func (v *LabelValidator) SeriesCount() int {
	if v == nil {
		return 0
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	n := 0
	for _, vals := range v.seen {
		n += len(vals)
	}
	return n
}

// NormalizeResult maps dynamic results to controlled enums.
func NormalizeResult(raw string) string {
	r := strings.ToLower(strings.TrimSpace(raw))
	switch r {
	case "success", "ok", "passed", "completed":
		return "success"
	case "failure", "failed", "error":
		return "failure"
	case "timeout", "provider_timeout":
		return "timeout"
	case "rate_limited", "rate_limit":
		return "rate_limited"
	case "environment_blocked", "deferred", "blocked":
		return "environment_blocked"
	case "unknown", "unknown_result":
		return "unknown"
	case "retryable":
		return "retryable"
	case "permanent":
		return "permanent"
	default:
		if r == "" {
			return "unknown"
		}
		return "other"
	}
}

// StatusClass maps HTTP status to 2xx/4xx/5xx.
func StatusClass(code int) string {
	switch {
	case code >= 500:
		return "5xx"
	case code >= 400:
		return "4xx"
	case code >= 200 && code < 300:
		return "2xx"
	default:
		return "other"
	}
}
