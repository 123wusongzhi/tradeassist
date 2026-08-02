package collectrule

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Register mounts JWT-protected collect rule routes under /api/v1 (group g).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	// Collect rules are currently instance-wide resources. Until the model is
	// tenant-scoped, expose every read, test, and mutation only to the global
	// settings administrator so a tenant business role cannot poison rules used
	// by other tenants.
	rg := g.Group("/collect/rules")
	rg.Use(func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "collect rules unavailable")
			c.Abort()
			return
		}
		if !adminperm.RequireGlobalAdmin(c, h.Svc.DB) {
			c.Abort()
			return
		}
		if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermSettingsManage) {
			c.Abort()
		}
	})
	rg.GET("", h.List)
	rg.POST("", h.Create)
	rg.GET("/:id", h.Get)
	rg.PUT("/:id", h.Update)
	rg.DELETE("/:id", h.Delete)
	rg.POST("/:id/enable", h.Enable)
	rg.POST("/:id/disable", h.Disable)
	rg.POST("/:id/test", h.Test)
}
