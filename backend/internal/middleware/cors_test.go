package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/config"
)

func TestCORS_allowedOrigin(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		AppEnv:               config.EnvProduction,
		CORSAllowedOrigins:   []string{"https://admin.example.com"},
		CORSAllowCredentials: true,
	}
	r := gin.New()
	r.Use(CORS(cfg))
	r.GET("/ping", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "https://admin.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Header().Get("Access-Control-Allow-Origin") != "https://admin.example.com" {
		t.Fatalf("expected allowed origin header, got %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_unknownOriginRejected(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		AppEnv:             config.EnvProduction,
		CORSAllowedOrigins: []string{"https://admin.example.com"},
	}
	r := gin.New()
	r.Use(CORS(cfg))
	r.GET("/ping", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("unknown origin should not get ACAO header")
	}
}

func TestCORS_preflight(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		AppEnv:             config.EnvDevelopment,
		CORSAllowedOrigins: []string{"http://localhost:8000"},
	}
	r := gin.New()
	r.Use(CORS(cfg))
	r.GET("/ping", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest(http.MethodOptions, "/ping", nil)
	req.Header.Set("Origin", "http://localhost:8000")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestCORS_localhostDevelopment(t *testing.T) {
	t.Parallel()
	if !originAllowed("http://localhost:8000", nil, config.EnvDevelopment) {
		t.Fatal("localhost should be allowed in development")
	}
}

func TestCORS_wildcardProductionRejected(t *testing.T) {
	t.Parallel()
	if originAllowed("http://localhost:8000", []string{"*"}, config.EnvProduction) {
		t.Fatal("wildcard must not allow in production")
	}
}
