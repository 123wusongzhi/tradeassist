package authcookie

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/config"
)

// RefreshCookieName is the HttpOnly cookie storing refresh tokens.
const RefreshCookieName = "tm_refresh_token"

// ReadRefresh extracts refresh token from cookie.
func ReadRefresh(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	cookie, err := c.Cookie(RefreshCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie)
}

// SetRefresh writes HttpOnly refresh token cookie.
func SetRefresh(c *gin.Context, cfg *config.Config, token string, expires time.Time) {
	if c == nil || cfg == nil {
		return
	}
	maxAge := int(time.Until(expires).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	sameSite := http.SameSiteLaxMode
	switch strings.ToLower(strings.TrimSpace(cfg.Auth.CookieSameSite)) {
	case "strict":
		sameSite = http.SameSiteStrictMode
	case "none":
		sameSite = http.SameSiteNoneMode
	}
	c.SetSameSite(sameSite)
	secure := cfg.Auth.SecureCookie
	c.SetCookie(RefreshCookieName, token, maxAge, "/api/v1/auth", cfg.Auth.CookieDomain, secure, true)
}

// Clear removes refresh token cookie.
func Clear(c *gin.Context, cfg *config.Config) {
	if c == nil || cfg == nil {
		return
	}
	c.SetCookie(RefreshCookieName, "", -1, "/api/v1/auth", cfg.Auth.CookieDomain, cfg.Auth.SecureCookie, true)
}
