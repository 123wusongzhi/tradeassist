package douyinruntime

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Register mounts Douyin runtime control routes.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	requireGlobalAdmin := func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "douyin runtime routes unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequireGlobalAdmin(c, h.Svc.DB) {
			c.Abort()
			return
		}
	}
	write := func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "douyin runtime routes unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermConfigManage) {
			c.Abort()
			return
		}
	}
	g.GET("/platform/douyin/runtime-status", requireGlobalAdmin, h.Get)
	g.POST("/platform/douyin/runtime-status/pause", requireGlobalAdmin, write, h.Pause)
	g.POST("/platform/douyin/runtime-status/resume", requireGlobalAdmin, write, h.Resume)
	g.POST("/platform/douyin/runtime-status/emergency-disable", requireGlobalAdmin, write, h.EmergencyDisable)
	g.GET("/platform/douyin/health", requireGlobalAdmin, h.GetHealth)
	g.GET("/platform/douyin/metrics-summary", requireGlobalAdmin, h.GetMetricsSummary)
	g.GET("/platform/douyin/release-gate", requireGlobalAdmin, h.GetReleaseGate)
	g.POST("/platform/douyin/run-health-check", requireGlobalAdmin, write, h.RunHealthCheck)
}
