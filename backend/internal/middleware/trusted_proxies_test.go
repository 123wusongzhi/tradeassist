package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func clientIPRouter(t *testing.T, proxies []string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := r.SetTrustedProxies(proxies); err != nil {
		t.Fatal(err)
	}
	r.GET("/ip", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })
	return r
}

func requestClientIP(t *testing.T, r http.Handler, remoteAddr, xff string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec.Body.String()
}

func TestClientIPIgnoresForwardedHeadersWithoutTrustedProxy(t *testing.T) {
	got := requestClientIP(t, clientIPRouter(t, []string{}), "203.0.113.10:1234", "198.51.100.50")
	if got != "203.0.113.10" {
		t.Fatalf("spoofed XFF changed client IP to %q", got)
	}
}

func TestClientIPUsesTrustedProxyChainAndStopsAtUntrustedHop(t *testing.T) {
	r := clientIPRouter(t, []string{"10.0.0.0/8"})
	if got := requestClientIP(t, r, "10.0.0.8:443", "198.51.100.50, 10.0.0.7"); got != "198.51.100.50" {
		t.Fatalf("trusted chain client IP = %q", got)
	}
	if got := requestClientIP(t, r, "10.0.0.8:443", "198.51.100.50, 172.16.0.9"); got != "172.16.0.9" {
		t.Fatalf("untrusted intermediary was skipped: %q", got)
	}
}

func TestRateLimitAndMetricsCannotBeBypassedWithForwardedHeader(t *testing.T) {
	r := gin.New()
	if err := r.SetTrustedProxies([]string{}); err != nil {
		t.Fatal(err)
	}
	r.GET("/key", func(c *gin.Context) { c.String(http.StatusOK, rateLimitKey(c)) })
	r.GET("/internal/metrics", MetricsGuard(true, nil), func(c *gin.Context) { c.Status(http.StatusOK) })

	keyReq := httptest.NewRequest(http.MethodGet, "/key", nil)
	keyReq.RemoteAddr = "203.0.113.10:1234"
	keyReq.Header.Set("X-Forwarded-For", "10.1.2.3")
	keyRec := httptest.NewRecorder()
	r.ServeHTTP(keyRec, keyReq)
	if got := keyRec.Body.String(); got[:15] != "ip:203.0.113.10" {
		t.Fatalf("rate-limit key accepted spoofed XFF: %q", got)
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/internal/metrics", nil)
	metricsReq.RemoteAddr = "203.0.113.10:1234"
	metricsReq.Header.Set("X-Forwarded-For", "10.1.2.3")
	metricsRec := httptest.NewRecorder()
	r.ServeHTTP(metricsRec, metricsReq)
	if metricsRec.Code != http.StatusForbidden {
		t.Fatalf("metrics bypassed with spoofed XFF: %d", metricsRec.Code)
	}
}
