package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/logging"
	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
	"github.com/trademind-ai/trademind/backend/internal/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ObservabilityHTTP records HTTP metrics and tracing spans.
func ObservabilityHTTP(obs *observability.Observability) gin.HandlerFunc {
	return func(c *gin.Context) {
		if obs == nil || obs.Catalog == nil {
			c.Next()
			return
		}
		start := time.Now()
		route := routeTemplate(c)
		if obs.Catalog.HTTPRequestsInFlight != nil {
			obs.Catalog.HTTPRequestsInFlight.Inc()
			defer obs.Catalog.HTTPRequestsInFlight.Dec()
		}
		var span trace.Span
		ctx := c.Request.Context()
		if obs.Tracer != nil {
			tr := obs.Tracer.Tracer()
			ctx, span = tr.Start(ctx, "http.server",
				trace.WithAttributes(
					attribute.String("http.request.method", c.Request.Method),
					attribute.String("http.route", route),
				),
			)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
		status := c.Writer.Status()
		result := "success"
		if status >= 500 {
			result = "failure"
		} else if status >= 400 {
			result = "client_error"
		}
		obs.Catalog.ObserveHTTP(c.Request.Method, route, status, result, time.Since(start))
		if span != nil {
			span.SetAttributes(attribute.Int("http.response.status_code", status))
			if status >= 500 {
				span.SetStatus(1, "server_error")
			}
			span.End()
		}
	}
}

func routeTemplate(c *gin.Context) string {
	if c == nil {
		return "unknown"
	}
	if p := c.FullPath(); p != "" {
		return p
	}
	path := c.Request.URL.Path
	if path == "" {
		return "unknown"
	}
	if c.Writer.Status() == http.StatusNotFound || !c.IsAborted() && c.Writer.Status() == 0 {
		// normalize 404 paths
		if strings.HasPrefix(path, "/api/") {
			return "/api/*"
		}
	}
	return path
}

// ContextCorrelation injects request_id into request context for structured logging.
func ContextCorrelation() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid, _ := c.Get(TraceIDContextKey)
		id, _ := rid.(string)
		ctx := logging.WithRequestID(c.Request.Context(), id)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// MetricsGuard protects /internal/metrics from public access.
func MetricsGuard(internalOnly bool, allowlist []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !internalOnly {
			c.Next()
			return
		}
		ip := strings.TrimSpace(c.ClientIP())
		allowed := ip == "127.0.0.1" || ip == "::1" || strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "192.168.")
		for _, a := range allowlist {
			if strings.TrimSpace(a) == ip {
				allowed = true
				break
			}
		}
		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "message": "metrics endpoint is internal only"})
			return
		}
		c.Next()
	}
}

// RecordHTTPPanic increments panic metric when recovery handles panic.
func RecordHTTPPanic(obs *observability.Observability) {
	if obs != nil && obs.Catalog != nil && obs.Catalog.HTTPPanicsTotal != nil {
		obs.Catalog.HTTPPanicsTotal.Inc()
	}
}

// SafeRouteLabel returns low-cardinality route label for metrics.
func SafeRouteLabel(method, path string) string {
	_ = method
	p := strings.TrimSpace(path)
	if p == "" {
		return "unknown"
	}
	return p
}

// StatusClass re-exports metrics status class helper.
func StatusClass(code int) string {
	return metrics.StatusClass(code)
}
