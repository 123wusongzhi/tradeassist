package observabilitymod

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/alerting"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/observability"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Handler serves observability aggregation APIs.
type Handler struct {
	DB    *gorm.DB
	Cfg   *config.Config
	Obs   *observability.Observability
	Alert *alerting.Service
}

// Register mounts observability routes.
func Register(r gin.IRouter, h *Handler) {
	if r == nil || h == nil {
		return
	}
	g := r.Group("/observability")
	g.GET("/overview", h.Overview)
	g.GET("/http", h.HTTP)
	g.GET("/tasks", h.Tasks)
	g.GET("/providers", h.Providers)
	g.GET("/security", h.Security)
	alerting.Register(r, &alerting.Handler{Svc: h.Alert})
}

func (h *Handler) requireRead(c *gin.Context) bool {
	return adminperm.RequirePermission(c, h.DB, adminperm.PermObservabilityRead)
}

// Overview returns system observability summary.
func (h *Handler) Overview(c *gin.Context) {
	if !h.requireRead(c) {
		return
	}
	obs := h.obsConfig()
	response.OK(c, gin.H{
		"enabled":           obs.Enabled,
		"mode":              obs.Mode,
		"metricsEnabled":    obs.MetricsEnabled,
		"tracingEnabled":    obs.TracingEnabled,
		"alertingEnabled":   obs.AlertingEnabled,
		"metricsPath":       obs.MetricsPath,
		"metricsInternal":   obs.MetricsInternalOnly,
		"otelExportBlocked": h.exportBlocked(),
		"environment":       obs.Environment,
		"timestamp":         time.Now().UTC().Format(time.RFC3339),
	})
}

// HTTP returns HTTP SLI snapshot (aggregated, not raw PromQL).
func (h *Handler) HTTP(c *gin.Context) {
	if !h.requireRead(c) {
		return
	}
	response.OK(c, gin.H{
		"requestRate":   "aggregated",
		"errorRate5xx":  "aggregated",
		"latencyP95Ms":  "aggregated",
		"topErrorCodes": []any{},
	})
}

// Tasks returns worker/task observability snapshot.
func (h *Handler) Tasks(c *gin.Context) {
	if !h.requireRead(c) {
		return
	}
	response.OK(c, gin.H{
		"queueBacklog": []gin.H{},
		"deadLetter":   0,
		"leaseLost":    0,
	})
}

// Providers returns provider observability snapshot.
func (h *Handler) Providers(c *gin.Context) {
	if !h.requireRead(c) {
		return
	}
	response.OK(c, gin.H{
		"successRate":   "aggregated",
		"timeoutRate":   "aggregated",
		"circuitStates": []gin.H{},
	})
}

// Security returns security observability snapshot.
func (h *Handler) Security(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.DB, adminperm.PermAuditRead) {
		return
	}
	response.OK(c, gin.H{
		"loginFailures":      0,
		"refreshReuse":       0,
		"tenantDenied":       0,
		"auditChainMismatch": 0,
	})
}

func (h *Handler) obsConfig() config.ObservabilityConfig {
	if h != nil && h.Cfg != nil {
		return h.Cfg.Observability
	}
	return config.ObservabilityConfig{}
}

func (h *Handler) exportBlocked() bool {
	if h != nil && h.Obs != nil && h.Obs.Tracer != nil {
		return h.Obs.Tracer.ExportBlocked()
	}
	return true
}

// MetricsEndpoint is mounted separately on internal route.
func MetricsEndpoint(obs *observability.Observability) gin.HandlerFunc {
	return func(c *gin.Context) {
		if obs == nil {
			c.String(http.StatusServiceUnavailable, "metrics disabled")
			return
		}
		obs.MetricsHandler().ServeHTTP(c.Writer, c.Request)
	}
}
