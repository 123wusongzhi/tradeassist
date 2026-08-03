package security

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/config"
)

const testBrowserExtensionOrigin = "chrome-extension://abcdefghijklmnopabcdefghijklmnop"

func browserExtensionCSRFTestConfig() *config.Config {
	return &config.Config{
		AppEnv:         config.EnvProduction,
		AdminPublicURL: "https://admin.example.com",
		APIPublicURL:   "https://api.example.com",
		Auth:           config.AuthConfig{SessionMode: config.AuthSessionModeSecure},
	}
}

func TestCSRFProtectionAllowsExactBrowserExtensionNonCookieRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CSRFProtection(browserExtensionCSRFTestConfig()))

	allowed := []struct {
		pattern string
		path    string
	}{
		{
			pattern: "/api/v1/collect/browser-extension/pairings/exchange",
			path:    "/api/v1/collect/browser-extension/pairings/exchange",
		},
		{
			pattern: "/api/v1/collect/browser-extension/tasks",
			path:    "/api/v1/collect/browser-extension/tasks",
		},
		{
			pattern: "/api/v1/collect/browser-extension/tasks/:id/result",
			path:    "/api/v1/collect/browser-extension/tasks/11111111-1111-1111-1111-111111111111/result",
		},
		{
			pattern: "/api/v1/collect/browser-extension/tasks/:id/failure",
			path:    "/api/v1/collect/browser-extension/tasks/11111111-1111-1111-1111-111111111111/failure",
		},
	}
	for _, route := range allowed {
		router.POST(route.pattern, func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		})
	}

	for _, route := range allowed {
		t.Run(route.pattern, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, route.path, nil)
			request.Header.Set("Origin", testBrowserExtensionOrigin)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != http.StatusNoContent {
				t.Fatalf("browser extension route returned %d, want %d", response.Code, http.StatusNoContent)
			}
		})
	}
}

func TestCSRFProtectionKeepsBrowserExtensionBoundaryExact(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CSRFProtection(browserExtensionCSRFTestConfig()))

	router.POST("/api/v1/collect/browser-extension/pairings", csrfBoundaryUnexpectedHandler)
	router.DELETE("/api/v1/collect/browser-extension/devices/:id", csrfBoundaryUnexpectedHandler)
	router.POST("/api/v1/collect/browser-extension/pairings/exchange-near", csrfBoundaryUnexpectedHandler)
	router.POST("/api/v1/collect/browser-extension/tasks/:id/result/extra", csrfBoundaryUnexpectedHandler)
	router.PUT("/api/v1/collect/browser-extension/pairings/exchange", csrfBoundaryUnexpectedHandler)
	router.POST("/api/v1/products", csrfBoundaryUnexpectedHandler)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "admin pairing creation",
			method: http.MethodPost,
			path:   "/api/v1/collect/browser-extension/pairings",
		},
		{
			name:   "admin device revocation",
			method: http.MethodDelete,
			path:   "/api/v1/collect/browser-extension/devices/11111111-1111-1111-1111-111111111111",
		},
		{
			name:   "similar exchange path",
			method: http.MethodPost,
			path:   "/api/v1/collect/browser-extension/pairings/exchange-near",
		},
		{
			name:   "nested result path",
			method: http.MethodPost,
			path:   "/api/v1/collect/browser-extension/tasks/11111111-1111-1111-1111-111111111111/result/extra",
		},
		{
			name:   "wrong method",
			method: http.MethodPut,
			path:   "/api/v1/collect/browser-extension/pairings/exchange",
		},
		{
			name:   "ordinary admin write",
			method: http.MethodPost,
			path:   "/api/v1/products",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.path, nil)
			request.Header.Set("Origin", testBrowserExtensionOrigin)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != http.StatusForbidden {
				t.Fatalf("request returned %d, want %d", response.Code, http.StatusForbidden)
			}
		})
	}
}

func TestCSRFProtectionBrowserExtensionBoundaryRequiresExtensionOriginAndNoCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CSRFProtection(browserExtensionCSRFTestConfig()))
	router.POST("/api/v1/collect/browser-extension/pairings/exchange", csrfBoundaryUnexpectedHandler)

	tests := []struct {
		name      string
		origin    string
		hasCookie bool
	}{
		{
			name:   "ordinary unknown origin",
			origin: "https://unknown.example.com",
		},
		{
			name:   "invalid extension id",
			origin: "chrome-extension://not-a-chromium-extension-id",
		},
		{
			name:   "extension origin with path",
			origin: testBrowserExtensionOrigin + "/sidepanel.html",
		},
		{
			name:      "extension request carrying admin cookie",
			origin:    testBrowserExtensionOrigin,
			hasCookie: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/collect/browser-extension/pairings/exchange", nil)
			request.Header.Set("Origin", tt.origin)
			if tt.hasCookie {
				request.AddCookie(&http.Cookie{Name: "tm_refresh_token", Value: "test"})
			}
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != http.StatusForbidden {
				t.Fatalf("request returned %d, want %d", response.Code, http.StatusForbidden)
			}
		})
	}
}

func csrfBoundaryUnexpectedHandler(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
