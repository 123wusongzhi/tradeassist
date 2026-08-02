package aiprompt

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Register mounts /api/v1/ai/prompts routes on an authenticated group.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	rg := g.Group("/ai/prompts")
	rg.Use(func(c *gin.Context) {
		if h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "ai prompts unavailable")
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
	rg.PUT("/:id", h.Put)
	rg.DELETE("/:id", h.Delete)
	rg.POST("/:id/enable", h.Enable)
	rg.POST("/:id/disable", h.Disable)
}
