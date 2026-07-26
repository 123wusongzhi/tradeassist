package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/config"
)

// CORSConfig holds production CORS settings.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// CORSFromConfig builds CORS config from app config.
func CORSFromConfig(cfg *config.Config) CORSConfig {
	if cfg == nil {
		return CORSConfig{}
	}
	return CORSConfig{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   cfg.CORSAllowedMethods,
		AllowedHeaders:   cfg.CORSAllowedHeaders,
		ExposedHeaders:   cfg.CORSExposedHeaders,
		AllowCredentials: cfg.CORSAllowCredentials,
		MaxAge:           cfg.CORSMaxAge,
	}
}

// CORS returns middleware enforcing origin whitelist.
func CORS(cfg *config.Config) gin.HandlerFunc {
	cc := CORSFromConfig(cfg)
	env := config.EnvDevelopment
	if cfg != nil {
		env = config.NormalizeEnv(cfg.AppEnv)
	}
	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin == "" {
			c.Next()
			return
		}
		if !originAllowed(origin, cc.AllowedOrigins, env) {
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Next()
			return
		}
		c.Header("Access-Control-Allow-Origin", origin)
		if cc.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		methods := joinOrDefault(cc.AllowedMethods, "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Methods", methods)
		headers := joinOrDefault(cc.AllowedHeaders, "Authorization, Content-Type, X-Request-Id")
		c.Header("Access-Control-Allow-Headers", headers)
		if len(cc.ExposedHeaders) > 0 {
			c.Header("Access-Control-Expose-Headers", strings.Join(cc.ExposedHeaders, ", "))
		} else {
			c.Header("Access-Control-Expose-Headers", "X-Request-Id")
		}
		if cc.MaxAge > 0 {
			c.Header("Access-Control-Max-Age", strconv.Itoa(cc.MaxAge))
		} else {
			c.Header("Access-Control-Max-Age", strconv.Itoa(int((12 * time.Hour).Seconds())))
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func originAllowed(origin string, allowed []string, env string) bool {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" {
		return false
	}
	for _, a := range allowed {
		a = strings.TrimRight(strings.TrimSpace(a), "/")
		if a == "" {
			continue
		}
		if a == "*" {
			if config.IsStagingOrProduction(env) {
				return false
			}
			if config.NormalizeEnv(env) == config.EnvProduction {
				return false
			}
			return true
		}
		if strings.EqualFold(origin, a) {
			return true
		}
	}
	// development/demo: allow localhost origins when not explicitly listed
	if !config.IsStagingOrProduction(env) {
		if isLocalhostOrigin(origin) {
			return true
		}
	}
	return false
}

func isLocalhostOrigin(origin string) bool {
	lower := strings.ToLower(origin)
	return strings.HasPrefix(lower, "http://localhost:") ||
		strings.HasPrefix(lower, "https://localhost:") ||
		strings.HasPrefix(lower, "http://127.0.0.1:") ||
		strings.HasPrefix(lower, "https://127.0.0.1:")
}

func joinOrDefault(items []string, def string) string {
	if len(items) == 0 {
		return def
	}
	return strings.Join(items, ", ")
}

// ValidateCORSConfig returns error for invalid production CORS settings.
func ValidateCORSConfig(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	if !config.IsStagingOrProduction(cfg.AppEnv) {
		return nil
	}
	if len(cfg.CORSAllowedOrigins) == 0 {
		return config.CORSError("CORS_ALLOWED_ORIGINS is required in staging/production")
	}
	for _, o := range cfg.CORSAllowedOrigins {
		if strings.TrimSpace(o) == "*" && cfg.CORSAllowCredentials {
			return config.CORSError("wildcard origin not allowed with credentials")
		}
		if strings.TrimSpace(o) == "*" && config.IsStagingOrProduction(cfg.AppEnv) {
			return config.CORSError("wildcard origin not allowed in staging/production")
		}
	}
	return nil
}
