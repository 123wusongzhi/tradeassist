package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ratelimit"
	"golang.org/x/time/rate"
)

// RateLimit applies a conservative local token bucket. P7 keeps Redis-backed
// multi-instance limiting as a separately verified closure item.
func RateLimit(cfg *config.Config) gin.HandlerFunc {
	if cfg == nil || !cfg.P7.RateLimitEnabled {
		return func(c *gin.Context) { c.Next() }
	}
	policy := ratelimit.Policy{
		ID:        strings.TrimSpace(cfg.P7.RateLimitPolicyVersion),
		Rate:      rate.Limit(20),
		Burst:     40,
		TTL:       10 * time.Minute,
		RetryHint: time.Second,
	}
	lim := ratelimit.NewLocalLimiter(policy)
	return func(c *gin.Context) {
		if isRateLimitExemptPath(c.FullPath(), c.Request.URL.Path) {
			c.Next()
			return
		}
		decision := lim.Allow(c.Request.Context(), rateLimitKey(c))
		c.Header("X-RateLimit-Policy", decision.PolicyID)
		if decision.Allowed {
			c.Next()
			return
		}
		retryAfter := int(decision.RetryAfter.Seconds())
		if retryAfter < 1 {
			retryAfter = 1
		}
		c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"code":       "RATE_LIMITED",
			"message":    "请求过于频繁，请稍后再试",
			"retryAfter": retryAfter,
		})
	}
}

func isRateLimitExemptPath(fullPath, path string) bool {
	p := strings.TrimSpace(fullPath)
	if p == "" {
		p = strings.TrimSpace(path)
	}
	return strings.HasPrefix(p, "/health") || strings.HasPrefix(p, "/api/v1/health") || strings.HasPrefix(p, "/internal/metrics")
}

func rateLimitKey(c *gin.Context) string {
	parts := []string{"ip", normalizeIP(c.ClientIP()), "route", c.FullPath()}
	if v, ok := c.Get(ctxkey.AdminID); ok {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			parts = append(parts, "user", s)
		}
	}
	return strings.Join(parts, ":")
}

func normalizeIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "unknown"
	}
	ip := net.ParseIP(raw)
	if ip == nil {
		return "unknown"
	}
	return ip.String()
}
