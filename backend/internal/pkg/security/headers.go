package security

import (
	"net"
	"net/http"
	"net/url"
	"strconv"
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
		allowLocalProfileLoopback := localProfileAllowsLoopback(cfg.AppEnv)
		if originAllowed(origin, allowed) || refererAllowed(referer, allowed) ||
			(allowLocalProfileLoopback && (loopbackOriginAllowed(origin, false) || loopbackOriginAllowed(referer, true))) {
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
	origin, ok := parseHTTPOrigin(raw, false)
	if !ok {
		return false
	}
	return isAllowedOrigin(origin, allowed)
}

func refererAllowed(raw string, allowed []string) bool {
	origin, ok := parseHTTPOrigin(raw, true)
	if !ok {
		return false
	}
	return isAllowedOrigin(origin, allowed)
}

func localProfileAllowsLoopback(env string) bool {
	switch config.NormalizeEnv(env) {
	case config.EnvDevelopment, config.EnvDemo, config.EnvPerformance, config.EnvTest:
		return true
	default:
		return false
	}
}

// loopbackOriginAllowed keeps local secure-session development usable without
// weakening the explicit public-origin requirement in staging or production.
// Parsing the complete origin first avoids prefix-based localhost spoofing.
func loopbackOriginAllowed(raw string, allowPath bool) bool {
	origin, ok := parseHTTPOrigin(raw, allowPath)
	if !ok {
		return false
	}
	if origin.host == "localhost" {
		return true
	}
	ip := net.ParseIP(origin.host)
	return ip != nil && ip.IsLoopback()
}

type httpOrigin struct {
	scheme string
	host   string
	port   int
}

func isAllowedOrigin(origin httpOrigin, allowed []string) bool {
	for _, a := range allowed {
		candidate, ok := parseHTTPOrigin(a, true)
		if !ok {
			continue
		}
		if origin == candidate {
			return true
		}
	}
	return false
}

// parseHTTPOrigin canonicalizes an HTTP(S) URL to its origin. Origin headers
// must contain an origin only; Referer and configured public URLs may include a
// path, which is intentionally discarded before comparison.
func parseHTTPOrigin(raw string, allowPath bool) (httpOrigin, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return httpOrigin{}, false
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil || u == nil || u.User != nil || u.Host == "" || u.Opaque != "" {
		return httpOrigin{}, false
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return httpOrigin{}, false
	}
	if !allowPath && (u.Path != "" || u.RawQuery != "" || u.Fragment != "") {
		return httpOrigin{}, false
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return httpOrigin{}, false
	}
	port := 80
	if scheme == "https" {
		port = 443
	}
	if rawPort := u.Port(); rawPort != "" {
		parsedPort, err := strconv.Atoi(rawPort)
		if err != nil || parsedPort < 1 || parsedPort > 65535 {
			return httpOrigin{}, false
		}
		port = parsedPort
	}
	return httpOrigin{scheme: scheme, host: host, port: port}, true
}
