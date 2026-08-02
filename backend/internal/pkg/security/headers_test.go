package security

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/config"
)

func TestOriginAllowedRequiresExactOrigin(t *testing.T) {
	allowed := []string{"https://admin.example.com"}
	if originAllowed("https://admin.example.com.attacker.tld", allowed) {
		t.Fatal("suffix spoofed origin must be rejected")
	}
	if !originAllowed("https://admin.example.com", allowed) {
		t.Fatal("configured origin must be allowed")
	}
}

func TestOriginAllowedNormalizesDefaultPort(t *testing.T) {
	allowed := []string{"https://admin.example.com:443"}
	if !originAllowed("https://admin.example.com", allowed) {
		t.Fatal("default HTTPS port must compare equal to explicit :443")
	}
	if !refererAllowed("https://admin.example.com/dashboard?tab=home", allowed) {
		t.Fatal("referer must compare by origin only")
	}
}

func TestOriginAllowedRejectsUserInfoAndPaths(t *testing.T) {
	allowed := []string{"https://admin.example.com"}
	if originAllowed("https://admin.example.com@attacker.tld", allowed) {
		t.Fatal("userinfo origin must be rejected")
	}
	if originAllowed("https://admin.example.com/path", allowed) {
		t.Fatal("origin header containing a path must be rejected")
	}
}

func TestCSRFProtectionAllowsDevelopmentLoopbackOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		AppEnv: config.EnvDevelopment,
		Auth:   config.AuthConfig{SessionMode: config.AuthSessionModeSecure},
	}

	for _, origin := range []string{
		"http://localhost:8000",
		"http://127.0.0.1:8000",
		"http://[::1]:8000",
	} {
		t.Run(origin, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
			request.Header.Set("Origin", origin)
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Request = request

			CSRFProtection(cfg)(context)

			if response.Code != http.StatusOK {
				t.Fatalf("development loopback origin rejected with status %d", response.Code)
			}
			if context.IsAborted() {
				t.Fatal("development loopback origin must not abort the request")
			}
		})
	}
}

func TestCSRFProtectionKeepsProductionLoopbackBlocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		AppEnv:         config.EnvProduction,
		AdminPublicURL: "https://admin.example.com",
		APIPublicURL:   "https://api.example.com",
		Auth:           config.AuthConfig{SessionMode: config.AuthSessionModeSecure},
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	request.Header.Set("Origin", "http://127.0.0.1:8000")
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = request

	CSRFProtection(cfg)(context)

	if response.Code != http.StatusForbidden {
		t.Fatalf("production loopback origin returned status %d, want %d", response.Code, http.StatusForbidden)
	}
	if !context.IsAborted() {
		t.Fatal("production loopback origin must abort the request")
	}
}

func TestLoopbackFallbackRejectsSpoofedHostsAndUnknownProfiles(t *testing.T) {
	for _, origin := range []string{
		"http://localhost.attacker.example:8000",
		"http://127.0.0.1.attacker.example:8000",
	} {
		if loopbackOriginAllowed(origin, false) {
			t.Fatalf("spoofed loopback origin %q must be rejected", origin)
		}
	}
	if localProfileAllowsLoopback("custom") {
		t.Fatal("unknown environment profiles must not enable the loopback fallback")
	}
}
