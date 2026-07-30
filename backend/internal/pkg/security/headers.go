package security

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/config"
)

// SecurityHeaders applies baseline security response headers.
func SecurityHeaders(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c == nil {
			return
		}
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("X-Frame-Options", "DENY")
		if cfg != nil && config.IsProduction(cfg.AppEnv) {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		// Compatible CSP — report-only style baseline without breaking admin bundles.
		if h.Get("Content-Security-Policy") == "" {
			h.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		}
		c.Next()
	}
}

// CSRFProtection validates Origin/Referer for cookie-based session write requests.
func CSRFProtection(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c == nil || cfg == nil || !cfg.UsesSecureSession() {
			c.Next()
			return
		}
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		referer := strings.TrimSpace(c.GetHeader("Referer"))
		allowed := []string{strings.TrimSpace(cfg.AdminPublicURL), strings.TrimSpace(cfg.APIPublicURL)}
		if originAllowed(origin, allowed) || originAllowed(referer, allowed) {
			c.Next()
			return
		}
		if origin == "" && referer == "" {
			// Bearer-only clients without cookies.
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"code":    "ORIGIN_NOT_ALLOWED",
			"message": "请求来源未授权",
		})
	}
}

func originAllowed(raw string, allowed []string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	for _, a := range allowed {
		a = strings.TrimRight(strings.TrimSpace(a), "/")
		if a == "" {
			continue
		}
		if strings.HasPrefix(raw, a) {
			return true
		}
	}
	return false
}
